package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// GetKV читает значение из kv_cache.
func (s *Store) GetKV(ctx context.Context, key string) ([]byte, time.Time, error) {
	var value []byte
	var updatedAt time.Time
	err := s.pool.QueryRow(ctx, `SELECT value, updated_at FROM kv_cache WHERE key = $1`, key).
		Scan(&value, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, time.Time{}, ErrNotFound
	}
	return value, updatedAt, err
}

// PutKV сохраняет JSON‑значение в kv_cache.
func (s *Store) PutKV(ctx context.Context, key string, value []byte) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO kv_cache (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, value)
	return err
}
