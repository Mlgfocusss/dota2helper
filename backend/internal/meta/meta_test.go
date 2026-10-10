package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeStratz struct {
	available bool
	response  string
	err       error
	vars      map[string]any
}

func (f *fakeStratz) Available() bool { return f.available }

func (f *fakeStratz) Query(_ context.Context, _ string, vars map[string]any, out any) error {
	f.vars = vars
	if f.err != nil {
		return f.err
	}
	return json.Unmarshal([]byte(f.response), out)
}

type fakeOD struct{ stats []opendota.HeroStat }

func (f fakeOD) HeroStats(context.Context) ([]opendota.HeroStat, error) { return f.stats, nil }

type fakePatch struct{}

func (fakePatch) Patch() (dota.Patch, bool) { return dota.Patch{Number: "7.41", Label: "7.41F"}, true }

type memStore struct {
	rows      map[string][]store.MetaRow
	updatedAt time.Time
	kv        map[string][]byte
}

func newMemStore() *memStore {
	return &memStore{rows: map[string][]store.MetaRow{}, kv: map[string][]byte{}}
}

func (m *memStore) ReplaceMetaPositions(_ context.Context, patch string, rows []store.MetaRow) error {
	m.rows[patch] = rows
	m.updatedAt = time.Now()
	return nil
}

func (m *memStore) MetaPositions(_ context.Context, patch string) ([]store.MetaRow, time.Time, error) {
	rows, ok := m.rows[patch]
	if !ok {
		return nil, time.Time{}, store.ErrNotFound
	}
	return rows, m.updatedAt, nil
}

func (m *memStore) LatestMetaPatch(context.Context) (string, error) {
	for p := range m.rows {
		return p, nil
	}
	return "", store.ErrNotFound
}

func (m *memStore) GetKV(_ context.Context, key string) ([]byte, time.Time, error) {
	v, ok := m.kv[key]
	if !ok {
		return nil, time.Time{}, store.ErrNotFound
	}
	return v, time.Now(), nil
}

func (m *memStore) PutKV(_ context.Context, key string, value []byte) error {
	m.kv[key] = value
	return nil
}

// тестовые теги: герой 1 — позиция 1, герой 2 — позиции 1 и 3, герой 3 — позиция 5
func testTags(t *testing.T) *dota.HeroTagSet {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tags.json")
	body := `[
	  {"hero_id":1,"positions":[1],"complexity":1,"timing":["mid"],"tags":[]},
	  {"hero_id":2,"positions":[1,3],"complexity":1,"timing":["mid"],"tags":[]},
	  {"hero_id":3,"positions":[5],"complexity":1,"timing":["mid"],"tags":[]}
	]`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	set, err := dota.LoadHeroTags(path)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func newService(t *testing.T, st *fakeStratz, od fakeOD, bracket string) (*Service, *memStore) {
	t.Helper()
	ms := newMemStore()
	svc, err := New(Deps{
		Stratz: st, OpenDota: od, Tags: testTags(t), Store: ms, Patch: fakePatch{},
		Bracket: bracket, MinMatches: 200, Logger: quiet,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, ms
}

func TestOpenDotaFallback(t *testing.T) {
	od := fakeOD{stats: []opendota.HeroStat{
		{ID: 1, PubPick: 1000, PubWin: 520},
		{ID: 2, PubPick: 2000, PubWin: 1100},
		{ID: 3, PubPick: 100, PubWin: 70}, // меньше 200 матчей — не попадёт в топ
		{ID: 99, PubPick: 5000, PubWin: 2600},
	}}
	svc, _ := newService(t, &fakeStratz{available: false}, od, "ALL")

	if _, err := svc.Snapshot(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("до сбора ожидалась ErrNotReady, получено %v", err)
	}
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	snap, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != SourceOpenDota || snap.Patch.Label != "7.41F" {
		t.Errorf("source=%s patch=%+v", snap.Source, snap.Patch)
	}
	pos1 := snap.ByPosition[0]
	if len(pos1) != 2 || pos1[0].HeroID != 2 || pos1[1].HeroID != 1 {
		t.Fatalf("позиция 1 по убыванию винрейта: %+v", pos1)
	}
	if pos1[0].Winrate != 0.55 {
		t.Errorf("winrate = %v", pos1[0].Winrate)
	}
	// герой 2 числится на позициях 1 и 3, но в мете стоит только на основной
	if len(snap.ByPosition[2]) != 0 || len(snap.ByPosition[4]) != 0 {
		t.Errorf("позиции 3 и 5 должны быть пусты: %+v / %+v", snap.ByPosition[2], snap.ByPosition[4])
	}
}

func TestBracketFilterForOpenDota(t *testing.T) {
	od := fakeOD{stats: []opendota.HeroStat{
		{ID: 1, Pick7: 300, Win7: 150, Pick8: 300, Win8: 210, PubPick: 9000, PubWin: 4000},
	}}
	svc, ms := newService(t, &fakeStratz{}, od, "DIVINE, immortal")
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := ms.rows["7.41"][0]; r.Matches != 600 || r.Wins != 360 {
		t.Errorf("брекеты 7+8: %+v", r)
	}
	if _, _, err := ParseBrackets("SUPERSTAR"); err == nil {
		t.Error("неизвестный брекет должен отклоняться")
	}
}

func stratzResponse(heroesPerPos int) string {
	positions := map[string][]stratzWin{}
	for pos := 1; pos <= 5; pos++ {
		var list []stratzWin
		for h := 1; h <= heroesPerPos; h++ {
			// у каждого героя своя основная позиция; на остальных его почти не играют
			matches := 300 + h
			if (h+2)%5+1 == pos {
				matches = 5000 + h
			}
			list = append(list,
				stratzWin{GameVersionID: 180, HeroID: h, MatchCount: matches, WinCount: matches/2 + h*pos},
				// старая версия игры — должна игнорироваться
				stratzWin{GameVersionID: 179, HeroID: h, MatchCount: 99999, WinCount: 99999},
			)
		}
		positions[fmt.Sprintf("p%d", pos)] = list
	}
	raw, _ := json.Marshal(map[string]any{"heroStats": positions})
	return string(raw)
}

func TestStratzSource(t *testing.T) {
	st := &fakeStratz{available: true, response: stratzResponse(12)}
	svc, ms := newService(t, st, fakeOD{}, "DIVINE,IMMORTAL")
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := st.vars["brackets"]; fmt.Sprint(got) != "[DIVINE IMMORTAL]" {
		t.Errorf("brackets = %v", got)
	}
	if n := len(ms.rows["7.41"]); n != 60 {
		t.Fatalf("строк %d, ожидалось 5 позиций × 12 героев", n)
	}
	for _, r := range ms.rows["7.41"] {
		if r.Matches > 50000 {
			t.Fatalf("данные старой версии игры попали в мету: %+v", r)
		}
	}
	snap, _ := svc.Snapshot(context.Background())
	if snap.Source != SourceStratz || snap.ByPosition[4][0].HeroID != 12 {
		t.Errorf("source=%s top5=%+v", snap.Source, snap.ByPosition[4][:1])
	}
}

func TestStratzFailureFallsBack(t *testing.T) {
	od := fakeOD{stats: []opendota.HeroStat{{ID: 1, PubPick: 1000, PubWin: 500}}}
	cases := map[string]*fakeStratz{
		"ошибка запроса":       {available: true, err: errors.New("Cannot query field")},
		"мало героев в ответе": {available: true, response: stratzResponse(3)},
	}
	for name, st := range cases {
		svc, _ := newService(t, st, od, "ALL")
		if err := svc.Refresh(context.Background()); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if snap, _ := svc.Snapshot(context.Background()); snap.Source != SourceOpenDota {
			t.Errorf("%s: ожидался фоллбэк на OpenDota", name)
		}
	}
}

func TestRefreshIfStaleSkipsFreshData(t *testing.T) {
	od := fakeOD{stats: []opendota.HeroStat{{ID: 1, PubPick: 1000, PubWin: 500}}}
	svc, ms := newService(t, &fakeStratz{}, od, "ALL")
	if err := svc.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := ms.updatedAt
	if err := svc.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ms.updatedAt.Equal(first) {
		t.Error("свежая мета не пересобирается")
	}
}

func TestRefreshIfStaleRebuildsOldFormat(t *testing.T) {
	od := fakeOD{stats: []opendota.HeroStat{{ID: 1, PubPick: 1000, PubWin: 500}}}
	svc, ms := newService(t, &fakeStratz{}, od, "ALL")
	if err := svc.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	// данные собраны прошлой версией кода — меты без метки формата
	delete(ms.kv, formatKey("7.41"))
	ms.updatedAt = time.Time{}
	if err := svc.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ms.updatedAt.IsZero() {
		t.Error("мета старого формата должна пересобираться сразу")
	}
}

func TestRefreshIfStaleSwitchesToStratz(t *testing.T) {
	od := fakeOD{stats: []opendota.HeroStat{{ID: 1, PubPick: 1000, PubWin: 500}}}
	st := &fakeStratz{}
	svc, ms := newService(t, st, od, "ALL")
	if err := svc.Refresh(context.Background()); err != nil { // STRATZ недоступен — мета из OpenDota
		t.Fatal(err)
	}
	st.available, st.response = true, stratzResponse(12)
	ms.updatedAt = time.Time{}
	if err := svc.RefreshIfStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	if ms.updatedAt.IsZero() {
		t.Error("когда STRATZ стал доступен, мета должна пересобраться сразу")
	}
	if snap, _ := svc.Snapshot(context.Background()); snap == nil || snap.Source != SourceStratz {
		t.Errorf("источник после перехода: %+v", snap)
	}
}

func TestRareRolesAreNotInTop(t *testing.T) {
	ms := newMemStore()
	svc, _ := New(Deps{Tags: testTags(t), Store: ms, Patch: fakePatch{}, MinMatches: 200, Logger: quiet})
	ms.rows["7.41"] = []store.MetaRow{
		{Position: 1, HeroID: 1, Matches: 50000, Wins: 26000}, // обычный керри
		{Position: 1, HeroID: 2, Matches: 500, Wins: 335},     // 67% винрейт, но 0,7% игр позиции
		{Position: 1, HeroID: 3, Matches: 20000, Wins: 12000}, // 38% игр героя — нормальная роль
		{Position: 3, HeroID: 3, Matches: 33000, Wins: 17000}, // основная роль героя
		{Position: 2, HeroID: 4, Matches: 8000, Wins: 5000},   // 8% игр героя на этой позиции
		{Position: 3, HeroID: 4, Matches: 92000, Wins: 49000},
		{Position: 2, HeroID: 5, Matches: 40000, Wins: 21000},
	}
	snap, err := svc.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var pos1, pos2 []int
	for _, h := range snap.ByPosition[0] {
		pos1 = append(pos1, h.HeroID)
	}
	for _, h := range snap.ByPosition[1] {
		pos2 = append(pos2, h.HeroID)
	}
	if fmt.Sprint(pos1) != "[3 1]" {
		t.Errorf("керри: %v (герой 2 — редкая роль, должен отсеяться)", pos1)
	}
	if fmt.Sprint(pos2) != "[5]" {
		t.Errorf("мид: %v (герой 4 играет мид в 8%% игр — не его роль)", pos2)
	}
}
