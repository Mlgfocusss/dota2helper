package heropage

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/meta"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeHeroes struct{}

func (fakeHeroes) Hero(id int) (dota.Hero, bool) { return dota.Hero{ID: id}, id > 0 && id < 100 }

type fakeMeta struct {
	snap *meta.Snapshot
	err  error
}

func (f fakeMeta) Snapshot(context.Context) (*meta.Snapshot, error) { return f.snap, f.err }

type fakeTags struct{}

func (fakeTags) Get(id int) (dota.HeroTags, bool) {
	return dota.HeroTags{HeroID: id, Complexity: 2}, true
}

type fakeOD struct {
	pop     *opendota.ItemPopularity
	popErr  error
	consts  map[string]opendota.ItemConstant
	popHits int
}

func (f *fakeOD) HeroItemPopularity(context.Context, int) (*opendota.ItemPopularity, error) {
	f.popHits++
	return f.pop, f.popErr
}

func (f *fakeOD) ItemConstants(context.Context) (map[string]opendota.ItemConstant, error) {
	return f.consts, nil
}

type fakeStore struct {
	positions []store.MetaRow
	vs, with  []store.MatchupRow
}

func (f fakeStore) HeroPositionRows(context.Context, string, int) ([]store.MetaRow, error) {
	return f.positions, nil
}

func (f fakeStore) HeroMatchupsFor(_ context.Context, _ int, kind string, min int) ([]store.MatchupRow, error) {
	src := f.vs
	if kind == "with" {
		src = f.with
	}
	var out []store.MatchupRow
	for _, r := range src {
		if r.Matches >= min {
			out = append(out, r)
		}
	}
	return out, nil
}

func newService(od *fakeOD, st fakeStore, m fakeMeta) *Service {
	return New(Deps{Heroes: fakeHeroes{}, Meta: m, Tags: fakeTags{}, OpenDota: od, Store: st, MinMatches: 200, Logger: quiet})
}

func matchup(other int, matches int, adv float32) store.MatchupRow {
	return store.MatchupRow{OtherID: other, Matches: matches, Advantage: adv}
}

func TestPage(t *testing.T) {
	od := &fakeOD{
		pop: &opendota.ItemPopularity{
			Start: map[string]int{"1": 50, "2": 90},
			Mid:   map[string]int{"3": 10, "4": 70, "5": 70, "6": 5, "7": 1, "8": 1, "9": 1, "10": 1},
		},
		consts: map[string]opendota.ItemConstant{
			"tango": {ID: 1, DName: "Tango"}, "branches": {ID: 2, DName: "Iron Branch"},
			"blink": {ID: 3, DName: "Blink Dagger"}, "bkb": {ID: 4, DName: "Black King Bar"},
			"recipe_bkb": {ID: 5, DName: "Recipe"},
		},
	}
	st := fakeStore{
		positions: []store.MetaRow{
			{Position: 1, Matches: 5000, Wins: 2600},
			{Position: 3, Matches: 199, Wins: 120}, // меньше порога
			{Position: 2, Matches: 300, Wins: 150},
		},
		vs: []store.MatchupRow{
			matchup(10, 500, 5), matchup(11, 500, 9), matchup(12, 500, -2), matchup(13, 500, -8),
			matchup(14, 50, 20), // мало матчей — шум
			matchup(15, 500, 0), // ровный матчап не попадает ни в один список
			matchup(16, 500, 1), matchup(17, 500, 2), matchup(18, 500, 3), matchup(19, 500, 4),
		},
		with: []store.MatchupRow{matchup(20, 500, 3), matchup(21, 500, 7), matchup(22, 500, -4)},
	}
	svc := newService(od, st, fakeMeta{snap: &meta.Snapshot{Source: meta.SourceStratz, Patch: dota.Patch{Number: "7.41"}}})

	p, err := svc.Page(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if p.Difficulty != 2 || !p.ByPosition {
		t.Errorf("header: %+v", p)
	}
	if len(p.Positions) != 2 || p.Positions[0].Position != 1 || p.Positions[0].Winrate != 0.52 {
		t.Errorf("positions below the threshold must be hidden: %+v", p.Positions)
	}
	if len(p.StrongAgainst) != 5 || p.StrongAgainst[0].HeroID != 11 || p.StrongAgainst[1].HeroID != 10 {
		t.Errorf("strong against: %+v", p.StrongAgainst)
	}
	for _, h := range p.StrongAgainst {
		if h.HeroID == 14 || h.HeroID == 15 || h.Value <= 0 {
			t.Errorf("noise or zero matchup in strong list: %+v", h)
		}
	}
	if len(p.WeakAgainst) != 2 || p.WeakAgainst[0].HeroID != 13 || p.WeakAgainst[0].Value != -8 {
		t.Errorf("weak against starts with the worst: %+v", p.WeakAgainst)
	}
	if len(p.Allies) != 2 || p.Allies[0].HeroID != 21 {
		t.Errorf("allies: %+v", p.Allies)
	}

	if got := p.Items["start"]; len(got) != 2 || got[0].Key != "branches" || got[1].Name != "Tango" {
		t.Errorf("start items by popularity: %+v", got)
	}
	// рецепт отфильтрован, неизвестные id пропущены
	if got := p.Items["mid"]; len(got) != 2 || got[0].Key != "bkb" || got[1].Key != "blink" {
		t.Errorf("mid items: %+v", got)
	}
	if len(p.Items["early"]) != 0 || len(p.Items["late"]) != 0 {
		t.Errorf("empty stages must stay empty lists: %+v", p.Items)
	}

	// повторный запрос берётся из кэша
	if _, err := svc.Page(context.Background(), 5); err != nil || od.popHits != 1 {
		t.Errorf("page must be cached, OpenDota hits: %d", od.popHits)
	}
}

func TestPageSurvivesMissingSources(t *testing.T) {
	// нет меты и OpenDota недоступна: страница всё равно собирается
	od := &fakeOD{popErr: errors.New("down")}
	svc := newService(od, fakeStore{vs: []store.MatchupRow{matchup(10, 500, 3)}}, fakeMeta{err: meta.ErrNotReady})
	p, err := svc.Page(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Positions) != 0 || p.ByPosition || len(p.StrongAgainst) != 1 {
		t.Errorf("degraded page: %+v", p)
	}
	for stage, list := range p.Items {
		if list == nil || len(list) != 0 {
			t.Errorf("stage %s must be an empty list, got %v", stage, list)
		}
	}
}

func TestPageUnknownHero(t *testing.T) {
	svc := newService(&fakeOD{}, fakeStore{}, fakeMeta{err: meta.ErrNotReady})
	if _, err := svc.Page(context.Background(), 500); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown hero: %v", err)
	}
	// ошибка меты, отличная от «не собрана», — это ошибка страницы
	svc = newService(&fakeOD{}, fakeStore{}, fakeMeta{err: errors.New("db down")})
	if _, err := svc.Page(context.Background(), 5); err == nil {
		t.Error("meta failure must surface")
	}
}
