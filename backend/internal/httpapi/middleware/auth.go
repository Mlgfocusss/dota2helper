package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"d2helper/internal/auth"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/store"
)

// SessionResolver находит пользователя по токену сессии (auth.Sessions).
type SessionResolver interface {
	Resolve(ctx context.Context, token string) (store.User, time.Time, error)
}

// OptionalAuth кладёт пользователя в контекст, если у запроса есть действующая
// сессия; иначе запрос идёт дальше как гостевой. Продлённая сессия получает
// обновлённую cookie.
func OptionalAuth(sessions SessionResolver, cookieSecure bool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie(auth.SessionCookie)
			if err != nil || c.Value == "" {
				next.ServeHTTP(w, r)
				return
			}
			user, extendedUntil, err := sessions.Resolve(r.Context(), c.Value)
			switch {
			case errors.Is(err, store.ErrNotFound):
				http.SetCookie(w, auth.ClearSessionCookie(cookieSecure))
				next.ServeHTTP(w, r)
				return
			case err != nil:
				// БД недоступна — не ломаем публичные страницы, работаем как с гостем
				logger.Warn("resolve session", "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if !extendedUntil.IsZero() {
				http.SetCookie(w, auth.SessionCookieFor(c.Value, extendedUntil, cookieSecure))
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), &user)))
		})
	}
}

// RequireAuth отвечает 401, если пользователь не вошёл.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth.UserFrom(r.Context()) == nil {
			render.Error(w, http.StatusUnauthorized, "unauthorized", "Войди через Steam, чтобы продолжить")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// NoStore запрещает кэшировать ответы API; ручки с общими данными ставят свой Cache-Control.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
