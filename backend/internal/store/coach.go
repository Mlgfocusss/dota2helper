package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// PlayerCache — профиль игрока из OpenDota, сохранённый при последнем синке.
type PlayerCache struct {
	AccountID       int64
	Profile         []byte // JSON профиля OpenDota (ник, аватар)
	RankTier        *int
	LeaderboardRank *int
	UpdatedAt       time.Time
}

func (s *Store) GetPlayerCache(ctx context.Context, accountID int64) (PlayerCache, error) {
	var p PlayerCache
	err := s.pool.QueryRow(ctx, `
		SELECT account_id, profile, rank_tier, leaderboard_rank, updated_at
		FROM player_cache WHERE account_id = $1`, accountID).
		Scan(&p.AccountID, &p.Profile, &p.RankTier, &p.LeaderboardRank, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PlayerCache{}, ErrNotFound
	}
	return p, err
}

func (s *Store) PutPlayerCache(ctx context.Context, p PlayerCache) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO player_cache (account_id, profile, rank_tier, leaderboard_rank, updated_at)
		VALUES ($1, $2::jsonb, $3, $4, now())
		ON CONFLICT (account_id) DO UPDATE SET
			profile = EXCLUDED.profile, rank_tier = EXCLUDED.rank_tier,
			leaderboard_rank = EXCLUDED.leaderboard_rank, updated_at = now()`,
		p.AccountID, string(p.Profile), p.RankTier, p.LeaderboardRank)
	return err
}

// PlayerMatch — сохранённый матч игрока.
type PlayerMatch struct {
	AccountID  int64
	MatchID    int64
	HeroID     int
	PlayerSlot int
	RadiantWin bool
	Win        bool
	Kills      int
	Deaths     int
	Assists    int
	Duration   int
	StartTime  time.Time
	GameMode   int
	LobbyType  int
	LaneRole   *int
	PartySize  *int
}

// UpsertPlayerMatches сохраняет матчи; уже известные обновляются.
func (s *Store) UpsertPlayerMatches(ctx context.Context, matches []PlayerMatch) error {
	if len(matches) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, m := range matches {
		batch.Queue(`
			INSERT INTO player_matches (account_id, match_id, hero_id, player_slot, radiant_win, win,
				kills, deaths, assists, duration, start_time, game_mode, lobby_type, lane_role, party_size)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
			ON CONFLICT (account_id, match_id) DO UPDATE SET
				hero_id = EXCLUDED.hero_id, player_slot = EXCLUDED.player_slot, radiant_win = EXCLUDED.radiant_win,
				win = EXCLUDED.win, kills = EXCLUDED.kills, deaths = EXCLUDED.deaths, assists = EXCLUDED.assists,
				duration = EXCLUDED.duration, start_time = EXCLUDED.start_time, game_mode = EXCLUDED.game_mode,
				lobby_type = EXCLUDED.lobby_type, lane_role = EXCLUDED.lane_role, party_size = EXCLUDED.party_size`,
			m.AccountID, m.MatchID, m.HeroID, m.PlayerSlot, m.RadiantWin, m.Win, m.Kills, m.Deaths, m.Assists,
			m.Duration, m.StartTime, m.GameMode, m.LobbyType, m.LaneRole, m.PartySize)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// MatchCursor — позиция в списке матчей: следующая страница начинается строго после неё.
type MatchCursor struct {
	StartTime time.Time
	MatchID   int64
}

// ListPlayerMatches — матчи игрока от новых к старым. filter: "" (все), "win" или "loss".
func (s *Store) ListPlayerMatches(ctx context.Context, accountID int64, filter string, cur *MatchCursor, limit int) ([]PlayerMatch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT match_id, hero_id, coalesce(player_slot, 0), coalesce(radiant_win, false), win, kills, deaths, assists,
			duration, start_time, coalesce(game_mode, 0), coalesce(lobby_type, 0), lane_role, party_size
		FROM player_matches
		WHERE account_id = $1
		  AND ($2 = '' OR ($2 = 'win' AND win) OR ($2 = 'loss' AND NOT win))
		  AND ($3::timestamptz IS NULL OR (start_time, match_id) < ($3, $4))
		ORDER BY start_time DESC, match_id DESC
		LIMIT $5`,
		accountID, filter, cursorTime(cur), cursorID(cur), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PlayerMatch
	for rows.Next() {
		m := PlayerMatch{AccountID: accountID}
		if err := rows.Scan(&m.MatchID, &m.HeroID, &m.PlayerSlot, &m.RadiantWin, &m.Win, &m.Kills, &m.Deaths,
			&m.Assists, &m.Duration, &m.StartTime, &m.GameMode, &m.LobbyType, &m.LaneRole, &m.PartySize); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func cursorTime(c *MatchCursor) *time.Time {
	if c == nil {
		return nil
	}
	return &c.StartTime
}

func cursorID(c *MatchCursor) int64 {
	if c == nil {
		return 0
	}
	return c.MatchID
}

// MatchCounts — сколько матчей сохранено: всего, побед и поражений.
type MatchCounts struct {
	All  int `json:"all"`
	Win  int `json:"win"`
	Loss int `json:"loss"`
}

func (s *Store) PlayerMatchCounts(ctx context.Context, accountID int64) (MatchCounts, error) {
	var c MatchCounts
	err := s.pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE win), count(*) FILTER (WHERE NOT win)
		FROM player_matches WHERE account_id = $1`, accountID).Scan(&c.All, &c.Win, &c.Loss)
	return c, err
}

// HeroForm — результаты игрока на одном герое.
type HeroForm struct {
	HeroID  int
	Matches int
	Wins    int
}

// PlayerHeroForm — герои игрока с не менее чем minMatches матчей. since нулевой — за всё время.
func (s *Store) PlayerHeroForm(ctx context.Context, accountID int64, since time.Time, minMatches int) ([]HeroForm, error) {
	var from *time.Time
	if !since.IsZero() {
		from = &since
	}
	rows, err := s.pool.Query(ctx, `
		SELECT hero_id, count(*), count(*) FILTER (WHERE win)
		FROM player_matches
		WHERE account_id = $1 AND ($2::timestamptz IS NULL OR start_time >= $2)
		GROUP BY hero_id HAVING count(*) >= $3`, accountID, from, minMatches)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HeroForm
	for rows.Next() {
		var h HeroForm
		if err := rows.Scan(&h.HeroID, &h.Matches, &h.Wins); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// SetTrackedAccount запоминает, чей профиль пользователь смотрит в «Тренере».
func (s *Store) SetTrackedAccount(ctx context.Context, userID, accountID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET tracked_account_id = $2 WHERE id = $1`, userID, accountID)
	return err
}
