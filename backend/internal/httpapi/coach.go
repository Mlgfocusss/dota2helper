package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/auth"
	"d2helper/internal/coach"
	"d2helper/internal/httpapi/render"
)

// CoachService — то, что нужно хендлерам «Тренера».
type CoachService interface {
	Resolve(ctx context.Context, input string) (*coach.Resolved, error)
	Summary(ctx context.Context, accountID int64) (*coach.Summary, error)
	Matches(ctx context.Context, accountID int64, filter, cursor string, limit int) (*coach.MatchesPage, error)
	Refresh(ctx context.Context, accountID int64) error
	Match(ctx context.Context, matchID, accountID int64) (*coach.MatchDetail, error)
}

// TrackedStore сохраняет, чей профиль пользователь смотрит в «Тренере».
type TrackedStore interface {
	SetTrackedAccount(ctx context.Context, userID, accountID int64) error
}

const (
	maxAccountID    = 1<<32 - 1
	maxProfileInput = 300
	matchesPageSize = 20
)

// coachError переводит ошибки сервиса в ответ API с текстом для пользователя.
func (a *api) coachError(w http.ResponseWriter, err error) {
	var tooSoon *coach.TooSoonError
	switch {
	case errors.Is(err, coach.ErrBadInput):
		render.Error(w, http.StatusBadRequest, "bad_profile_input",
			"Не получилось разобрать ссылку или ID. Подойдёт ссылка на Steam-профиль, SteamID или ID из Dota")
	case errors.Is(err, coach.ErrNoSteamKey):
		render.Error(w, http.StatusUnprocessableEntity, "vanity_unavailable",
			"Поиск по короткому имени сейчас недоступен. Вставь ссылку вида steamcommunity.com/profiles/… или ID из Dota")
	case errors.Is(err, coach.ErrPlayerNotFound):
		render.Error(w, http.StatusNotFound, "player_not_found", "Профиль не найден. Проверь ссылку")
	case errors.Is(err, coach.ErrMatchNotFound):
		render.Error(w, http.StatusNotFound, "match_not_found", "Матч не найден")
	case errors.Is(err, coach.ErrBadFilter), errors.Is(err, coach.ErrBadCursor):
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
	case errors.As(err, &tooSoon):
		w.Header().Set("Retry-After", strconv.Itoa(int(tooSoon.RetryAfter.Seconds())+1))
		render.Error(w, http.StatusTooManyRequests, "refresh_too_soon", "Обновлять можно раз в минуту. Попробуй чуть позже")
	default:
		a.logger.Error("coach", "err", err)
		render.Error(w, http.StatusBadGateway, "upstream_unavailable",
			"Не удалось получить данные из OpenDota. Попробуй через минуту")
	}
}

// accountIDParam — {account_id} из пути: положительное число в пределах 32 бит.
func accountIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "account_id"), 10, 64)
	if err != nil || id < 1 || id > maxAccountID {
		render.Error(w, http.StatusBadRequest, "bad_account_id", "Некорректный ID игрока")
		return 0, false
	}
	return id, true
}

type resolveRequest struct {
	Input string `json:"input"`
}

// profileResolve — POST /api/profile/resolve { input } → { account_id, persona_name, avatar }.
func (a *api) profileResolve(w http.ResponseWriter, r *http.Request) {
	var req resolveRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if utf8.RuneCountInString(req.Input) > maxProfileInput {
		a.coachError(w, coach.ErrBadInput)
		return
	}
	found, err := a.coach.Resolve(r.Context(), req.Input)
	if err != nil {
		a.coachError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, found)
}

type trackedRequest struct {
	AccountID int64 `json:"account_id"`
}

// profileTracked — PUT /api/profile/tracked { account_id }: сменить отслеживаемый профиль.
func (a *api) profileTracked(w http.ResponseWriter, r *http.Request) {
	var req trackedRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.AccountID < 1 || req.AccountID > maxAccountID {
		render.Error(w, http.StatusBadRequest, "bad_account_id", "Некорректный ID игрока")
		return
	}
	user := auth.UserFrom(r.Context())
	if err := a.tracked.SetTrackedAccount(r.Context(), user.ID, req.AccountID); err != nil {
		a.logger.Error("set tracked account", "err", err)
		render.Internal(w)
		return
	}
	render.JSON(w, http.StatusOK, trackedRequest{AccountID: req.AccountID})
}

// coachSummary — GET /api/coach/{account_id}/summary.
func (a *api) coachSummary(w http.ResponseWriter, r *http.Request) {
	id, ok := accountIDParam(w, r)
	if !ok {
		return
	}
	sum, err := a.coach.Summary(r.Context(), id)
	if err != nil {
		a.coachError(w, err)
		return
	}
	// Summary общий для всех пользователей: статус разбора ставим на копию
	out := *sum
	if sum.LastMatch != nil {
		out.LastMatch = &a.withAnalyses(r, []coach.MatchRow{*sum.LastMatch})[0]
	}
	render.JSON(w, http.StatusOK, out)
}

// coachMatches — GET /api/coach/{account_id}/matches?filter=all|win|loss&cursor=.
func (a *api) coachMatches(w http.ResponseWriter, r *http.Request) {
	id, ok := accountIDParam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, err := a.coach.Matches(r.Context(), id, q.Get("filter"), q.Get("cursor"), matchesPageSize)
	if err != nil {
		a.coachError(w, err)
		return
	}
	out := *page
	out.Items = a.withAnalyses(r, page.Items)
	render.JSON(w, http.StatusOK, out)
}

// coachRefresh — POST /api/coach/{account_id}/refresh: заново загрузить историю игрока.
func (a *api) coachRefresh(w http.ResponseWriter, r *http.Request) {
	id, ok := accountIDParam(w, r)
	if !ok {
		return
	}
	if err := a.coach.Refresh(r.Context(), id); err != nil {
		a.coachError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// matchGet — GET /api/matches/{match_id}?account_id=.
func (a *api) matchGet(w http.ResponseWriter, r *http.Request) {
	matchID, err := strconv.ParseInt(chi.URLParam(r, "match_id"), 10, 64)
	if err != nil || matchID < 1 {
		render.Error(w, http.StatusNotFound, "match_not_found", "Матч не найден")
		return
	}
	accountID, _ := strconv.ParseInt(r.URL.Query().Get("account_id"), 10, 64)
	m, err := a.coach.Match(r.Context(), matchID, accountID)
	if err != nil {
		a.coachError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	render.JSON(w, http.StatusOK, m)
}
