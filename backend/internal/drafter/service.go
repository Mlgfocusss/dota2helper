package drafter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"d2helper/internal/matchups"
	"d2helper/internal/meta"
	"d2helper/internal/store"
)

const (
	// StaleTrainingAfter — неоценённые тренировки старше этого срока удаляются джобой
	StaleTrainingAfter = 7 * 24 * time.Hour
	HistoryPageSize    = 20
	LeaderboardSize    = 100
	// BestScoresCount — сколько лучших результатов месяца входит в счёт лидерборда
	BestScoresCount = 5
	boardTTL        = 5 * time.Minute
	payloadTTL      = 10 * time.Minute
)

var (
	ErrNotReady        = errors.New("drafter: data is not ready")
	ErrBadTeam         = errors.New("drafter: bad team")
	ErrTrainingMissing = errors.New("drafter: training not found")
	ErrTrainingClosed  = errors.New("drafter: training is closed")
	ErrBadMonth        = errors.New("drafter: bad month")
	ErrBadCursor       = errors.New("drafter: bad cursor")
)

// TooManyError — слишком много оценённых тренировок за час.
type TooManyError struct{ RetryAfter time.Duration }

func (e *TooManyError) Error() string { return "drafter: too many trainings" }

// Moscow — часовой пояс календарного месяца для топа (без перехода на летнее время).
var Moscow = time.FixedZone("MSK", 3*60*60)

type (
	MatrixSource interface {
		Matrix(ctx context.Context) (*matchups.Matrix, error)
	}
	MetaSource interface {
		Snapshot(ctx context.Context) (*meta.Snapshot, error)
	}
	Store interface {
		CreateTraining(ctx context.Context, userID *int64, enemies []byte) (string, error)
		TrainingByID(ctx context.Context, id string) (store.Training, error)
		SubmitTraining(ctx context.Context, id string, userID int64, team []byte, score float64, breakdown []byte) error
		CountSubmittedSince(ctx context.Context, userID int64, since time.Time) (int, error)
		TrainingHistory(ctx context.Context, userID int64, before *time.Time, limit int) ([]store.Training, error)
		MonthScores(ctx context.Context, from, to time.Time) ([]store.MonthUserScores, error)
		DeleteStaleTrainings(ctx context.Context, olderThan time.Duration) (int64, error)
	}
)

type Deps struct {
	Matrix MatrixSource
	Meta   MetaSource
	Tags   TagSource
	Store  Store
	Coef   Coefficients
	// TrainingsPerHour — анти-абуз: столько оценённых тренировок в час на пользователя
	TrainingsPerHour int
	Logger           *slog.Logger
}

type Service struct {
	d   Deps
	now func() time.Time

	payloadMu sync.Mutex
	payload   *payloadEntry

	boardMu sync.Mutex
	boards  map[string]boardEntry
}

type payloadEntry struct {
	m    *matchups.Matrix
	body []byte
	etag string
	at   time.Time
}

type boardEntry struct {
	rows []LeaderRow
	at   time.Time
}

func New(d Deps) *Service {
	return &Service{d: d, now: time.Now, boards: map[string]boardEntry{}}
}

// positions — данные по позициям из меты; без меты (первый запуск) — только теги героев.
func (s *Service) positions(ctx context.Context) PositionData {
	snap, err := s.d.Meta.Snapshot(ctx)
	if err != nil {
		snap = nil
		if !errors.Is(err, meta.ErrNotReady) {
			s.d.Logger.Warn("drafter: meta snapshot", "err", err)
		}
	}
	return BuildPositions(snap, s.d.Tags)
}

func (s *Service) matrix(ctx context.Context) (*matchups.Matrix, error) {
	m, err := s.d.Matrix.Matrix(ctx)
	if errors.Is(err, matchups.ErrNotReady) {
		return nil, ErrNotReady
	}
	return m, err
}

// ----- матрица для клиента ---------------------------------------------------

// MatrixPayload — JSON матрицы для клиента (SPEC 5.1) и её ETag. Значения округлены до 0.1.
// playable[i] — герои, которых реально играют на позиции i+1.
func (s *Service) MatrixPayload(ctx context.Context) ([]byte, string, error) {
	m, err := s.matrix(ctx)
	if err != nil {
		return nil, "", err
	}
	s.payloadMu.Lock()
	defer s.payloadMu.Unlock()
	if p := s.payload; p != nil && p.m == m && s.now().Sub(p.at) < payloadTTL {
		return p.body, p.etag, nil
	}
	body := encodeMatrix(m, s.positions(ctx))
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	s.payload = &payloadEntry{m: m, body: body, etag: etag, at: s.now()}
	return body, etag, nil
}

func appendTenths(b []byte, v float32) []byte {
	f := float64(v)
	if f > -0.05 && f < 0.05 {
		return append(b, '0')
	}
	return strconv.AppendFloat(b, f, 'f', 1, 32)
}

func appendMatrix(b []byte, rows [][]float32) []byte {
	b = append(b, '[')
	for i, row := range rows {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '[')
		for j, v := range row {
			if j > 0 {
				b = append(b, ',')
			}
			b = appendTenths(b, v)
		}
		b = append(b, ']')
	}
	return append(b, ']')
}

func encodeMatrix(m *matchups.Matrix, pd PositionData) []byte {
	var buf bytes.Buffer
	ids, _ := json.Marshal(m.IDs)
	buf.WriteString(`{"ids":`)
	buf.Write(ids)
	buf.WriteString(`,"vs":`)
	buf.Write(appendMatrix(nil, m.VS))
	buf.WriteString(`,"with":`)
	if m.HasWith {
		buf.Write(appendMatrix(nil, m.With))
	} else {
		buf.WriteString("null")
	}
	buf.WriteString(`,"playable":[`)
	for pos := 1; pos <= 5; pos++ {
		if pos > 1 {
			buf.WriteByte(',')
		}
		list, _ := json.Marshal(pd.PlayableList(pos))
		buf.Write(list)
	}
	updated, _ := json.Marshal(m.UpdatedAt)
	fmt.Fprintf(&buf, `],"has_with":%t,"updated_at":%s}`, m.HasWith, updated)
	return buf.Bytes()
}

// ----- анализ (запасной вариант без клиентского расчёта) -----------------------

// ValidateSlots проверяет героев драфта: известные, без повторов между всеми слотами и банами.
func (s *Service) ValidateSlots(ctx context.Context, radiant, dire []Slot, bans []int) error {
	m, err := s.matrix(ctx)
	if err != nil {
		return err
	}
	if len(radiant) > 5 || len(dire) > 5 || len(bans) > 10 {
		return ErrBadTeam
	}
	seen := map[int]bool{}
	check := func(id int) bool {
		if !m.Has(id) || seen[id] {
			return false
		}
		seen[id] = true
		return true
	}
	for _, team := range [][]Slot{radiant, dire} {
		for _, sl := range team {
			if !check(sl.HeroID) || sl.Position < 1 || sl.Position > 5 {
				return ErrBadTeam
			}
		}
	}
	for _, id := range bans {
		if !check(id) {
			return ErrBadTeam
		}
	}
	return nil
}

// Analyze — серверный расчёт «Анализа» (POST /drafter/analyze).
func (s *Service) Analyze(ctx context.Context, radiant, dire []Slot, bans []int) (*Analysis, error) {
	if err := s.ValidateSlots(ctx, radiant, dire, bans); err != nil {
		return nil, err
	}
	m, err := s.matrix(ctx)
	if err != nil {
		return nil, err
	}
	a := Analyze(m, radiant, dire)
	return &a, nil
}

// ----- тренировка ------------------------------------------------------------

// NewTraining создаёт тренировку и отдаёт врагов. userID = nil — гость.
func (s *Service) NewTraining(ctx context.Context, userID *int64) (string, []Slot, error) {
	if _, err := s.matrix(ctx); err != nil {
		return "", nil, err
	}
	rnd := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	enemies, err := GenerateEnemies(s.positions(ctx), rnd)
	if err != nil {
		return "", nil, ErrNotReady
	}
	raw, err := json.Marshal(enemies)
	if err != nil {
		return "", nil, err
	}
	id, err := s.d.Store.CreateTraining(ctx, userID, raw)
	if err != nil {
		return "", nil, err
	}
	return id, enemies, nil
}

// Result — итог оценки тренировки.
type Result struct {
	Score     float64   `json:"score"`
	Breakdown Breakdown `json:"breakdown"`
	// MonthRank — место в топе месяца
	MonthRank *int `json:"month_rank"`
}

// Submit оценивает команду игрока против врагов тренировки. Гостевая тренировка
// привязывается к пользователю. Клиенту не доверяем: счёт считает только сервер.
func (s *Service) Submit(ctx context.Context, userID int64, trainingID string, team []Slot) (*Result, error) {
	t, err := s.d.Store.TrainingByID(ctx, trainingID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrTrainingMissing
	}
	if err != nil {
		return nil, err
	}
	if t.Status != "open" || (t.UserID != nil && *t.UserID != userID) {
		return nil, ErrTrainingClosed
	}

	since := s.now().Add(-time.Hour)
	n, err := s.d.Store.CountSubmittedSince(ctx, userID, since)
	if err != nil {
		return nil, err
	}
	if n >= s.d.TrainingsPerHour {
		return nil, &TooManyError{RetryAfter: time.Hour}
	}

	var enemies []Slot
	if err := json.Unmarshal(t.Enemies, &enemies); err != nil {
		return nil, fmt.Errorf("training %s: bad enemies: %w", t.ID, err)
	}
	m, err := s.matrix(ctx)
	if err != nil {
		return nil, err
	}
	if err := validateTeam(m, team, enemies); err != nil {
		return nil, err
	}

	score, bd := ScoreTeam(m, s.positions(ctx), s.d.Coef, team, enemies)
	teamJSON, _ := json.Marshal(team)
	bdJSON, err := json.Marshal(bd)
	if err != nil {
		return nil, err
	}
	err = s.d.Store.SubmitTraining(ctx, t.ID, userID, teamJSON, score, bdJSON)
	if errors.Is(err, store.ErrTrainingClosed) {
		return nil, ErrTrainingClosed
	}
	if err != nil {
		return nil, err
	}

	res := &Result{Score: score, Breakdown: bd}
	month, _, _ := MonthRange("", s.now())
	s.invalidateBoard(month)
	if rows, err := s.rows(ctx, month); err == nil {
		for _, r := range rows {
			if r.UserID == userID {
				rank := r.Rank
				res.MonthRank = &rank
				break
			}
		}
	}
	return res, nil
}

// validateTeam — пять разных героев на позициях 1–5, не из вражеской команды.
func validateTeam(m *matchups.Matrix, team, enemies []Slot) error {
	if len(team) != 5 {
		return ErrBadTeam
	}
	heroes := map[int]bool{}
	for _, e := range enemies {
		heroes[e.HeroID] = true
	}
	positions := map[int]bool{}
	for _, sl := range team {
		if !m.Has(sl.HeroID) || heroes[sl.HeroID] || sl.Position < 1 || sl.Position > 5 || positions[sl.Position] {
			return ErrBadTeam
		}
		heroes[sl.HeroID] = true
		positions[sl.Position] = true
	}
	return nil
}

// ----- история ---------------------------------------------------------------

// HistoryItem — одна оценённая тренировка.
type HistoryItem struct {
	ID          string     `json:"id"`
	SubmittedAt time.Time  `json:"submitted_at"`
	Score       float64    `json:"score"`
	Team        []Slot     `json:"team"`
	Enemies     []Slot     `json:"enemies"`
	Breakdown   *Breakdown `json:"breakdown"`
}

// HistoryPage — страница истории.
type HistoryPage struct {
	Items      []HistoryItem `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

// History — оценённые тренировки пользователя, новые первыми.
func (s *Service) History(ctx context.Context, userID int64, cursor string) (*HistoryPage, error) {
	var before *time.Time
	if cursor != "" {
		micro, err := strconv.ParseInt(cursor, 10, 64)
		if err != nil || micro <= 0 {
			return nil, ErrBadCursor
		}
		t := time.UnixMicro(micro)
		before = &t
	}
	rows, err := s.d.Store.TrainingHistory(ctx, userID, before, HistoryPageSize+1)
	if err != nil {
		return nil, err
	}
	page := &HistoryPage{Items: make([]HistoryItem, 0, min(len(rows), HistoryPageSize))}
	for i, t := range rows {
		if i == HistoryPageSize {
			next := strconv.FormatInt(rows[HistoryPageSize-1].SubmittedAt.UnixMicro(), 10)
			page.NextCursor = &next
			break
		}
		item := HistoryItem{ID: t.ID, Team: []Slot{}, Enemies: []Slot{}}
		if t.SubmittedAt != nil {
			item.SubmittedAt = *t.SubmittedAt
		}
		if t.Score != nil {
			item.Score = round1(*t.Score)
		}
		_ = json.Unmarshal(t.Team, &item.Team)
		_ = json.Unmarshal(t.Enemies, &item.Enemies)
		if len(t.Breakdown) > 0 {
			var bd Breakdown
			if json.Unmarshal(t.Breakdown, &bd) == nil {
				item.Breakdown = &bd
			}
		}
		page.Items = append(page.Items, item)
	}
	return page, nil
}

// Cleanup — джоба: удаляет брошенные неоценённые тренировки.
func (s *Service) Cleanup(ctx context.Context) error {
	n, err := s.d.Store.DeleteStaleTrainings(ctx, StaleTrainingAfter)
	if n > 0 {
		s.d.Logger.Info("stale trainings deleted", "count", n)
	}
	return err
}
