// Package matchups — матрица матчапов и синергий героев: сбор из STRATZ (или OpenDota),
// хранение в hero_matchups и выдача в виде плотной матрицы для драфтера (SPEC 5.1).
package matchups

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

const (
	// RefreshEvery — период джобы matchups (SPEC 3.6)
	RefreshEvery = 24 * time.Hour
	matrixTTL    = 10 * time.Minute

	// пары с меньшим числом матчей слишком шумные: их вклад считаем нулевым
	minPairMatches = 50
	// предел значения: единичные выбросы не должны ломать оценку драфта
	maxAdvantage = 30.0
	// героев в одном запросе STRATZ
	stratzBatch = 6
	// меньше героев с данными — ответ подозрительный, ничего не сохраняем
	minCoverage = 0.8
)

type Source string

const (
	SourceStratz   Source = "stratz"
	SourceOpenDota Source = "opendota"
)

// ErrNotReady — матрицу ещё не собрали (первый запуск).
var ErrNotReady = errors.New("matchups: not collected yet")

type (
	StratzQuerier interface {
		Available() bool
		Query(ctx context.Context, query string, vars map[string]any, out any) error
	}
	OpenDotaSource interface {
		HeroStats(ctx context.Context) ([]opendota.HeroStat, error)
		HeroMatchups(ctx context.Context, heroID int) ([]opendota.HeroMatchup, error)
	}
	HeroSource interface {
		Heroes() ([]dota.Hero, bool)
	}
	Store interface {
		ReplaceHeroMatchups(ctx context.Context, rows []store.MatchupRow) error
		HeroMatchups(ctx context.Context) ([]store.MatchupRow, time.Time, error)
		GetKV(ctx context.Context, key string) ([]byte, time.Time, error)
		PutKV(ctx context.Context, key string, value []byte) error
	}
)

type Deps struct {
	Stratz   StratzQuerier
	OpenDota OpenDotaSource
	Heroes   HeroSource
	Store    Store
	Logger   *slog.Logger
}

type Service struct {
	d      Deps
	matrix atomic.Pointer[matrixEntry]
	now    func() time.Time
}

func New(d Deps) *Service {
	return &Service{d: d, now: time.Now}
}

type matrixEntry struct {
	m        *Matrix
	loadedAt time.Time
}

const sourceKey = "matchups:source"

// Matrix — текущая матрица (из БД, кэш в памяти на 10 минут). ErrNotReady — данных ещё нет.
func (s *Service) Matrix(ctx context.Context) (*Matrix, error) {
	if e := s.matrix.Load(); e != nil && s.now().Sub(e.loadedAt) < matrixTTL {
		return e.m, nil
	}
	rows, updatedAt, err := s.d.Store.HeroMatchups(ctx)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotReady
	}
	if err != nil {
		return nil, err
	}
	heroes, ok := s.d.Heroes.Heroes()
	if !ok {
		return nil, ErrNotReady
	}
	ids := make([]int, len(heroes))
	for i, h := range heroes {
		ids[i] = h.ID
	}
	m := BuildMatrix(ids, rows, updatedAt)
	s.matrix.Store(&matrixEntry{m: m, loadedAt: s.now()})
	return m, nil
}

// Refresh собирает матрицу заново и сохраняет её целиком.
func (s *Service) Refresh(ctx context.Context) error {
	heroes, ok := s.d.Heroes.Heroes()
	if !ok {
		return errors.New("matchups: heroes are not loaded yet")
	}
	ids := make([]int, len(heroes))
	for i, h := range heroes {
		ids[i] = h.ID
	}

	var (
		rows   []store.MatchupRow
		source Source
		err    error
	)
	if s.d.Stratz.Available() {
		rows, err = s.fromStratz(ctx, ids)
		source = SourceStratz
		if err != nil {
			s.d.Logger.Warn("matchups: STRATZ failed, using OpenDota", "err", err)
		}
	}
	if source == "" || err != nil {
		rows, err = s.fromOpenDota(ctx, ids)
		source = SourceOpenDota
		if err != nil {
			return err
		}
	}

	if err := s.d.Store.ReplaceHeroMatchups(ctx, rows); err != nil {
		return err
	}
	raw, _ := json.Marshal(string(source))
	if err := s.d.Store.PutKV(ctx, sourceKey, raw); err != nil {
		s.d.Logger.Warn("matchups: save source", "err", err)
	}
	s.matrix.Store(nil)
	s.d.Logger.Info("matchups refreshed", "source", source, "rows", len(rows))
	return nil
}

// RefreshIfStale — для джобы: после перезапуска не пересобирать свежую матрицу.
func (s *Service) RefreshIfStale(ctx context.Context) error {
	_, updatedAt, err := s.d.Store.HeroMatchups(ctx)
	fresh := err == nil && s.now().Sub(updatedAt) < RefreshEvery-time.Hour
	// STRATZ появился (токен заработал) — синергии нужны, не ждём сутки
	upgrade := s.d.Stratz.Available() && s.loadSource(ctx) != SourceStratz
	if fresh && !upgrade {
		return nil
	}
	return s.Refresh(ctx)
}

func (s *Service) loadSource(ctx context.Context) Source {
	raw, _, err := s.d.Store.GetKV(ctx, sourceKey)
	if err != nil {
		return SourceOpenDota
	}
	var src string
	if json.Unmarshal(raw, &src) != nil || src != string(SourceStratz) {
		return SourceOpenDota
	}
	return SourceStratz
}

// ----- STRATZ ------------------------------------------------------------------

// Схема сверена интроспекцией: heroStats.heroVsHeroMatchup(heroId) → advantage[]
// { heroId, vs[], with[] }, где элемент — HeroStatsHeroDryadType
// { heroId2, matchCount, winCount, synergy }; synergy — преимущество героя в п. п.
const stratzHeroQuery = `h%d: heroVsHeroMatchup(heroId: %d, matchLimit: %d) {
  advantage { heroId vs { heroId2 matchCount synergy } with { heroId2 matchCount synergy } }
}`

type stratzPair struct {
	HeroID2    int     `json:"heroId2"`
	MatchCount int     `json:"matchCount"`
	Synergy    float64 `json:"synergy"`
}

type stratzHero struct {
	Advantage []struct {
		HeroID int          `json:"heroId"`
		Vs     []stratzPair `json:"vs"`
		With   []stratzPair `json:"with"`
	} `json:"advantage"`
}

func (s *Service) fromStratz(ctx context.Context, ids []int) ([]store.MatchupRow, error) {
	known := make(map[int]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}
	var rows []store.MatchupRow
	covered := 0
	for start := 0; start < len(ids); start += stratzBatch {
		batch := ids[start:min(start+stratzBatch, len(ids))]
		query := "query Matchups { heroStats {\n"
		for _, id := range batch {
			query += fmt.Sprintf(stratzHeroQuery, id, id, minPairMatches) + "\n"
		}
		query += "} }"
		var out struct {
			HeroStats map[string]stratzHero `json:"heroStats"`
		}
		if err := s.d.Stratz.Query(ctx, query, nil, &out); err != nil {
			return nil, err
		}
		for _, id := range batch {
			h := out.HeroStats[fmt.Sprintf("h%d", id)]
			if len(h.Advantage) == 0 {
				continue
			}
			adv := h.Advantage[0]
			if len(adv.Vs) > 0 {
				covered++
			}
			for _, p := range adv.Vs {
				if known[p.HeroID2] && p.HeroID2 != id {
					rows = append(rows, row(id, p.HeroID2, "vs", p.MatchCount, p.Synergy))
				}
			}
			for _, p := range adv.With {
				if known[p.HeroID2] && p.HeroID2 != id {
					rows = append(rows, row(id, p.HeroID2, "with", p.MatchCount, p.Synergy))
				}
			}
		}
	}
	if float64(covered) < minCoverage*float64(len(ids)) {
		return nil, fmt.Errorf("stratz: matchups for only %d of %d heroes", covered, len(ids))
	}
	return rows, nil
}

func row(hero, other int, kind string, matches int, advantage float64) store.MatchupRow {
	// мало матчей — значение шумное, считаем его нулевым
	if matches < minPairMatches {
		advantage = 0
	}
	advantage = math.Max(-maxAdvantage, math.Min(maxAdvantage, advantage))
	return store.MatchupRow{HeroID: hero, OtherID: other, Kind: kind, Matches: matches, Advantage: float32(advantage)}
}

// ----- OpenDota ----------------------------------------------------------------

// fromOpenDota — фоллбэк: только «против». Преимущество = (wins/games − общий винрейт героя) × 100.
func (s *Service) fromOpenDota(ctx context.Context, ids []int) ([]store.MatchupRow, error) {
	stats, err := s.d.OpenDota.HeroStats(ctx)
	if err != nil {
		return nil, err
	}
	base := map[int]float64{}
	for _, st := range stats {
		if st.PubPick > 0 {
			base[st.ID] = float64(st.PubWin) / float64(st.PubPick)
		}
	}
	known := make(map[int]bool, len(ids))
	for _, id := range ids {
		known[id] = true
	}

	var rows []store.MatchupRow
	covered := 0
	for _, id := range ids {
		b, ok := base[id]
		if !ok {
			continue
		}
		list, err := s.d.OpenDota.HeroMatchups(ctx, id)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.d.Logger.Warn("matchups: opendota hero", "hero_id", id, "err", err)
			continue
		}
		covered++
		for _, m := range list {
			if !known[m.HeroID] || m.HeroID == id || m.GamesPlayed == 0 {
				continue
			}
			adv := (float64(m.Wins)/float64(m.GamesPlayed) - b) * 100
			rows = append(rows, row(id, m.HeroID, "vs", m.GamesPlayed, adv))
		}
	}
	if float64(covered) < minCoverage*float64(len(ids)) {
		return nil, fmt.Errorf("opendota: matchups for only %d of %d heroes", covered, len(ids))
	}
	return rows, nil
}

// ----- матрица ---------------------------------------------------------------

// Matrix — плотная матрица преимуществ: VS[i][j] — насколько winrate героя IDs[i]
// против IDs[j] выше ожидаемого (п. п.), With[i][j] — синергия в одной команде.
type Matrix struct {
	IDs       []int
	VS        [][]float32
	With      [][]float32
	HasWith   bool
	UpdatedAt time.Time
	index     map[int]int
}

// BuildMatrix собирает матрицу из строк таблицы; герои идут по возрастанию id.
func BuildMatrix(heroIDs []int, rows []store.MatchupRow, updatedAt time.Time) *Matrix {
	ids := append([]int(nil), heroIDs...)
	sort.Ints(ids)
	m := &Matrix{IDs: ids, UpdatedAt: updatedAt, index: make(map[int]int, len(ids))}
	for i, id := range ids {
		m.index[id] = i
	}
	n := len(ids)
	m.VS = make([][]float32, n)
	m.With = make([][]float32, n)
	for i := range m.VS {
		m.VS[i] = make([]float32, n)
		m.With[i] = make([]float32, n)
	}
	for _, r := range rows {
		i, ok1 := m.index[r.HeroID]
		j, ok2 := m.index[r.OtherID]
		if !ok1 || !ok2 {
			continue
		}
		switch r.Kind {
		case "vs":
			m.VS[i][j] = r.Advantage
		case "with":
			m.With[i][j] = r.Advantage
			m.HasWith = true
		}
	}
	return m
}

// Has — есть ли герой в матрице.
func (m *Matrix) Has(id int) bool {
	_, ok := m.index[id]
	return ok
}

// Vs — преимущество героя a над b (0, если данных нет).
func (m *Matrix) Vs(a, b int) float64 {
	i, ok1 := m.index[a]
	j, ok2 := m.index[b]
	if !ok1 || !ok2 {
		return 0
	}
	return float64(m.VS[i][j])
}

// Synergy — синергия a и b в одной команде (0, если данных нет).
func (m *Matrix) Synergy(a, b int) float64 {
	i, ok1 := m.index[a]
	j, ok2 := m.index[b]
	if !ok1 || !ok2 {
		return 0
	}
	return float64(m.With[i][j])
}
