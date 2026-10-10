// Package opendota — типизированный клиент OpenDota API (https://docs.opendota.com).
package opendota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"d2helper/internal/cache"
	"d2helper/internal/clients/httpx"
)

const DefaultBaseURL = "https://api.opendota.com/api"

// ErrNotFound — OpenDota не знает такого игрока или матч.
var ErrNotFound = errors.New("opendota: not found")

// Сколько держать ответы в памяти. Ошибки не кэшируются; если OpenDota
// недоступна, ещё час можно отдавать последний успешный ответ.
const (
	playerTTL     = 5 * time.Minute
	heroStatsTTL  = time.Hour
	cacheEntries  = 2000
	cacheStaleFor = time.Hour
)

type Client struct {
	base   string
	apiKey string
	http   *httpx.Client
	cache  *cache.TTL[[]byte]
}

// New создаёт клиент. Без ключа у OpenDota низкий лимит, поэтому perMinute по умолчанию 50.
func New(apiKey string, perMinute int, logger *slog.Logger) *Client {
	return &Client{
		base:   DefaultBaseURL,
		apiKey: apiKey,
		http: httpx.New(httpx.Options{
			Name:       "opendota",
			PerMinute:  perMinute,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
			Logger:     logger,
		}),
		cache: cache.New[[]byte](cacheEntries, cacheStaleFor),
	}
}

// WithBaseURL — для тестов.
func (c *Client) WithBaseURL(base string) *Client {
	c.base = base
	return c
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	q := url.Values{}
	for k, v := range query {
		q[k] = v
	}
	if c.apiKey != "" {
		q.Set("api_key", c.apiKey)
	}
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	err := c.http.GetJSON(ctx, u, nil, out)
	if httpx.IsStatus(err, http.StatusNotFound) {
		return ErrNotFound
	}
	return err
}

// getCached — get с кэшем ответа в памяти на ttl.
func (c *Client) getCached(ctx context.Context, path string, query url.Values, ttl time.Duration, out any) error {
	key := path + "?" + query.Encode()
	raw, err := c.cache.GetOrLoad(ctx, key, ttl, func(ctx context.Context) ([]byte, error) {
		var msg json.RawMessage
		if err := c.get(ctx, path, query, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// HeroConstant — элемент /constants/heroes.
type HeroConstant struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	LocalizedName string   `json:"localized_name"`
	PrimaryAttr   string   `json:"primary_attr"`
	AttackType    string   `json:"attack_type"`
	Roles         []string `json:"roles"`
}

// HeroConstants — справочник героев, ключ — id героя строкой.
func (c *Client) HeroConstants(ctx context.Context) (map[string]HeroConstant, error) {
	var out map[string]HeroConstant
	if err := c.get(ctx, "/constants/heroes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PatchConstant — элемент /constants/patch.
type PatchConstant struct {
	ID   int       `json:"id"`
	Name string    `json:"name"`
	Date time.Time `json:"date"`
}

func (c *Client) PatchConstants(ctx context.Context) ([]PatchConstant, error) {
	var out []PatchConstant
	if err := c.get(ctx, "/constants/patch", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

type PlayerProfile struct {
	AccountID   int64  `json:"account_id"`
	PersonaName string `json:"personaname"`
	AvatarFull  string `json:"avatarfull"`
	ProfileURL  string `json:"profileurl"`
}

// Player — ответ /players/{account_id}. Profile = nil, если OpenDota не видела игрока.
type Player struct {
	Profile         *PlayerProfile `json:"profile"`
	RankTier        *int           `json:"rank_tier"`
	LeaderboardRank *int           `json:"leaderboard_rank"`
}

func (c *Client) Player(ctx context.Context, accountID int64) (*Player, error) {
	var out Player
	if err := c.getCached(ctx, "/players/"+strconv.FormatInt(accountID, 10), nil, playerTTL, &out); err != nil {
		return nil, fmt.Errorf("player %d: %w", accountID, err)
	}
	return &out, nil
}

// HeroStat — элемент /heroStats: пики и победы по брекетам рангов за последнее время.
// Брекеты 1–8 — от Рекрута до Титана, pub — все публичные матчи.
type HeroStat struct {
	ID      int `json:"id"`
	Pick1   int `json:"1_pick"`
	Win1    int `json:"1_win"`
	Pick2   int `json:"2_pick"`
	Win2    int `json:"2_win"`
	Pick3   int `json:"3_pick"`
	Win3    int `json:"3_win"`
	Pick4   int `json:"4_pick"`
	Win4    int `json:"4_win"`
	Pick5   int `json:"5_pick"`
	Win5    int `json:"5_win"`
	Pick6   int `json:"6_pick"`
	Win6    int `json:"6_win"`
	Pick7   int `json:"7_pick"`
	Win7    int `json:"7_win"`
	Pick8   int `json:"8_pick"`
	Win8    int `json:"8_win"`
	PubPick int `json:"pub_pick"`
	PubWin  int `json:"pub_win"`
}

// Bracket — пики и победы в брекете 1–8; 0 — все публичные матчи.
func (h HeroStat) Bracket(n int) (picks, wins int) {
	switch n {
	case 1:
		return h.Pick1, h.Win1
	case 2:
		return h.Pick2, h.Win2
	case 3:
		return h.Pick3, h.Win3
	case 4:
		return h.Pick4, h.Win4
	case 5:
		return h.Pick5, h.Win5
	case 6:
		return h.Pick6, h.Win6
	case 7:
		return h.Pick7, h.Win7
	case 8:
		return h.Pick8, h.Win8
	}
	return h.PubPick, h.PubWin
}

func (c *Client) HeroStats(ctx context.Context) ([]HeroStat, error) {
	var out []HeroStat
	if err := c.getCached(ctx, "/heroStats", nil, heroStatsTTL, &out); err != nil {
		return nil, fmt.Errorf("hero stats: %w", err)
	}
	return out, nil
}

// MatchSummary — матч игрока из /players/{id}/matches (только нужные поля).
type MatchSummary struct {
	MatchID    int64 `json:"match_id"`
	PlayerSlot int   `json:"player_slot"`
	RadiantWin bool  `json:"radiant_win"`
	Duration   int   `json:"duration"`
	GameMode   int   `json:"game_mode"`
	LobbyType  int   `json:"lobby_type"`
	HeroID     int   `json:"hero_id"`
	StartTime  int64 `json:"start_time"`
	Kills      int   `json:"kills"`
	Deaths     int   `json:"deaths"`
	Assists    int   `json:"assists"`
	LaneRole   *int  `json:"lane_role"`
	PartySize  *int  `json:"party_size"`
}

// Win — выиграл ли игрок: слоты 0–127 — Силы Света, 128+ — Силы Тьмы.
func (m MatchSummary) Win() bool { return (m.PlayerSlot < 128) == m.RadiantWin }

var matchProjection = []string{
	"match_id", "player_slot", "radiant_win", "duration", "game_mode", "lobby_type",
	"hero_id", "start_time", "kills", "deaths", "assists", "lane_role", "party_size",
}

// PlayerMatches — последние limit матчей игрока (новые первыми). Пустой список
// значит, что история скрыта или матчей ещё нет. Не кэшируется: синк сам решает, когда ходить.
func (c *Client) PlayerMatches(ctx context.Context, accountID int64, limit int) ([]MatchSummary, error) {
	var out []MatchSummary
	q := url.Values{"limit": {strconv.Itoa(limit)}, "project": matchProjection}
	if err := c.get(ctx, "/players/"+strconv.FormatInt(accountID, 10)+"/matches", q, &out); err != nil {
		return nil, fmt.Errorf("player %d matches: %w", accountID, err)
	}
	return out, nil
}

// WinLoss — ответ /players/{id}/wl.
type WinLoss struct {
	Win  int `json:"win"`
	Lose int `json:"lose"`
}

// PlayerWL — победы и поражения за последние days дней.
func (c *Client) PlayerWL(ctx context.Context, accountID int64, days int) (WinLoss, error) {
	var out WinLoss
	q := url.Values{"date": {strconv.Itoa(days)}}
	if err := c.getCached(ctx, "/players/"+strconv.FormatInt(accountID, 10)+"/wl", q, playerTTL, &out); err != nil {
		return WinLoss{}, fmt.Errorf("player %d wl: %w", accountID, err)
	}
	return out, nil
}

// RefreshPlayer просит OpenDota заново загрузить историю матчей игрока.
func (c *Client) RefreshPlayer(ctx context.Context, accountID int64) error {
	u := c.base + "/players/" + strconv.FormatInt(accountID, 10) + "/refresh"
	if c.apiKey != "" {
		u += "?" + url.Values{"api_key": {c.apiKey}}.Encode()
	}
	if err := c.http.PostJSON(ctx, u, nil, struct{}{}, nil); err != nil {
		return fmt.Errorf("refresh player %d: %w", accountID, err)
	}
	return nil
}

// MatchPlayer — игрок в /matches/{id}.
type MatchPlayer struct {
	AccountID   *int64  `json:"account_id"`
	PersonaName *string `json:"personaname"`
	Name        *string `json:"name"`
	HeroID      int     `json:"hero_id"`
	PlayerSlot  int     `json:"player_slot"`
	Level       int     `json:"level"`
	Kills       int     `json:"kills"`
	Deaths      int     `json:"deaths"`
	Assists     int     `json:"assists"`
	LastHits    int     `json:"last_hits"`
	Denies      int     `json:"denies"`
	GPM         int     `json:"gold_per_min"`
	XPM         int     `json:"xp_per_min"`
	NetWorth    int     `json:"net_worth"`
	HeroDamage  int     `json:"hero_damage"`
	TowerDamage int     `json:"tower_damage"`
	HeroHealing int     `json:"hero_healing"`
	Item0       int     `json:"item_0"`
	Item1       int     `json:"item_1"`
	Item2       int     `json:"item_2"`
	Item3       int     `json:"item_3"`
	Item4       int     `json:"item_4"`
	Item5       int     `json:"item_5"`
	ItemNeutral int     `json:"item_neutral"`
}

// Items — шесть основных слотов предметов (id, 0 — пусто).
func (p MatchPlayer) Items() [6]int {
	return [6]int{p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5}
}

// MatchDetail — ответ /matches/{id} (нужные поля).
type MatchDetail struct {
	MatchID      int64         `json:"match_id"`
	Duration     int           `json:"duration"`
	StartTime    int64         `json:"start_time"`
	RadiantWin   bool          `json:"radiant_win"`
	RadiantScore int           `json:"radiant_score"`
	DireScore    int           `json:"dire_score"`
	GameMode     int           `json:"game_mode"`
	LobbyType    int           `json:"lobby_type"`
	Players      []MatchPlayer `json:"players"`
}

const matchTTL = 10 * time.Minute

func (c *Client) Match(ctx context.Context, matchID int64) (*MatchDetail, error) {
	var out MatchDetail
	if err := c.getCached(ctx, "/matches/"+strconv.FormatInt(matchID, 10), nil, matchTTL, &out); err != nil {
		return nil, fmt.Errorf("match %d: %w", matchID, err)
	}
	return &out, nil
}

// ItemConstant — элемент /constants/items.
type ItemConstant struct {
	ID    int    `json:"id"`
	DName string `json:"dname"`
	Cost  int    `json:"cost"`
}

// ItemConstants — справочник предметов, ключ — короткое имя («blink»).
func (c *Client) ItemConstants(ctx context.Context) (map[string]ItemConstant, error) {
	var out map[string]ItemConstant
	if err := c.getCached(ctx, "/constants/items", nil, 24*time.Hour, &out); err != nil {
		return nil, fmt.Errorf("item constants: %w", err)
	}
	return out, nil
}

// HeroMatchup — как герой (из пути запроса) играл против HeroID.
type HeroMatchup struct {
	HeroID      int `json:"hero_id"`
	GamesPlayed int `json:"games_played"`
	Wins        int `json:"wins"`
}

const heroPageTTL = time.Hour

// HeroMatchups — /heroes/{id}/matchups: результаты героя против каждого соперника.
func (c *Client) HeroMatchups(ctx context.Context, heroID int) ([]HeroMatchup, error) {
	var out []HeroMatchup
	if err := c.getCached(ctx, "/heroes/"+strconv.Itoa(heroID)+"/matchups", nil, heroPageTTL, &out); err != nil {
		return nil, fmt.Errorf("hero %d matchups: %w", heroID, err)
	}
	return out, nil
}

// ItemPopularity — /heroes/{id}/itemPopularity: id предмета → сколько раз куплен, по стадиям игры.
type ItemPopularity struct {
	Start map[string]int `json:"start_game_items"`
	Early map[string]int `json:"early_game_items"`
	Mid   map[string]int `json:"mid_game_items"`
	Late  map[string]int `json:"late_game_items"`
}

func (c *Client) HeroItemPopularity(ctx context.Context, heroID int) (*ItemPopularity, error) {
	var out ItemPopularity
	if err := c.getCached(ctx, "/heroes/"+strconv.Itoa(heroID)+"/itemPopularity", nil, heroPageTTL, &out); err != nil {
		return nil, fmt.Errorf("hero %d item popularity: %w", heroID, err)
	}
	return &out, nil
}

// ----- распарсенный матч для разбора ---------------------------------------------

// LogEntry — событие из логов игрока (убийства, покупки…): время в секундах от начала.
type LogEntry struct {
	Time int    `json:"time"`
	Key  string `json:"key"`
}

// DeathEntry — смерть игрока: кем убит, сколько золота потеряно и сколько секунд мёртв.
type DeathEntry struct {
	Time     int    `json:"time"`
	Key      string `json:"key"`
	GoldLost int    `json:"gold_lost"`
	TimeDead int    `json:"time_dead"`
}

// Benchmark — значение игрока и его процентиль среди игроков на этом герое.
type Benchmark struct {
	Raw float64 `json:"raw"`
	Pct float64 `json:"pct"`
}

// WardEntry — установка варда (время и координаты на карте).
type WardEntry struct {
	Time int `json:"time"`
	// координаты на карте — дробные
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type TeamfightPlayer struct {
	Deaths    int                       `json:"deaths"`
	Buybacks  int                       `json:"buybacks"`
	Damage    int                       `json:"damage"`
	Healing   int                       `json:"healing"`
	GoldDelta int                       `json:"gold_delta"`
	XPDelta   int                       `json:"xp_delta"`
	DeathsPos map[string]map[string]int `json:"deaths_pos"`
}

type Teamfight struct {
	Start     int               `json:"start"`
	End       int               `json:"end"`
	LastDeath int               `json:"last_death"`
	Deaths    int               `json:"deaths"`
	Players   []TeamfightPlayer `json:"players"`
}

// ParsedPlayer — игрок распарсенного матча (только то, что нужно разбору).
type ParsedPlayer struct {
	AccountID    *int64   `json:"account_id"`
	PlayerSlot   int      `json:"player_slot"`
	HeroID       int      `json:"hero_id"`
	IsRadiant    bool     `json:"isRadiant"`
	Win          int      `json:"win"`
	Kills        int      `json:"kills"`
	Deaths       int      `json:"deaths"`
	Assists      int      `json:"assists"`
	LastHits     int      `json:"last_hits"`
	Denies       int      `json:"denies"`
	GPM          int      `json:"gold_per_min"`
	XPM          int      `json:"xp_per_min"`
	Level        int      `json:"level"`
	NetWorth     int      `json:"net_worth"`
	HeroDamage   int      `json:"hero_damage"`
	TowerDamage  int      `json:"tower_damage"`
	HeroHealing  int      `json:"hero_healing"`
	Lane         int      `json:"lane"`
	LaneRole     int      `json:"lane_role"`
	IsRoaming    bool     `json:"is_roaming"`
	LaneEffPct   *float64 `json:"lane_efficiency_pct"`
	ObsPlaced    int      `json:"obs_placed"`
	SenPlaced    int      `json:"sen_placed"`
	CampsStacked int      `json:"camps_stacked"`
	// доля участия в драках, 0..1
	TeamfightParticipation float64 `json:"teamfight_participation"`
	BuybackCount           int     `json:"buyback_count"`

	LHT         []int                `json:"lh_t"`
	DNT         []int                `json:"dn_t"`
	GoldT       []int                `json:"gold_t"`
	XPT         []int                `json:"xp_t"`
	DeathsLog   []DeathEntry         `json:"deaths_log"`
	PurchaseLog []LogEntry           `json:"purchase_log"`
	BuybackLog  []LogEntry           `json:"buyback_log"`
	ObsLog      []WardEntry          `json:"obs_log"`
	SenLog      []WardEntry          `json:"sen_log"`
	Benchmarks  map[string]Benchmark `json:"benchmarks"`
	Item0       int                  `json:"item_0"`
	Item1       int                  `json:"item_1"`
	Item2       int                  `json:"item_2"`
	Item3       int                  `json:"item_3"`
	Item4       int                  `json:"item_4"`
	Item5       int                  `json:"item_5"`
}

// Items — шесть основных слотов предметов (id, 0 — пусто).
func (p ParsedPlayer) Items() [6]int {
	return [6]int{p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5}
}

// ParsedMatch — /matches/{id} с данными парсера реплея. Version == nil — матч ещё не разобран.
type ParsedMatch struct {
	MatchID        int64          `json:"match_id"`
	Version        *int           `json:"version"`
	Duration       int            `json:"duration"`
	StartTime      int64          `json:"start_time"`
	RadiantWin     bool           `json:"radiant_win"`
	RadiantScore   int            `json:"radiant_score"`
	DireScore      int            `json:"dire_score"`
	GameMode       int            `json:"game_mode"`
	LobbyType      int            `json:"lobby_type"`
	FirstBloodTime int            `json:"first_blood_time"`
	Teamfights     []Teamfight    `json:"teamfights"`
	Players        []ParsedPlayer `json:"players"`
}

// Parsed сообщает, разобран ли реплей матча.
func (m *ParsedMatch) Parsed() bool { return m.Version != nil }

// ParsedMatch загружает матч целиком. Без кэша: ответ большой, а нужен один раз на разбор.
func (c *Client) ParsedMatch(ctx context.Context, matchID int64) (*ParsedMatch, error) {
	var out ParsedMatch
	if err := c.get(ctx, "/matches/"+strconv.FormatInt(matchID, 10), nil, &out); err != nil {
		return nil, fmt.Errorf("match %d: %w", matchID, err)
	}
	return &out, nil
}

// RequestParse просит OpenDota разобрать реплей матча (POST /request/{id}).
func (c *Client) RequestParse(ctx context.Context, matchID int64) error {
	u := c.base + "/request/" + strconv.FormatInt(matchID, 10)
	if c.apiKey != "" {
		u += "?" + url.Values{"api_key": {c.apiKey}}.Encode()
	}
	if err := c.http.PostJSON(ctx, u, nil, struct{}{}, nil); err != nil {
		return fmt.Errorf("request parse %d: %w", matchID, err)
	}
	return nil
}
