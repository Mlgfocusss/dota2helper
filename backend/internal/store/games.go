package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// HLRow — значение героя в снапшоте «Больше / Меньше».
type HLRow struct {
	HeroID int
	Value  float32
}

// ReplaceHLSnapshot целиком заменяет снапшот режима одной транзакцией.
func (s *Store) ReplaceHLSnapshot(ctx context.Context, mode string, rows []HLRow) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM hl_snapshot WHERE mode = $1`, mode); err != nil {
			return fmt.Errorf("delete snapshot: %w", err)
		}
		now := time.Now()
		_, err := tx.CopyFrom(ctx,
			pgx.Identifier{"hl_snapshot"},
			[]string{"mode", "hero_id", "value", "updated_at"},
			pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
				return []any{mode, rows[i].HeroID, rows[i].Value, now}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("insert snapshot: %w", err)
		}
		return nil
	})
}

// HLSnapshot — снапшот режима и время обновления. ErrNotFound — снапшота ещё нет.
func (s *Store) HLSnapshot(ctx context.Context, mode string) ([]HLRow, time.Time, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT hero_id, value, updated_at FROM hl_snapshot WHERE mode = $1 ORDER BY hero_id`, mode)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	var out []HLRow
	var updatedAt time.Time
	for rows.Next() {
		var r HLRow
		var ts time.Time
		if err := rows.Scan(&r.HeroID, &r.Value, &ts); err != nil {
			return nil, time.Time{}, err
		}
		if ts.After(updatedAt) {
			updatedAt = ts
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, err
	}
	if len(out) == 0 {
		return nil, time.Time{}, ErrNotFound
	}
	return out, updatedAt, nil
}

// UpsertGameRecord запоминает результат игры: рекорд — максимум, plays растёт на 1. Возвращает рекорд.
func (s *Store) UpsertGameRecord(ctx context.Context, userID int64, game, mode string, score int) (int, error) {
	var best int
	err := s.pool.QueryRow(ctx, `
		INSERT INTO game_records (user_id, game, mode, best, plays, updated_at)
		VALUES ($1, $2, $3, $4, 1, now())
		ON CONFLICT (user_id, game, mode) DO UPDATE SET
			best = GREATEST(game_records.best, EXCLUDED.best),
			plays = game_records.plays + 1,
			updated_at = now()
		RETURNING best`, userID, game, mode, score).Scan(&best)
	return best, err
}

// GameRecords — рекорды пользователя в игре по режимам.
func (s *Store) GameRecords(ctx context.Context, userID int64, game string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT mode, best FROM game_records WHERE user_id = $1 AND game = $2`, userID, game)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var mode string
		var best int
		if err := rows.Scan(&mode, &best); err != nil {
			return nil, err
		}
		out[mode] = best
	}
	return out, rows.Err()
}

// SaveQuizResult сохраняет результат квиза пользователя.
func (s *Store) SaveQuizResult(ctx context.Context, userID int64, quiz string, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx,
		`INSERT INTO quiz_results (user_id, quiz, result) VALUES ($1, $2, $3::jsonb)`, userID, quiz, string(raw))
	return err
}

// LastQuizResult — последний результат квиза пользователя (JSON). ErrNotFound — не проходил.
func (s *Store) LastQuizResult(ctx context.Context, userID int64, quiz string) ([]byte, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx,
		`SELECT result FROM quiz_results WHERE user_id = $1 AND quiz = $2 ORDER BY created_at DESC, id DESC LIMIT 1`,
		userID, quiz).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return raw, err
}
