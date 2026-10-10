package store

import (
	"context"
	"fmt"
	"time"
)

// CreateSession сохраняет сессию; id — хеш токена из cookie, сам токен в БД не хранится.
func (s *Store) CreateSession(ctx context.Context, id string, userID int64, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`, id, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionUser — пользователь действующей сессии и срок её действия.
func (s *Store) SessionUser(ctx context.Context, id string) (User, time.Time, error) {
	var expiresAt time.Time
	row := s.pool.QueryRow(ctx, `
		SELECT `+userColumns+`, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.expires_at > now()`, id)
	u, err := scanUser(row, &expiresAt)
	if err != nil {
		return User{}, time.Time{}, err
	}
	return u, expiresAt, nil
}

func (s *Store) ExtendSession(ctx context.Context, id string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET expires_at = $2 WHERE id = $1`, id, expiresAt)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

// DeleteExpiredSessions — для фоновой очистки.
func (s *Store) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at <= now()`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
