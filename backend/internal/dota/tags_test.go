package dota

import (
	"os"
	"path/filepath"
	"testing"
)

const tagsFile = "../../data/heroes_tags.json"

// Группы вариантов из квиза «Подбор героя» (SPEC 5.9): у героя должен найтись
// хотя бы один тег из каждого вопроса его основной позиции, иначе квиз
// никогда не сможет его посоветовать.
var quizGroups = map[int][][]string{
	1: {{"physical", "hybrid", "magic", "illusions"}, {"safe_farm", "mobility", "lane_strong"}},
	2: {{"ganker", "duel", "farmer"}, {"burst_magic", "physical", "control", "initiator"}},
	3: {{"initiator", "tank", "aura"}, {"kill_lane", "survive", "push"}},
	4: {{"level_dependent", "item_dependent"}, {"control", "nuke", "save", "heal"}},
	5: {{"level_dependent", "item_dependent"}, {"control", "nuke", "save", "heal"}},
}

func TestHeroTagsFileIsValid(t *testing.T) {
	set, err := LoadHeroTags(tagsFile)
	if err != nil {
		t.Fatalf("LoadHeroTags: %v", err)
	}
	if set.Len() < 120 {
		t.Errorf("в файле %d героев — ожидались все герои игры", set.Len())
	}
	for pos := 1; pos <= 5; pos++ {
		if n := len(set.HeroesForPosition(pos)); n < 15 {
			t.Errorf("позиция %d: всего %d героев", pos, n)
		}
	}
	for _, h := range set.byID {
		primary := h.Positions[0]
		for i, group := range quizGroups[primary] {
			found := false
			for _, tag := range group {
				if h.Has(tag) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s (%d), позиция %d: нет тегов для вопроса %d %v", h.Name, h.HeroID, primary, i+1, group)
			}
		}
	}
}

func TestLoadHeroTagsRejectsBadData(t *testing.T) {
	cases := map[string]string{
		"неизвестный тег":     `[{"hero_id":1,"positions":[1],"complexity":1,"timing":["mid"],"tags":["flying"]}]`,
		"позиция 6":           `[{"hero_id":1,"positions":[6],"complexity":1,"timing":["mid"],"tags":[]}]`,
		"сложность 0":         `[{"hero_id":1,"positions":[1],"complexity":0,"timing":["mid"],"tags":[]}]`,
		"дубль героя":         `[{"hero_id":1,"positions":[1],"complexity":1,"timing":["mid"],"tags":[]},{"hero_id":1,"positions":[2],"complexity":1,"timing":["mid"],"tags":[]}]`,
		"неизвестный тайминг": `[{"hero_id":1,"positions":[1],"complexity":1,"timing":["night"],"tags":[]}]`,
	}
	for name, body := range cases {
		path := filepath.Join(t.TempDir(), "tags.json")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadHeroTags(path); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

func TestMissingHeroes(t *testing.T) {
	set := &HeroTagSet{byID: map[int]HeroTags{1: {HeroID: 1}}}
	missing := set.Missing([]Hero{{ID: 1}, {ID: 999, LocalizedName: "New Hero"}})
	if len(missing) != 1 || missing[0].ID != 999 {
		t.Errorf("Missing = %+v", missing)
	}
}
