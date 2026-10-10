package auth

import (
	"context"
	"log/slog"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/clients/steam"
)

type SteamSummaries interface {
	HasKey() bool
	PlayerSummaries(ctx context.Context, steamIDs ...string) ([]steam.PlayerSummary, error)
}

type OpenDotaPlayers interface {
	Player(ctx context.Context, accountID int64) (*opendota.Player, error)
}

// Profiles ищет ник и аватар при входе. Steam Web API отдаёт актуальные данные,
// поэтому он первый, если задан ключ; иначе — профиль из OpenDota.
type Profiles struct {
	steam  SteamSummaries
	od     OpenDotaPlayers
	logger *slog.Logger
}

func NewProfiles(s SteamSummaries, od OpenDotaPlayers, logger *slog.Logger) *Profiles {
	return &Profiles{steam: s, od: od, logger: logger}
}

// Lookup никогда не мешает входу: если оба источника недоступны, вернёт пустые строки.
func (p *Profiles) Lookup(ctx context.Context, steamID64 string, accountID int64) (persona, avatar string) {
	if p.steam.HasKey() {
		players, err := p.steam.PlayerSummaries(ctx, steamID64)
		if err == nil && len(players) > 0 {
			return players[0].PersonaName, players[0].AvatarFull
		}
		if err != nil {
			p.logger.Warn("steam profile lookup", "err", err)
		}
	}
	player, err := p.od.Player(ctx, accountID)
	if err != nil {
		p.logger.Warn("opendota profile lookup", "err", err)
		return "", ""
	}
	if player.Profile == nil {
		return "", ""
	}
	return player.Profile.PersonaName, player.Profile.AvatarFull
}
