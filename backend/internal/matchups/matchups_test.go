package matchups

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestBuildMatrix(t *testing.T) {
	rows := []store.MatchupRow{
		{HeroID: 5, OtherID: 2, Kind: "vs", Advantage: 3.5},
		{HeroID: 2, OtherID: 5, Kind: "vs", Advantage: -3.4},
		{HeroID: 5, OtherID: 2, Kind: "with", Advantage: 1.2},
		{HeroID: 99, OtherID: 2, Kind: "vs", Advantage: 7}, // героя нет в справочнике
	}
	m := BuildMatrix([]int{5, 2, 9}, rows, time.Now())

	if len(m.IDs) != 3 || m.IDs[0] != 2 || m.IDs[2] != 9 {
		t.Fatalf("ids must be sorted: %v", m.IDs)
	}
	if got := m.Vs(5, 2); got < 3.49 || got > 3.51 {
		t.Errorf("vs(5,2) = %v", got)
	}
	if got := m.Vs(2, 5); got > -3.39 || got < -3.41 {
		t.Errorf("vs(2,5) = %v", got)
	}
	if !m.HasWith || m.Synergy(5, 2) < 1.19 || m.Synergy(2, 5) != 0 {
		t.Errorf("synergy: has=%v %v %v", m.HasWith, m.Synergy(5, 2), m.Synergy(2, 5))
	}
	if m.Vs(99, 2) != 0 || m.Vs(5, 99) != 0 || m.Has(99) || !m.Has(9) {
		t.Error("unknown heroes must give zero and not be in the matrix")
	}
	if BuildMatrix([]int{1, 2}, []store.MatchupRow{{HeroID: 1, OtherID: 2, Kind: "vs", Advantage: 1}}, time.Now()).HasWith {
		t.Error("no with rows → HasWith must be false")
	}
}

func TestRowClampsAndZeroesNoise(t *testing.T) {
	if got := row(1, 2, "vs", 10, 8); got.Advantage != 0 {
		t.Errorf("few matches must give 0, got %v", got.Advantage)
	}
	if got := row(1, 2, "vs", 1000, 99); got.Advantage != maxAdvantage {
		t.Errorf("outlier must be clamped, got %v", got.Advantage)
	}
	if got := row(1, 2, "with", 1000, -99); got.Advantage != -maxAdvantage {
		t.Errorf("negative outlier: %v", got.Advantage)
	}
	if got := row(1, 2, "vs", 1000, 2.5); got.Advantage != 2.5 || got.Matches != 1000 {
		t.Errorf("normal value must stay: %+v", got)
	}
}

// ----- сбор -----------------------------------------------------------------

type fakeStratz struct {
	available bool
	err       error
	// heroes, по которым ответ непустой
	covered map[int]bool
	queries int
}

func (f *fakeStratz) Available() bool { return f.available }

func (f *fakeStratz) Query(_ context.Context, query string, _ map[string]any, out any) error {
	f.queries++
	if f.err != nil {
		return f.err
	}
	// достаём id героев из алиасов h<ID>
	type hero = stratzHero
	resp := struct {
		HeroStats map[string]hero `json:"heroStats"`
	}{HeroStats: map[string]hero{}}
	for id := 1; id <= 20; id++ {
		if !contains(query, fmt.Sprintf("h%d: ", id)) || (f.covered != nil && !f.covered[id]) {
			continue
		}
		h := hero{}
		h.Advantage = append(h.Advantage, struct {
			HeroID int          `json:"heroId"`
			Vs     []stratzPair `json:"vs"`
			With   []stratzPair `json:"with"`
		}{
			HeroID: id,
			Vs: []stratzPair{
				{HeroID2: id%10 + 1, MatchCount: 500, Synergy: 4},
				{HeroID2: id, MatchCount: 500, Synergy: 9},  // сам с собой — пропускаем
				{HeroID2: 999, MatchCount: 500, Synergy: 9}, // неизвестный герой
			},
			With: []stratzPair{{HeroID2: (id+1)%10 + 1, MatchCount: 10, Synergy: 5}}, // мало матчей → 0
		})
		resp.HeroStats[fmt.Sprintf("h%d", id)] = h
	}
	o := out.(*struct {
		HeroStats map[string]stratzHero `json:"heroStats"`
	})
	o.HeroStats = resp.HeroStats
	return nil
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type fakeOD struct {
	stats    []opendota.HeroStat
	matchups map[int][]opendota.HeroMatchup
	calls    int
}

func (f *fakeOD) HeroStats(context.Context) ([]opendota.HeroStat, error) { return f.stats, nil }

func (f *fakeOD) HeroMatchups(_ context.Context, id int) ([]opendota.HeroMatchup, error) {
	f.calls++
	return f.matchups[id], nil
}

type fakeHeroes []dota.Hero

func (f fakeHeroes) Heroes() ([]dota.Hero, bool) { return f, len(f) > 0 }

type fakeStore struct {
	rows      []store.MatchupRow
	updatedAt time.Time
	kv        map[string][]byte
}

func (f *fakeStore) ReplaceHeroMatchups(_ context.Context, rows []store.MatchupRow) error {
	f.rows, f.updatedAt = rows, time.Now()
	return nil
}

func (f *fakeStore) HeroMatchups(context.Context) ([]store.MatchupRow, time.Time, error) {
	if len(f.rows) == 0 {
		return nil, time.Time{}, store.ErrNotFound
	}
	return f.rows, f.updatedAt, nil
}

func (f *fakeStore) GetKV(_ context.Context, key string) ([]byte, time.Time, error) {
	v, ok := f.kv[key]
	if !ok {
		return nil, time.Time{}, store.ErrNotFound
	}
	return v, time.Now(), nil
}

func (f *fakeStore) PutKV(_ context.Context, key string, v []byte) error {
	if f.kv == nil {
		f.kv = map[string][]byte{}
	}
	f.kv[key] = v
	return nil
}

func heroes(n int) fakeHeroes {
	out := make(fakeHeroes, n)
	for i := range out {
		out[i] = dota.Hero{ID: i + 1}
	}
	return out
}

func TestRefreshFromStratz(t *testing.T) {
	st := &fakeStore{}
	svc := New(Deps{Stratz: &fakeStratz{available: true}, OpenDota: &fakeOD{}, Heroes: heroes(10), Store: st, Logger: quiet})
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	var vs, with int
	for _, r := range st.rows {
		if r.OtherID == r.HeroID || r.OtherID > 10 {
			t.Fatalf("bad row kept: %+v", r)
		}
		switch r.Kind {
		case "vs":
			vs++
			if r.Advantage != 4 {
				t.Errorf("vs advantage: %+v", r)
			}
		case "with":
			with++
			if r.Advantage != 0 { // 10 матчей — шум
				t.Errorf("noisy synergy must be zeroed: %+v", r)
			}
		}
	}
	if vs != 10 || with != 10 {
		t.Errorf("rows: vs=%d with=%d", vs, with)
	}
	if svc.loadSource(context.Background()) != SourceStratz {
		t.Error("source must be remembered")
	}

	m, err := svc.Matrix(context.Background())
	if err != nil || !m.HasWith || m.Vs(1, 2) != 4 {
		t.Fatalf("matrix after refresh: %v %v", m, err)
	}
}

func TestRefreshFallsBackToOpenDota(t *testing.T) {
	st := &fakeStore{}
	od := &fakeOD{
		stats: []opendota.HeroStat{{ID: 1, PubPick: 1000, PubWin: 500}, {ID: 2, PubPick: 1000, PubWin: 500}, {ID: 3, PubPick: 1000, PubWin: 500}},
		matchups: map[int][]opendota.HeroMatchup{
			1: {{HeroID: 2, GamesPlayed: 200, Wins: 120}, {HeroID: 3, GamesPlayed: 20, Wins: 20}, {HeroID: 1, GamesPlayed: 5, Wins: 5}},
			2: {{HeroID: 1, GamesPlayed: 200, Wins: 80}},
			3: {{HeroID: 1, GamesPlayed: 100, Wins: 50}},
		},
	}
	// STRATZ упал — работаем через OpenDota
	svc := New(Deps{Stratz: &fakeStratz{available: true, err: errors.New("boom")}, OpenDota: od, Heroes: heroes(3), Store: st, Logger: quiet})
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	m, err := svc.Matrix(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// 120/200 = 60% против общего 50% → +10 п. п.
	if got := m.Vs(1, 2); got < 9.99 || got > 10.01 {
		t.Errorf("vs(1,2) = %v, want +10", got)
	}
	if got := m.Vs(1, 3); got != 0 { // 20 игр — мало
		t.Errorf("few games must give 0, got %v", got)
	}
	if m.HasWith {
		t.Error("OpenDota has no synergy data")
	}
	if svc.loadSource(context.Background()) != SourceOpenDota {
		t.Error("source must be opendota")
	}
}

func TestRefreshRejectsThinStratzAnswer(t *testing.T) {
	st := &fakeStore{}
	// STRATZ ответил по двум героям из десяти, OpenDota тоже ничего не дала — ничего не сохраняем
	svc := New(Deps{
		Stratz:   &fakeStratz{available: true, covered: map[int]bool{1: true, 2: true}},
		OpenDota: &fakeOD{}, Heroes: heroes(10), Store: st, Logger: quiet,
	})
	if err := svc.Refresh(context.Background()); err == nil {
		t.Fatal("thin data must be an error")
	}
	if len(st.rows) != 0 {
		t.Error("table must stay untouched on failure")
	}
	if _, err := svc.Matrix(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Errorf("matrix: %v", err)
	}
}

func TestRefreshIfStale(t *testing.T) {
	st := &fakeStore{}
	stratz := &fakeStratz{available: true}
	svc := New(Deps{Stratz: stratz, OpenDota: &fakeOD{}, Heroes: heroes(10), Store: st, Logger: quiet})
	ctx := context.Background()

	if err := svc.RefreshIfStale(ctx); err != nil || len(st.rows) == 0 {
		t.Fatalf("first run must refresh: %v", err)
	}
	queries := stratz.queries
	if err := svc.RefreshIfStale(ctx); err != nil || stratz.queries != queries {
		t.Error("fresh data must not be refreshed again")
	}

	// данные из OpenDota, а STRATZ уже доступен — обновляем раньше срока ради синергий
	st.kv[sourceKey] = []byte(`"opendota"`)
	if err := svc.RefreshIfStale(ctx); err != nil || stratz.queries == queries {
		t.Error("STRATZ became available: must refresh early")
	}

	// устаревшие данные обновляются
	st.updatedAt = time.Now().Add(-RefreshEvery)
	queries = stratz.queries
	if err := svc.RefreshIfStale(ctx); err != nil || stratz.queries == queries {
		t.Error("stale data must be refreshed")
	}
}
