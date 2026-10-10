// Package dota — справочники и утилиты Dota 2: герои, патч, идентификаторы игроков.
package dota

import (
	"errors"
	"strconv"
)

// steamID64Base — SteamID64 = account_id + эта константа (индивидуальные аккаунты).
const steamID64Base = 76561197960265728

var ErrBadSteamID = errors.New("некорректный SteamID")

// AccountIDFromSteamID64 переводит SteamID64 ("7656119…") в account_id из Dota.
func AccountIDFromSteamID64(steamID64 string) (int64, error) {
	id, err := strconv.ParseUint(steamID64, 10, 64)
	if err != nil || len(steamID64) != 17 || id <= steamID64Base {
		return 0, ErrBadSteamID
	}
	accountID := id - steamID64Base
	if accountID > 1<<32-1 {
		return 0, ErrBadSteamID
	}
	return int64(accountID), nil
}

// SteamID64FromAccountID — обратное преобразование.
func SteamID64FromAccountID(accountID int64) string {
	return strconv.FormatUint(uint64(accountID)+steamID64Base, 10)
}
