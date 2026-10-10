package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"d2helper/internal/auth"
	"d2helper/internal/httpapi/render"
)

const (
	feedbackMaxText    = 2000
	feedbackMaxContact = 200
	maxJSONBody        = 64 << 10
)

type feedbackRequest struct {
	Text    string `json:"text"`
	Contact string `json:"contact"`
}

// feedback — POST /api/feedback { text, contact? }.
func (a *api) feedback(w http.ResponseWriter, r *http.Request) {
	var req feedbackRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	contact := strings.TrimSpace(req.Contact)
	switch {
	case utf8.RuneCountInString(text) < 2:
		render.Error(w, http.StatusBadRequest, "feedback_empty", "Напиши хотя бы пару слов")
		return
	case utf8.RuneCountInString(text) > feedbackMaxText:
		render.Error(w, http.StatusBadRequest, "feedback_too_long", "Слишком длинный текст: максимум 2000 символов")
		return
	case utf8.RuneCountInString(contact) > feedbackMaxContact:
		render.Error(w, http.StatusBadRequest, "contact_too_long", "Слишком длинный контакт: максимум 200 символов")
		return
	}

	var userID *int64
	who := "гость"
	if u := auth.UserFrom(r.Context()); u != nil {
		userID = &u.ID
		who = fmt.Sprintf("%s (id %d, account %d)", u.PersonaName, u.ID, u.AccountID)
	}
	id, err := a.feedbackStore.CreateFeedback(r.Context(), userID, text, contact)
	if err != nil {
		a.logger.Error("save feedback", "err", err)
		render.Internal(w)
		return
	}

	if a.notifier != nil && a.notifier.Enabled() {
		msg := fmt.Sprintf("Отзыв #%d от %s\n\n%s", id, who, text)
		if contact != "" {
			msg += "\n\nКонтакт: " + contact
		}
		// отправка в Telegram не должна задерживать ответ пользователю
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := a.notifier.Send(ctx, msg); err != nil {
				a.logger.Warn("feedback: telegram", "err", err)
			}
		}()
	}
	render.JSON(w, http.StatusCreated, map[string]int64{"id": id})
}

// decodeJSON читает тело запроса; при ошибке сам отвечает 400.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err := dec.Decode(dst); err != nil {
		render.Error(w, http.StatusBadRequest, "bad_request", "Некорректный запрос")
		return false
	}
	return true
}
