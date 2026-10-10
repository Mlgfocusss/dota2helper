// Package dota2 — клиент публичного datafeed на dota2.com: патчноуты и названия способностей.
package dota2

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"time"

	"d2helper/internal/clients/httpx"
)

const DefaultBaseURL = "https://www.dota2.com/datafeed"

// ErrNotFound — для версии нет патчноутов.
var ErrNotFound = errors.New("dota2: patch notes not found")

type Client struct {
	base string
	http *httpx.Client
}

func New(logger *slog.Logger) *Client {
	return &Client{
		base: DefaultBaseURL,
		http: httpx.New(httpx.Options{
			Name:       "dota2com",
			PerMinute:  30,
			Timeout:    30 * time.Second,
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

// Note — строка патчноута; Indent — уровень вложенности (1 — верхний).
type Note struct {
	Indent int    `json:"indent_level"`
	Text   string `json:"note"`
}

// Entry — блок заметок: способность, предмет или заголовок раздела (ID = -1).
type Entry struct {
	AbilityID int    `json:"ability_id"`
	Title     string `json:"title"`
	Notes     []Note `json:"ability_notes"`
}

type HeroNotes struct {
	HeroID    int     `json:"hero_id"`
	Notes     []Note  `json:"hero_notes"`
	Talents   []Note  `json:"talent_notes"`
	Abilities []Entry `json:"abilities"`
}

type GeneralNotes struct {
	Title string `json:"title"`
	Notes []Note `json:"generic"`
}

type Patch struct {
	Number    string         `json:"patch_number"`
	Name      string         `json:"patch_name"`
	Timestamp int64          `json:"patch_timestamp"`
	General   []GeneralNotes `json:"general_notes"`
	Items     []Entry        `json:"items"`
	Neutral   []Entry        `json:"neutral_items"`
	Heroes    []HeroNotes    `json:"heroes"`
	Success   bool           `json:"success"`
}

// Patch — патчноуты версии вида 7.41f на русском. Неизвестная версия — ErrNotFound.
func (c *Client) Patch(ctx context.Context, version string) (*Patch, error) {
	q := url.Values{"version": {version}, "language": {"russian"}}
	var p Patch
	if err := c.http.GetJSON(ctx, c.base+"/patchnotes?"+q.Encode(), nil, &p); err != nil {
		return nil, err
	}
	if !p.Success {
		return nil, ErrNotFound
	}
	return &p, nil
}

// AbilityNames — названия способностей и предметов по id на русском: у способностей и
// предметов отдельные списки с общей нумерацией.
func (c *Client) AbilityNames(ctx context.Context) (map[int]string, error) {
	names := map[int]string{}
	var firstErr error
	for _, list := range []string{"abilitylist", "itemlist"} {
		if err := c.loadNames(ctx, list, names); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if len(names) == 0 {
		return nil, firstErr
	}
	return names, nil
}

func (c *Client) loadNames(ctx context.Context, list string, into map[int]string) error {
	var out struct {
		Result struct {
			Data struct {
				Abilities []struct {
					ID      int    `json:"id"`
					NameLoc string `json:"name_loc"`
					English string `json:"name_english_loc"`
				} `json:"itemabilities"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := c.http.GetJSON(ctx, c.base+"/"+list+"?language=russian", nil, &out); err != nil {
		return err
	}
	for _, a := range out.Result.Data.Abilities {
		name := a.NameLoc
		if name == "" {
			name = a.English
		}
		if name != "" {
			into[a.ID] = name
		}
	}
	return nil
}
