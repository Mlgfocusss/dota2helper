package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type News struct {
	GID     string
	Title   string
	URL     string
	Date    time.Time
	HTML    string
	Excerpt string
}

// UpsertNews сохраняет новости; уже известные обновляются (Valve правит анонсы).
func (s *Store) UpsertNews(ctx context.Context, items []News) error {
	batch := &pgx.Batch{}
	for _, n := range items {
		batch.Queue(`
			INSERT INTO news (gid, title, url, date, html, excerpt) VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (gid) DO UPDATE SET
				title = EXCLUDED.title, url = EXCLUDED.url, date = EXCLUDED.date,
				html = EXCLUDED.html, excerpt = EXCLUDED.excerpt`,
			n.GID, n.Title, n.URL, n.Date, n.HTML, n.Excerpt)
	}
	return s.pool.SendBatch(ctx, batch).Close()
}

// ListNews — последние новости без полного текста.
func (s *Store) ListNews(ctx context.Context, limit int) ([]News, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT gid, title, url, date, excerpt FROM news ORDER BY date DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []News
	for rows.Next() {
		var n News
		if err := rows.Scan(&n.GID, &n.Title, &n.URL, &n.Date, &n.Excerpt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) NewsByGID(ctx context.Context, gid string) (News, error) {
	var n News
	err := s.pool.QueryRow(ctx,
		`SELECT gid, title, url, date, html, excerpt FROM news WHERE gid = $1`, gid).
		Scan(&n.GID, &n.Title, &n.URL, &n.Date, &n.HTML, &n.Excerpt)
	if errors.Is(err, pgx.ErrNoRows) {
		return News{}, ErrNotFound
	}
	return n, err
}
