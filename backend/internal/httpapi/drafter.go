package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/auth"
	"d2helper/internal/drafter"
	"d2helper/internal/heropage"
	"d2helper/internal/httpapi/render"
)

// DrafterService — матрица, анализ, тренировки, история и топ драфтеров.
type DrafterService interface {
	MatrixPayload(ctx context.Context) (body []byte, etag string, err error)
	Analyze(ctx context.Context, radiant, dire []drafter.Slot, bans []int) (*drafter.Analysis, error)
	NewTraining(ctx context.Context, userID *int64) (string, []drafter.Slot, error)
	Submit(ctx context.Context, userID int64, trainingID string, team []drafter.Slot) (*drafter.Result, error)
	History(ctx context.Context, userID int64, cursor string) (*drafter.HistoryPage, error)
	Leaderboard(ctx context.Context, month string, viewerID int64) (*drafter.Board, error)
}

// HeroPageService — страница героя.
type HeroPageService interface {
	Page(ctx context.Context, heroID int) (*heropage.Page, error)
}

func (a *api) drafterError(w http.ResponseWriter, err error) {
	var tooMany *drafter.TooManyError
	switch {
	case errors.Is(err, drafter.ErrNotReady):
		w.Header().Set("Retry-After", "60")
		render.Error(w, http.StatusServiceUnavailable, "drafter_not_ready",
			"Собираем данные о матчапах. Загляни через пару минут")
	case errors.Is(err, drafter.ErrBadTeam):
		render.Error(w, http.StatusBadRequest, "bad_team", "Некорректный состав: нужны пять разных героев на позициях 1–5")
	case errors.Is(err, drafter.ErrTrainingMissing):
		render.Error(w, http.StatusNotFound, "training_not_found", "Тренировка не найдена. Начни новую")
	case errors.Is(err, drafter.ErrTrainingClosed):
		render.Error(w, http.StatusConflict, "training_closed", "Эта тренировка уже оценена или устарела. Начни новую")
	case errors.Is(err, drafter.ErrBadMonth), errors.Is(err, drafter.ErrBadCursor):
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
	case errors.As(err, &tooMany):
		w.Header().Set("Retry-After", strconv.Itoa(int(tooMany.RetryAfter.Seconds())))
		render.Error(w, http.StatusTooManyRequests, "too_many_trainings",
			"Слишком много тренировок за час. Передохни немного")
	default:
		a.logger.Error("drafter", "err", err)
		render.Internal(w)
	}
}

// drafterMatrix — GET /api/drafter/matrix: плотная матрица для расчёта «Анализа» на клиенте.
func (a *api) drafterMatrix(w http.ResponseWriter, r *http.Request) {
	body, etag, err := a.drafter.MatrixPayload(r.Context())
	if err != nil {
		a.drafterError(w, err)
		return
	}
	w.Header().Set("ETag", etag)
	// данные пересчитываются раз в сутки, состав позиций — раз в несколько часов: час с ревалидацией
	w.Header().Set("Cache-Control", "public, max-age=3600")
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(body)
}

type analyzeRequest struct {
	Radiant []drafter.Slot `json:"radiant"`
	Dire    []drafter.Slot `json:"dire"`
	Bans    []int          `json:"bans"`
}

// drafterAnalyze — POST /api/drafter/analyze: запасной серверный расчёт.
func (a *api) drafterAnalyze(w http.ResponseWriter, r *http.Request) {
	var req analyzeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.drafter.Analyze(r.Context(), req.Radiant, req.Dire, req.Bans)
	if err != nil {
		a.drafterError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, res)
}

// trainingNew — POST /api/drafter/training/new → { training_id, enemies }.
func (a *api) trainingNew(w http.ResponseWriter, r *http.Request) {
	var userID *int64
	if u := auth.UserFrom(r.Context()); u != nil {
		userID = &u.ID
	}
	id, enemies, err := a.drafter.NewTraining(r.Context(), userID)
	if err != nil {
		a.drafterError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, map[string]any{"training_id": id, "enemies": enemies})
}

type submitRequest struct {
	Team []drafter.Slot `json:"team"`
}

// trainingSubmit — POST /api/drafter/training/{id}/submit { team } → { score, breakdown, month_rank }.
func (a *api) trainingSubmit(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.drafter.Submit(r.Context(), auth.UserFrom(r.Context()).ID, chi.URLParam(r, "id"), req.Team)
	if err != nil {
		a.drafterError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, res)
}

// drafterHistory — GET /api/drafter/history?cursor=.
func (a *api) drafterHistory(w http.ResponseWriter, r *http.Request) {
	page, err := a.drafter.History(r.Context(), auth.UserFrom(r.Context()).ID, r.URL.Query().Get("cursor"))
	if err != nil {
		a.drafterError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, page)
}

// drafterLeaderboard — GET /api/drafter/leaderboard?month=YYYY-MM.
func (a *api) drafterLeaderboard(w http.ResponseWriter, r *http.Request) {
	var viewer int64
	if u := auth.UserFrom(r.Context()); u != nil {
		viewer = u.ID
	}
	board, err := a.drafter.Leaderboard(r.Context(), r.URL.Query().Get("month"), viewer)
	if err != nil {
		a.drafterError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, board)
}

// heroPage — GET /api/heroes/{hero_id}/page.
func (a *api) heroPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "hero_id"))
	if err != nil || id < 1 {
		render.Error(w, http.StatusNotFound, "hero_not_found", "Такого героя нет")
		return
	}
	page, err := a.heroPages.Page(r.Context(), id)
	switch {
	case errors.Is(err, heropage.ErrNotFound):
		render.Error(w, http.StatusNotFound, "hero_not_found", "Такого героя нет")
		return
	case err != nil:
		a.logger.Error("hero page", "hero_id", id, "err", err)
		render.Internal(w)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	render.JSON(w, http.StatusOK, page)
}
