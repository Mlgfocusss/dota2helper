// Package heropage — данные страницы героя: винрейт по позициям, матчапы, союзники, предметы (SPEC 4.3).
package heropage

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/meta"
	"d2helper/internal/store"
)

const (
	pageTTL = 30 * time.Minute
	// сколько героев в списках «силён против», «слаб против», «союзники» и предметов на стадию
	listSize  = 5
	itemsSize = 6
	// пары с меньшим числом матчей слишком шумные для рейтинга
	minPairMatches = 200
	itemsTTL       = 24 * time.Hour
)

// ErrNotFound — такого героя нет.
var ErrNotFound = errors.New("heropage: hero not found")

type (
	HeroSource interface {
		Hero(id int) (dota.Hero, bool)
	}
	MetaSource interface {
		Snapshot(ctx context.Context) (*meta.Snapshot, error)
	}
	TagSource interface {
		Get(heroID int) (dota.HeroTags, bool)
	}
	OpenDotaSource interface {
		HeroItemPopularity(ctx context.Context, heroID int) (*opendota.ItemPopularity, error)
		ItemConstants(ctx context.Context) (map[string]opendota.ItemConstant, error)
	}
	Store interface {
		HeroPositionRows(ctx context.Context, patch string, heroID int) ([]store.MetaRow, error)
		HeroMatchupsFor(ctx context.Context, heroID int, kind string, minMatches int) ([]store.MatchupRow, error)
	}
)

type Deps struct {
	Heroes   HeroSource
	Meta     MetaSource
	Tags     TagSource
	OpenDota OpenDotaSource
	Store    Store
	// MinMatches — минимум матчей на позиции для показа (META_MIN_MATCHES)
	MinMatches int
	Logger     *slog.Logger
}

type PositionStat struct {
	Position int     `json:"position"`
	Matches  int     `json:"matches"`
	Winrate  float64 `json:"winrate"`
}

type MatchupHero struct {
	HeroID int     `json:"hero_id"`
	Value  float64 `json:"value"`
}

type ItemRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type Page struct {
	HeroID     int `json:"hero_id"`
	Difficulty int `json:"difficulty"`
	// ByPosition = false: STRATZ недоступен, винрейт — общий на основной позиции героя
	ByPosition    bool                 `json:"by_position"`
	Positions     []PositionStat       `json:"positions"`
	StrongAgainst []MatchupHero        `json:"strong_against"`
	WeakAgainst   []MatchupHero        `json:"weak_against"`
	Allies        []MatchupHero        `json:"allies"`
	Items         map[string][]ItemRef `json:"items"`
}

type Service struct {
	d   Deps
	now func() time.Time

	mu    sync.Mutex
	pages map[int]pageEntry

	itemsMu sync.Mutex
	itemIdx map[int]ItemRef
	itemsAt time.Time
}

type pageEntry struct {
	page *Page
	at   time.Time
}

func New(d Deps) *Service {
	return &Service{d: d, now: time.Now, pages: map[int]pageEntry{}}
}

// Page собирает страницу героя. Недоступность отдельных источников (меты, предметов)
// не ломает страницу: соответствующий блок просто пустой.
func (s *Service) Page(ctx context.Context, heroID int) (*Page, error) {
	if _, ok := s.d.Heroes.Hero(heroID); !ok {
		return nil, ErrNotFound
	}
	s.mu.Lock()
	if e, ok := s.pages[heroID]; ok && s.now().Sub(e.at) < pageTTL {
		s.mu.Unlock()
		return e.page, nil
	}
	s.mu.Unlock()

	p := &Page{
		HeroID:        heroID,
		Positions:     []PositionStat{},
		StrongAgainst: []MatchupHero{},
		WeakAgainst:   []MatchupHero{},
		Allies:        []MatchupHero{},
		Items:         map[string][]ItemRef{"start": {}, "early": {}, "mid": {}, "late": {}},
	}
	if tags, ok := s.d.Tags.Get(heroID); ok {
		p.Difficulty = tags.Complexity
	}

	if snap, err := s.d.Meta.Snapshot(ctx); err == nil {
		p.ByPosition = snap.Source == meta.SourceStratz
		rows, err := s.d.Store.HeroPositionRows(ctx, snap.Patch.Number, heroID)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if r.Matches >= s.d.MinMatches && r.Matches > 0 && r.Position >= 1 && r.Position <= 5 {
				p.Positions = append(p.Positions, PositionStat{
					Position: r.Position, Matches: r.Matches, Winrate: float64(r.Wins) / float64(r.Matches),
				})
			}
		}
	} else if !errors.Is(err, meta.ErrNotReady) {
		return nil, err
	}

	var err error
	if p.StrongAgainst, p.WeakAgainst, err = s.versus(ctx, heroID); err != nil {
		return nil, err
	}
	if p.Allies, err = s.allies(ctx, heroID); err != nil {
		return nil, err
	}
	s.fillItems(ctx, heroID, p)

	s.mu.Lock()
	s.pages[heroID] = pageEntry{page: p, at: s.now()}
	s.mu.Unlock()
	return p, nil
}

func toHeroes(rows []store.MatchupRow, n int) []MatchupHero {
	out := make([]MatchupHero, 0, min(len(rows), n))
	for i := 0; i < len(rows) && i < n; i++ {
		out = append(out, MatchupHero{HeroID: rows[i].OtherID, Value: math.Round(float64(rows[i].Advantage)*10) / 10})
	}
	return out
}

// versus — кого герой побеждает чаще ожидаемого и кому проигрывает.
func (s *Service) versus(ctx context.Context, heroID int) (strong, weak []MatchupHero, err error) {
	rows, err := s.d.Store.HeroMatchupsFor(ctx, heroID, "vs", minPairMatches)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Advantage > rows[j].Advantage })
	var pos, neg []store.MatchupRow
	for _, r := range rows {
		switch {
		case r.Advantage > 0:
			pos = append(pos, r)
		case r.Advantage < 0:
			neg = append(neg, r)
		}
	}
	// слабые места — с самого отрицательного
	for i, j := 0, len(neg)-1; i < j; i, j = i+1, j-1 {
		neg[i], neg[j] = neg[j], neg[i]
	}
	return toHeroes(pos, listSize), toHeroes(neg, listSize), nil
}

func (s *Service) allies(ctx context.Context, heroID int) ([]MatchupHero, error) {
	rows, err := s.d.Store.HeroMatchupsFor(ctx, heroID, "with", minPairMatches)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Advantage > rows[j].Advantage })
	var pos []store.MatchupRow
	for _, r := range rows {
		if r.Advantage > 0 {
			pos = append(pos, r)
		}
	}
	return toHeroes(pos, listSize), nil
}

func (s *Service) fillItems(ctx context.Context, heroID int, p *Page) {
	pop, err := s.d.OpenDota.HeroItemPopularity(ctx, heroID)
	if err != nil {
		s.d.Logger.Warn("hero page: item popularity", "hero_id", heroID, "err", err)
		return
	}
	idx := s.itemIndex(ctx)
	if len(idx) == 0 {
		return
	}
	p.Items["start"] = topItems(pop.Start, idx)
	p.Items["early"] = topItems(pop.Early, idx)
	p.Items["mid"] = topItems(pop.Mid, idx)
	p.Items["late"] = topItems(pop.Late, idx)
}

// topItems — самые покупаемые предметы стадии; рецепты и неизвестные id пропускаются.
func topItems(counts map[string]int, idx map[int]ItemRef) []ItemRef {
	type kv struct {
		id, n int
	}
	list := make([]kv, 0, len(counts))
	for k, n := range counts {
		if id, err := strconv.Atoi(k); err == nil {
			list = append(list, kv{id, n})
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].id < list[j].id
	})
	out := make([]ItemRef, 0, itemsSize)
	for _, e := range list {
		it, ok := idx[e.id]
		if !ok || strings.HasPrefix(it.Key, "recipe_") {
			continue
		}
		out = append(out, it)
		if len(out) == itemsSize {
			break
		}
	}
	return out
}

// itemIndex — справочник предметов по id; при недоступности OpenDota блок предметов пустой.
func (s *Service) itemIndex(ctx context.Context) map[int]ItemRef {
	s.itemsMu.Lock()
	defer s.itemsMu.Unlock()
	if s.itemIdx != nil && s.now().Sub(s.itemsAt) < itemsTTL {
		return s.itemIdx
	}
	consts, err := s.d.OpenDota.ItemConstants(ctx)
	if err != nil {
		s.d.Logger.Warn("hero page: item constants", "err", err)
		return s.itemIdx
	}
	idx := make(map[int]ItemRef, len(consts))
	for key, it := range consts {
		idx[it.ID] = ItemRef{Key: key, Name: it.DName}
	}
	s.itemIdx, s.itemsAt = idx, s.now()
	return idx
}
