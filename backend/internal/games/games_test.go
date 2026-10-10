package games

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeStratz struct {
	available bool
	response  string
	err       error
}

func (f *fakeStratz) Available() bool { return f.available }

func (f *fakeStratz) Query(_ context.Context, _ string, _ map[string]any, out any) error {
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.response), out)
}

type fakeOD struct{ stats []opendota.HeroStat }

func (f fakeOD) HeroStats(context.Context) ([]opendota.HeroStat, error) { return f.stats, nil }

type memStore struct {
	snaps     map[string][]store.HLRow
	updatedAt time.Time
	records   map[string]int
	plays     int
}

func newMemStore() *memStore {
	return &memStore{snaps: map[string][]store.HLRow{}, records: map[string]int{}}
}

func (m *memStore) ReplaceHLSnapshot(_ context.Context, mode string, rows []store.HLRow) error {
	m.snaps[mode] = rows
	m.updatedAt = time.Now()
	return nil
}

func (m *memStore) HLSnapshot(_ context.Context, mode string) ([]store.HLRow, time.Time, error) {
	rows, ok := m.snaps[mode]
	if !ok {
		return nil, time.Time{}, store.ErrNotFound
	}
	return rows, m.updatedAt, nil
}

func (m *memStore) UpsertGameRecord(_ context.Context, _ int64, _, mode string, score int) (int, error) {
	m.plays++
	m.records[mode] = max(m.records[mode], score)
	return m.records[mode], nil
}

func (m *memStore) GameRecords(context.Context, int64, string) (map[string]int, error) {
	return m.records, nil
}

func stratzStats(n int) string {
	var list []map[string]any
	for i := 1; i <= n; i++ {
		list = append(list, map[string]any{"heroId": i, "matchCount": 5000 + i, "kills": 5.5, "deaths": 6.25})
	}
	// герой с малым числом матчей отсекается
	list = append(list, map[string]any{"heroId": 999, "matchCount": 10, "kills": 20.0, "deaths": 0.0})
	raw, _ := json.Marshal(map[string]any{"heroStats": map[string]any{"stats": list}})
	return string(raw)
}

func TestRefreshFromStratz(t *testing.T) {
	ms := newMemStore()
	svc := New(&fakeStratz{available: true, response: stratzStats(60)}, fakeOD{}, ms, quiet)
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, mode := range Modes {
		rows, err := svc.Snapshot(context.Background(), mode)
		if err != nil || len(rows) != 60 {
			t.Fatalf("%s: %d строк, %v", mode, len(rows), err)
		}
	}
	kills, _ := svc.Snapshot(context.Background(), Kills)
	if kills[0].Value != 5.5 {
		t.Errorf("убийства: %+v", kills[0])
	}
}

func TestRefreshFallsBackToOpenDota(t *testing.T) {
	var stats []opendota.HeroStat
	for i := 1; i <= 70; i++ {
		stats = append(stats, opendota.HeroStat{ID: i, PubPick: 2000 + i})
	}
	for name, st := range map[string]*fakeStratz{
		"нет токена":           {available: false},
		"ошибка запроса":       {available: true, err: errors.New("boom")},
		"мало героев в ответе": {available: true, response: stratzStats(5)},
	} {
		ms := newMemStore()
		svc := New(st, fakeOD{stats: stats}, ms, quiet)
		if err := svc.Refresh(context.Background()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if rows, err := svc.Snapshot(context.Background(), Popularity); err != nil || len(rows) != 70 {
			t.Errorf("%s: популярность %d, %v", name, len(rows), err)
		}
		if _, err := svc.Snapshot(context.Background(), Kills); !errors.Is(err, ErrNotReady) {
			t.Errorf("%s: убийства без STRATZ недоступны, получено %v", name, err)
		}
	}
}

func TestRefreshIfStale(t *testing.T) {
	ms := newMemStore()
	st := &fakeStratz{}
	svc := New(st, fakeOD{stats: func() []opendota.HeroStat {
		var s []opendota.HeroStat
		for i := 1; i <= 70; i++ {
			s = append(s, opendota.HeroStat{ID: i, PubPick: 3000})
		}
		return s
	}()}, ms, quiet)

	ctx := context.Background()
	if err := svc.RefreshIfStale(ctx); err != nil { // первый запуск
		t.Fatal(err)
	}
	first := ms.updatedAt
	time.Sleep(2 * time.Millisecond)
	if err := svc.RefreshIfStale(ctx); err != nil || !ms.updatedAt.Equal(first) {
		t.Error("свежий снапшот не пересобирается")
	}

	// появился STRATZ: снапшот только с популярностью заменяется сразу
	st.available, st.response = true, stratzStats(60)
	if err := svc.RefreshIfStale(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Snapshot(ctx, Kills); err != nil {
		t.Errorf("после появления STRATZ должны быть убийства: %v", err)
	}
}

func TestRecordScore(t *testing.T) {
	ms := newMemStore()
	svc := New(&fakeStratz{}, fakeOD{}, ms, quiet)
	ctx := context.Background()

	for _, streak := range []int{5, 12, 7} {
		if _, err := svc.RecordScore(ctx, 1, Kills, streak); err != nil {
			t.Fatal(err)
		}
	}
	if best, _ := svc.RecordScore(ctx, 1, Kills, 3); best != 12 {
		t.Errorf("рекорд = %d, ожидалось 12", best)
	}
	for _, bad := range []int{-1, MaxStreak + 1, 100000} {
		if _, err := svc.RecordScore(ctx, 1, Kills, bad); !errors.Is(err, ErrBadStreak) {
			t.Errorf("серия %d: %v", bad, err)
		}
	}
	if ms.plays != 4 {
		t.Errorf("сыграно %d, ожидалось 4: отклонённые серии не считаются", ms.plays)
	}

	rec, _ := svc.Records(ctx, 1)
	if rec["kills"] != 12 || rec["deaths"] != 0 || len(rec) != 3 {
		t.Errorf("рекорды: %v", fmt.Sprint(rec))
	}
}

func TestParseMode(t *testing.T) {
	for _, ok := range []string{"popularity", "kills", "deaths"} {
		if _, found := ParseMode(ok); !found {
			t.Errorf("%s", ok)
		}
	}
	for _, bad := range []string{"", "Kills", "assists", "../x"} {
		if _, found := ParseMode(bad); found {
			t.Errorf("%q не режим", bad)
		}
	}
}
