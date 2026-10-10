// Package meta — мета по позициям: сбор из STRATZ (или OpenDota), хранение и выдача топов.
package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

const (
	// RefreshEvery — период джобы meta_positions (SPEC 3.6)
	RefreshEvery = 3 * time.Hour
	snapshotTTL  = 5 * time.Minute
	// меньше героев на позиции — ответ STRATZ подозрительный, берём OpenDota
	minHeroesPerPosition = 10
	// В топ попадают только те, кого реально играют на позиции: без этого сверху оказываются
	// герои с сотнями матчей на чужой роли (Tidehunter-керри) и огромным винрейтом.
	// minPositionShare — доля героя среди всех игр позиции, minRoleShare — доля игр героя
	// на этой позиции от всех его игр.
	minPositionShare = 0.01
	minRoleShare     = 0.3
)

// formatVersion растёт, когда меняется способ сбора меты: данные в БД, собранные
// по-старому, пересобираются сразу, а не через RefreshEvery.
const formatVersion = "2"

// Source — откуда мета: по позициям (STRATZ) или общий винрейт (OpenDota).
type Source string

const (
	SourceStratz   Source = "stratz"
	SourceOpenDota Source = "opendota"
)

// ErrNotReady — мету ещё не собрали (первый запуск).
var ErrNotReady = errors.New("meta: not collected yet")

type (
	StratzQuerier interface {
		Available() bool
		Query(ctx context.Context, query string, vars map[string]any, out any) error
	}
	HeroStatsSource interface {
		HeroStats(ctx context.Context) ([]opendota.HeroStat, error)
	}
	PatchSource interface {
		Patch() (dota.Patch, bool)
	}
	Store interface {
		ReplaceMetaPositions(ctx context.Context, patch string, rows []store.MetaRow) error
		MetaPositions(ctx context.Context, patch string) ([]store.MetaRow, time.Time, error)
		LatestMetaPatch(ctx context.Context) (string, error)
		GetKV(ctx context.Context, key string) ([]byte, time.Time, error)
		PutKV(ctx context.Context, key string, value []byte) error
	}
)

type Deps struct {
	Stratz   StratzQuerier
	OpenDota HeroStatsSource
	Tags     *dota.HeroTagSet
	Store    Store
	Patch    PatchSource
	// Bracket — META_BRACKET: ALL или список брекетов через запятую (DIVINE,IMMORTAL)
	Bracket    string
	MinMatches int
	Logger     *slog.Logger
}

type Service struct {
	d              Deps
	stratzBrackets []string
	odBrackets     []int
	snap           atomic.Pointer[snapshotEntry]
	now            func() time.Time
}

func New(d Deps) (*Service, error) {
	sb, ob, err := ParseBrackets(d.Bracket)
	if err != nil {
		return nil, err
	}
	return &Service{d: d, stratzBrackets: sb, odBrackets: ob, now: time.Now}, nil
}

// Номера брекетов OpenDota (/heroStats: 1_pick … 8_pick).
var openDotaBrackets = map[string]int{
	"HERALD": 1, "GUARDIAN": 2, "CRUSADER": 3, "ARCHON": 4,
	"LEGEND": 5, "ANCIENT": 6, "DIVINE": 7, "IMMORTAL": 8,
}

// ParseBrackets разбирает META_BRACKET. ALL (или пусто) — все ранги: фильтр не нужен.
func ParseBrackets(raw string) (stratz []string, od []int, err error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" || raw == "ALL" {
		return nil, nil, nil
	}
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		n, ok := openDotaBrackets[name]
		if !ok {
			return nil, nil, fmt.Errorf("META_BRACKET: неизвестный брекет %q (допустимо ALL или HERALD…IMMORTAL через запятую)", name)
		}
		stratz = append(stratz, name)
		od = append(od, n)
	}
	return stratz, od, nil
}

// HeroStat — герой в топе позиции.
type HeroStat struct {
	HeroID  int     `json:"hero_id"`
	Winrate float64 `json:"winrate"`
	Matches int     `json:"matches"`
}

// Snapshot — мета патча, готовая к выдаче.
type Snapshot struct {
	Patch     dota.Patch
	Source    Source
	UpdatedAt time.Time
	// ByPosition[i] — герои позиции i+1 по убыванию винрейта, не меньше MinMatches матчей
	ByPosition [5][]HeroStat
}

type snapshotEntry struct {
	snap     *Snapshot
	loadedAt time.Time
}

// Snapshot — текущая мета (из БД, кэш в памяти на 5 минут).
func (s *Service) Snapshot(ctx context.Context) (*Snapshot, error) {
	if e := s.snap.Load(); e != nil && s.now().Sub(e.loadedAt) < snapshotTTL {
		return e.snap, nil
	}
	patch, ok := s.d.Patch.Patch()
	if !ok {
		return nil, ErrNotReady
	}
	rows, updatedAt, err := s.d.Store.MetaPositions(ctx, patch.Number)
	if errors.Is(err, store.ErrNotFound) {
		// по новому патчу ещё не собрали — показываем последнюю мету
		prev, perr := s.d.Store.LatestMetaPatch(ctx)
		if perr != nil {
			if errors.Is(perr, store.ErrNotFound) {
				return nil, ErrNotReady
			}
			return nil, perr
		}
		patch = dota.Patch{Number: prev, Label: prev}
		rows, updatedAt, err = s.d.Store.MetaPositions(ctx, prev)
	}
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{Patch: patch, Source: s.loadSource(ctx, patch.Number), UpdatedAt: updatedAt}
	var positionTotal [5]int
	heroTotal := map[int]int{}
	for _, r := range rows {
		if r.Position >= 1 && r.Position <= 5 {
			positionTotal[r.Position-1] += r.Matches
			heroTotal[r.HeroID] += r.Matches
		}
	}
	for _, r := range rows {
		if r.Position < 1 || r.Position > 5 || r.Matches < s.d.MinMatches || r.Matches == 0 {
			continue
		}
		if float64(r.Matches) < minPositionShare*float64(positionTotal[r.Position-1]) ||
			float64(r.Matches) < minRoleShare*float64(heroTotal[r.HeroID]) {
			continue
		}
		snap.ByPosition[r.Position-1] = append(snap.ByPosition[r.Position-1], HeroStat{
			HeroID:  r.HeroID,
			Winrate: float64(r.Wins) / float64(r.Matches),
			Matches: r.Matches,
		})
	}
	for i := range snap.ByPosition {
		list := snap.ByPosition[i]
		sort.Slice(list, func(a, b int) bool {
			if list[a].Winrate != list[b].Winrate {
				return list[a].Winrate > list[b].Winrate
			}
			return list[a].Matches > list[b].Matches
		})
	}
	s.snap.Store(&snapshotEntry{snap: snap, loadedAt: s.now()})
	return snap, nil
}

// Refresh собирает мету и сохраняет её для текущего патча.
func (s *Service) Refresh(ctx context.Context) error {
	patch, ok := s.d.Patch.Patch()
	if !ok {
		return errors.New("meta: patch is unknown yet")
	}
	rows, source, err := s.collect(ctx)
	if err != nil {
		return err
	}
	if err := s.d.Store.ReplaceMetaPositions(ctx, patch.Number, rows); err != nil {
		return err
	}
	raw, _ := json.Marshal(string(source))
	if err := s.d.Store.PutKV(ctx, sourceKey(patch.Number), raw); err != nil {
		s.d.Logger.Warn("meta: save source", "err", err)
	}
	if err := s.d.Store.PutKV(ctx, formatKey(patch.Number), []byte(strconv.Quote(formatVersion))); err != nil {
		s.d.Logger.Warn("meta: save format", "err", err)
	}
	s.snap.Store(nil)
	s.d.Logger.Info("meta refreshed", "patch", patch.Number, "source", source, "rows", len(rows))
	return nil
}

// RefreshIfStale — для джобы: после перезапуска не пересобирать свежую мету.
func (s *Service) RefreshIfStale(ctx context.Context) error {
	if patch, ok := s.d.Patch.Patch(); ok {
		_, updatedAt, err := s.d.Store.MetaPositions(ctx, patch.Number)
		fresh := err == nil && s.now().Sub(updatedAt) < RefreshEvery-10*time.Minute && s.formatCurrent(ctx, patch.Number)
		// STRATZ появился (токен заработал) — не ждём, пока устареет мета из OpenDota
		upgrade := s.d.Stratz.Available() && s.loadSource(ctx, patch.Number) != SourceStratz
		if fresh && !upgrade {
			return nil
		}
	}
	return s.Refresh(ctx)
}

func (s *Service) collect(ctx context.Context) ([]store.MetaRow, Source, error) {
	if s.d.Stratz.Available() {
		rows, err := s.fromStratz(ctx)
		if err == nil {
			return rows, SourceStratz, nil
		}
		s.d.Logger.Warn("meta: STRATZ failed, using OpenDota", "err", err)
	}
	rows, err := s.fromOpenDota(ctx)
	if err != nil {
		return nil, "", err
	}
	return rows, SourceOpenDota, nil
}

func sourceKey(patch string) string { return "meta:source:" + patch }
func formatKey(patch string) string { return "meta:format:" + patch }

func (s *Service) formatCurrent(ctx context.Context, patch string) bool {
	raw, _, err := s.d.Store.GetKV(ctx, formatKey(patch))
	if err != nil {
		return false
	}
	var v string
	return json.Unmarshal(raw, &v) == nil && v == formatVersion
}

func (s *Service) loadSource(ctx context.Context, patch string) Source {
	raw, _, err := s.d.Store.GetKV(ctx, sourceKey(patch))
	if err != nil {
		return SourceOpenDota
	}
	var src string
	if json.Unmarshal(raw, &src) != nil || src != string(SourceStratz) {
		return SourceOpenDota
	}
	return SourceStratz
}

// Винрейт по каждой позиции за текущую версию игры в рейтинговых матчах.
// Схема сверена со сгенерированными моделями STRATZ: heroStats.winGameVersion
// (positionIds: [MatchPlayerPositionType], bracketIds: [RankBracket], gameModeIds: [GameModeEnumType]).
const stratzMetaQuery = `query Meta($brackets: [RankBracket]) {
  heroStats {
    p1: winGameVersion(positionIds: [POSITION_1], bracketIds: $brackets, gameModeIds: [ALL_PICK_RANKED], take: 1) { gameVersionId heroId matchCount winCount }
    p2: winGameVersion(positionIds: [POSITION_2], bracketIds: $brackets, gameModeIds: [ALL_PICK_RANKED], take: 1) { gameVersionId heroId matchCount winCount }
    p3: winGameVersion(positionIds: [POSITION_3], bracketIds: $brackets, gameModeIds: [ALL_PICK_RANKED], take: 1) { gameVersionId heroId matchCount winCount }
    p4: winGameVersion(positionIds: [POSITION_4], bracketIds: $brackets, gameModeIds: [ALL_PICK_RANKED], take: 1) { gameVersionId heroId matchCount winCount }
    p5: winGameVersion(positionIds: [POSITION_5], bracketIds: $brackets, gameModeIds: [ALL_PICK_RANKED], take: 1) { gameVersionId heroId matchCount winCount }
  }
}`

type stratzWin struct {
	GameVersionID int `json:"gameVersionId"`
	HeroID        int `json:"heroId"`
	MatchCount    int `json:"matchCount"`
	WinCount      int `json:"winCount"`
}

func (s *Service) fromStratz(ctx context.Context) ([]store.MetaRow, error) {
	vars := map[string]any{}
	if len(s.stratzBrackets) > 0 {
		vars["brackets"] = s.stratzBrackets
	}
	var out struct {
		HeroStats map[string][]stratzWin `json:"heroStats"`
	}
	if err := s.d.Stratz.Query(ctx, stratzMetaQuery, vars, &out); err != nil {
		return nil, err
	}

	var rows []store.MetaRow
	for pos := 1; pos <= 5; pos++ {
		list := out.HeroStats[fmt.Sprintf("p%d", pos)]
		latest := 0
		for _, w := range list {
			latest = max(latest, w.GameVersionID)
		}
		// строки могут быть разбиты по длительности матча — складываем по герою
		agg := map[int]*store.MetaRow{}
		for _, w := range list {
			if w.GameVersionID != latest || w.HeroID <= 0 {
				continue
			}
			r, ok := agg[w.HeroID]
			if !ok {
				r = &store.MetaRow{Position: pos, HeroID: w.HeroID}
				agg[w.HeroID] = r
			}
			r.Matches += w.MatchCount
			r.Wins += w.WinCount
		}
		if len(agg) < minHeroesPerPosition {
			return nil, fmt.Errorf("stratz: position %d has only %d heroes", pos, len(agg))
		}
		for _, r := range agg {
			rows = append(rows, *r)
		}
	}
	return rows, nil
}

// fromOpenDota — фоллбэк: общий винрейт героя на его основной позиции из heroes_tags.json.
// Только основной: общий винрейт один на всех позициях героя, и без этого один и тот же
// герой с одинаковым процентом попадал бы в топ нескольких позиций сразу.
func (s *Service) fromOpenDota(ctx context.Context) ([]store.MetaRow, error) {
	stats, err := s.d.OpenDota.HeroStats(ctx)
	if err != nil {
		return nil, err
	}
	var rows []store.MetaRow
	for _, st := range stats {
		tags, ok := s.d.Tags.Get(st.ID)
		if !ok {
			continue
		}
		picks, wins := s.openDotaPicks(st)
		if picks == 0 {
			continue
		}
		if len(tags.Positions) == 0 {
			continue
		}
		rows = append(rows, store.MetaRow{Position: tags.Positions[0], HeroID: st.ID, Matches: picks, Wins: wins})
	}
	if len(rows) == 0 {
		return nil, errors.New("opendota: empty hero stats")
	}
	return rows, nil
}

func (s *Service) openDotaPicks(st opendota.HeroStat) (picks, wins int) {
	if len(s.odBrackets) == 0 {
		return st.Bracket(0)
	}
	for _, b := range s.odBrackets {
		p, w := st.Bracket(b)
		picks += p
		wins += w
	}
	return picks, wins
}
