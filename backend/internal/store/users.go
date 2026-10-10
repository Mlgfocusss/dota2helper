package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// User — пользователь, вошедший через Steam.
type User struct {
	ID               int64
	SteamID64        string
	AccountID        int64
	PersonaName      string
	AvatarURL        string
	TrackedAccountID *int64
	Credits          int
	HasPaid          bool
	CreatedAt        time.Time
}

const userColumns = `u.id, u.steam_id64, u.account_id, coalesce(u.persona_name, ''), coalesce(u.avatar_url, ''),
	u.tracked_account_id, u.analysis_credits, u.has_paid, u.created_at`

func scanUser(row pgx.Row, extra ...any) (User, error) {
	var u User
	dest := append([]any{
		&u.ID, &u.SteamID64, &u.AccountID, &u.PersonaName, &u.AvatarURL,
		&u.TrackedAccountID, &u.Credits, &u.HasPaid, &u.CreatedAt,
	}, extra...)
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// UpsertSteamUser создаёт пользователя при первом входе или обновляет ник и
// аватар при повторном. Новому пользователю отслеживаемым профилем ставится свой.
// Пустые persona/avatar не затирают сохранённые значения.
func (s *Store) UpsertSteamUser(ctx context.Context, steamID64 string, accountID int64, persona, avatar string) (User, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO users AS u (steam_id64, account_id, persona_name, avatar_url, tracked_account_id, last_login_at)
		VALUES ($1, $2, nullif($3, ''), nullif($4, ''), $2, now())
		ON CONFLICT (steam_id64) DO UPDATE SET
			persona_name  = coalesce(EXCLUDED.persona_name, u.persona_name),
			avatar_url    = coalesce(EXCLUDED.avatar_url, u.avatar_url),
			last_login_at = now()
		RETURNING `+userColumns,
		steamID64, accountID, persona, avatar)
	u, err := scanUser(row)
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users u WHERE u.id = $1`, id))
}
