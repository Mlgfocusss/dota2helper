package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// MatchupRow — преимущество героя HeroID над OtherID (kind = "vs") или
// синергия в одной команде (kind = "with"), в процентных пунктах.
type MatchupRow struct {
	HeroID    int
	OtherID   int
	Kind      string
	Matches   int
	Advantage float32
}

// ReplaceHeroMatchups целиком заменяет таблицу одной транзакцией.
func (s *Store) ReplaceHeroMatchups(ctx context.Context, rows []MatchupRow) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM hero_matchups`); err != nil {
			return fmt.Errorf("delete matchups: %w", err)
		}
		now := time.Now()
		_, err := tx.CopyFrom(ctx,
			pgx.Identifier{"hero_matchups"},
			[]string{"hero_id", "other_id", "kind", "matches", "advantage", "updated_at"},
			pgx.CopyFromSlice(len(rows), func(i int) ([]any, error) {
				r := rows[i]
				return []any{r.HeroID, r.OtherID, r.Kind, r.Matches, r.Advantage, now}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("insert matchups: %w", err)
		}
		return nil
	})
}

// HeroMatchups — вся таблица и время обновления. ErrNotFound — ещё не собрана.
func (s *Store) HeroMatchups(ctx context.Context) ([]MatchupRow, time.Time, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT hero_id, other_id, kind, matches, advantage, updated_at FROM hero_matchups`)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	var (
		out       []MatchupRow
		updatedAt time.Time
	)
	for rows.Next() {
		var r MatchupRow
		var ts time.Time
		if err := rows.Scan(&r.HeroID, &r.OtherID, &r.Kind, &r.Matches, &r.Advantage, &ts); err != nil {
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

// HeroMatchupsFor — строки одного героя и вида с не менее чем minMatches матчей.
func (s *Store) HeroMatchupsFor(ctx context.Context, heroID int, kind string, minMatches int) ([]MatchupRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT hero_id, other_id, kind, matches, advantage FROM hero_matchups
		WHERE hero_id = $1 AND kind = $2 AND matches >= $3`, heroID, kind, minMatches)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchupRow
	for rows.Next() {
		var r MatchupRow
		if err := rows.Scan(&r.HeroID, &r.OtherID, &r.Kind, &r.Matches, &r.Advantage); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// HeroPositionRows — строки меты героя по позициям за патч (без отсечения по доле).
func (s *Store) HeroPositionRows(ctx context.Context, patch string, heroID int) ([]MetaRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT position, hero_id, matches, wins FROM meta_hero_position
		WHERE patch = $1 AND hero_id = $2 ORDER BY position`, patch, heroID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MetaRow
	for rows.Next() {
		var r MetaRow
		if err := rows.Scan(&r.Position, &r.HeroID, &r.Matches, &r.Wins); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
