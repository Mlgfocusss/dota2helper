package store

import "context"

// CreateFeedback сохраняет отзыв; userID = nil для гостя.
func (s *Store) CreateFeedback(ctx context.Context, userID *int64, text, contact string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx,
		`INSERT INTO feedback (user_id, text, contact) VALUES ($1, $2, nullif($3, '')) RETURNING id`,
		userID, text, contact).Scan(&id)
	return id, err
}
