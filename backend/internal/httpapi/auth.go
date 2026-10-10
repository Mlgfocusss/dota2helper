package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"d2helper/internal/auth"
	"d2helper/internal/dota"
	"d2helper/internal/httpapi/render"
)

const (
	callbackPath = "/api/auth/steam/callback"
	// cookie с nonce входа: state действителен только в том браузере, где начали вход
	loginNonceCookie = "oid_nonce"
	loginCookiePath  = "/api/auth/steam"
)

// steamLogin — GET /api/auth/steam/login?return_to=/coach → редирект на Steam.
func (a *api) steamLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := auth.SafeReturnTo(r.URL.Query().Get("return_to"))
	nonce, err := auth.NewNonce()
	if err != nil {
		a.logger.Error("steam login: nonce", "err", err)
		render.Internal(w)
		return
	}
	base := a.baseURL(r)
	state := a.states.Sign(returnTo, nonce)
	http.SetCookie(w, a.loginNonceCookie(nonce, int(auth.StateTTL.Seconds())))
	http.Redirect(w, r, a.openid.LoginURL(base, callbackURL(base, state)), http.StatusFound)
}

// steamCallback — сюда Steam возвращает пользователя после входа.
func (a *api) steamCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	var nonce string
	if c, err := r.Cookie(loginNonceCookie); err == nil {
		nonce = c.Value
	}
	http.SetCookie(w, a.loginNonceCookie("", -1))

	returnTo, err := a.states.Verify(state, nonce)
	if err != nil {
		a.logger.Warn("steam login: bad state", "err", err)
		http.Redirect(w, r, withQuery("/", "login", "error"), http.StatusFound)
		return
	}

	base := a.baseURL(r)
	steamID, err := a.openid.Verify(r.Context(), q, callbackURL(base, state))
	if errors.Is(err, auth.ErrCancelled) {
		http.Redirect(w, r, withQuery(returnTo, "login", "cancelled"), http.StatusFound)
		return
	}
	if err != nil {
		a.logger.Warn("steam login: verification failed", "err", err)
		http.Redirect(w, r, withQuery(returnTo, "login", "error"), http.StatusFound)
		return
	}
	accountID, err := dota.AccountIDFromSteamID64(steamID)
	if err != nil {
		a.logger.Warn("steam login: bad steam id", "steam_id", steamID)
		http.Redirect(w, r, withQuery(returnTo, "login", "error"), http.StatusFound)
		return
	}

	lookupCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	persona, avatar := a.profiles.Lookup(lookupCtx, steamID, accountID)
	cancel()

	user, err := a.users.UpsertSteamUser(r.Context(), steamID, accountID, persona, avatar)
	if err != nil {
		a.logger.Error("steam login: save user", "err", err)
		http.Redirect(w, r, withQuery(returnTo, "login", "error"), http.StatusFound)
		return
	}
	token, expires, err := a.sessions.Create(r.Context(), user.ID)
	if err != nil {
		a.logger.Error("steam login: create session", "err", err)
		http.Redirect(w, r, withQuery(returnTo, "login", "error"), http.StatusFound)
		return
	}
	http.SetCookie(w, auth.SessionCookieFor(token, expires, a.cfg.CookieSecure))
	a.logger.Info("user logged in", "user_id", user.ID, "account_id", accountID)
	http.Redirect(w, r, returnTo, http.StatusFound)
}

// logout — POST /api/auth/logout. Без сессии тоже отвечает 204: выйти можно всегда.
func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(auth.SessionCookie); err == nil && c.Value != "" {
		if err := a.sessions.Delete(r.Context(), c.Value); err != nil {
			a.logger.Warn("logout: delete session", "err", err)
		}
	}
	http.SetCookie(w, auth.ClearSessionCookie(a.cfg.CookieSecure))
	w.WriteHeader(http.StatusNoContent)
}

type meUser struct {
	ID          int64  `json:"id"`
	AccountID   int64  `json:"account_id"`
	PersonaName string `json:"persona_name"`
	Avatar      string `json:"avatar"`
}

type meResponse struct {
	User             *meUser `json:"user"`
	TrackedAccountID *int64  `json:"tracked_account_id"`
}

// me — GET /api/me: пользователь или null для гостя.
func (a *api) me(w http.ResponseWriter, r *http.Request) {
	u := auth.UserFrom(r.Context())
	if u == nil {
		render.JSON(w, http.StatusOK, meResponse{})
		return
	}
	tracked := u.TrackedAccountID
	if tracked == nil {
		tracked = &u.AccountID
	}
	render.JSON(w, http.StatusOK, meResponse{
		User: &meUser{
			ID:          u.ID,
			AccountID:   u.AccountID,
			PersonaName: u.PersonaName,
			Avatar:      u.AvatarURL,
		},
		TrackedAccountID: tracked,
	})
}

func (a *api) loginNonceCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     loginNonceCookie,
		Value:    value,
		Path:     loginCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	}
}

// baseURL — адрес сайта: PUBLIC_URL, а если он не задан (разработка) — схема и
// хост текущего запроса. nginx и Vite передают исходный Host.
func (a *api) baseURL(r *http.Request) string {
	if a.cfg.PublicURL != "" {
		return a.cfg.PublicURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := r.Header.Get("X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	return scheme + "://" + r.Host
}

func callbackURL(base, state string) string {
	return base + callbackPath + "?" + url.Values{"state": {state}}.Encode()
}

// withQuery добавляет параметр к относительному адресу: /coach → /coach?login=error.
func withQuery(path, key, value string) string {
	u, err := url.Parse(path)
	if err != nil {
		return "/?" + url.Values{key: {value}}.Encode()
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}
