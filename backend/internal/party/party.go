// Package party — анкеты и поиск пати: валидация анкеты, подъём, жалобы, выдача.
package party

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"d2helper/internal/store"
)

const (
	MaxAbout  = 300
	MaxHours  = 50000
	MaxFavs   = 3
	MaxRank   = 8
	PageSize  = 12
	MaxTag    = 40
	maxHeroID = 1000

	// BumpCooldown — как часто можно поднимать анкету (SPEC 4.4)
	BumpCooldown = 24 * time.Hour
	// ExpireAfter — анкета без активности дольше этого срока скрывается джобой party_expire
	ExpireAfter = 14 * 24 * time.Hour
	// HideReports — столько жалоб скрывают анкету до проверки
	HideReports = 3
)

var (
	Modes   = []string{"ranked", "normal", "turbo"}
	Goals   = []string{"rating", "fun", "stack", "scrim"}
	Looking = []string{"team", "players"}
	Reasons = []string{"spam", "abuse", "fake"}
)

var (
	ErrNoCard          = errors.New("party: no card")
	ErrBadReason       = errors.New("party: bad report reason")
	ErrSelfReport      = errors.New("party: report on self")
	ErrAlreadyReported = errors.New("party: already reported")
	ErrBadCursor       = errors.New("party: bad cursor")
)

var (
	telegramRe = regexp.MustCompile(`^[a-zA-Z0-9_]{5,32}$`)
	discordRe  = regexp.MustCompile(`^[\p{L}\p{N}_.\-#]{2,40}$`)
)

// TooSoonError — подъём раньше срока.
type TooSoonError struct{ RetryAfter time.Duration }

func (e *TooSoonError) Error() string { return "party: bump too soon" }

// ValidationError — ошибки по полям формы: имя поля → текст для пользователя.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return fmt.Sprintf("party: invalid fields: %v", e.Fields) }

// Input — анкета в том виде, в каком её присылает форма.
type Input struct {
	RankMedal  int      `json:"rank_medal"`
	Hours      int      `json:"hours"`
	Positions  []int    `json:"positions"`
	Modes      []string `json:"modes"`
	Goal       string   `json:"goal"`
	LookingFor string   `json:"looking_for"`
	Mic        bool     `json:"mic"`
	Discord    bool     `json:"discord"`
	DiscordTag string   `json:"discord_tag"`
	Telegram   string   `json:"telegram"`
	FavHeroes  []int    `json:"fav_heroes"`
	About      string   `json:"about"`
}

// Card — анкета в ответах API.
type Card struct {
	UserID      int64     `json:"user_id"`
	AccountID   int64     `json:"account_id"`
	PersonaName string    `json:"persona_name"`
	Avatar      string    `json:"avatar"`
	RankMedal   int       `json:"rank_medal"`
	Hours       int       `json:"hours"`
	Positions   []int16   `json:"positions"`
	Modes       []string  `json:"modes"`
	Goal        string    `json:"goal"`
	LookingFor  string    `json:"looking_for"`
	Mic         bool      `json:"mic"`
	Discord     bool      `json:"discord"`
	DiscordTag  string    `json:"discord_tag,omitempty"`
	Telegram    string    `json:"telegram,omitempty"`
	FavHeroes   []int32   `json:"fav_heroes"`
	About       string    `json:"about"`
	IsActive    bool      `json:"is_active"`
	BumpedAt    time.Time `json:"bumped_at"`
	// NextBumpAt — когда можно поднять снова; только в «моей» анкете
	NextBumpAt *time.Time `json:"next_bump_at,omitempty"`
}

func cardFrom(p store.PartyProfile) Card {
	return Card{
		UserID: p.UserID, AccountID: p.AccountID, PersonaName: p.PersonaName, Avatar: p.AvatarURL,
		RankMedal: p.RankMedal, Hours: p.Hours, Positions: p.Positions, Modes: p.Modes,
		Goal: p.Goal, LookingFor: p.LookingFor, Mic: p.Mic, Discord: p.Discord,
		DiscordTag: p.DiscordTag, Telegram: p.Telegram, FavHeroes: p.FavHeroes, About: p.About,
		IsActive: p.IsActive, BumpedAt: p.BumpedAt,
	}
}

// Normalize проверяет и приводит анкету к виду для хранения; ошибки — *ValidationError.
func (in Input) Normalize(knownHero func(id int) bool) (store.PartyProfile, error) {
	bad := map[string]string{}
	p := store.PartyProfile{
		RankMedal: in.RankMedal, Hours: in.Hours, Goal: in.Goal, LookingFor: in.LookingFor,
		Mic: in.Mic, Discord: in.Discord,
	}

	if in.RankMedal < 0 || in.RankMedal > MaxRank {
		bad["rank_medal"] = "Выбери ранг"
	}
	if in.Hours < 0 || in.Hours > MaxHours {
		bad["hours"] = fmt.Sprintf("Часов в игре — от 0 до %d", MaxHours)
	}

	positions := slices.Clone(in.Positions)
	slices.Sort(positions)
	positions = slices.Compact(positions)
	if len(positions) == 0 || positions[0] < 1 || positions[len(positions)-1] > 5 {
		bad["positions"] = "Выбери хотя бы одну позицию"
	} else {
		for _, v := range positions {
			p.Positions = append(p.Positions, int16(v))
		}
	}

	for _, m := range Modes { // порядок фиксирован, дубликаты отпадают
		if slices.Contains(in.Modes, m) {
			p.Modes = append(p.Modes, m)
		}
	}
	if len(p.Modes) == 0 {
		bad["modes"] = "Выбери хотя бы один режим"
	}
	for _, m := range in.Modes {
		if !slices.Contains(Modes, m) {
			bad["modes"] = "Неизвестный режим игры"
		}
	}

	if !slices.Contains(Goals, in.Goal) {
		bad["goal"] = "Выбери цель"
	}
	if !slices.Contains(Looking, in.LookingFor) {
		bad["looking_for"] = "Выбери, кого ты ищешь"
	}

	if in.Discord {
		tag := strings.TrimSpace(in.DiscordTag)
		if !discordRe.MatchString(tag) {
			bad["discord_tag"] = "Укажи ник в Discord"
		}
		p.DiscordTag = tag
	}

	tg := strings.TrimPrefix(strings.TrimSpace(in.Telegram), "@")
	if tg != "" {
		if !telegramRe.MatchString(tg) {
			bad["telegram"] = "Ник в Telegram: 5–32 символа, латиница, цифры и «_»"
		}
		p.Telegram = tg
	}

	if len(in.FavHeroes) > MaxFavs {
		bad["fav_heroes"] = fmt.Sprintf("Любимых героев не больше %d", MaxFavs)
	}
	seen := map[int]bool{}
	for _, id := range in.FavHeroes {
		switch {
		case id < 1 || id > maxHeroID || (knownHero != nil && !knownHero(id)):
			bad["fav_heroes"] = "Такого героя нет"
		case seen[id]:
		default:
			seen[id] = true
			p.FavHeroes = append(p.FavHeroes, int32(id))
		}
	}
	if p.FavHeroes == nil {
		p.FavHeroes = []int32{}
	}

	about := strings.TrimSpace(in.About)
	switch {
	case utf8.RuneCountInString(about) > MaxAbout:
		bad["about"] = fmt.Sprintf("Слишком длинный текст: максимум %d символов", MaxAbout)
	case ContainsForbidden(about):
		bad["about"] = "В тексте нельзя ссылки и грубые выражения"
	}
	p.About = about

	if len(bad) > 0 {
		return store.PartyProfile{}, &ValidationError{Fields: bad}
	}
	return p, nil
}

type Store interface {
	PartyProfileByUser(ctx context.Context, userID int64) (store.PartyProfile, error)
	UpsertPartyProfile(ctx context.Context, p store.PartyProfile) error
	SetPartyActive(ctx context.Context, userID int64, active bool) error
	DeletePartyProfile(ctx context.Context, userID int64) error
	BumpParty(ctx context.Context, userID int64, cooldown time.Duration) (time.Time, bool, error)
	SearchParty(ctx context.Context, f store.PartyFilter) ([]store.PartyProfile, error)
	ReportParty(ctx context.Context, reporterID, targetID int64, reason string) error
	ExpirePartyProfiles(ctx context.Context, idle time.Duration) (int64, error)
	GetPlayerCache(ctx context.Context, accountID int64) (store.PlayerCache, error)
}

type Service struct {
	store     Store
	knownHero func(id int) bool
	logger    *slog.Logger
	now       func() time.Time
}

// New создаёт сервис. knownHero может быть nil — тогда проверяется только диапазон id.
func New(st Store, knownHero func(id int) bool, logger *slog.Logger) *Service {
	return &Service{store: st, knownHero: knownHero, logger: logger, now: time.Now}
}

// Mine — «моя анкета» и подсказка ранга для формы.
type Mine struct {
	Card *Card `json:"card"`
	// SuggestedRank — медаль из ранга OpenDota (последний синк «Тренера»); nil, если ранга нет
	SuggestedRank *int `json:"suggested_rank"`
}

func (s *Service) Mine(ctx context.Context, user store.User) (*Mine, error) {
	out := &Mine{}
	p, err := s.store.PartyProfileByUser(ctx, user.ID)
	switch {
	case err == nil:
		card := s.mineCard(p)
		out.Card = &card
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}
	for _, id := range []int64{trackedOrOwn(user), user.AccountID} {
		pc, err := s.store.GetPlayerCache(ctx, id)
		if err == nil && pc.RankTier != nil && *pc.RankTier >= 10 {
			medal := min(MaxRank, *pc.RankTier/10)
			out.SuggestedRank = &medal
			break
		}
	}
	return out, nil
}

func trackedOrOwn(u store.User) int64 {
	if u.TrackedAccountID != nil {
		return *u.TrackedAccountID
	}
	return u.AccountID
}

func (s *Service) mineCard(p store.PartyProfile) Card {
	card := cardFrom(p)
	next := p.BumpedAt.Add(BumpCooldown)
	card.NextBumpAt = &next
	return card
}

// Save создаёт или обновляет анкету пользователя.
func (s *Service) Save(ctx context.Context, userID int64, in Input) (*Card, error) {
	p, err := in.Normalize(s.knownHero)
	if err != nil {
		return nil, err
	}
	p.UserID = userID
	if err := s.store.UpsertPartyProfile(ctx, p); err != nil {
		return nil, err
	}
	return s.mine(ctx, userID)
}

func (s *Service) mine(ctx context.Context, userID int64) (*Card, error) {
	p, err := s.store.PartyProfileByUser(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNoCard
	}
	if err != nil {
		return nil, err
	}
	card := s.mineCard(p)
	return &card, nil
}

// SetActive скрывает или показывает анкету.
func (s *Service) SetActive(ctx context.Context, userID int64, active bool) (*Card, error) {
	if err := s.store.SetPartyActive(ctx, userID, active); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoCard
		}
		return nil, err
	}
	return s.mine(ctx, userID)
}

func (s *Service) Delete(ctx context.Context, userID int64) error {
	return s.store.DeletePartyProfile(ctx, userID)
}

// Bump поднимает анкету наверх выдачи: не чаще раза в BumpCooldown.
func (s *Service) Bump(ctx context.Context, userID int64) (*Card, error) {
	last, ok, err := s.store.BumpParty(ctx, userID, BumpCooldown)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNoCard
	}
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, &TooSoonError{RetryAfter: last.Add(BumpCooldown).Sub(s.now())}
	}
	return s.mine(ctx, userID)
}

// Query — параметры поиска, уже разобранные из запроса.
type Query struct {
	LookingFor string
	RankMin    int
	RankMax    int
	Positions  []int16
	Modes      []string
	Goals      []string
	Mic        bool
	Discord    bool
	Cursor     string
}

// Page — страница выдачи.
type Page struct {
	Items      []Card  `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

// Search ищет чужие анкеты (viewerID — кто смотрит; свою анкету не показываем).
func (s *Service) Search(ctx context.Context, viewerID int64, q Query) (*Page, error) {
	cursor, err := decodeCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	rows, err := s.store.SearchParty(ctx, store.PartyFilter{
		ViewerID: viewerID, LookingFor: q.LookingFor, RankMin: q.RankMin, RankMax: q.RankMax,
		Positions: q.Positions, Modes: q.Modes, Goals: q.Goals, Mic: q.Mic, Discord: q.Discord,
		HideReports: HideReports, Limit: PageSize + 1, Cursor: cursor,
	})
	if err != nil {
		return nil, err
	}
	page := &Page{Items: make([]Card, 0, min(len(rows), PageSize))}
	for i, p := range rows {
		if i == PageSize {
			next := encodeCursor(rows[PageSize-1])
			page.NextCursor = &next
			break
		}
		page.Items = append(page.Items, cardFrom(p))
	}
	return page, nil
}

// Report сохраняет жалобу на анкету.
func (s *Service) Report(ctx context.Context, reporterID, targetID int64, reason string) error {
	switch {
	case !slices.Contains(Reasons, reason):
		return ErrBadReason
	case reporterID == targetID:
		return ErrSelfReport
	}
	err := s.store.ReportParty(ctx, reporterID, targetID, reason)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrNoCard
	case errors.Is(err, store.ErrAlreadyReported):
		return ErrAlreadyReported
	}
	return err
}

// Expire — джоба party_expire: скрывает давно забытые анкеты.
func (s *Service) Expire(ctx context.Context) error {
	n, err := s.store.ExpirePartyProfiles(ctx, ExpireAfter)
	if n > 0 {
		s.logger.Info("party profiles expired", "count", n)
	}
	return err
}
