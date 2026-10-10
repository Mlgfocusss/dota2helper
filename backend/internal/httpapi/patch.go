package httpapi

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/httpapi/render"
	"d2helper/internal/patchnotes"
)

// patchGet — GET /api/patches/{version}: патчноуты (7.41f) для окна патча на странице новости.
func (a *api) patchGet(w http.ResponseWriter, r *http.Request) {
	p, err := a.patches.Get(r.Context(), chi.URLParam(r, "version"))
	if errors.Is(err, patchnotes.ErrNotFound) {
		render.Error(w, http.StatusNotFound, "patch_not_found", "Список изменений этого патча не найден")
		return
	}
	if err != nil {
		a.logger.Error("patch notes", "err", err)
		render.Error(w, http.StatusBadGateway, "patch_unavailable", "Не удалось загрузить список изменений. Попробуй позже")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=900")
	render.JSON(w, http.StatusOK, p)
}
