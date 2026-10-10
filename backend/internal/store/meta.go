package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MetaRow — статистика героя на позиции за патч.
type MetaRow struct {
	Position int
	HeroID   int
	Matches  int
	Wins     int
}

// ReplaceMetaPositions целиком заменяет мету патча одной транзакцией:
// читатели видят либо старые данные, либо новые, но не смесь.
func (s *Store) ReplaceMetaPositions(ctx context.Context, patch string, rows []MetaRow) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM meta_hero_position WHERE patch = $1`, patch); err != nil {
			return fmt.Errorf("delete meta: %w", err)
		}
		now := time.Now()
		_, err := tx.CopyFrom(ctx,
			pgx.Identifier{"meta_hero_position"},
			[]string{"patch", "position", "hero_id", "matches", "wins", "updated_at"},
			pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
				r := rows[i]
				return []any{patch, int16(r.Position), r.HeroID, r.Matches, r.Wins, now}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("insert meta: %w", err)
		}
		return nil
	})
}

// MetaPositions — вся мета патча и время её обновления. ErrNotFound — данных нет.
func (s *Store) MetaPositions(ctx context.Context, patch string) ([]MetaRow, time.Time, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT position, hero_id, matches, wins, updated_at
		FROM meta_hero_position WHERE patch = $1`, patch)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()

	var out []MetaRow
	var updatedAt time.Time
	for rows.Next() {
		var r MetaRow
		var pos int16
		var ts time.Time
		if err := rows.Scan(&pos, &r.HeroID, &r.Matches, &r.Wins, &ts); err != nil {
			return nil, time.Time{}, err
		}
		r.Position = int(pos)
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

// LatestMetaPatch — патч с самыми свежими данными меты (если по текущему ещё не собрали).
func (s *Store) LatestMetaPatch(ctx context.Context) (string, error) {
	var patch string
	err := s.pool.QueryRow(ctx,
		`SELECT patch FROM meta_hero_position ORDER BY updated_at DESC LIMIT 1`).Scan(&patch)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return patch, err
}
