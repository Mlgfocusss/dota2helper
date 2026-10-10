// Package coach — раздел «Тренер»: профиль игрока, синк его матчей из OpenDota, сводка и страница матча.
package coach

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/clients/steam"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

const (
	// syncEvery — как давно обновлённый кэш игрока считается свежим (SPEC 5.3)
	syncEvery = 10 * time.Minute
	// matchLimit — сколько последних матчей подтягиваем за один синк
	matchLimit = 100
	// refreshEvery — «обновить» можно раз в минуту на игрока (SPEC 4.4)
	refreshEvery = time.Minute
	// sixMonthsDays — окно для «побед за 6 месяцев»
	sixMonthsDays = 180
	formHeroes    = 3
	formMinGames  = 3
	lockStripes   = 64
)

var (
	ErrBadInput       = errors.New("coach: bad profile input")
	ErrNoSteamKey     = errors.New("coach: steam key is not set")
	ErrPlayerNotFound = errors.New("coach: player not found")
	ErrMatchNotFound  = errors.New("coach: match not found")
	ErrBadFilter      = errors.New("coach: bad filter")
	ErrBadCursor      = errors.New("coach: bad cursor")
)

// TooSoonError — «обновить» нажали раньше чем через минуту.
type TooSoonError struct{ RetryAfter time.Duration }

func (e *TooSoonError) Error() string { return "coach: refresh is too frequent" }

type (
	OpenDota interface {
		Player(ctx context.Context, accountID int64) (*opendota.Player, error)
		PlayerMatches(ctx context.Context, accountID int64, limit int) ([]opendota.MatchSummary, error)
		PlayerWL(ctx context.Context, accountID int64, days int) (opendota.WinLoss, error)
		RefreshPlayer(ctx context.Context, accountID int64) error
		Match(ctx context.Context, matchID int64) (*opendota.MatchDetail, error)
		ItemConstants(ctx context.Context) (map[string]opendota.ItemConstant, error)
	}
	Steam interface {
		ResolveVanity(ctx context.Context, vanity string) (string, error)
	}
	Store interface {
		GetPlayerCache(ctx context.Context, accountID int64) (store.PlayerCache, error)
		PutPlayerCache(ctx context.Context, p store.PlayerCache) error
		UpsertPlayerMatches(ctx context.Context, matches []store.PlayerMatch) error
		ListPlayerMatches(ctx context.Context, accountID int64, filter string, cur *store.MatchCursor, limit int) ([]store.PlayerMatch, error)
		PlayerMatchCounts(ctx context.Context, accountID int64) (store.MatchCounts, error)
		PlayerHeroForm(ctx context.Context, accountID int64, since time.Time, minMatches int) ([]store.HeroForm, error)
	}
	Patches interface {
		Patch() (dota.Patch, bool)
		PatchStart() (time.Time, bool)
	}
)

type Service struct {
	od      OpenDota
	steam   Steam
	store   Store
	patches Patches
	logger  *slog.Logger
	now     func() time.Time

	locks [lockStripes]sync.Mutex

	refreshMu sync.Mutex
	refreshAt map[int64]time.Time

	itemsMu   sync.Mutex
	itemIndex map[int]itemInfo
	itemsAt   time.Time
}

func New(od OpenDota, st Steam, s Store, patches Patches, logger *slog.Logger) *Service {
	return &Service{od: od, steam: st, store: s, patches: patches, logger: logger, now: time.Now, refreshAt: map[int64]time.Time{}}
}

// ----- поиск профиля ---------------------------------------------------------

// Resolved — найденный игрок.
type Resolved struct {
	AccountID   int64   `json:"account_id"`
	PersonaName *string `json:"persona_name"`
	Avatar      *string `json:"avatar"`
}

// Resolve понимает ссылку/ID из строки ввода и проверяет, что игрок существует (SPEC 4.2).
func (s *Service) Resolve(ctx context.Context, input string) (*Resolved, error) {
	ref, err := dota.ParseProfileInput(input)
	if err != nil {
		return nil, ErrBadInput
	}
	accountID := ref.AccountID
	switch {
	case ref.SteamID64 != "":
		accountID, err = dota.AccountIDFromSteamID64(ref.SteamID64)
		if err != nil {
			return nil, ErrBadInput
		}
	case ref.Vanity != "":
		steamID, err := s.steam.ResolveVanity(ctx, ref.Vanity)
		switch {
		case errors.Is(err, steam.ErrNoKey):
			return nil, ErrNoSteamKey
		case errors.Is(err, steam.ErrVanityNotFound):
			return nil, ErrPlayerNotFound
		case err != nil:
			return nil, fmt.Errorf("resolve vanity: %w", err)
		}
		if accountID, err = dota.AccountIDFromSteamID64(steamID); err != nil {
			return nil, ErrPlayerNotFound
		}
	}

	player, err := s.od.Player(ctx, accountID)
	if errors.Is(err, opendota.ErrNotFound) {
		return nil, ErrPlayerNotFound
	}
	if err != nil {
		return nil, err
	}
	if player.Profile == nil {
		return nil, ErrPlayerNotFound
	}
	return &Resolved{
		AccountID:   accountID,
		PersonaName: nonEmpty(player.Profile.PersonaName),
		Avatar:      nonEmpty(player.Profile.AvatarFull),
	}, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ----- синк ------------------------------------------------------------------

func (s *Service) lock(accountID int64) *sync.Mutex {
	return &s.locks[uint64(accountID)%lockStripes]
}

// ensureSynced подтягивает профиль и матчи игрока, если кэш старше syncEvery (или force).
// Если OpenDota недоступна, а в БД уже есть данные — отдаём их.
func (s *Service) ensureSynced(ctx context.Context, accountID int64, force bool) error {
	mu := s.lock(accountID)
	mu.Lock()
	defer mu.Unlock()

	cached, err := s.store.GetPlayerCache(ctx, accountID)
	have := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if have && !force && s.now().Sub(cached.UpdatedAt) < syncEvery {
		return nil
	}

	player, err := s.od.Player(ctx, accountID)
	if errors.Is(err, opendota.ErrNotFound) {
		return ErrPlayerNotFound
	}
	if err != nil {
		return s.syncFailed(have, accountID, err)
	}
	matches, err := s.od.PlayerMatches(ctx, accountID, matchLimit)
	if err != nil {
		return s.syncFailed(have, accountID, err)
	}
	if player.Profile == nil && len(matches) == 0 && !have {
		return ErrPlayerNotFound
	}

	rows := make([]store.PlayerMatch, 0, len(matches))
	for _, m := range matches {
		rows = append(rows, toStoreMatch(accountID, m))
	}
	if err := s.store.UpsertPlayerMatches(ctx, rows); err != nil {
		return fmt.Errorf("save matches: %w", err)
	}
	profile, _ := json.Marshal(player.Profile)
	return s.store.PutPlayerCache(ctx, store.PlayerCache{
		AccountID: accountID, Profile: profile,
		RankTier: player.RankTier, LeaderboardRank: player.LeaderboardRank,
	})
}

func (s *Service) syncFailed(haveData bool, accountID int64, err error) error {
	if !haveData {
		return err
	}
	s.logger.Warn("coach: sync failed, serving saved data", "account_id", accountID, "err", err)
	return nil
}

func toStoreMatch(accountID int64, m opendota.MatchSummary) store.PlayerMatch {
	return store.PlayerMatch{
		AccountID: accountID, MatchID: m.MatchID, HeroID: m.HeroID, PlayerSlot: m.PlayerSlot,
		RadiantWin: m.RadiantWin, Win: m.Win(), Kills: m.Kills, Deaths: m.Deaths, Assists: m.Assists,
		Duration: m.Duration, StartTime: time.Unix(m.StartTime, 0).UTC(),
		GameMode: m.GameMode, LobbyType: m.LobbyType, LaneRole: m.LaneRole, PartySize: m.PartySize,
	}
}

// Refresh — кнопка «Проверить снова»: просим OpenDota перезагрузить историю и синкаем заново.
func (s *Service) Refresh(ctx context.Context, accountID int64) error {
	if wait := s.refreshWait(accountID); wait > 0 {
		return &TooSoonError{RetryAfter: wait}
	}
	if err := s.od.RefreshPlayer(ctx, accountID); err != nil {
		// не критично: ниже всё равно перечитаем то, что у OpenDota уже есть
		s.logger.Warn("coach: opendota refresh", "account_id", accountID, "err", err)
	}
	return s.ensureSynced(ctx, accountID, true)
}

// refreshWait занимает слот обновления; >0 — сколько ещё ждать.
func (s *Service) refreshWait(accountID int64) time.Duration {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	now := s.now()
	if last, ok := s.refreshAt[accountID]; ok && now.Sub(last) < refreshEvery {
		return refreshEvery - now.Sub(last)
	}
	if len(s.refreshAt) > 10000 {
		for id, at := range s.refreshAt {
			if now.Sub(at) >= refreshEvery {
				delete(s.refreshAt, id)
			}
		}
	}
	s.refreshAt[accountID] = now
	return 0
}

// ----- сводка ----------------------------------------------------------------

type Player struct {
	AccountID       int64   `json:"account_id"`
	PersonaName     *string `json:"persona_name"`
	Avatar          *string `json:"avatar"`
	RankTier        *int    `json:"rank_tier"`
	LeaderboardRank *int    `json:"leaderboard_rank"`
}

type MatchRow struct {
	MatchID   int64     `json:"match_id"`
	HeroID    int       `json:"hero_id"`
	Win       bool      `json:"win"`
	Kills     int       `json:"kills"`
	Deaths    int       `json:"deaths"`
	Assists   int       `json:"assists"`
	Duration  int       `json:"duration"`
	StartTime time.Time `json:"start_time"`
	GameMode  int       `json:"game_mode"`
	// Analysis — разбор этого матча у текущего пользователя; заполняет HTTP-слой
	Analysis *store.AnalysisRef `json:"analysis,omitempty"`
}

type Last20 struct {
	Wins   int      `json:"wins"`
	Losses int      `json:"losses"`
	Result []string `json:"results"` // от старых к новым: "W" / "L"
}

type SixMonths struct {
	Winrate float64 `json:"winrate"`
	Matches int     `json:"matches"`
}

type FormHero struct {
	HeroID  int     `json:"hero_id"`
	Winrate float64 `json:"winrate"`
	Matches int     `json:"matches"`
}

type FormPeriod struct {
	Period string     `json:"period"` // "all" или номер патча
	Heroes []FormHero `json:"heroes"`
}

type Summary struct {
	Player        Player       `json:"player"`
	HasHistory    bool         `json:"has_history"`
	Last20        Last20       `json:"last20"`
	SixMonths     SixMonths    `json:"six_months"`
	HeroesInForm  []FormPeriod `json:"heroes_in_form"`
	LastMatch     *MatchRow    `json:"last_match"`
	Patch         dota.Patch   `json:"patch"`
	MatchesStored int          `json:"matches_stored"`
}

func (s *Service) Summary(ctx context.Context, accountID int64) (*Summary, error) {
	if err := s.ensureSynced(ctx, accountID, false); err != nil {
		return nil, err
	}
	player, err := s.player(ctx, accountID)
	if err != nil {
		return nil, err
	}
	counts, err := s.store.PlayerMatchCounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	recent, err := s.store.ListPlayerMatches(ctx, accountID, "", nil, 20)
	if err != nil {
		return nil, err
	}

	sum := &Summary{
		Player:        player,
		HasHistory:    counts.All > 0,
		Last20:        last20(recent),
		HeroesInForm:  []FormPeriod{},
		MatchesStored: counts.All,
	}
	if len(recent) > 0 {
		row := toRow(recent[0])
		sum.LastMatch = &row
	}
	if patch, ok := s.patches.Patch(); ok {
		sum.Patch = patch
	}

	if wl, err := s.od.PlayerWL(ctx, accountID, sixMonthsDays); err != nil {
		s.logger.Warn("coach: wl", "account_id", accountID, "err", err)
	} else if total := wl.Win + wl.Lose; total > 0 {
		sum.SixMonths = SixMonths{Winrate: float64(wl.Win) / float64(total), Matches: total}
	}

	if sum.HasHistory {
		all, err := s.store.PlayerHeroForm(ctx, accountID, time.Time{}, formMinGames)
		if err != nil {
			return nil, err
		}
		sum.HeroesInForm = append(sum.HeroesInForm, FormPeriod{Period: "all", Heroes: topForm(all, formHeroes)})
		if start, ok := s.patches.PatchStart(); ok && sum.Patch.Number != "" {
			cur, err := s.store.PlayerHeroForm(ctx, accountID, start, formMinGames)
			if err != nil {
				return nil, err
			}
			sum.HeroesInForm = append(sum.HeroesInForm, FormPeriod{Period: sum.Patch.Number, Heroes: topForm(cur, formHeroes)})
		}
	}
	return sum, nil
}

func (s *Service) player(ctx context.Context, accountID int64) (Player, error) {
	p := Player{AccountID: accountID}
	cached, err := s.store.GetPlayerCache(ctx, accountID)
	if errors.Is(err, store.ErrNotFound) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	p.RankTier, p.LeaderboardRank = cached.RankTier, cached.LeaderboardRank
	var profile *opendota.PlayerProfile
	if json.Unmarshal(cached.Profile, &profile) == nil && profile != nil {
		p.PersonaName = nonEmpty(profile.PersonaName)
		p.Avatar = nonEmpty(profile.AvatarFull)
	}
	return p, nil
}

func toRow(m store.PlayerMatch) MatchRow {
	return MatchRow{
		MatchID: m.MatchID, HeroID: m.HeroID, Win: m.Win, Kills: m.Kills, Deaths: m.Deaths,
		Assists: m.Assists, Duration: m.Duration, StartTime: m.StartTime, GameMode: m.GameMode,
	}
}

// last20 считает победы и поражения; recent — от новых к старым, в ответе — от старых к новым.
func last20(recent []store.PlayerMatch) Last20 {
	out := Last20{Result: make([]string, 0, len(recent))}
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].Win {
			out.Wins++
			out.Result = append(out.Result, "W")
		} else {
			out.Losses++
			out.Result = append(out.Result, "L")
		}
	}
	return out
}

// topForm — лучшие герои в форме: сортировка по winrate * ln(matches+1) (SPEC 5.3),
// чтобы 3 победы из 3 не перебивали 18 из 25.
func topForm(forms []store.HeroForm, n int) []FormHero {
	out := make([]FormHero, 0, len(forms))
	for _, f := range forms {
		if f.Matches == 0 {
			continue
		}
		out = append(out, FormHero{HeroID: f.HeroID, Winrate: float64(f.Wins) / float64(f.Matches), Matches: f.Matches})
	}
	score := func(h FormHero) float64 { return h.Winrate * math.Log(float64(h.Matches)+1) }
	sort.Slice(out, func(i, j int) bool {
		if si, sj := score(out[i]), score(out[j]); si != sj {
			return si > sj
		}
		if out[i].Matches != out[j].Matches {
			return out[i].Matches > out[j].Matches
		}
		return out[i].HeroID < out[j].HeroID
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// ----- список матчей ---------------------------------------------------------

type MatchesPage struct {
	Items      []MatchRow        `json:"items"`
	Counts     store.MatchCounts `json:"counts"`
	NextCursor *string           `json:"next_cursor"`
}

// Matches — сохранённые матчи игрока; filter: "", "all", "win" или "loss".
func (s *Service) Matches(ctx context.Context, accountID int64, filter, cursor string, limit int) (*MatchesPage, error) {
	switch filter {
	case "all":
		filter = ""
	case "", "win", "loss":
	default:
		return nil, ErrBadFilter
	}
	cur, err := parseCursor(cursor)
	if err != nil {
		return nil, err
	}
	if err := s.ensureSynced(ctx, accountID, false); err != nil {
		return nil, err
	}
	counts, err := s.store.PlayerMatchCounts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	// на один больше страницы — так видно, есть ли следующая
	rows, err := s.store.ListPlayerMatches(ctx, accountID, filter, cur, limit+1)
	if err != nil {
		return nil, err
	}
	page := &MatchesPage{Items: make([]MatchRow, 0, len(rows)), Counts: counts}
	if len(rows) > limit {
		rows = rows[:limit]
		next := formatCursor(rows[len(rows)-1])
		page.NextCursor = &next
	}
	for _, m := range rows {
		page.Items = append(page.Items, toRow(m))
	}
	return page, nil
}

func formatCursor(m store.PlayerMatch) string {
	return strconv.FormatInt(m.StartTime.Unix(), 10) + ":" + strconv.FormatInt(m.MatchID, 10)
}

func parseCursor(raw string) (*store.MatchCursor, error) {
	if raw == "" {
		return nil, nil
	}
	ts, id, ok := strings.Cut(raw, ":")
	if !ok {
		return nil, ErrBadCursor
	}
	sec, err1 := strconv.ParseInt(ts, 10, 64)
	matchID, err2 := strconv.ParseInt(id, 10, 64)
	if err1 != nil || err2 != nil || sec < 0 || matchID < 0 {
		return nil, ErrBadCursor
	}
	return &store.MatchCursor{StartTime: time.Unix(sec, 0).UTC(), MatchID: matchID}, nil
}
