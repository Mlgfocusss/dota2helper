package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/analysis"
	"d2helper/internal/auth"
	"d2helper/internal/coach"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/store"
)

// AnalysisService — глубокий разбор матчей.
type AnalysisService interface {
	Available() bool
	PerDay() int
	Quota(ctx context.Context, userID int64) (analysis.Quota, error)
	Create(ctx context.Context, user store.User, matchID int64) (*analysis.View, error)
	Get(ctx context.Context, userID int64, id string) (*analysis.View, error)
	ExampleView() (*analysis.View, bool)
}

// AnalysisRefs — статусы разборов пользователя по матчам (для списков матчей).
type AnalysisRefs interface {
	AnalysesForMatches(ctx context.Context, userID int64, matchIDs []int64) (map[int64]store.AnalysisRef, error)
	ListAnalyses(ctx context.Context, userID int64, limit int) ([]store.AnalysisListItem, error)
}

type createAnalysisRequest struct {
	MatchID int64 `json:"match_id"`
}

// analysisCreate — POST /api/analyses { match_id }. Новый заказ — 201, уже существующий — 200.
func (a *api) analysisCreate(w http.ResponseWriter, r *http.Request) {
	var req createAnalysisRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.MatchID < 1 {
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
		return
	}
	user := auth.UserFrom(r.Context())
	view, err := a.analysis.Create(r.Context(), *user, req.MatchID)

	var quota *analysis.QuotaError
	var exists *analysis.ExistsError
	switch {
	case err == nil:
		render.JSON(w, http.StatusCreated, view)
	case errors.As(err, &exists):
		existing, gerr := a.analysis.Get(r.Context(), user.ID, exists.ID)
		if gerr != nil {
			a.logger.Error("analysis: get existing", "err", gerr)
			render.Internal(w)
			return
		}
		render.JSON(w, http.StatusOK, existing)
	case errors.Is(err, analysis.ErrUnavailable):
		render.Error(w, http.StatusServiceUnavailable, "analysis_unavailable",
			"Разбор матчей сейчас недоступен. Загляни позже")
	case errors.Is(err, analysis.ErrNotYourMatch):
		render.Error(w, http.StatusForbidden, "match_not_yours",
			"Этого матча нет в истории твоего профиля. Выбери матч из списка «Мои матчи»")
	case errors.As(err, &quota):
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(time.Until(quota.ResetAt).Seconds()))))
		render.Error(w, http.StatusTooManyRequests, "analysis_quota",
			"Дневной лимит разборов исчерпан. Новый появится завтра")
	default:
		a.logger.Error("analysis: create", "err", err)
		render.Internal(w)
	}
}

// analysisGet — GET /api/analyses/{id}: статус и, когда готов, результат разбора.
func (a *api) analysisGet(w http.ResponseWriter, r *http.Request) {
	view, err := a.analysis.Get(r.Context(), auth.UserFrom(r.Context()).ID, chi.URLParam(r, "id"))
	switch {
	case errors.Is(err, analysis.ErrNotFound):
		render.Error(w, http.StatusNotFound, "analysis_not_found", "Разбор не найден")
	case err != nil:
		a.logger.Error("analysis: get", "err", err)
		render.Internal(w)
	default:
		render.JSON(w, http.StatusOK, view)
	}
}

// analysisListItem — строка списка «Разборы»: сведения о матче и всё нужное для блока «Что тренировать».
type analysisListItem struct {
	ID        string    `json:"id"`
	MatchID   int64     `json:"match_id"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	// из истории матчей игрока; пусто, если матч уже вытеснен из неё
	HeroID    *int       `json:"hero_id,omitempty"`
	Win       *bool      `json:"win,omitempty"`
	Kills     *int       `json:"kills,omitempty"`
	Deaths    *int       `json:"deaths,omitempty"`
	Assists   *int       `json:"assists,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	// из результата готового разбора
	Rating        *int                   `json:"rating,omitempty"`
	Improvements  []analysis.Improvement `json:"improvements,omitempty"`
	NextGamesPlan []string               `json:"next_games_plan,omitempty"`
}

const analysisListLimit = 50

// analysisList — GET /api/analyses: все разборы пользователя, новые сверху.
func (a *api) analysisList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.analysisRefs.ListAnalyses(r.Context(), auth.UserFrom(r.Context()).ID, analysisListLimit)
	if err != nil {
		a.logger.Error("analysis: list", "err", err)
		render.Internal(w)
		return
	}
	items := make([]analysisListItem, 0, len(rows))
	for _, row := range rows {
		it := analysisListItem{
			ID: row.ID, MatchID: row.MatchID, Status: row.Status, Error: row.Error, CreatedAt: row.CreatedAt,
			HeroID: row.HeroID, Win: row.Win, Kills: row.Kills, Deaths: row.Deaths, Assists: row.Assists,
			StartTime: row.StartTime,
		}
		if row.Status == "done" && len(row.Result) > 0 {
			var res analysis.Result
			if err := json.Unmarshal(row.Result, &res); err != nil {
				a.logger.Warn("analysis: bad result", "id", row.ID, "err", err)
			} else {
				rating := res.Summary.Rating
				it.Rating, it.Improvements, it.NextGamesPlan = &rating, res.Improvements, res.NextGamesPlan
			}
		}
		items = append(items, it)
	}
	render.JSON(w, http.StatusOK, map[string]any{"items": items})
}

// analysisExample — GET /api/analyses/example: пример разбора для всех.
func (a *api) analysisExample(w http.ResponseWriter, _ *http.Request) {
	view, ok := a.analysis.ExampleView()
	if !ok {
		render.Error(w, http.StatusNotFound, "analysis_not_found", "Примера пока нет")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	render.JSON(w, http.StatusOK, view)
}

// analysisQuota — GET /api/analyses/quota: сколько разборов осталось сегодня.
func (a *api) analysisQuota(w http.ResponseWriter, r *http.Request) {
	q, err := a.analysis.Quota(r.Context(), auth.UserFrom(r.Context()).ID)
	if err != nil {
		a.logger.Error("analysis: quota", "err", err)
		render.Internal(w)
		return
	}
	render.JSON(w, http.StatusOK, q)
}

// withAnalyses возвращает копию списка матчей со статусами разборов текущего пользователя.
// Списки кэшируются в сервисе «Тренера» и общие для всех, поэтому исходные данные не меняем.
func (a *api) withAnalyses(r *http.Request, rows []coach.MatchRow) []coach.MatchRow {
	u := auth.UserFrom(r.Context())
	out := append([]coach.MatchRow(nil), rows...)
	if u == nil || len(rows) == 0 {
		return out
	}
	ids := make([]int64, len(rows))
	for i, m := range rows {
		ids[i] = m.MatchID
	}
	refs, err := a.analysisRefs.AnalysesForMatches(r.Context(), u.ID, ids)
	if err != nil {
		a.logger.Warn("analysis refs", "err", err)
		return out
	}
	for i := range out {
		if ref, ok := refs[out[i].MatchID]; ok {
			ref := ref
			out[i].Analysis = &ref
		}
	}
	return out
}
