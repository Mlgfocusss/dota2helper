package dota

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// HeroTags — описание героя из data/heroes_tags.json (SPEC 9.1): позиции,
// сложность, тайминг силы и теги стиля для квизов и фоллбэка меты.
type HeroTags struct {
	HeroID     int      `json:"hero_id"`
	Name       string   `json:"name,omitempty"`
	Positions  []int    `json:"positions"`
	Complexity int      `json:"complexity"`
	Timing     []string `json:"timing"`
	Tags       []string `json:"tags"`
}

// Has — есть ли у героя тег.
func (h HeroTags) Has(tag string) bool {
	for _, t := range h.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// AllowedTags — словарь тегов из SPEC 9.1.
var AllowedTags = []string{
	"physical", "magic", "hybrid", "illusions", "safe_farm", "mobility", "lane_strong",
	"ganker", "duel", "farmer", "burst_magic", "initiator", "tank", "aura", "kill_lane",
	"survive", "push", "level_dependent", "item_dependent", "control", "nuke", "save", "heal",
}

var allowedTiming = map[string]bool{"early": true, "mid": true, "late": true}

type HeroTagSet struct {
	byID map[int]HeroTags
}

// LoadHeroTags читает и проверяет heroes_tags.json. Ошибка в файле — ошибка запуска:
// это наши данные, лучше узнать о проблеме сразу.
func LoadHeroTags(path string) (*HeroTagSet, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read hero tags: %w", err)
	}
	var list []HeroTags
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	allowed := make(map[string]bool, len(AllowedTags))
	for _, t := range AllowedTags {
		allowed[t] = true
	}

	set := &HeroTagSet{byID: make(map[int]HeroTags, len(list))}
	var errs []error
	for _, h := range list {
		if err := validateHeroTags(h, allowed); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, dup := set.byID[h.HeroID]; dup {
			errs = append(errs, fmt.Errorf("hero %d: duplicate entry", h.HeroID))
			continue
		}
		set.byID[h.HeroID] = h
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("%s:\n%w", path, err)
	}
	return set, nil
}

func validateHeroTags(h HeroTags, allowed map[string]bool) error {
	if h.HeroID <= 0 {
		return fmt.Errorf("hero %q: bad hero_id %d", h.Name, h.HeroID)
	}
	if len(h.Positions) == 0 {
		return fmt.Errorf("hero %d: no positions", h.HeroID)
	}
	seen := map[int]bool{}
	for _, p := range h.Positions {
		if p < 1 || p > 5 || seen[p] {
			return fmt.Errorf("hero %d: bad positions %v", h.HeroID, h.Positions)
		}
		seen[p] = true
	}
	if h.Complexity < 1 || h.Complexity > 3 {
		return fmt.Errorf("hero %d: complexity must be 1..3", h.HeroID)
	}
	if len(h.Timing) == 0 {
		return fmt.Errorf("hero %d: no timing", h.HeroID)
	}
	for _, t := range h.Timing {
		if !allowedTiming[t] {
			return fmt.Errorf("hero %d: unknown timing %q", h.HeroID, t)
		}
	}
	for _, t := range h.Tags {
		if !allowed[t] {
			return fmt.Errorf("hero %d: unknown tag %q", h.HeroID, t)
		}
	}
	return nil
}

func (s *HeroTagSet) Get(heroID int) (HeroTags, bool) {
	h, ok := s.byID[heroID]
	return h, ok
}

func (s *HeroTagSet) Len() int { return len(s.byID) }

// HeroesForPosition — id героев, которые играются на позиции, по возрастанию.
func (s *HeroTagSet) HeroesForPosition(pos int) []int {
	var ids []int
	for id, h := range s.byID {
		for _, p := range h.Positions {
			if p == pos {
				ids = append(ids, id)
				break
			}
		}
	}
	sort.Ints(ids)
	return ids
}

// Missing — герои из справочника, для которых нет тегов (новый герой в игре).
func (s *HeroTagSet) Missing(heroes []Hero) []Hero {
	var out []Hero
	for _, h := range heroes {
		if _, ok := s.byID[h.ID]; !ok {
			out = append(out, h)
		}
	}
	return out
}
