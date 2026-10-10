package dota

import (
	"sort"
	"strconv"
	"strings"

	"d2helper/internal/clients/opendota"
)

const heroCDN = "https://cdn.cloudflare.steamstatic.com/apps/dota2/images/dota_react/heroes"

// Hero — герой в формате API (/api/heroes).
type Hero struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	LocalizedName string   `json:"localized_name"`
	ShortName     string   `json:"short_name"`
	PrimaryAttr   string   `json:"primary_attr"`
	Roles         []string `json:"roles"`
	Img           string   `json:"img"`
	Icon          string   `json:"icon"`
}

// HeroPortraitURL — портрет 256×144 с CDN Valve.
func HeroPortraitURL(shortName string) string {
	return heroCDN + "/" + shortName + ".png"
}

// HeroIconURL — маленькая квадратная иконка.
func HeroIconURL(shortName string) string {
	return heroCDN + "/icons/" + shortName + ".png"
}

// HeroesFromOpenDota превращает /constants/heroes в список героев по алфавиту.
func HeroesFromOpenDota(constants map[string]opendota.HeroConstant) []Hero {
	heroes := make([]Hero, 0, len(constants))
	for key, h := range constants {
		id := h.ID
		if id == 0 {
			id, _ = strconv.Atoi(key)
		}
		short := strings.TrimPrefix(h.Name, "npc_dota_hero_")
		if id == 0 || short == "" {
			continue
		}
		roles := h.Roles
		if roles == nil {
			roles = []string{}
		}
		heroes = append(heroes, Hero{
			ID:            id,
			Name:          h.Name,
			LocalizedName: h.LocalizedName,
			ShortName:     short,
			PrimaryAttr:   h.PrimaryAttr,
			Roles:         roles,
			Img:           HeroPortraitURL(short),
			Icon:          HeroIconURL(short),
		})
	}
	sort.Slice(heroes, func(i, j int) bool {
		return heroes[i].LocalizedName < heroes[j].LocalizedName
	})
	return heroes
}
