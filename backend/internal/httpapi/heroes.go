package httpapi

import (
	"net/http"
	"strings"

	"d2helper/internal/httpapi/render"
)

// heroes — GET /api/heroes: справочник героев, кэш на сутки и ETag.
func (a *api) heroes(w http.ResponseWriter, r *http.Request) {
	body, etag, ok := a.constants.HeroesJSON()
	if !ok {
		constantsLoading(w)
		return
	}
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(body)
}

// patch — GET /api/patch: { number, label }.
func (a *api) patch(w http.ResponseWriter, _ *http.Request) {
	p, ok := a.constants.Patch()
	if !ok {
		constantsLoading(w)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	render.JSON(w, http.StatusOK, p)
}

func constantsLoading(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	render.Error(w, http.StatusServiceUnavailable, "constants_loading", "Загружаем данные о героях. Попробуй через минуту")
}

// etagMatches учитывает слабые ETag: nginx со сжатием превращает "x" в W/"x".
func etagMatches(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimPrefix(strings.TrimSpace(part), "W/")
		if p == etag || p == "*" {
			return true
		}
	}
	return false
}
