package coach

import (
	"context"
	"errors"
	"time"

	"d2helper/internal/clients/opendota"
)

// ItemRef — предмет: key для картинки на CDN Valve, name для подсказки.
type ItemRef struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type MatchPlayer struct {
	AccountID   *int64     `json:"account_id"`
	Name        string     `json:"name"`
	HeroID      int        `json:"hero_id"`
	Team        string     `json:"team"` // "radiant" или "dire"
	Level       int        `json:"level"`
	Kills       int        `json:"kills"`
	Deaths      int        `json:"deaths"`
	Assists     int        `json:"assists"`
	LastHits    int        `json:"last_hits"`
	Denies      int        `json:"denies"`
	GPM         int        `json:"gpm"`
	XPM         int        `json:"xpm"`
	NetWorth    int        `json:"net_worth"`
	HeroDamage  int        `json:"hero_damage"`
	TowerDamage int        `json:"tower_damage"`
	Healing     int        `json:"healing"`
	Items       []*ItemRef `json:"items"` // 6 слотов, null — пусто
	Neutral     *ItemRef   `json:"neutral"`
	IsMe        bool       `json:"is_me"`
}

type MatchDetail struct {
	MatchID      int64         `json:"match_id"`
	RadiantWin   bool          `json:"radiant_win"`
	Win          *bool         `json:"win"` // результат просматриваемого игрока; null — его нет в матче
	Duration     int           `json:"duration"`
	StartTime    time.Time     `json:"start_time"`
	GameMode     int           `json:"game_mode"`
	LobbyType    int           `json:"lobby_type"`
	RadiantScore int           `json:"radiant_score"`
	DireScore    int           `json:"dire_score"`
	Players      []MatchPlayer `json:"players"`
}

// Match — подробности матча. accountID (0 — не указан) помечает «своего» игрока.
func (s *Service) Match(ctx context.Context, matchID, accountID int64) (*MatchDetail, error) {
	m, err := s.od.Match(ctx, matchID)
	if errors.Is(err, opendota.ErrNotFound) {
		return nil, ErrMatchNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(m.Players) == 0 {
		return nil, ErrMatchNotFound
	}
	byID := s.itemNames(ctx)

	out := &MatchDetail{
		MatchID: m.MatchID, RadiantWin: m.RadiantWin, Duration: m.Duration,
		StartTime: time.Unix(m.StartTime, 0).UTC(), GameMode: m.GameMode, LobbyType: m.LobbyType,
		RadiantScore: m.RadiantScore, DireScore: m.DireScore,
		Players: make([]MatchPlayer, 0, len(m.Players)),
	}
	for _, p := range m.Players {
		radiant := p.PlayerSlot < 128
		mp := MatchPlayer{
			AccountID: p.AccountID, Name: playerName(p), HeroID: p.HeroID, Team: "dire",
			Level: p.Level, Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists,
			LastHits: p.LastHits, Denies: p.Denies, GPM: p.GPM, XPM: p.XPM, NetWorth: p.NetWorth,
			HeroDamage: p.HeroDamage, TowerDamage: p.TowerDamage, Healing: p.HeroHealing,
			Items: make([]*ItemRef, 0, 6), Neutral: itemRef(byID, p.ItemNeutral),
		}
		if radiant {
			mp.Team = "radiant"
		}
		for _, id := range p.Items() {
			mp.Items = append(mp.Items, itemRef(byID, id))
		}
		if accountID != 0 && p.AccountID != nil && *p.AccountID == accountID {
			mp.IsMe = true
			win := radiant == m.RadiantWin
			out.Win = &win
		}
		out.Players = append(out.Players, mp)
	}
	return out, nil
}

func playerName(p opendota.MatchPlayer) string {
	if p.PersonaName != nil && *p.PersonaName != "" {
		return *p.PersonaName
	}
	if p.Name != nil {
		return *p.Name
	}
	return ""
}

func itemRef(byID map[int]itemInfo, id int) *ItemRef {
	if id == 0 {
		return nil
	}
	it, ok := byID[id]
	if !ok {
		return nil
	}
	return &ItemRef{Key: it.key, Name: it.name}
}

type itemInfo struct{ key, name string }

const itemsTTL = 24 * time.Hour

// itemNames — справочник предметов по id; при недоступности OpenDota предметы просто не показываются.
func (s *Service) itemNames(ctx context.Context) map[int]itemInfo {
	s.itemsMu.Lock()
	defer s.itemsMu.Unlock()
	if s.itemIndex != nil && s.now().Sub(s.itemsAt) < itemsTTL {
		return s.itemIndex
	}
	consts, err := s.od.ItemConstants(ctx)
	if err != nil {
		s.logger.Warn("coach: item constants", "err", err)
		return s.itemIndex
	}
	index := make(map[int]itemInfo, len(consts))
	for key, it := range consts {
		index[it.ID] = itemInfo{key: key, name: it.DName}
	}
	s.itemIndex, s.itemsAt = index, s.now()
	return index
}
