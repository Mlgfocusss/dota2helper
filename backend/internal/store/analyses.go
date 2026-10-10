package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrAnalysisExists — по этому матчу у пользователя уже есть действующий разбор.
var ErrAnalysisExists = errors.New("store: analysis already exists")

// Analysis — заказ глубокого разбора матча.
type Analysis struct {
	ID        string
	UserID    int64
	AccountID int64
	MatchID   int64
	Status    string
	Error     string
	Result    []byte // JSON; nil, пока разбор не готов
	CreatedAt time.Time
	UpdatedAt time.Time
}

const analysisColumns = `id::text, user_id, account_id, match_id, status, coalesce(error, ''), result, created_at, updated_at`

func scanAnalysis(row pgx.Row) (Analysis, error) {
	var a Analysis
	err := row.Scan(&a.ID, &a.UserID, &a.AccountID, &a.MatchID, &a.Status, &a.Error, &a.Result, &a.CreatedAt, &a.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Analysis{}, ErrNotFound
	}
	return a, err
}

// CreateAnalysis ставит разбор в очередь. Если действующий разбор по матчу уже есть
// (в очереди, в работе или готов) — возвращает его вместе с ErrAnalysisExists.
func (s *Store) CreateAnalysis(ctx context.Context, userID, accountID, matchID int64) (Analysis, error) {
	a, err := scanAnalysis(s.pool.QueryRow(ctx, `
		INSERT INTO analyses (user_id, account_id, match_id) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, match_id) WHERE status <> 'failed' DO NOTHING
		RETURNING `+analysisColumns, userID, accountID, matchID))
	if errors.Is(err, ErrNotFound) {
		existing, err := scanAnalysis(s.pool.QueryRow(ctx, `
			SELECT `+analysisColumns+` FROM analyses
			WHERE user_id = $1 AND match_id = $2 AND status <> 'failed'`, userID, matchID))
		if err != nil {
			return Analysis{}, err
		}
		return existing, ErrAnalysisExists
	}
	if err != nil {
		return Analysis{}, fmt.Errorf("create analysis: %w", err)
	}
	return a, nil
}

// AnalysisByID — разбор по id. ErrNotFound — нет такого (или id не uuid).
func (s *Store) AnalysisByID(ctx context.Context, id string) (Analysis, error) {
	return scanAnalysis(s.pool.QueryRow(ctx,
		`SELECT `+analysisColumns+` FROM analyses WHERE id::text = $1`, id))
}

// ClaimQueuedAnalysis берёт самый старый разбор из очереди и переводит его в parsing.
// Параллельные воркеры не берут один и тот же разбор (SKIP LOCKED). nil — очередь пуста.
func (s *Store) ClaimQueuedAnalysis(ctx context.Context) (*Analysis, error) {
	a, err := scanAnalysis(s.pool.QueryRow(ctx, `
		UPDATE analyses SET status = 'parsing', updated_at = now()
		WHERE id = (
			SELECT id FROM analyses WHERE status = 'queued'
			ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1
		)
		RETURNING `+analysisColumns))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) SetAnalysisStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE analyses SET status = $2, updated_at = now() WHERE id::text = $1`, id, status)
	return err
}

func (s *Store) FinishAnalysis(ctx context.Context, id string, result []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE analyses SET status = 'done', result = $2::jsonb, error = NULL, updated_at = now()
		WHERE id::text = $1`, id, string(result))
	return err
}

// FailAnalysis завершает разбор ошибкой; такой заказ не считается в дневном лимите
// и не мешает заказать разбор матча заново.
func (s *Store) FailAnalysis(ctx context.Context, id, message string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE analyses SET status = 'failed', error = $2, updated_at = now()
		WHERE id::text = $1`, id, message)
	return err
}

// RequeueStuckAnalyses возвращает в очередь разборы, застрявшие в работе после перезапуска сервера.
func (s *Store) RequeueStuckAnalyses(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE analyses SET status = 'queued', updated_at = now()
		WHERE status IN ('parsing', 'analyzing')`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// CountAnalysesSince — сколько разборов пользователь заказал с момента since (без неудавшихся)
// и когда был заказан самый старый из них — по нему считается, когда освободится место.
func (s *Store) CountAnalysesSince(ctx context.Context, userID int64, since time.Time) (int, *time.Time, error) {
	var n int
	var oldest *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT count(*), min(created_at) FROM analyses
		WHERE user_id = $1 AND status <> 'failed' AND created_at >= $2`, userID, since).Scan(&n, &oldest)
	return n, oldest, err
}

// AnalysisRef — краткие сведения о разборе для списков матчей.
type AnalysisRef struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// AnalysesForMatches — последний разбор пользователя по каждому из матчей.
func (s *Store) AnalysesForMatches(ctx context.Context, userID int64, matchIDs []int64) (map[int64]AnalysisRef, error) {
	out := map[int64]AnalysisRef{}
	if len(matchIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (match_id) match_id, id::text, status FROM analyses
		WHERE user_id = $1 AND match_id = ANY($2)
		ORDER BY match_id, created_at DESC`, userID, matchIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var matchID int64
		var ref AnalysisRef
		if err := rows.Scan(&matchID, &ref.ID, &ref.Status); err != nil {
			return nil, err
		}
		out[matchID] = ref
	}
	return out, rows.Err()
}

// HasPlayerMatch — есть ли матч в сохранённой истории игрока.
func (s *Store) HasPlayerMatch(ctx context.Context, accountID, matchID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM player_matches WHERE account_id = $1 AND match_id = $2)`,
		accountID, matchID).Scan(&ok)
	return ok, err
}

// AnalysisListItem — разбор в списке «Разборы»: заказ плюс сведения о матче из истории игрока.
type AnalysisListItem struct {
	ID        string
	MatchID   int64
	Status    string
	Error     string
	Result    []byte
	CreatedAt time.Time
	HeroID    *int
	Win       *bool
	Kills     *int
	Deaths    *int
	Assists   *int
	StartTime *time.Time
}

// ListAnalyses — последние разборы пользователя, новые сверху.
func (s *Store) ListAnalyses(ctx context.Context, userID int64, limit int) ([]AnalysisListItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id::text, a.match_id, a.status, coalesce(a.error, ''), a.result, a.created_at,
		       pm.hero_id, pm.win, pm.kills, pm.deaths, pm.assists, pm.start_time
		FROM analyses a
		LEFT JOIN player_matches pm ON pm.account_id = a.account_id AND pm.match_id = a.match_id
		WHERE a.user_id = $1
		ORDER BY a.created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list analyses: %w", err)
	}
	defer rows.Close()
	var out []AnalysisListItem
	for rows.Next() {
		var it AnalysisListItem
		if err := rows.Scan(&it.ID, &it.MatchID, &it.Status, &it.Error, &it.Result, &it.CreatedAt,
			&it.HeroID, &it.Win, &it.Kills, &it.Deaths, &it.Assists, &it.StartTime); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
