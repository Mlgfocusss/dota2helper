// Package games — мини-игры: снапшот героев для «Больше / Меньше» и рекорды игроков.
package games

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/store"
)

const (
	// Game — идентификатор игры в game_records
	Game = "higher_lower"
	// RefreshEvery — период джобы higher_lower_snapshot (SPEC 3.6)
	RefreshEvery = 24 * time.Hour
	// MaxStreak — рекорд выше считаем накруткой клиента (SPEC 5.7)
	MaxStreak = 200
	// у героев с меньшим числом матчей средние значения шумные
	minMatches = 1000
	// меньше героев в ответе STRATZ — данные подозрительные
	minHeroes = 50
)

type Mode string

const (
	Popularity Mode = "popularity"
	Kills      Mode = "kills"
	Deaths     Mode = "deaths"
)

var Modes = []Mode{Popularity, Kills, Deaths}

func ParseMode(s string) (Mode, bool) {
	for _, m := range Modes {
		if string(m) == s {
			return m, true
		}
	}
	return "", false
}

var (
	ErrNotReady  = errors.New("games: snapshot is not ready")
	ErrBadStreak = errors.New("games: bad streak")
)

type (
	StratzQuerier interface {
		Available() bool
		Query(ctx context.Context, query string, vars map[string]any, out any) error
	}
	HeroStatsSource interface {
		HeroStats(ctx context.Context) ([]opendota.HeroStat, error)
	}
	Store interface {
		ReplaceHLSnapshot(ctx context.Context, mode string, rows []store.HLRow) error
		HLSnapshot(ctx context.Context, mode string) ([]store.HLRow, time.Time, error)
		UpsertGameRecord(ctx context.Context, userID int64, game, mode string, score int) (int, error)
		GameRecords(ctx context.Context, userID int64, game string) (map[string]int, error)
	}
)

type Service struct {
	stratz   StratzQuerier
	opendota HeroStatsSource
	store    Store
	logger   *slog.Logger
	now      func() time.Time
}

func New(stratz StratzQuerier, od HeroStatsSource, st Store, logger *slog.Logger) *Service {
	return &Service{stratz: stratz, opendota: od, store: st, logger: logger, now: time.Now}
}

// HeroValue — значение героя в режиме.
type HeroValue struct {
	HeroID int     `json:"hero_id"`
	Value  float32 `json:"value"`
}

// Snapshot — значения героев для режима.
func (s *Service) Snapshot(ctx context.Context, mode Mode) ([]HeroValue, error) {
	rows, _, err := s.store.HLSnapshot(ctx, string(mode))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotReady
	}
	if err != nil {
		return nil, err
	}
	out := make([]HeroValue, len(rows))
	for i, r := range rows {
		out[i] = HeroValue{HeroID: r.HeroID, Value: r.Value}
	}
	return out, nil
}

// RecordScore сохраняет серию игрока и возвращает его рекорд в режиме.
func (s *Service) RecordScore(ctx context.Context, userID int64, mode Mode, streak int) (int, error) {
	if streak < 0 || streak > MaxStreak {
		return 0, ErrBadStreak
	}
	return s.store.UpsertGameRecord(ctx, userID, Game, string(mode), streak)
}

// Records — рекорды игрока по всем режимам (нули, если не играл).
func (s *Service) Records(ctx context.Context, userID int64) (map[string]int, error) {
	got, err := s.store.GameRecords(ctx, userID, Game)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(Modes))
	for _, m := range Modes {
		out[string(m)] = got[string(m)]
	}
	return out, nil
}

// ----- сбор снапшота ---------------------------------------------------------

// Средние убийства и смерти и число матчей по героям за всё время актуальных данных STRATZ.
const stratzQuery = `query HL {
  heroStats {
    stats(groupByPosition: false, groupByBracket: false, groupByTime: false) { heroId matchCount kills deaths }
  }
}`

type stratzStat struct {
	HeroID     int     `json:"heroId"`
	MatchCount int     `json:"matchCount"`
	Kills      float64 `json:"kills"`
	Deaths     float64 `json:"deaths"`
}

// Refresh собирает снапшот: все три режима из STRATZ, а без него — только популярность из OpenDota.
func (s *Service) Refresh(ctx context.Context) error {
	if s.stratz.Available() {
		err := s.fromStratz(ctx)
		if err == nil {
			return nil
		}
		s.logger.Warn("games: STRATZ failed, using OpenDota", "err", err)
	}
	return s.fromOpenDota(ctx)
}

// RefreshIfStale — для джобы: раз в сутки, а если появился STRATZ — сразу,
// чтобы не ждать, пока устареет снапшот только с популярностью.
func (s *Service) RefreshIfStale(ctx context.Context) error {
	_, updatedAt, err := s.store.HLSnapshot(ctx, string(Popularity))
	if err == nil && s.now().Sub(updatedAt) < RefreshEvery-10*time.Minute {
		_, _, killsErr := s.store.HLSnapshot(ctx, string(Kills))
		if killsErr == nil || !s.stratz.Available() {
			return nil
		}
	}
	return s.Refresh(ctx)
}

func (s *Service) fromStratz(ctx context.Context) error {
	var out struct {
		HeroStats struct {
			Stats []stratzStat `json:"stats"`
		} `json:"heroStats"`
	}
	if err := s.stratz.Query(ctx, stratzQuery, nil, &out); err != nil {
		return err
	}
	var pop, kills, deaths []store.HLRow
	for _, st := range out.HeroStats.Stats {
		if st.HeroID <= 0 || st.MatchCount < minMatches {
			continue
		}
		pop = append(pop, store.HLRow{HeroID: st.HeroID, Value: float32(st.MatchCount)})
		kills = append(kills, store.HLRow{HeroID: st.HeroID, Value: float32(st.Kills)})
		deaths = append(deaths, store.HLRow{HeroID: st.HeroID, Value: float32(st.Deaths)})
	}
	if len(pop) < minHeroes {
		return fmt.Errorf("stratz: only %d heroes with enough matches", len(pop))
	}
	for mode, rows := range map[Mode][]store.HLRow{Popularity: pop, Kills: kills, Deaths: deaths} {
		if err := s.store.ReplaceHLSnapshot(ctx, string(mode), rows); err != nil {
			return err
		}
	}
	s.logger.Info("higher-lower snapshot refreshed", "source", "stratz", "heroes", len(pop))
	return nil
}

func (s *Service) fromOpenDota(ctx context.Context) error {
	stats, err := s.opendota.HeroStats(ctx)
	if err != nil {
		return err
	}
	var pop []store.HLRow
	for _, st := range stats {
		if st.PubPick >= minMatches {
			pop = append(pop, store.HLRow{HeroID: st.ID, Value: float32(st.PubPick)})
		}
	}
	if len(pop) < minHeroes {
		return fmt.Errorf("opendota: only %d heroes with enough matches", len(pop))
	}
	if err := s.store.ReplaceHLSnapshot(ctx, string(Popularity), pop); err != nil {
		return err
	}
	s.logger.Info("higher-lower snapshot refreshed", "source", "opendota", "heroes", len(pop))
	return nil
}
