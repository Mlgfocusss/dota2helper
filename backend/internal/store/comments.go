package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Comment — комментарий к новости вместе с данными автора.
type Comment struct {
	ID         int64
	GID        string
	UserID     int64
	AuthorName string
	AuthorAva  string
	Text       string
	CreatedAt  time.Time
}

const commentSelect = `
	SELECT c.id, c.gid, c.user_id, coalesce(u.persona_name, ''), coalesce(u.avatar_url, ''), c.text, c.created_at
	FROM news_comments c JOIN users u ON u.id = c.user_id`

func scanComment(row pgx.Row) (Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.GID, &c.UserID, &c.AuthorName, &c.AuthorAva, &c.Text, &c.CreatedAt)
	return c, err
}

// ListComments — комментарии новости от новых к старым. before > 0 — только id меньше before
// (постраничная подгрузка). Второй результат — общее число комментариев новости.
func (s *Store) ListComments(ctx context.Context, gid string, before int64, limit int) ([]Comment, int, error) {
	rows, err := s.pool.Query(ctx, commentSelect+`
		WHERE c.gid = $1 AND ($2 = 0 OR c.id < $2)
		ORDER BY c.id DESC LIMIT $3`, gid, before, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM news_comments WHERE gid = $1`, gid).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// CreateComment сохраняет комментарий. ErrNotFound — такой новости нет.
func (s *Store) CreateComment(ctx context.Context, gid string, userID int64, text string) (Comment, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO news_comments (gid, user_id, text) VALUES ($1, $2, $3) RETURNING id`,
		gid, userID, text).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation: нет новости
		return Comment{}, ErrNotFound
	}
	if err != nil {
		return Comment{}, err
	}
	return scanComment(s.pool.QueryRow(ctx, commentSelect+` WHERE c.id = $1`, id))
}

// DeleteComment удаляет комментарий, только если он принадлежит пользователю.
// ErrNotFound — комментария нет или он чужой.
func (s *Store) DeleteComment(ctx context.Context, gid string, id, userID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM news_comments WHERE id = $1 AND gid = $2 AND user_id = $3`, id, gid, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
