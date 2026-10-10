package coach

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/clients/steam"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeOD struct {
	mu          sync.Mutex
	player      *opendota.Player
	playerErr   error
	matches     []opendota.MatchSummary
	matchesErr  error
	wl          opendota.WinLoss
	playerCalls int
	refreshes   int
	match       *opendota.MatchDetail
	items       map[string]opendota.ItemConstant
}

func (f *fakeOD) Player(context.Context, int64) (*opendota.Player, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.playerCalls++
	return f.player, f.playerErr
}

func (f *fakeOD) PlayerMatches(context.Context, int64, int) ([]opendota.MatchSummary, error) {
	return f.matches, f.matchesErr
}

func (f *fakeOD) PlayerWL(context.Context, int64, int) (opendota.WinLoss, error) { return f.wl, nil }

func (f *fakeOD) RefreshPlayer(context.Context, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshes++
	return nil
}

func (f *fakeOD) Match(context.Context, int64) (*opendota.MatchDetail, error) {
	if f.match == nil {
		return nil, opendota.ErrNotFound
	}
	return f.match, nil
}

func (f *fakeOD) ItemConstants(context.Context) (map[string]opendota.ItemConstant, error) {
	return f.items, nil
}

type fakeSteam struct {
	id  string
	err error
}

func (f fakeSteam) ResolveVanity(context.Context, string) (string, error) { return f.id, f.err }

type fakeStore struct {
	cache   *store.PlayerCache
	matches map[int64]store.PlayerMatch
}

func newFakeStore() *fakeStore { return &fakeStore{matches: map[int64]store.PlayerMatch{}} }

func (f *fakeStore) GetPlayerCache(context.Context, int64) (store.PlayerCache, error) {
	if f.cache == nil {
		return store.PlayerCache{}, store.ErrNotFound
	}
	return *f.cache, nil
}

func (f *fakeStore) PutPlayerCache(_ context.Context, p store.PlayerCache) error {
	p.UpdatedAt = time.Now()
	f.cache = &p
	return nil
}

func (f *fakeStore) UpsertPlayerMatches(_ context.Context, ms []store.PlayerMatch) error {
	for _, m := range ms {
		f.matches[m.MatchID] = m
	}
	return nil
}

func (f *fakeStore) sorted(filter string) []store.PlayerMatch {
	var out []store.PlayerMatch
	for _, m := range f.matches {
		if filter == "win" && !m.Win || filter == "loss" && m.Win {
			continue
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].StartTime.Equal(out[j].StartTime) {
			return out[i].StartTime.After(out[j].StartTime)
		}
		return out[i].MatchID > out[j].MatchID
	})
	return out
}

func (f *fakeStore) ListPlayerMatches(_ context.Context, _ int64, filter string, cur *store.MatchCursor, limit int) ([]store.PlayerMatch, error) {
	var out []store.PlayerMatch
	for _, m := range f.sorted(filter) {
		if cur != nil && !(m.StartTime.Before(cur.StartTime) || m.StartTime.Equal(cur.StartTime) && m.MatchID < cur.MatchID) {
			continue
		}
		out = append(out, m)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeStore) PlayerMatchCounts(context.Context, int64) (store.MatchCounts, error) {
	c := store.MatchCounts{}
	for _, m := range f.matches {
		c.All++
		if m.Win {
			c.Win++
		} else {
			c.Loss++
		}
	}
	return c, nil
}

func (f *fakeStore) PlayerHeroForm(_ context.Context, _ int64, since time.Time, minMatches int) ([]store.HeroForm, error) {
	agg := map[int]*store.HeroForm{}
	for _, m := range f.matches {
		if !since.IsZero() && m.StartTime.Before(since) {
			continue
		}
		h := agg[m.HeroID]
		if h == nil {
			h = &store.HeroForm{HeroID: m.HeroID}
			agg[m.HeroID] = h
		}
		h.Matches++
		if m.Win {
			h.Wins++
		}
	}
	var out []store.HeroForm
	for _, h := range agg {
		if h.Matches >= minMatches {
			out = append(out, *h)
		}
	}
	return out, nil
}

type fakePatches struct{ start time.Time }

func (fakePatches) Patch() (dota.Patch, bool) {
	return dota.Patch{Number: "7.41", Label: "7.41F"}, true
}

func (f fakePatches) PatchStart() (time.Time, bool) { return f.start, !f.start.IsZero() }

// матч игрока: слот 0 — Силы Света, radiantWin определяет результат
func match(id int64, hero int, win bool, ago time.Duration) opendota.MatchSummary {
	return opendota.MatchSummary{
		MatchID: id, HeroID: hero, PlayerSlot: 0, RadiantWin: win, Kills: 5, Deaths: 2, Assists: 9,
		Duration: 1800, StartTime: time.Now().Add(-ago).Unix(),
	}
}

func newService(od *fakeOD, st *fakeStore) *Service {
	return New(od, fakeSteam{}, st, fakePatches{start: time.Now().Add(-48 * time.Hour)}, quiet)
}

func rank(n int) *int { return &n }

func TestSummary(t *testing.T) {
	od := &fakeOD{
		player: &opendota.Player{Profile: &opendota.PlayerProfile{PersonaName: "Mlgfocus", AvatarFull: "https://a/x.jpg"}, RankTier: rank(51)},
		wl:     opendota.WinLoss{Win: 55, Lose: 45},
	}
	// 3 матча на герое 1 (один из них до патча), 1 матч героя 2
	od.matches = []opendota.MatchSummary{
		match(4, 2, false, time.Hour),
		match(3, 1, true, 5*time.Hour),
		match(2, 1, true, 24*time.Hour),
		match(1, 1, false, 100*time.Hour),
	}
	svc := newService(od, newFakeStore())

	sum, err := svc.Summary(context.Background(), 111)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.HasHistory || sum.Player.PersonaName == nil || *sum.Player.PersonaName != "Mlgfocus" || *sum.Player.RankTier != 51 {
		t.Fatalf("игрок: %+v", sum.Player)
	}
	if sum.Last20.Wins != 2 || sum.Last20.Losses != 2 || len(sum.Last20.Result) != 4 {
		t.Errorf("last20: %+v", sum.Last20)
	}
	// от старых к новым: 1(L) 2(W) 3(W) 4(L)
	if got := sum.Last20.Result; got[0] != "L" || got[1] != "W" || got[3] != "L" {
		t.Errorf("порядок результатов: %v", got)
	}
	if sum.LastMatch == nil || sum.LastMatch.MatchID != 4 {
		t.Errorf("последний матч: %+v", sum.LastMatch)
	}
	if sum.SixMonths.Matches != 100 || sum.SixMonths.Winrate != 0.55 {
		t.Errorf("6 месяцев: %+v", sum.SixMonths)
	}
	if len(sum.HeroesInForm) != 2 || sum.HeroesInForm[0].Period != "all" || sum.HeroesInForm[1].Period != "7.41" {
		t.Fatalf("периоды: %+v", sum.HeroesInForm)
	}
	// за всё время подходит только герой 1 (3 матча); герой 2 сыграл один матч
	if h := sum.HeroesInForm[0].Heroes; len(h) != 1 || h[0].HeroID != 1 || h[0].Matches != 3 {
		t.Errorf("герои в форме (все): %+v", h)
	}
	// за патч у героя 1 только 2 матча — меньше порога
	if h := sum.HeroesInForm[1].Heroes; len(h) != 0 {
		t.Errorf("герои в форме (патч): %+v", h)
	}
}

func TestSyncIsCachedAndServesStaleOnFailure(t *testing.T) {
	od := &fakeOD{player: &opendota.Player{Profile: &opendota.PlayerProfile{}}, matches: []opendota.MatchSummary{match(1, 1, true, time.Hour)}}
	st := newFakeStore()
	svc := newService(od, st)
	ctx := context.Background()

	for range 3 {
		if _, err := svc.Summary(ctx, 5); err != nil {
			t.Fatal(err)
		}
	}
	if od.playerCalls != 1 {
		t.Errorf("свежий кэш не должен ходить в OpenDota повторно: %d вызовов", od.playerCalls)
	}

	// кэш устарел, OpenDota упала — отдаём сохранённое
	st.cache.UpdatedAt = time.Now().Add(-time.Hour)
	od.playerErr = errors.New("boom")
	sum, err := svc.Summary(ctx, 5)
	if err != nil || !sum.HasHistory {
		t.Errorf("при сбое OpenDota ожидались сохранённые данные: %v %+v", err, sum)
	}
}

func TestUnknownPlayer(t *testing.T) {
	od := &fakeOD{playerErr: opendota.ErrNotFound}
	if _, err := newService(od, newFakeStore()).Summary(context.Background(), 9); !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("err = %v", err)
	}
	// OpenDota упала, а данных у нас ещё нет — это ошибка, а не пустая сводка
	od = &fakeOD{playerErr: errors.New("boom")}
	if _, err := newService(od, newFakeStore()).Summary(context.Background(), 9); err == nil || errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("err = %v", err)
	}
}

func TestPrivateProfileHasNoHistory(t *testing.T) {
	od := &fakeOD{player: &opendota.Player{Profile: &opendota.PlayerProfile{PersonaName: "Hidden"}}}
	sum, err := newService(od, newFakeStore()).Summary(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if sum.HasHistory || sum.LastMatch != nil || len(sum.HeroesInForm) != 0 || len(sum.Last20.Result) != 0 {
		t.Errorf("пустая история: %+v", sum)
	}
}

func TestMatchesPaginationAndFilter(t *testing.T) {
	od := &fakeOD{player: &opendota.Player{Profile: &opendota.PlayerProfile{}}}
	for i := range 45 {
		od.matches = append(od.matches, match(int64(1000-i), 1, i%3 != 0, time.Duration(i+1)*time.Hour))
	}
	svc := newService(od, newFakeStore())
	ctx := context.Background()

	p1, err := svc.Matches(ctx, 1, "all", "", 20)
	if err != nil || len(p1.Items) != 20 || p1.NextCursor == nil || p1.Counts.All != 45 {
		t.Fatalf("страница 1: %v %+v", err, p1)
	}
	p2, _ := svc.Matches(ctx, 1, "", *p1.NextCursor, 20)
	p3, _ := svc.Matches(ctx, 1, "", *p2.NextCursor, 20)
	if len(p2.Items) != 20 || len(p3.Items) != 5 || p3.NextCursor != nil {
		t.Fatalf("страницы: %d / %d / %v", len(p2.Items), len(p3.Items), p3.NextCursor)
	}
	seen := map[int64]bool{}
	for _, p := range []*MatchesPage{p1, p2, p3} {
		for _, m := range p.Items {
			if seen[m.MatchID] {
				t.Fatalf("матч %d повторился на разных страницах", m.MatchID)
			}
			seen[m.MatchID] = true
		}
	}
	if len(seen) != 45 {
		t.Errorf("всего %d матчей", len(seen))
	}

	wins, _ := svc.Matches(ctx, 1, "win", "", 50)
	for _, m := range wins.Items {
		if !m.Win {
			t.Fatal("в «Победах» оказалось поражение")
		}
	}
	if wins.Counts.Win+wins.Counts.Loss != wins.Counts.All {
		t.Errorf("счётчики: %+v", wins.Counts)
	}
	if _, err := svc.Matches(ctx, 1, "draw", "", 20); !errors.Is(err, ErrBadFilter) {
		t.Errorf("фильтр: %v", err)
	}
	if _, err := svc.Matches(ctx, 1, "", "garbage", 20); !errors.Is(err, ErrBadCursor) {
		t.Errorf("курсор: %v", err)
	}
}

func TestRefreshRateLimit(t *testing.T) {
	od := &fakeOD{player: &opendota.Player{Profile: &opendota.PlayerProfile{}}}
	svc := newService(od, newFakeStore())
	ctx := context.Background()

	if err := svc.Refresh(ctx, 1); err != nil {
		t.Fatal(err)
	}
	var tooSoon *TooSoonError
	if err := svc.Refresh(ctx, 1); !errors.As(err, &tooSoon) || tooSoon.RetryAfter <= 0 {
		t.Errorf("повторное обновление сразу: %v", err)
	}
	if err := svc.Refresh(ctx, 2); err != nil {
		t.Errorf("другой игрок не должен упираться в лимит: %v", err)
	}
	if od.refreshes != 2 {
		t.Errorf("запросов refresh в OpenDota: %d", od.refreshes)
	}
	svc.now = func() time.Time { return time.Now().Add(2 * time.Minute) }
	if err := svc.Refresh(ctx, 1); err != nil {
		t.Errorf("через минуту обновление снова разрешено: %v", err)
	}
}

func TestResolve(t *testing.T) {
	od := &fakeOD{player: &opendota.Player{Profile: &opendota.PlayerProfile{PersonaName: "Nick", AvatarFull: "https://a/b.jpg"}}}
	svc := newService(od, newFakeStore())
	ctx := context.Background()

	r, err := svc.Resolve(ctx, "https://steamcommunity.com/profiles/76561199174956609")
	if err != nil || r.AccountID != 1214690881 || *r.PersonaName != "Nick" {
		t.Fatalf("по SteamID64: %v %+v", err, r)
	}
	if r, err = svc.Resolve(ctx, "https://www.opendota.com/players/1214690881"); err != nil || r.AccountID != 1214690881 {
		t.Errorf("по ссылке OpenDota: %v %+v", err, r)
	}

	if _, err := svc.Resolve(ctx, "привет"); !errors.Is(err, ErrBadInput) {
		t.Errorf("мусор: %v", err)
	}

	// короткое имя
	svc.steam = fakeSteam{id: "76561199174956609"}
	if r, err = svc.Resolve(ctx, "steamcommunity.com/id/mynick"); err != nil || r.AccountID != 1214690881 {
		t.Errorf("vanity: %v %+v", err, r)
	}
	svc.steam = fakeSteam{err: steam.ErrNoKey}
	if _, err := svc.Resolve(ctx, "steamcommunity.com/id/mynick"); !errors.Is(err, ErrNoSteamKey) {
		t.Errorf("без ключа: %v", err)
	}
	svc.steam = fakeSteam{err: steam.ErrVanityNotFound}
	if _, err := svc.Resolve(ctx, "steamcommunity.com/id/nobody"); !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("нет такого имени: %v", err)
	}

	od.playerErr = opendota.ErrNotFound
	if _, err := svc.Resolve(ctx, "123456"); !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("неизвестный игрок: %v", err)
	}
	od.playerErr, od.player = nil, &opendota.Player{} // OpenDota знает id, но профиля нет
	if _, err := svc.Resolve(ctx, "123456"); !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("игрок без профиля: %v", err)
	}
}

func TestTopForm(t *testing.T) {
	got := topForm([]store.HeroForm{
		{HeroID: 1, Matches: 3, Wins: 3},   // 100%, но всего 3 матча: 1.0*ln(4)=1.39
		{HeroID: 2, Matches: 25, Wins: 18}, // 72%: 0.72*ln(26)=2.35
		{HeroID: 3, Matches: 10, Wins: 2},
		{HeroID: 4, Matches: 12, Wins: 9},
		{HeroID: 5, Matches: 0},
	}, 3)
	ids := []int{}
	for _, h := range got {
		ids = append(ids, h.HeroID)
	}
	if len(ids) != 3 || ids[0] != 2 || ids[1] != 4 || ids[2] != 1 {
		t.Errorf("порядок героев в форме: %v", ids)
	}
}

func TestMatchDetail(t *testing.T) {
	me := int64(111)
	name := "Игрок"
	od := &fakeOD{
		items: map[string]opendota.ItemConstant{"blink": {ID: 1, DName: "Blink Dagger"}},
		match: &opendota.MatchDetail{
			MatchID: 9, RadiantWin: false, Duration: 2400, StartTime: 1700000000, RadiantScore: 30, DireScore: 41,
			Players: []opendota.MatchPlayer{
				{AccountID: &me, PersonaName: &name, HeroID: 1, PlayerSlot: 3, Kills: 7, Item0: 1, Item1: 999},
				{HeroID: 2, PlayerSlot: 130},
			},
		},
	}
	svc := newService(od, newFakeStore())
	m, err := svc.Match(context.Background(), 9, 111)
	if err != nil {
		t.Fatal(err)
	}
	p0 := m.Players[0]
	if p0.Team != "radiant" || !p0.IsMe || m.Players[1].Team != "dire" || m.Players[1].IsMe {
		t.Errorf("команды и «я»: %+v", m.Players)
	}
	if m.Win == nil || *m.Win {
		t.Errorf("игрок за Свет, победила Тьма — ожидалось поражение: %v", m.Win)
	}
	if len(p0.Items) != 6 || p0.Items[0] == nil || p0.Items[0].Key != "blink" || p0.Items[1] != nil || p0.Items[2] != nil {
		t.Errorf("предметы: %+v", p0.Items)
	}
	if m2, _ := svc.Match(context.Background(), 9, 0); m2.Win != nil {
		t.Error("без account_id результат игрока не определён")
	}
	od.match = nil
	if _, err := svc.Match(context.Background(), 10, 0); !errors.Is(err, ErrMatchNotFound) {
		t.Errorf("нет матча: %v", err)
	}
}
