package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"time"

	"d2helper/internal/store"
)

const (
	SessionCookie = "sid"
	SessionTTL    = 30 * 24 * time.Hour
	// продлеваем сессию не чаще раза в сутки, чтобы не писать в БД на каждый запрос
	extendEvery = 24 * time.Hour
)

// SessionStore — хранилище сессий (store.Store).
type SessionStore interface {
	CreateSession(ctx context.Context, id string, userID int64, expiresAt time.Time) error
	SessionUser(ctx context.Context, id string) (store.User, time.Time, error)
	ExtendSession(ctx context.Context, id string, expiresAt time.Time) error
	DeleteSession(ctx context.Context, id string) error
}

// Sessions выдаёт и проверяет сессии. В cookie лежит случайный токен,
// в БД — только его SHA‑256: утечка таблицы не даёт войти под пользователем.
type Sessions struct {
	store SessionStore
	now   func() time.Time
}

func NewSessions(s SessionStore) *Sessions {
	return &Sessions{store: s, now: time.Now}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Create открывает сессию и возвращает токен для cookie.
func (s *Sessions) Create(ctx context.Context, userID int64) (string, time.Time, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	expires := s.now().Add(SessionTTL)
	if err := s.store.CreateSession(ctx, hashToken(token), userID, expires); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// Resolve находит пользователя по токену. Если сессию пора продлить, продлевает
// её и возвращает новый срок (иначе нулевое время). store.ErrNotFound — сессии нет.
func (s *Sessions) Resolve(ctx context.Context, token string) (store.User, time.Time, error) {
	id := hashToken(token)
	user, expires, err := s.store.SessionUser(ctx, id)
	if err != nil {
		return store.User{}, time.Time{}, err
	}
	now := s.now()
	if expires.Sub(now) > SessionTTL-extendEvery {
		return user, time.Time{}, nil
	}
	newExpires := now.Add(SessionTTL)
	if err := s.store.ExtendSession(ctx, id, newExpires); err != nil {
		// не продлили — не страшно, сессия ещё действует
		return user, time.Time{}, nil
	}
	return user, newExpires, nil
}

func (s *Sessions) Delete(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, hashToken(token))
}

// SessionCookieFor — cookie сессии: httpOnly, SameSite=Lax.
func SessionCookieFor(token string, expires time.Time, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie удаляет cookie сессии в браузере.
func ClearSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

type ctxKey struct{}

// WithUser кладёт пользователя в контекст запроса.
func WithUser(ctx context.Context, u *store.User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// UserFrom — пользователь из контекста; nil для гостя.
func UserFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(ctxKey{}).(*store.User)
	return u
}
