package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/httpapi/render"
	"d2helper/internal/news"
	"d2helper/internal/patchnotes"
)

type newsListItem struct {
	GID     string    `json:"gid"`
	Title   string    `json:"title"`
	Date    time.Time `json:"date"`
	URL     string    `json:"url"`
	Excerpt string    `json:"excerpt"`
}

type newsItem struct {
	GID   string    `json:"gid"`
	Title string    `json:"title"`
	Date  time.Time `json:"date"`
	URL   string    `json:"url"`
	// HTML уже очищен санитайзером на бэке
	HTML string `json:"html"`
	// Patch — номер патча (7.41f), если новость о патче: фронт покажет окно с изменениями
	Patch string `json:"patch,omitempty"`
}

// newsList — GET /api/news?limit=20.
func (a *api) newsList(w http.ResponseWriter, r *http.Request) {
	items, err := a.news.List(r.Context(), intQuery(r, "limit", 20, 1, 50))
	if err != nil {
		a.logger.Error("list news", "err", err)
		render.Internal(w)
		return
	}
	out := make([]newsListItem, 0, len(items))
	for _, n := range items {
		out = append(out, newsListItem{GID: n.GID, Title: n.Title, Date: n.Date, URL: n.URL, Excerpt: n.Excerpt})
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	render.JSON(w, http.StatusOK, out)
}

// newsGet — GET /api/news/{gid}.
func (a *api) newsGet(w http.ResponseWriter, r *http.Request) {
	n, err := a.news.Get(r.Context(), chi.URLParam(r, "gid"))
	if errors.Is(err, news.ErrNotFound) {
		render.Error(w, http.StatusNotFound, "news_not_found", "Новость не найдена")
		return
	}
	if err != nil {
		a.logger.Error("get news", "err", err)
		render.Internal(w)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	render.JSON(w, http.StatusOK, newsItem{GID: n.GID, Title: n.Title, Date: n.Date, URL: n.URL, HTML: n.HTML, Patch: patchnotes.VersionFromTitle(n.Title)})
}
