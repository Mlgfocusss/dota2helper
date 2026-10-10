package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"

	"d2helper/internal/httpapi/render"
)

// Recover перехватывает панику в хендлере, пишет её в лог и отвечает 500.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				logger.Error("panic in handler",
					"panic", rec,
					"path", r.URL.Path,
					"req_id", chimw.GetReqID(r.Context()),
					"stack", string(debug.Stack()),
				)
				render.Internal(w)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
