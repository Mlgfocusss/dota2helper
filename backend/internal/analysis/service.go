package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/llm"
	"d2helper/internal/store"
)

// Тексты ошибок разбора показываются пользователю как есть.
const (
	msgReplayGone    = "Реплей этого матча больше недоступен"
	msgParseTimeout  = "OpenDota не успела разобрать реплей. Попробуй чуть позже"
	msgMatchNotFound = "Матч не найден в OpenDota"
	msgOpenDota      = "Не удалось получить данные матча из OpenDota. Попробуй через несколько минут"
	msgNotInMatch    = "Не нашли тебя в этом матче: возможно, профиль скрыт"
	msgAuth          = "AI-сервис отклонил ключ доступа. Сообщи об этом в поддержку"
	msgBusy          = "AI-сервис сейчас перегружен. Попробуй через несколько минут — этот разбор не засчитан"
	msgBadAnswer     = "Модель вернула некорректный ответ. Попробуй ещё раз — этот разбор не засчитан"
	msgInternal      = "Что-то пошло не так при разборе. Попробуй ещё раз"
)

var (
	// ErrUnavailable — разбор выключен или языковая модель не настроена.
	ErrUnavailable = errors.New("analysis: unavailable")
	// ErrNotYourMatch — матча нет в истории профиля пользователя.
	ErrNotYourMatch = errors.New("analysis: match is not in the user's history")
	// ErrNotFound — разбор не найден (или принадлежит другому пользователю).
	ErrNotFound = errors.New("analysis: not found")
)

// QuotaError — исчерпан дневной лимит разборов.
type QuotaError struct {
	ResetAt time.Time
}

func (e *QuotaError) Error() string { return "analysis: daily quota exceeded" }

// ExistsError — по матчу уже есть действующий разбор.
type ExistsError struct{ ID string }

func (e *ExistsError) Error() string { return "analysis: already exists" }

type (
	Store interface {
		CreateAnalysis(ctx context.Context, userID, accountID, matchID int64) (store.Analysis, error)
		AnalysisByID(ctx context.Context, id string) (store.Analysis, error)
		ClaimQueuedAnalysis(ctx context.Context) (*store.Analysis, error)
		SetAnalysisStatus(ctx context.Context, id, status string) error
		FinishAnalysis(ctx context.Context, id string, result []byte) error
		FailAnalysis(ctx context.Context, id, message string) error
		RequeueStuckAnalyses(ctx context.Context) (int64, error)
		CountAnalysesSince(ctx context.Context, userID int64, since time.Time) (int, *time.Time, error)
		HasPlayerMatch(ctx context.Context, accountID, matchID int64) (bool, error)
	}
	Source interface {
		ParsedMatch(ctx context.Context, matchID int64) (*opendota.ParsedMatch, error)
		RequestParse(ctx context.Context, matchID int64) error
		ItemConstants(ctx context.Context) (map[string]opendota.ItemConstant, error)
	}
	HeroSource interface {
		Heroes() ([]dota.Hero, bool)
	}
)

// Config — настройки сервиса разбора.
type Config struct {
	// Enabled — флаг FEATURE_DEEP_ANALYSIS
	Enabled bool
	// PerDay — сколько разборов в сутки на пользователя (бесплатные модели не бездонные)
	PerDay int
	// MaxParallel — сколько разборов обрабатывается одновременно
	MaxParallel int
	// PollEvery — как часто спрашивать OpenDota, разобран ли реплей
	PollEvery time.Duration
	// ParseTimeout — сколько ждать разбора реплея
	ParseTimeout time.Duration
	// ReplayMaxAge — реплеи старше недоступны для разбора
	ReplayMaxAge time.Duration
	// Tick — как часто воркер заглядывает в очередь, если его не разбудили
	Tick time.Duration
}

func (c Config) withDefaults() Config {
	if c.PerDay < 1 {
		c.PerDay = 3
	}
	if c.MaxParallel < 1 {
		c.MaxParallel = 2
	}
	if c.PollEvery <= 0 {
		c.PollEvery = 20 * time.Second
	}
	if c.ParseTimeout <= 0 {
		c.ParseTimeout = 15 * time.Minute
	}
	if c.ReplayMaxAge <= 0 {
		c.ReplayMaxAge = 14 * 24 * time.Hour
	}
	if c.Tick <= 0 {
		c.Tick = 5 * time.Second
	}
	return c
}

type Deps struct {
	Store    Store
	OpenDota Source
	LLM      llm.Provider
	Heroes   HeroSource
	Config   Config
	Logger   *slog.Logger
	// Example — готовый пример разбора (backend/data/analysis_example.json); может быть nil
	Example json.RawMessage
}

type Service struct {
	d    Deps
	cfg  Config
	now  func() time.Time
	wake chan struct{}
}

func New(d Deps) *Service {
	return &Service{d: d, cfg: d.Config.withDefaults(), now: time.Now, wake: make(chan struct{}, 1)}
}

// LoadExample читает пример разбора для страницы «Пример»; файл проверяется на валидный JSON.
func LoadExample(dataDir string) (json.RawMessage, error) {
	raw, err := os.ReadFile(filepath.Join(dataDir, "analysis_example.json"))
	if err != nil {
		return nil, fmt.Errorf("read analysis example: %w", err)
	}
	var r Result
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parse analysis example: %w", err)
	}
	if r.Summary.Verdict == "" || len(r.Episodes) == 0 || len(r.Improvements) == 0 {
		return nil, errors.New("analysis example is incomplete")
	}
	return raw, nil
}

// Available — можно ли заказывать разбор: функция включена и модель настроена.
func (s *Service) Available() bool {
	return s.cfg.Enabled && s.d.LLM != nil && s.d.LLM.Configured()
}

// PerDay — дневной лимит разборов на пользователя.
func (s *Service) PerDay() int { return s.cfg.PerDay }

// Quota — сколько разборов осталось сегодня.
type Quota struct {
	PerDay int `json:"per_day"`
	Left   int `json:"left"`
	// ResetAt — когда освободится место (самый старый разбор «выйдет» из суток); nil, если лимит не исчерпан
	ResetAt *time.Time `json:"reset_at"`
}

func (s *Service) Quota(ctx context.Context, userID int64) (Quota, error) {
	n, oldest, err := s.d.Store.CountAnalysesSince(ctx, userID, s.now().Add(-24*time.Hour))
	if err != nil {
		return Quota{}, err
	}
	q := Quota{PerDay: s.cfg.PerDay, Left: max(0, s.cfg.PerDay-n)}
	if q.Left == 0 && oldest != nil {
		reset := oldest.Add(24 * time.Hour)
		q.ResetAt = &reset
	}
	return q, nil
}

// View — разбор в ответах API.
type View struct {
	ID        string          `json:"id"`
	MatchID   int64           `json:"match_id"`
	Status    string          `json:"status"`
	Error     string          `json:"error,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func viewOf(a store.Analysis) *View {
	v := &View{ID: a.ID, MatchID: a.MatchID, Status: a.Status, Error: a.Error, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
	if a.Status == "done" && len(a.Result) > 0 {
		v.Result = a.Result
	}
	return v
}

// Create заказывает разбор матча из истории профиля пользователя.
func (s *Service) Create(ctx context.Context, user store.User, matchID int64) (*View, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	account := user.AccountID
	if user.TrackedAccountID != nil {
		account = *user.TrackedAccountID
	}
	ok, err := s.d.Store.HasPlayerMatch(ctx, account, matchID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNotYourMatch
	}

	q, err := s.Quota(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	a, err := s.d.Store.CreateAnalysis(ctx, user.ID, account, matchID)
	switch {
	case errors.Is(err, store.ErrAnalysisExists):
		// уже заказан — отдаём существующий, лимит не тратится
		return nil, &ExistsError{ID: a.ID}
	case err != nil:
		return nil, err
	}
	// проверка после вставки закрывает гонку двух одновременных заказов
	if q.Left == 0 {
		_ = s.d.Store.FailAnalysis(ctx, a.ID, "Дневной лимит разборов исчерпан")
		reset := s.now().Add(24 * time.Hour)
		if q.ResetAt != nil {
			reset = *q.ResetAt
		}
		return nil, &QuotaError{ResetAt: reset}
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return viewOf(a), nil
}

// Get отдаёт разбор владельцу.
func (s *Service) Get(ctx context.Context, userID int64, id string) (*View, error) {
	a, err := s.d.Store.AnalysisByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && a.UserID != userID) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return viewOf(a), nil
}

// ExampleView — готовый пример разбора для всех (в том числе гостей).
func (s *Service) ExampleView() (*View, bool) {
	if len(s.d.Example) == 0 {
		return nil, false
	}
	return &View{ID: "example", Status: "done", Result: s.d.Example}, true
}

// ----- воркер ------------------------------------------------------------------

// Run обрабатывает очередь, пока не отменён ctx. Запускается один раз при старте сервера.
func (s *Service) Run(ctx context.Context) {
	if n, err := s.d.Store.RequeueStuckAnalyses(ctx); err != nil {
		s.d.Logger.Error("analysis: requeue stuck", "err", err)
	} else if n > 0 {
		s.d.Logger.Info("analysis: requeued after restart", "count", n)
	}

	sem := make(chan struct{}, s.cfg.MaxParallel)
	var wg sync.WaitGroup
	defer wg.Wait()
	ticker := time.NewTicker(s.cfg.Tick)
	defer ticker.Stop()

	for {
		s.drain(ctx, sem, &wg)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
	}
}

// drain запускает обработку, пока в очереди есть заказы и свободны места.
func (s *Service) drain(ctx context.Context, sem chan struct{}, wg *sync.WaitGroup) {
	for ctx.Err() == nil {
		select {
		case sem <- struct{}{}:
		default:
			return // все места заняты
		}
		a, err := s.d.Store.ClaimQueuedAnalysis(ctx)
		if err != nil || a == nil {
			<-sem
			if err != nil && ctx.Err() == nil {
				s.d.Logger.Error("analysis: claim", "err", err)
			}
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			s.process(ctx, *a)
		}()
	}
}

// process проводит один разбор через все стадии. При любой ошибке разбор помечается
// неудавшимся: он не считается в дневном лимите, и матч можно заказать снова.
func (s *Service) process(ctx context.Context, a store.Analysis) {
	log := s.d.Logger.With("analysis_id", a.ID, "match_id", a.MatchID)
	fail := func(msg string) {
		// ctx мог быть отменён при остановке сервера — запись ошибки не должна от этого зависеть
		wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := s.d.Store.FailAnalysis(wctx, a.ID, msg); err != nil {
			log.Error("analysis: save failure", "err", err)
		}
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("analysis: panic", "panic", r, "stack", string(debug.Stack()))
			fail(msgInternal)
		}
	}()

	match, failMsg, err := s.waitParsed(ctx, a)
	if err != nil {
		if ctx.Err() != nil {
			return // сервер останавливается: разбор вернётся в очередь при старте
		}
		log.Warn("analysis: match", "err", err)
		fail(failMsg)
		return
	}

	if err := s.d.Store.SetAnalysisStatus(ctx, a.ID, "analyzing"); err != nil {
		log.Error("analysis: set status", "err", err)
		return
	}
	result, failMsg, err := s.analyze(ctx, a, match)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Warn("analysis: failed", "err", err)
		fail(failMsg)
		return
	}
	raw, err := json.Marshal(result)
	if err != nil {
		log.Error("analysis: marshal result", "err", err)
		fail(msgInternal)
		return
	}
	if err := s.d.Store.FinishAnalysis(ctx, a.ID, raw); err != nil {
		log.Error("analysis: save result", "err", err)
		return
	}
	log.Info("analysis done", "rating", result.Summary.Rating)
}

// waitParsed получает распарсенный матч: если реплей ещё не разобран — просит OpenDota
// и ждёт (SPEC 6.3). Второе значение — текст ошибки для пользователя.
func (s *Service) waitParsed(ctx context.Context, a store.Analysis) (*opendota.ParsedMatch, string, error) {
	m, err := s.d.OpenDota.ParsedMatch(ctx, a.MatchID)
	if errors.Is(err, opendota.ErrNotFound) {
		return nil, msgMatchNotFound, err
	}
	if err != nil {
		return nil, msgOpenDota, err
	}
	if m.Parsed() {
		return m, "", nil
	}

	// слишком старый реплей Valve уже удалила — ждать бессмысленно
	if m.StartTime > 0 && s.now().Sub(time.Unix(m.StartTime, 0)) > s.cfg.ReplayMaxAge {
		return nil, msgReplayGone, errors.New("replay is too old")
	}
	if err := s.d.OpenDota.RequestParse(ctx, a.MatchID); err != nil {
		s.d.Logger.Warn("analysis: request parse", "match_id", a.MatchID, "err", err)
	}
	deadline := time.NewTimer(s.cfg.ParseTimeout)
	defer deadline.Stop()
	poll := time.NewTicker(s.cfg.PollEvery)
	defer poll.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-deadline.C:
			return nil, msgParseTimeout, errors.New("parse timeout")
		case <-poll.C:
			m, err := s.d.OpenDota.ParsedMatch(ctx, a.MatchID)
			if err != nil {
				// единичный сбой OpenDota не должен ронять ожидание
				s.d.Logger.Warn("analysis: poll", "match_id", a.MatchID, "err", err)
				continue
			}
			if m.Parsed() {
				return m, "", nil
			}
		}
	}
}

// catalog собирает справочники названий героев и предметов.
func (s *Service) catalog(ctx context.Context) Catalog {
	byID := map[int]string{}
	byKey := map[string]string{}
	if heroes, ok := s.d.Heroes.Heroes(); ok {
		for _, h := range heroes {
			byID[h.ID] = h.LocalizedName
			byKey[h.Name] = h.LocalizedName
		}
	}
	cat := Catalog{
		HeroName: func(id int) string {
			if n := byID[id]; n != "" {
				return n
			}
			return fmt.Sprintf("hero %d", id)
		},
		HeroByKey: func(key string) string { return byKey[strings.TrimSpace(key)] },
		ItemByKey: map[string]opendota.ItemConstant{},
		ItemName:  func(int) string { return "" },
	}
	consts, err := s.d.OpenDota.ItemConstants(ctx)
	if err != nil {
		s.d.Logger.Warn("analysis: item constants", "err", err)
		return cat
	}
	cat.ItemByKey = consts
	byItem := make(map[int]string, len(consts))
	for _, it := range consts {
		byItem[it.ID] = it.DName
	}
	cat.ItemName = func(id int) string { return byItem[id] }
	return cat
}

// analyze строит признаки, спрашивает модель и проверяет ответ. Один раз повторяет запрос,
// если ответ не прошёл проверку. Второе значение — текст ошибки для пользователя.
func (s *Service) analyze(ctx context.Context, a store.Analysis, m *opendota.ParsedMatch) (*Result, string, error) {
	features, err := BuildFeatures(m, a.AccountID, s.catalog(ctx))
	if errors.Is(err, ErrPlayerNotInMatch) {
		return nil, msgNotInMatch, err
	}
	if err != nil {
		return nil, msgInternal, err
	}
	req, err := BuildPrompt(features)
	if err != nil {
		return nil, msgInternal, err
	}

	var lastProblem error
	for attempt := 0; attempt < 2; attempt++ {
		r := req
		if lastProblem != nil {
			r = WithRetryHint(req, lastProblem)
		}
		raw, err := s.d.LLM.Complete(ctx, r)
		switch {
		case errors.Is(err, llm.ErrAuth):
			return nil, msgAuth, err
		case err != nil && ctx.Err() != nil:
			return nil, "", ctx.Err()
		case err != nil:
			return nil, msgBusy, err
		}
		answer, perr := ParseAnswer(raw)
		if perr == nil {
			return Assemble(answer, features), "", nil
		}
		s.d.Logger.Warn("analysis: model answer rejected", "attempt", attempt+1, "err", perr)
		lastProblem = perr
	}
	return nil, msgBadAnswer, lastProblem
}
