package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrTrainingClosed — тренировка уже оценена, чужая или не существует.
var ErrTrainingClosed = errors.New("store: training is closed")

// Training — тренировка драфтера.
type Training struct {
	ID          string
	UserID      *int64
	Enemies     []byte // JSON [{hero_id, position}]
	Team        []byte // JSON, nil пока не оценена
	Score       *float64
	Breakdown   []byte
	Status      string
	CreatedAt   time.Time
	SubmittedAt *time.Time
}

// CreateTraining создаёт открытую тренировку. У вошедшего пользователя она одна:
// прежняя неоценённая закрывается (удаляется).
func (s *Store) CreateTraining(ctx context.Context, userID *int64, enemies []byte) (string, error) {
	var id string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if userID != nil {
			if _, err := tx.Exec(ctx,
				`DELETE FROM drafter_trainings WHERE user_id = $1 AND status = 'open'`, *userID); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx,
			`INSERT INTO drafter_trainings (user_id, enemies) VALUES ($1, $2::jsonb) RETURNING id::text`,
			userID, string(enemies)).Scan(&id)
	})
	if err != nil {
		return "", fmt.Errorf("create training: %w", err)
	}
	return id, nil
}

const trainingColumns = `id::text, user_id, enemies, team, score, breakdown, status, created_at, submitted_at`

func scanTraining(row pgx.Row) (Training, error) {
	var t Training
	err := row.Scan(&t.ID, &t.UserID, &t.Enemies, &t.Team, &t.Score, &t.Breakdown, &t.Status, &t.CreatedAt, &t.SubmittedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Training{}, ErrNotFound
	}
	return t, err
}

// TrainingByID — тренировка по id. ErrNotFound — нет такой (или id не uuid).
func (s *Store) TrainingByID(ctx context.Context, id string) (Training, error) {
	t, err := scanTraining(s.pool.QueryRow(ctx,
		`SELECT `+trainingColumns+` FROM drafter_trainings WHERE id::text = $1`, id))
	return t, err
}

// SubmitTraining сохраняет оценку и привязывает гостевую тренировку к пользователю.
// ErrTrainingClosed — уже оценена или принадлежит другому пользователю.
func (s *Store) SubmitTraining(ctx context.Context, id string, userID int64, team []byte, score float64, breakdown []byte) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE drafter_trainings
		SET user_id = $2, team = $3::jsonb, score = $4, breakdown = $5::jsonb,
			status = 'submitted', submitted_at = now()
		WHERE id::text = $1 AND status = 'open' AND (user_id IS NULL OR user_id = $2)`,
		id, userID, string(team), score, string(breakdown))
	if err != nil {
		return fmt.Errorf("submit training: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrTrainingClosed
	}
	return nil
}

// CountSubmittedSince — сколько тренировок пользователь оценил с момента since (анти-абуз).
func (s *Store) CountSubmittedSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM drafter_trainings
		WHERE user_id = $1 AND status = 'submitted' AND submitted_at >= $2`, userID, since).Scan(&n)
	return n, err
}

// TrainingHistory — оценённые тренировки пользователя, новые первыми.
// before — граница по (submitted_at, id): nil — с начала.
func (s *Store) TrainingHistory(ctx context.Context, userID int64, before *time.Time, limit int) ([]Training, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+trainingColumns+` FROM drafter_trainings
		WHERE user_id = $1 AND status = 'submitted' AND ($2::timestamptz IS NULL OR submitted_at < $2)
		ORDER BY submitted_at DESC LIMIT $3`, userID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Training
	for rows.Next() {
		t, err := scanTraining(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MonthUserScores — результаты одного пользователя за месяц для лидерборда.
type MonthUserScores struct {
	UserID      int64
	PersonaName string
	AvatarURL   string
	// Scores — все оценки месяца по убыванию
	Scores []float64
}

// MonthScores — оценки всех пользователей за [from, to).
func (s *Store) MonthScores(ctx context.Context, from, to time.Time) ([]MonthUserScores, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id, coalesce(u.persona_name, ''), coalesce(u.avatar_url, ''),
			array_agg(t.score::float8 ORDER BY t.score DESC)
		FROM drafter_trainings t JOIN users u ON u.id = t.user_id
		WHERE t.status = 'submitted' AND t.submitted_at >= $1 AND t.submitted_at < $2
		GROUP BY u.id`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MonthUserScores
	for rows.Next() {
		var m MonthUserScores
		if err := rows.Scan(&m.UserID, &m.PersonaName, &m.AvatarURL, &m.Scores); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteStaleTrainings удаляет брошенные неоценённые тренировки старше olderThan.
func (s *Store) DeleteStaleTrainings(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM drafter_trainings
		WHERE status = 'open' AND created_at < now() - make_interval(secs => $1)`, olderThan.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
