package dota

import (
	"strings"

	"d2helper/internal/clients/opendota"
)

// Patch — текущий патч: номер из констант OpenDota и метка для интерфейса.
type Patch struct {
	Number string `json:"number"`
	Label  string `json:"label"`
}

// LatestPatch — номер самого нового патча из /constants/patch.
func LatestPatch(patches []opendota.PatchConstant) (string, bool) {
	if len(patches) == 0 {
		return "", false
	}
	latest := patches[0]
	for _, p := range patches[1:] {
		if p.ID > latest.ID {
			latest = p
		}
	}
	return latest.Name, latest.Name != ""
}

// PatchLabel — буквенная версия из PATCH_LABEL («7.41F»), если она относится
// к текущему патчу; иначе номер. Так устаревшая метка из env не переживёт
// выход нового патча.
func PatchLabel(number, envLabel string) string {
	if envLabel != "" && hasPatchPrefix(envLabel, number) {
		return envLabel
	}
	return number
}

// hasPatchPrefix: «7.41F» относится к «7.41», а «7.410» — нет.
func hasPatchPrefix(label, number string) bool {
	if !strings.HasPrefix(label, number) {
		return false
	}
	rest := label[len(number):]
	return rest == "" || (rest[0] >= 'a' && rest[0] <= 'z') || (rest[0] >= 'A' && rest[0] <= 'Z')
}
