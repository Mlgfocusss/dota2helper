// Package patchnotes — патчноуты Dota 2 для окна патча на странице новости.
package patchnotes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"d2helper/internal/clients/dota2"
)

const (
	cacheTTL = 6 * time.Hour
	// патчей в истории немного, но ограничиваем память от перебора версий
	maxCached = 32
)

var (
	ErrNotFound = errors.New("patchnotes: not found")

	versionRe = regexp.MustCompile(`^\d{1,2}\.\d{2}[a-z]?$`)
	// номер патча в заголовке новости: «7.41f Gameplay Patch», «Gameplay Patch 7.41e and …»
	titleVersionRe = regexp.MustCompile(`(?:^|\s)(\d{1,2}\.\d{2}[a-z]?)\b`)
)

// VersionFromTitle достаёт номер патча из заголовка новости; пусто — это не новость о патче.
func VersionFromTitle(title string) string {
	if !strings.Contains(strings.ToLower(title), "patch") {
		return ""
	}
	if m := titleVersionRe.FindStringSubmatch(title); m != nil {
		return m[1]
	}
	return ""
}

type Source interface {
	Patch(ctx context.Context, version string) (*dota2.Patch, error)
	AbilityNames(ctx context.Context) (map[int]string, error)
}

type Note struct {
	Indent int    `json:"indent"`
	Text   string `json:"text"`
}

// Section — блок заметок с заголовком (способность, предмет, раздел).
type Section struct {
	Title string `json:"title"`
	Notes []Note `json:"notes"`
}

type Hero struct {
	HeroID    int       `json:"hero_id"`
	Notes     []Note    `json:"notes"`
	Abilities []Section `json:"abilities"`
	Talents   []Note    `json:"talents"`
}

type Patch struct {
	Number  string    `json:"number"`
	Date    time.Time `json:"date"`
	General []Section `json:"general"`
	Items   []Section `json:"items"`
	Neutral []Section `json:"neutral_items"`
	Heroes  []Hero    `json:"heroes"`
}

type cached struct {
	patch  *Patch
	loaded time.Time
}

type Service struct {
	src    Source
	logger *slog.Logger
	now    func() time.Time

	mu      sync.Mutex
	patches map[string]cached
	names   map[int]string
	namesAt time.Time
}

func New(src Source, logger *slog.Logger) *Service {
	return &Service{src: src, logger: logger, now: time.Now, patches: map[string]cached{}}
}

// Get возвращает патчноуты версии (7.41f); результат кэшируется на несколько часов.
func (s *Service) Get(ctx context.Context, version string) (*Patch, error) {
	version = strings.ToLower(strings.TrimSpace(version))
	if !versionRe.MatchString(version) {
		return nil, ErrNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.patches[version]; ok && s.now().Sub(c.loaded) < cacheTTL {
		return c.patch, nil
	}
	raw, err := s.src.Patch(ctx, version)
	if errors.Is(err, dota2.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load patch %s: %w", version, err)
	}
	p := convert(raw, s.abilityNames(ctx))
	if len(s.patches) >= maxCached {
		clear(s.patches)
	}
	s.patches[version] = cached{patch: p, loaded: s.now()}
	return p, nil
}

// abilityNames — справочник названий; без него патч всё равно показывается, но с номерами.
func (s *Service) abilityNames(ctx context.Context) map[int]string {
	if s.names != nil && s.now().Sub(s.namesAt) < cacheTTL {
		return s.names
	}
	names, err := s.src.AbilityNames(ctx)
	if err != nil {
		s.logger.Warn("patchnotes: ability names", "err", err)
		return s.names
	}
	s.names, s.namesAt = names, s.now()
	return names
}

func convert(raw *dota2.Patch, names map[int]string) *Patch {
	p := &Patch{
		Number:  raw.Name,
		Date:    time.Unix(raw.Timestamp, 0).UTC(),
		General: []Section{},
		Heroes:  []Hero{},
	}
	for _, g := range raw.General {
		p.General = append(p.General, Section{Title: g.Title, Notes: notes(g.Notes)})
	}
	p.Items = entries(raw.Items, names)
	p.Neutral = entries(raw.Neutral, names)
	for _, h := range raw.Heroes {
		hero := Hero{
			HeroID:    h.HeroID,
			Notes:     notes(h.Notes),
			Talents:   notes(h.Talents),
			Abilities: entries(h.Abilities, names),
		}
		if len(hero.Notes)+len(hero.Talents)+len(hero.Abilities) > 0 {
			p.Heroes = append(p.Heroes, hero)
		}
	}
	return p
}

func entries(list []dota2.Entry, names map[int]string) []Section {
	out := []Section{}
	for _, e := range list {
		title := e.Title
		if title == "" {
			title = names[e.AbilityID]
		}
		if title == "" {
			title = fmt.Sprintf("Способность %d", e.AbilityID)
		}
		out = append(out, Section{Title: title, Notes: notes(e.Notes)})
	}
	return out
}

// notes убирает служебные строки‑переносы, которыми Valve разделяет блоки.
func notes(list []dota2.Note) []Note {
	out := []Note{}
	for _, n := range list {
		text := strings.TrimSpace(strings.NewReplacer("<br>", " ", "<br/>", " ").Replace(n.Text))
		if text == "" {
			continue
		}
		out = append(out, Note{Indent: max(1, n.Indent), Text: text})
	}
	return out
}
