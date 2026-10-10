// Package store — доступ к PostgreSQL: пул соединений, миграции, репозитории.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"d2helper/migrations"
)

// ErrNotFound — запись не найдена.
var ErrNotFound = errors.New("store: not found")

// Store — репозитории поверх пула соединений.
type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Connect открывает пул и ждёт, пока база станет доступна (до 30 секунд):
// при старте через docker compose Postgres может подниматься дольше бэкенда.
func Connect(ctx context.Context, databaseURL string, logger *slog.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	cfg.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	const attempts = 30
	for i := 1; ; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return pool, nil
		}
		if i == attempts {
			pool.Close()
			return nil, fmt.Errorf("database unavailable after %d attempts: %w", attempts, err)
		}
		logger.Info("waiting for database", "attempt", i, "err", err)
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Migrate применяет встроенные в бинарник миграции goose.
func Migrate(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	results, err := provider.Up(ctx)
	for _, res := range results {
		logger.Info("migration applied", "version", res.Source.Version, "file", res.Source.Path, "dur", res.Duration)
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
