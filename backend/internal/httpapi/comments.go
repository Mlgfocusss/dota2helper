package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/auth"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/store"
)

const (
	commentMaxText   = 1000
	commentsPageSize = 20
)

type commentJSON struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
	Author    string    `json:"author"`
	Avatar    string    `json:"avatar"`
	// Mine — комментарий текущего пользователя: его можно удалить
	Mine bool `json:"mine"`
}

type commentsResponse struct {
	Items []commentJSON `json:"items"`
	Total int           `json:"total"`
	// NextBefore — id, с которого просить следующую страницу; null — комментариев больше нет
	NextBefore *int64 `json:"next_before"`
}

func commentToJSON(c store.Comment, viewer *store.User) commentJSON {
	name := c.AuthorName
	if name == "" {
		name = "Игрок"
	}
	return commentJSON{
		ID: c.ID, Text: c.Text, CreatedAt: c.CreatedAt, Author: name, Avatar: c.AuthorAva,
		Mine: viewer != nil && viewer.ID == c.UserID,
	}
}

// commentsList — GET /api/news/{gid}/comments?before=<id>: новые сверху.
func (a *api) commentsList(w http.ResponseWriter, r *http.Request) {
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	before = max(before, 0)
	// на один больше страницы — так видно, есть ли следующая
	items, total, err := a.comments.ListComments(r.Context(), chi.URLParam(r, "gid"), before, commentsPageSize+1)
	if err != nil {
		a.logger.Error("list comments", "err", err)
		render.Internal(w)
		return
	}
	resp := commentsResponse{Items: make([]commentJSON, 0, len(items)), Total: total}
	if len(items) > commentsPageSize {
		items = items[:commentsPageSize]
		next := items[len(items)-1].ID
		resp.NextBefore = &next
	}
	viewer := auth.UserFrom(r.Context())
	for _, c := range items {
		resp.Items = append(resp.Items, commentToJSON(c, viewer))
	}
	render.JSON(w, http.StatusOK, resp)
}

type commentRequest struct {
	Text string `json:"text"`
}

// commentCreate — POST /api/news/{gid}/comments { text }; только для вошедших.
func (a *api) commentCreate(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	var req commentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	switch {
	case text == "":
		render.Error(w, http.StatusBadRequest, "comment_empty", "Напиши хотя бы пару слов")
		return
	case utf8.RuneCountInString(text) > commentMaxText:
		render.Error(w, http.StatusBadRequest, "comment_too_long", "Слишком длинный комментарий: максимум 1000 символов")
		return
	}
	c, err := a.comments.CreateComment(r.Context(), chi.URLParam(r, "gid"), user.ID, text)
	if errors.Is(err, store.ErrNotFound) {
		render.Error(w, http.StatusNotFound, "news_not_found", "Новость не найдена")
		return
	}
	if err != nil {
		a.logger.Error("create comment", "err", err)
		render.Internal(w)
		return
	}
	render.JSON(w, http.StatusCreated, commentToJSON(c, user))
}

// commentDelete — DELETE /api/news/{gid}/comments/{id}: только свой комментарий.
func (a *api) commentDelete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Error(w, http.StatusNotFound, "comment_not_found", "Комментарий не найден")
		return
	}
	err = a.comments.DeleteComment(r.Context(), chi.URLParam(r, "gid"), id, user.ID)
	if errors.Is(err, store.ErrNotFound) {
		render.Error(w, http.StatusNotFound, "comment_not_found", "Комментарий не найден")
		return
	}
	if err != nil {
		a.logger.Error("delete comment", "err", err)
		render.Internal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
