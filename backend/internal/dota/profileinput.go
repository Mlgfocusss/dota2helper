package dota

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ErrBadProfileInput — строка не похожа ни на один поддерживаемый формат.
var ErrBadProfileInput = errors.New("не удалось разобрать профиль")

// ProfileRef — что удалось понять из введённой пользователем строки.
// Заполнено ровно одно из полей: AccountID, SteamID64 или Vanity.
type ProfileRef struct {
	AccountID int64
	SteamID64 string
	// Vanity — короткое имя из steamcommunity.com/id/{vanity}; его ещё нужно
	// перевести в SteamID через Steam Web API
	Vanity string
}

const maxAccountID = 1<<32 - 1

var (
	vanityRe = regexp.MustCompile(`^[A-Za-z0-9_-]{2,64}$`)
	digitsRe = regexp.MustCompile(`^\d+$`)
)

// Сайты, у которых игрок открывается по /players/{account_id}.
var playerSites = map[string]bool{
	"opendota.com": true, "stratz.com": true, "dotabuff.com": true, "dota2protracker.com": true,
}

// ParseProfileInput понимает: ссылку steamcommunity.com/profiles/{steamid64},
// steamcommunity.com/id/{vanity}, SteamID64, account_id и ссылки
// OpenDota/STRATZ/Dotabuff вида /players/{account_id}.
func ParseProfileInput(input string) (ProfileRef, error) {
	s := strings.TrimSpace(input)
	if s == "" || len(s) > 300 {
		return ProfileRef{}, ErrBadProfileInput
	}
	if digitsRe.MatchString(s) {
		return refFromNumber(s)
	}

	// ссылка может быть без схемы: «steamcommunity.com/id/abc»
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" {
		return ProfileRef{}, ErrBadProfileInput
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	parts := splitPath(u.Path)

	switch {
	case host == "steamcommunity.com":
		if len(parts) >= 2 && parts[0] == "profiles" && digitsRe.MatchString(parts[1]) {
			return refFromNumber(parts[1])
		}
		if len(parts) >= 2 && parts[0] == "id" && vanityRe.MatchString(parts[1]) {
			return ProfileRef{Vanity: parts[1]}, nil
		}
	case playerSites[host]:
		if len(parts) >= 2 && parts[0] == "players" && digitsRe.MatchString(parts[1]) {
			return refFromAccountID(parts[1])
		}
	}
	return ProfileRef{}, ErrBadProfileInput
}

func splitPath(path string) []string {
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

// refFromNumber: 17 цифр — SteamID64, иначе account_id.
func refFromNumber(s string) (ProfileRef, error) {
	if len(s) == 17 {
		if _, err := AccountIDFromSteamID64(s); err != nil {
			return ProfileRef{}, ErrBadProfileInput
		}
		return ProfileRef{SteamID64: s}, nil
	}
	return refFromAccountID(s)
}

func refFromAccountID(s string) (ProfileRef, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id < 1 || id > maxAccountID {
		return ProfileRef{}, ErrBadProfileInput
	}
	return ProfileRef{AccountID: id}, nil
}
