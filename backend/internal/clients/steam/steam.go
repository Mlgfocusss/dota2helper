// Package steam — клиент Steam Web API: профили игроков и новости Dota 2.
package steam

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"d2helper/internal/clients/httpx"
)

const DefaultBaseURL = "https://api.steampowered.com"

// ErrNoKey — для метода нужен STEAM_API_KEY, а он не задан.
var ErrNoKey = errors.New("steam: STEAM_API_KEY is not set")

type Client struct {
	base string
	key  string
	http *httpx.Client
}

func New(key string, logger *slog.Logger) *Client {
	return &Client{
		base: DefaultBaseURL,
		key:  key,
		http: httpx.New(httpx.Options{
			Name:       "steam",
			PerMinute:  120,
			Timeout:    15 * time.Second,
			MaxRetries: 2,
			Logger:     logger,
		}),
	}
}

// WithBaseURL — для тестов.
func (c *Client) WithBaseURL(base string) *Client {
	c.base = base
	return c
}

// HasKey — задан ли ключ Web API (без него недоступны профили и поиск по vanity URL).
func (c *Client) HasKey() bool { return c.key != "" }

type PlayerSummary struct {
	SteamID     string `json:"steamid"`
	PersonaName string `json:"personaname"`
	AvatarFull  string `json:"avatarfull"`
	ProfileURL  string `json:"profileurl"`
}

// PlayerSummaries — ники и аватары по SteamID64 (до 100 за запрос).
func (c *Client) PlayerSummaries(ctx context.Context, steamIDs ...string) ([]PlayerSummary, error) {
	if c.key == "" {
		return nil, ErrNoKey
	}
	q := url.Values{"key": {c.key}, "steamids": {strings.Join(steamIDs, ",")}}
	var out struct {
		Response struct {
			Players []PlayerSummary `json:"players"`
		} `json:"response"`
	}
	if err := c.http.GetJSON(ctx, c.base+"/ISteamUser/GetPlayerSummaries/v2/?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Response.Players, nil
}

// NewsItem — элемент ISteamNews/GetNewsForApp; contents в BBCode.
type NewsItem struct {
	GID       string `json:"gid"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Author    string `json:"author"`
	Contents  string `json:"contents"`
	FeedLabel string `json:"feedlabel"`
	Date      int64  `json:"date"`
}

// News — последние анонсы приложения (для Dota 2 appid = 570). Ключ не нужен.
func (c *Client) News(ctx context.Context, appID, count int) ([]NewsItem, error) {
	q := url.Values{
		"appid":     {strconv.Itoa(appID)},
		"count":     {strconv.Itoa(count)},
		"maxlength": {"0"},
		"feeds":     {"steam_community_announcements"},
	}
	var out struct {
		AppNews struct {
			NewsItems []NewsItem `json:"newsitems"`
		} `json:"appnews"`
	}
	if err := c.http.GetJSON(ctx, c.base+"/ISteamNews/GetNewsForApp/v2/?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.AppNews.NewsItems, nil
}

// ErrVanityNotFound — у Steam нет профиля с таким коротким именем.
var ErrVanityNotFound = errors.New("steam: vanity url not found")

// ResolveVanity переводит короткое имя профиля (steamcommunity.com/id/{vanity}) в SteamID64.
func (c *Client) ResolveVanity(ctx context.Context, vanity string) (string, error) {
	if c.key == "" {
		return "", ErrNoKey
	}
	q := url.Values{"key": {c.key}, "vanityurl": {vanity}}
	var out struct {
		Response struct {
			Success int    `json:"success"`
			SteamID string `json:"steamid"`
		} `json:"response"`
	}
	if err := c.http.GetJSON(ctx, c.base+"/ISteamUser/ResolveVanityURL/v1/?"+q.Encode(), nil, &out); err != nil {
		return "", err
	}
	if out.Response.Success != 1 || out.Response.SteamID == "" {
		return "", ErrVanityNotFound
	}
	return out.Response.SteamID, nil
}
