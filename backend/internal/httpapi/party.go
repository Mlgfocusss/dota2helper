package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/auth"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/party"
	"d2helper/internal/store"
)

// PartyService — анкеты и поиск пати.
type PartyService interface {
	Mine(ctx context.Context, user store.User) (*party.Mine, error)
	Save(ctx context.Context, userID int64, in party.Input) (*party.Card, error)
	SetActive(ctx context.Context, userID int64, active bool) (*party.Card, error)
	Delete(ctx context.Context, userID int64) error
	Bump(ctx context.Context, userID int64) (*party.Card, error)
	Search(ctx context.Context, viewerID int64, q party.Query) (*party.Page, error)
	Report(ctx context.Context, reporterID, targetID int64, reason string) error
}

// partyError переводит ошибки сервиса в ответ API.
func (a *api) partyError(w http.ResponseWriter, err error) {
	var invalid *party.ValidationError
	var tooSoon *party.TooSoonError
	switch {
	case errors.As(err, &invalid):
		render.ValidationFailed(w, invalid.Fields)
	case errors.As(err, &tooSoon):
		w.Header().Set("Retry-After", strconv.Itoa(int(tooSoon.RetryAfter.Seconds())+1))
		render.Error(w, http.StatusTooManyRequests, "bump_too_soon", "Поднимать анкету можно раз в сутки")
	case errors.Is(err, party.ErrNoCard):
		render.Error(w, http.StatusNotFound, "no_party_card", "Анкета не найдена")
	case errors.Is(err, party.ErrBadReason), errors.Is(err, party.ErrBadCursor):
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
	case errors.Is(err, party.ErrSelfReport):
		render.Error(w, http.StatusBadRequest, "self_report", "На свою анкету жаловаться нельзя")
	case errors.Is(err, party.ErrAlreadyReported):
		render.Error(w, http.StatusConflict, "already_reported", "Ты уже отправил жалобу на эту анкету")
	default:
		a.logger.Error("party", "err", err)
		render.Internal(w)
	}
}

// partyMe — GET /api/party/me: моя анкета (card = null, если её нет) и подсказка ранга.
func (a *api) partyMe(w http.ResponseWriter, r *http.Request) {
	mine, err := a.party.Mine(r.Context(), *auth.UserFrom(r.Context()))
	if err != nil {
		a.partyError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, mine)
}

// partySave — PUT /api/party/me: создать или обновить анкету.
func (a *api) partySave(w http.ResponseWriter, r *http.Request) {
	var in party.Input
	if !decodeJSON(w, r, &in) {
		return
	}
	card, err := a.party.Save(r.Context(), auth.UserFrom(r.Context()).ID, in)
	if err != nil {
		a.partyError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, card)
}

type activeRequest struct {
	IsActive bool `json:"is_active"`
}

// partyActive — PATCH /api/party/me { is_active }: скрыть или показать анкету.
func (a *api) partyActive(w http.ResponseWriter, r *http.Request) {
	var req activeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	card, err := a.party.SetActive(r.Context(), auth.UserFrom(r.Context()).ID, req.IsActive)
	if err != nil {
		a.partyError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, card)
}

// partyDelete — DELETE /api/party/me.
func (a *api) partyDelete(w http.ResponseWriter, r *http.Request) {
	if err := a.party.Delete(r.Context(), auth.UserFrom(r.Context()).ID); err != nil {
		a.partyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// partyBump — POST /api/party/me/bump.
func (a *api) partyBump(w http.ResponseWriter, r *http.Request) {
	card, err := a.party.Bump(r.Context(), auth.UserFrom(r.Context()).ID)
	if err != nil {
		a.partyError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, card)
}

// partySearch — GET /api/party?looking_for&rank_min&rank_max&positions&modes&goals&mic&discord&cursor.
func (a *api) partySearch(w http.ResponseWriter, r *http.Request) {
	q, ok := parsePartyQuery(r)
	if !ok {
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
		return
	}
	page, err := a.party.Search(r.Context(), auth.UserFrom(r.Context()).ID, q)
	if err != nil {
		a.partyError(w, err)
		return
	}
	render.JSON(w, http.StatusOK, page)
}

func parsePartyQuery(r *http.Request) (party.Query, bool) {
	v := r.URL.Query()
	q := party.Query{
		LookingFor: "team",
		RankMin:    0,
		RankMax:    party.MaxRank,
		Mic:        v.Get("mic") == "1",
		Discord:    v.Get("discord") == "1",
		Cursor:     v.Get("cursor"),
	}
	if s := v.Get("looking_for"); s != "" {
		q.LookingFor = s
	}
	if q.LookingFor != "team" && q.LookingFor != "players" {
		return q, false
	}
	for name, dst := range map[string]*int{"rank_min": &q.RankMin, "rank_max": &q.RankMax} {
		if s := v.Get(name); s != "" {
			n, err := strconv.Atoi(s)
			if err != nil || n < 0 || n > party.MaxRank {
				return q, false
			}
			*dst = n
		}
	}
	if q.RankMin > q.RankMax {
		q.RankMin, q.RankMax = q.RankMax, q.RankMin
	}
	for _, s := range splitList(v.Get("positions")) {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 5 {
			return q, false
		}
		q.Positions = append(q.Positions, int16(n))
	}
	q.Modes = splitList(v.Get("modes"))
	q.Goals = splitList(v.Get("goals"))
	return q, true
}

// splitList разбирает «a,b,c»; пустые элементы отбрасываются, длина списка ограничена.
func splitList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" && len(out) < 10 {
			out = append(out, part)
		}
	}
	return out
}

type reportRequest struct {
	Reason string `json:"reason"`
}

// partyReport — POST /api/party/{user_id}/report { reason: spam|abuse|fake }.
func (a *api) partyReport(w http.ResponseWriter, r *http.Request) {
	target, err := strconv.ParseInt(chi.URLParam(r, "user_id"), 10, 64)
	if err != nil || target < 1 {
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
		return
	}
	var req reportRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := a.party.Report(r.Context(), auth.UserFrom(r.Context()).ID, target, req.Reason); err != nil {
		a.partyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// moscow — часовой пояс месяца для топа драфтеров (SPEC 6.3); без перехода на летнее время.
var moscow = time.FixedZone("MSK", 3*60*60)

// monthBounds — начало месяца и начало следующего по Москве.
func monthBounds(now time.Time) (time.Time, time.Time) {
	n := now.In(moscow)
	start := time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, moscow)
	return start, start.AddDate(0, 1, 0)
}
