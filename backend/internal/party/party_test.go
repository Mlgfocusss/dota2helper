package party

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func validInput() Input {
	return Input{
		RankMedal: 5, Hours: 3200, Positions: []int{2, 1}, Modes: []string{"turbo", "ranked"},
		Goal: "rating", LookingFor: "team", Mic: true, FavHeroes: []int{14, 1},
		About: "Играю вечером по будням, ищу адекватных напарников",
	}
}

func fieldsOf(t *testing.T, err error) map[string]string {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
	return ve.Fields
}

func TestNormalizeOK(t *testing.T) {
	p, err := validInput().Normalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Positions) != 2 || p.Positions[0] != 1 || p.Positions[1] != 2 {
		t.Errorf("positions should be sorted: %v", p.Positions)
	}
	// режимы хранятся в порядке интерфейса, а не в порядке запроса
	if len(p.Modes) != 2 || p.Modes[0] != "ranked" || p.Modes[1] != "turbo" {
		t.Errorf("modes order: %v", p.Modes)
	}
	if p.DiscordTag != "" || p.Telegram != "" {
		t.Errorf("empty contacts expected: %q %q", p.DiscordTag, p.Telegram)
	}
}

func TestNormalizeErrors(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Input)
		field string
	}{
		{"rank low", func(in *Input) { in.RankMedal = -1 }, "rank_medal"},
		{"rank high", func(in *Input) { in.RankMedal = 9 }, "rank_medal"},
		{"hours", func(in *Input) { in.Hours = 50001 }, "hours"},
		{"no positions", func(in *Input) { in.Positions = nil }, "positions"},
		{"bad position", func(in *Input) { in.Positions = []int{1, 6} }, "positions"},
		{"no modes", func(in *Input) { in.Modes = nil }, "modes"},
		{"bad mode", func(in *Input) { in.Modes = []string{"ranked", "hax"} }, "modes"},
		{"bad goal", func(in *Input) { in.Goal = "" }, "goal"},
		{"bad looking", func(in *Input) { in.LookingFor = "all" }, "looking_for"},
		{"discord without tag", func(in *Input) { in.Discord = true }, "discord_tag"},
		{"bad telegram", func(in *Input) { in.Telegram = "ab" }, "telegram"},
		{"4 heroes", func(in *Input) { in.FavHeroes = []int{1, 2, 3, 4} }, "fav_heroes"},
		{"unknown hero", func(in *Input) { in.FavHeroes = []int{999} }, "fav_heroes"},
		{"long about", func(in *Input) { in.About = strings.Repeat("я", MaxAbout+1) }, "about"},
		{"link in about", func(in *Input) { in.About = "пиши в t.me/spam" }, "about"},
	}
	known := func(id int) bool { return id < 200 }
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := validInput()
			c.edit(&in)
			_, err := in.Normalize(known)
			if _, ok := fieldsOf(t, err)[c.field]; !ok {
				t.Errorf("want error on %q, got %v", c.field, err)
			}
		})
	}
}

func TestNormalizeContacts(t *testing.T) {
	in := validInput()
	in.Discord = true
	in.DiscordTag = "  nick.name  "
	in.Telegram = "@dota_fan"
	in.FavHeroes = []int{5, 5, 7}
	p, err := in.Normalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.DiscordTag != "nick.name" || p.Telegram != "dota_fan" {
		t.Errorf("contacts: %q %q", p.DiscordTag, p.Telegram)
	}
	if len(p.FavHeroes) != 2 {
		t.Errorf("duplicate hero should collapse: %v", p.FavHeroes)
	}

	// ник Discord не нужен и не хранится, пока переключатель выключен
	in.Discord = false
	p, _ = in.Normalize(nil)
	if p.DiscordTag != "" {
		t.Errorf("discord tag must be dropped, got %q", p.DiscordTag)
	}
}

func TestContainsForbidden(t *testing.T) {
	bad := []string{
		"заходи на https://example.com", "мой сайт www.test.ru", "discord.gg/abc", "t.me/channel",
		"ты хуесос", "наебал всех", "ПИЗДЕЦ", "иди нахуй", "ну ты и мудак", "сука",
	}
	good := []string{
		"Играю вечером, люблю керри. Без токсичности!",
		"Небанальный подход, ребята, всегда на связи", // «ебан» внутри слова — не мат
		"Подсказываю по драфту, сукно не продаю",
		"Хожу в хай-ранги, ранг 5.2к",
		"Жду напарников на 1 и 5 позиции",
	}
	for _, s := range bad {
		if !ContainsForbidden(s) {
			t.Errorf("should be forbidden: %q", s)
		}
	}
	for _, s := range good {
		if ContainsForbidden(s) {
			t.Errorf("should be allowed: %q", s)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	p := store.PartyProfile{UserID: 42, BumpedAt: time.UnixMicro(1_700_000_123_456_789)}
	c, err := decodeCursor(encodeCursor(p))
	if err != nil || c == nil {
		t.Fatal(err)
	}
	if c.UserID != 42 || !c.BumpedAt.Equal(p.BumpedAt) {
		t.Errorf("cursor mismatch: %+v", c)
	}
	if c, err := decodeCursor(""); c != nil || err != nil {
		t.Errorf("empty cursor: %v %v", c, err)
	}
	for _, bad := range []string{"!!!", "YWJj", "MToxOjE"} {
		if _, err := decodeCursor(bad); !errors.Is(err, ErrBadCursor) {
			t.Errorf("%q: want ErrBadCursor, got %v", bad, err)
		}
	}
}

// ----- сервис ----------------------------------------------------------------

type fakeStore struct {
	cards   map[int64]store.PartyProfile
	reports map[[2]int64]bool
	rows    []store.PartyProfile // что вернёт SearchParty
	filter  store.PartyFilter
	cache   map[int64]store.PlayerCache
}

func newFake() *fakeStore {
	return &fakeStore{
		cards: map[int64]store.PartyProfile{}, reports: map[[2]int64]bool{},
		cache: map[int64]store.PlayerCache{},
	}
}

func (f *fakeStore) PartyProfileByUser(_ context.Context, id int64) (store.PartyProfile, error) {
	p, ok := f.cards[id]
	if !ok {
		return store.PartyProfile{}, store.ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) UpsertPartyProfile(_ context.Context, p store.PartyProfile) error {
	if old, ok := f.cards[p.UserID]; ok { // правка не трогает подъём и видимость
		p.BumpedAt, p.IsActive = old.BumpedAt, old.IsActive
	} else {
		p.BumpedAt, p.IsActive = time.Now(), true
	}
	f.cards[p.UserID] = p
	return nil
}

func (f *fakeStore) SetPartyActive(_ context.Context, id int64, active bool) error {
	p, ok := f.cards[id]
	if !ok {
		return store.ErrNotFound
	}
	p.IsActive = active
	f.cards[id] = p
	return nil
}

func (f *fakeStore) DeletePartyProfile(_ context.Context, id int64) error {
	delete(f.cards, id)
	return nil
}

func (f *fakeStore) BumpParty(_ context.Context, id int64, cooldown time.Duration) (time.Time, bool, error) {
	p, ok := f.cards[id]
	if !ok {
		return time.Time{}, false, store.ErrNotFound
	}
	if time.Since(p.BumpedAt) < cooldown {
		return p.BumpedAt, false, nil
	}
	p.BumpedAt = time.Now()
	f.cards[id] = p
	return p.BumpedAt, true, nil
}

func (f *fakeStore) SearchParty(_ context.Context, flt store.PartyFilter) ([]store.PartyProfile, error) {
	f.filter = flt
	return f.rows, nil
}

func (f *fakeStore) ReportParty(_ context.Context, reporter, target int64, _ string) error {
	if _, ok := f.cards[target]; !ok {
		return store.ErrNotFound
	}
	k := [2]int64{reporter, target}
	if f.reports[k] {
		return store.ErrAlreadyReported
	}
	f.reports[k] = true
	return nil
}

func (f *fakeStore) ExpirePartyProfiles(context.Context, time.Duration) (int64, error) { return 0, nil }

func (f *fakeStore) GetPlayerCache(_ context.Context, id int64) (store.PlayerCache, error) {
	pc, ok := f.cache[id]
	if !ok {
		return store.PlayerCache{}, store.ErrNotFound
	}
	return pc, nil
}

func newSvc(f *fakeStore) *Service { return New(f, nil, quiet) }

func TestSaveKeepsBumpAndVisibility(t *testing.T) {
	f, ctx := newFake(), context.Background()
	svc := newSvc(f)
	card, err := svc.Save(ctx, 1, validInput())
	if err != nil {
		t.Fatal(err)
	}
	if card.NextBumpAt == nil || !card.IsActive {
		t.Fatalf("new card: %+v", card)
	}
	first := card.BumpedAt

	if _, err := svc.SetActive(ctx, 1, false); err != nil {
		t.Fatal(err)
	}
	in := validInput()
	in.Hours = 100
	card, err = svc.Save(ctx, 1, in)
	if err != nil {
		t.Fatal(err)
	}
	if card.IsActive || !card.BumpedAt.Equal(first) || card.Hours != 100 {
		t.Errorf("edit must not change visibility or bump time: %+v", card)
	}
}

func TestBump(t *testing.T) {
	f, ctx := newFake(), context.Background()
	svc := newSvc(f)
	if _, err := svc.Bump(ctx, 1); !errors.Is(err, ErrNoCard) {
		t.Fatalf("no card: %v", err)
	}
	_, _ = svc.Save(ctx, 1, validInput())

	var tooSoon *TooSoonError
	if _, err := svc.Bump(ctx, 1); !errors.As(err, &tooSoon) {
		t.Fatalf("fresh card should not bump: %v", err)
	}
	if tooSoon.RetryAfter <= 0 || tooSoon.RetryAfter > BumpCooldown {
		t.Errorf("retry after: %v", tooSoon.RetryAfter)
	}

	p := f.cards[1]
	p.BumpedAt = time.Now().Add(-BumpCooldown - time.Minute)
	f.cards[1] = p
	card, err := svc.Bump(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(card.BumpedAt) > time.Minute {
		t.Errorf("bump time not refreshed: %v", card.BumpedAt)
	}
}

func TestReport(t *testing.T) {
	f, ctx := newFake(), context.Background()
	svc := newSvc(f)
	f.cards[2] = store.PartyProfile{UserID: 2}

	if err := svc.Report(ctx, 1, 2, "weird"); !errors.Is(err, ErrBadReason) {
		t.Errorf("bad reason: %v", err)
	}
	if err := svc.Report(ctx, 2, 2, "spam"); !errors.Is(err, ErrSelfReport) {
		t.Errorf("self report: %v", err)
	}
	if err := svc.Report(ctx, 1, 3, "spam"); !errors.Is(err, ErrNoCard) {
		t.Errorf("no card: %v", err)
	}
	if err := svc.Report(ctx, 1, 2, "spam"); err != nil {
		t.Errorf("first report: %v", err)
	}
	if err := svc.Report(ctx, 1, 2, "fake"); !errors.Is(err, ErrAlreadyReported) {
		t.Errorf("second report: %v", err)
	}
}

func TestSearchPaging(t *testing.T) {
	f, ctx := newFake(), context.Background()
	svc := newSvc(f)
	now := time.Now()
	for i := range PageSize + 1 { // на одну больше страницы — значит, есть продолжение
		f.rows = append(f.rows, store.PartyProfile{UserID: int64(100 - i), BumpedAt: now.Add(-time.Duration(i) * time.Minute)})
	}
	page, err := svc.Search(ctx, 7, Query{LookingFor: "team", RankMax: MaxRank})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != PageSize || page.NextCursor == nil {
		t.Fatalf("items=%d cursor=%v", len(page.Items), page.NextCursor)
	}
	if f.filter.ViewerID != 7 || f.filter.Limit != PageSize+1 || f.filter.HideReports != HideReports {
		t.Errorf("filter: %+v", f.filter)
	}

	// курсор указывает на последнюю показанную анкету
	cur, err := decodeCursor(*page.NextCursor)
	if err != nil || cur.UserID != page.Items[PageSize-1].UserID {
		t.Errorf("cursor: %+v %v", cur, err)
	}

	f.rows = f.rows[:3]
	page, _ = svc.Search(ctx, 7, Query{LookingFor: "team"})
	if len(page.Items) != 3 || page.NextCursor != nil {
		t.Errorf("last page: items=%d cursor=%v", len(page.Items), page.NextCursor)
	}
	if _, err := svc.Search(ctx, 7, Query{Cursor: "%%%"}); !errors.Is(err, ErrBadCursor) {
		t.Errorf("bad cursor: %v", err)
	}
}

func TestMineSuggestedRank(t *testing.T) {
	f, ctx := newFake(), context.Background()
	svc := newSvc(f)
	user := store.User{ID: 1, AccountID: 500}

	mine, err := svc.Mine(ctx, user)
	if err != nil || mine.Card != nil || mine.SuggestedRank != nil {
		t.Fatalf("empty: %+v %v", mine, err)
	}

	tier := 53 // Легенда 3
	f.cache[500] = store.PlayerCache{AccountID: 500, RankTier: &tier}
	mine, _ = svc.Mine(ctx, user)
	if mine.SuggestedRank == nil || *mine.SuggestedRank != 5 {
		t.Errorf("suggested rank: %v", mine.SuggestedRank)
	}

	// отслеживаемый профиль приоритетнее собственного аккаунта
	tracked := int64(900)
	tier = 81
	f.cache[900] = store.PlayerCache{AccountID: 900, RankTier: &tier}
	user.TrackedAccountID = &tracked
	mine, _ = svc.Mine(ctx, user)
	if mine.SuggestedRank == nil || *mine.SuggestedRank != 8 {
		t.Errorf("tracked rank: %v", mine.SuggestedRank)
	}

	// калибровка (rank_tier < 10) — подсказки нет
	tier = 0
	delete(f.cache, 900)
	f.cache[500] = store.PlayerCache{AccountID: 500, RankTier: &tier}
	mine, _ = svc.Mine(ctx, user)
	if mine.SuggestedRank != nil {
		t.Errorf("calibration should give no suggestion: %v", *mine.SuggestedRank)
	}
}
