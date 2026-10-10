package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrAlreadyReported — пользователь уже жаловался на эту анкету.
var ErrAlreadyReported = errors.New("store: already reported")

// PartyProfile — анкета для поиска пати вместе с публичными данными автора.
type PartyProfile struct {
	UserID      int64
	AccountID   int64
	PersonaName string
	AvatarURL   string

	RankMedal  int
	Hours      int
	Positions  []int16
	Modes      []string
	Goal       string
	LookingFor string
	Mic        bool
	Discord    bool
	DiscordTag string
	Telegram   string
	FavHeroes  []int32
	About      string

	IsActive  bool
	BumpedAt  time.Time
	UpdatedAt time.Time
	CreatedAt time.Time
}

const partyColumns = `p.user_id, u.account_id, coalesce(u.persona_name, ''), coalesce(u.avatar_url, ''),
	p.rank_medal, p.hours, p.positions, p.modes, p.goal, p.looking_for, p.mic, p.discord,
	coalesce(p.discord_tag, ''), coalesce(p.telegram, ''), p.fav_heroes, p.about,
	p.is_active, p.bumped_at, p.updated_at, p.created_at`

func scanParty(row pgx.Row) (PartyProfile, error) {
	var p PartyProfile
	err := row.Scan(&p.UserID, &p.AccountID, &p.PersonaName, &p.AvatarURL,
		&p.RankMedal, &p.Hours, &p.Positions, &p.Modes, &p.Goal, &p.LookingFor, &p.Mic, &p.Discord,
		&p.DiscordTag, &p.Telegram, &p.FavHeroes, &p.About,
		&p.IsActive, &p.BumpedAt, &p.UpdatedAt, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return PartyProfile{}, ErrNotFound
	}
	return p, err
}

// PartyProfileByUser — анкета пользователя. ErrNotFound — анкеты нет.
func (s *Store) PartyProfileByUser(ctx context.Context, userID int64) (PartyProfile, error) {
	return scanParty(s.pool.QueryRow(ctx, `
		SELECT `+partyColumns+` FROM party_profiles p JOIN users u ON u.id = p.user_id
		WHERE p.user_id = $1`, userID))
}

// UpsertPartyProfile создаёт анкету или обновляет поля формы. Видимость (is_active)
// и время подъёма при правке не меняются — иначе правкой можно обойти лимит подъёма.
func (s *Store) UpsertPartyProfile(ctx context.Context, p PartyProfile) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO party_profiles (user_id, rank_medal, hours, positions, modes, goal, looking_for,
			mic, discord, discord_tag, telegram, fav_heroes, about)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, nullif($10, ''), nullif($11, ''), $12, $13)
		ON CONFLICT (user_id) DO UPDATE SET
			rank_medal = EXCLUDED.rank_medal, hours = EXCLUDED.hours, positions = EXCLUDED.positions,
			modes = EXCLUDED.modes, goal = EXCLUDED.goal, looking_for = EXCLUDED.looking_for,
			mic = EXCLUDED.mic, discord = EXCLUDED.discord, discord_tag = EXCLUDED.discord_tag,
			telegram = EXCLUDED.telegram, fav_heroes = EXCLUDED.fav_heroes, about = EXCLUDED.about,
			updated_at = now()`,
		p.UserID, p.RankMedal, p.Hours, p.Positions, p.Modes, p.Goal, p.LookingFor,
		p.Mic, p.Discord, p.DiscordTag, p.Telegram, p.FavHeroes, p.About)
	if err != nil {
		return fmt.Errorf("upsert party profile: %w", err)
	}
	return nil
}

// SetPartyActive скрывает или показывает анкету. ErrNotFound — анкеты нет.
func (s *Store) SetPartyActive(ctx context.Context, userID int64, active bool) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE party_profiles SET is_active = $2, updated_at = now() WHERE user_id = $1`, userID, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeletePartyProfile(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM party_profiles WHERE user_id = $1`, userID)
	return err
}

// BumpParty поднимает анкету, если с прошлого подъёма прошло не меньше cooldown.
// Второй результат false — слишком рано (первый тогда содержит время прошлого подъёма).
// ErrNotFound — анкеты нет.
func (s *Store) BumpParty(ctx context.Context, userID int64, cooldown time.Duration) (time.Time, bool, error) {
	var bumped time.Time
	err := s.pool.QueryRow(ctx, `
		UPDATE party_profiles SET bumped_at = now(), updated_at = now()
		WHERE user_id = $1 AND bumped_at <= now() - make_interval(secs => $2)
		RETURNING bumped_at`, userID, cooldown.Seconds()).Scan(&bumped)
	if err == nil {
		return bumped, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, err
	}
	err = s.pool.QueryRow(ctx, `SELECT bumped_at FROM party_profiles WHERE user_id = $1`, userID).Scan(&bumped)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, ErrNotFound
	}
	return bumped, false, err
}

// PartyFilter — условия поиска анкет.
type PartyFilter struct {
	// ViewerID — свою анкету в выдаче не показываем
	ViewerID   int64
	LookingFor string
	RankMin    int
	RankMax    int
	Positions  []int16
	Modes      []string
	Goals      []string
	Mic        bool
	Discord    bool
	// HideReports — анкеты с таким числом жалоб и больше скрыты до проверки
	HideReports int
	Limit       int
	// Cursor — позиция последней показанной анкеты (nil — с начала)
	Cursor *PartyCursor
}

// PartyCursor — позиция в выдаче, отсортированной по (bumped_at, user_id) по убыванию.
type PartyCursor struct {
	BumpedAt time.Time
	UserID   int64
}

// SearchParty возвращает до Limit анкет по фильтру, свежие подъёмы — первыми.
func (s *Store) SearchParty(ctx context.Context, f PartyFilter) ([]PartyProfile, error) {
	var (
		where = []string{
			"p.is_active",
			"p.user_id <> $1",
			"p.looking_for = $2",
			"p.rank_medal BETWEEN $3 AND $4",
			"(SELECT count(*) FROM party_reports r WHERE r.target_id = p.user_id) < $5",
		}
		args = []any{f.ViewerID, f.LookingFor, f.RankMin, f.RankMax, f.HideReports}
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if len(f.Positions) > 0 {
		add("p.positions && ?::smallint[]", f.Positions)
	}
	if len(f.Modes) > 0 {
		add("p.modes && ?::text[]", f.Modes)
	}
	if len(f.Goals) > 0 {
		add("p.goal = ANY(?::text[])", f.Goals)
	}
	if f.Mic {
		where = append(where, "p.mic")
	}
	if f.Discord {
		where = append(where, "p.discord")
	}
	if f.Cursor != nil {
		args = append(args, f.Cursor.BumpedAt, f.Cursor.UserID)
		where = append(where, fmt.Sprintf("(p.bumped_at, p.user_id) < ($%d, $%d)", len(args)-1, len(args)))
	}
	args = append(args, f.Limit)

	rows, err := s.pool.Query(ctx, `
		SELECT `+partyColumns+` FROM party_profiles p JOIN users u ON u.id = p.user_id
		WHERE `+strings.Join(where, " AND ")+fmt.Sprintf(`
		ORDER BY p.bumped_at DESC, p.user_id DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PartyProfile
	for rows.Next() {
		p, err := scanParty(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReportParty сохраняет жалобу. ErrNotFound — у адресата нет анкеты,
// ErrAlreadyReported — жалоба от этого пользователя уже есть.
func (s *Store) ReportParty(ctx context.Context, reporterID, targetID int64, reason string) error {
	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM party_profiles WHERE user_id = $1)`, targetID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO party_reports (reporter_id, target_id, reason) VALUES ($1, $2, $3)`,
		reporterID, targetID, reason)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrAlreadyReported
	}
	return err
}

// ExpirePartyProfiles скрывает анкеты, к которым не прикасались дольше idle.
// Активностью считаются подъём и правка: берётся более позднее из bumped_at и updated_at.
func (s *Store) ExpirePartyProfiles(ctx context.Context, idle time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE party_profiles SET is_active = false
		WHERE is_active AND greatest(bumped_at, updated_at) < now() - make_interval(secs => $1)`,
		idle.Seconds())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DrafterStats — сводка по тренировкам драфтера для профиля.
type DrafterStats struct {
	Trainings int
	BestScore *float64
	// MonthRank — место в топе месяца; nil, если в этом месяце ещё не тренировался
	MonthRank *int
}

// DrafterStatsForUser считает оценённые тренировки пользователя и его место в топе месяца
// [monthStart, monthEnd): сумма 5 лучших результатов, у кого меньше 5 — все.
func (s *Store) DrafterStatsForUser(ctx context.Context, userID int64, monthStart, monthEnd time.Time) (DrafterStats, error) {
	var st DrafterStats
	err := s.pool.QueryRow(ctx, `
		SELECT count(*), max(score) FROM drafter_trainings
		WHERE user_id = $1 AND status = 'submitted'`, userID).Scan(&st.Trainings, &st.BestScore)
	if err != nil {
		return st, err
	}
	err = s.pool.QueryRow(ctx, `
		WITH ranked AS (
			SELECT user_id, score,
				row_number() OVER (PARTITION BY user_id ORDER BY score DESC) AS rn
			FROM drafter_trainings
			WHERE status = 'submitted' AND user_id IS NOT NULL
				AND submitted_at >= $1 AND submitted_at < $2
		), totals AS (
			SELECT user_id, sum(score) AS total FROM ranked WHERE rn <= 5 GROUP BY user_id
		), places AS (
			SELECT user_id, rank() OVER (ORDER BY total DESC) AS place FROM totals
		)
		SELECT place FROM places WHERE user_id = $3`, monthStart, monthEnd, userID).Scan(&st.MonthRank)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, nil
	}
	return st, err
}
