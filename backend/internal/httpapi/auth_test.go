package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"d2helper/internal/auth"
	"d2helper/internal/config"
	"d2helper/internal/dota"
	"d2helper/internal/store"
)

// --- фейки ---------------------------------------------------------------

type fakeOpenID struct {
	steamID  string
	err      error
	expected string // return_to, с которым вызвали Verify
}

func (f *fakeOpenID) LoginURL(realm, returnTo string) string {
	return "https://steam.test/login?" + url.Values{"realm": {realm}, "return_to": {returnTo}}.Encode()
}

func (f *fakeOpenID) Verify(_ context.Context, _ url.Values, expected string) (string, error) {
	f.expected = expected
	return f.steamID, f.err
}

type fakeUsers struct{ last store.User }

func (f *fakeUsers) UpsertSteamUser(_ context.Context, steamID64 string, accountID int64, persona, avatar string) (store.User, error) {
	f.last = store.User{ID: 42, SteamID64: steamID64, AccountID: accountID, PersonaName: persona, AvatarURL: avatar}
	return f.last, nil
}

type fakeSessions struct {
	users map[string]store.User
}

func (f *fakeSessions) Create(_ context.Context, userID int64) (string, time.Time, error) {
	token := "token-" + time.Now().Format("150405.000000")
	f.users[token] = store.User{ID: userID, AccountID: 1214690881, PersonaName: "Mlgfocus"}
	return token, time.Now().Add(auth.SessionTTL), nil
}

func (f *fakeSessions) Resolve(_ context.Context, token string) (store.User, time.Time, error) {
	u, ok := f.users[token]
	if !ok {
		return store.User{}, time.Time{}, store.ErrNotFound
	}
	return u, time.Time{}, nil
}

func (f *fakeSessions) Delete(_ context.Context, token string) error {
	delete(f.users, token)
	return nil
}

type fakeProfiles struct{}

func (fakeProfiles) Lookup(context.Context, string, int64) (string, string) {
	return "Mlgfocus", "https://avatars.test/a.jpg"
}

type fakeConstants struct{ loaded bool }

func (f fakeConstants) HeroesJSON() ([]byte, string, bool) {
	return []byte(`[{"id":1}]`), `"abc"`, f.loaded
}

func (f fakeConstants) Patch() (dota.Patch, bool) {
	return dota.Patch{Number: "7.41", Label: "7.41F"}, f.loaded
}

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

type testEnv struct {
	handler  http.Handler
	openid   *fakeOpenID
	users    *fakeUsers
	sessions *fakeSessions
}

func newTestEnv(constantsLoaded bool) *testEnv {
	env := &testEnv{
		openid:   &fakeOpenID{steamID: "76561199174956609"},
		users:    &fakeUsers{},
		sessions: &fakeSessions{users: map[string]store.User{}},
	}
	env.handler = NewRouter(Deps{
		Config:    config.Config{SessionSecret: "secret", AppName: "D2Helper"},
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:        okPinger{},
		Users:     env.users,
		Sessions:  env.sessions,
		States:    auth.NewStateSigner("secret"),
		OpenID:    env.openid,
		Profiles:  fakeProfiles{},
		Constants: fakeConstants{loaded: constantsLoaded},
	})
	return env
}

func (e *testEnv) do(r *http.Request) *http.Response {
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	return w.Result()
}

func cookieByName(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// startLogin проходит /login и возвращает state и cookie с nonce.
func (e *testEnv) startLogin(t *testing.T, returnTo string) (string, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/steam/login?return_to="+url.QueryEscape(returnTo), nil)
	req.Host = "localhost:5173"
	resp := e.do(req)
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login: статус %d", resp.StatusCode)
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if loc.Query().Get("realm") != "http://localhost:5173" {
		t.Fatalf("realm = %q", loc.Query().Get("realm"))
	}
	cb, _ := url.Parse(loc.Query().Get("return_to"))
	if cb.String() == "" || !strings.HasPrefix(cb.String(), "http://localhost:5173/api/auth/steam/callback?state=") {
		t.Fatalf("return_to = %q", cb)
	}
	nonce := cookieByName(resp, loginNonceCookie)
	if nonce == nil || !nonce.HttpOnly || nonce.Value == "" {
		t.Fatal("нет httpOnly cookie с nonce")
	}
	return cb.Query().Get("state"), nonce
}

func (e *testEnv) callback(state string, nonce *http.Cookie) *http.Response {
	q := url.Values{"state": {state}, "openid.mode": {"id_res"}}
	req := httptest.NewRequest(http.MethodGet, "/api/auth/steam/callback?"+q.Encode(), nil)
	req.Host = "localhost:5173"
	if nonce != nil {
		req.AddCookie(nonce)
	}
	return e.do(req)
}

// --- тесты ---------------------------------------------------------------

func TestSteamLoginFlow(t *testing.T) {
	env := newTestEnv(true)
	state, nonce := env.startLogin(t, "/coach")

	resp := env.callback(state, nonce)
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/coach" {
		t.Fatalf("callback: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if env.openid.expected != "http://localhost:5173/api/auth/steam/callback?state="+url.QueryEscape(state) {
		t.Errorf("Verify вызван с return_to %q", env.openid.expected)
	}
	if env.users.last.AccountID != 1214690881 || env.users.last.PersonaName != "Mlgfocus" {
		t.Errorf("сохранён пользователь %+v", env.users.last)
	}
	sid := cookieByName(resp, auth.SessionCookie)
	if sid == nil || !sid.HttpOnly || sid.SameSite != http.SameSiteLaxMode || sid.MaxAge <= 0 {
		t.Fatalf("cookie сессии: %+v", sid)
	}

	// /me с этой cookie — пользователь
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(sid)
	var me meResponse
	_ = json.NewDecoder(env.do(req).Body).Decode(&me)
	if me.User == nil || me.User.ID != 42 || me.TrackedAccountID == nil || *me.TrackedAccountID != 1214690881 {
		t.Fatalf("/me = %+v", me)
	}

	// выход удаляет сессию
	req = httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(sid)
	resp = env.do(req)
	if resp.StatusCode != http.StatusNoContent || cookieByName(resp, auth.SessionCookie).MaxAge >= 0 {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(sid)
	me = meResponse{}
	_ = json.NewDecoder(env.do(req).Body).Decode(&me)
	if me.User != nil {
		t.Error("после выхода /me возвращает гостя")
	}
}

func TestCallbackRejectsForeignBrowser(t *testing.T) {
	env := newTestEnv(true)
	state, _ := env.startLogin(t, "/coach")

	// ссылка с чужим state открыта без cookie нашего браузера (login CSRF)
	resp := env.callback(state, nil)
	if resp.Header.Get("Location") != "/?login=error" || cookieByName(resp, auth.SessionCookie) != nil {
		t.Fatalf("ожидался отказ: %q", resp.Header.Get("Location"))
	}
	if env.users.last.ID != 0 {
		t.Error("пользователь не должен создаваться")
	}
}

func TestCallbackCancelledAndFailed(t *testing.T) {
	env := newTestEnv(true)

	env.openid.err = auth.ErrCancelled
	state, nonce := env.startLogin(t, "/coach?tab=1")
	if loc := env.callback(state, nonce).Header.Get("Location"); loc != "/coach?login=cancelled&tab=1" {
		t.Errorf("отмена: %q", loc)
	}

	env.openid.err = errors.New("signature rejected")
	state, nonce = env.startLogin(t, "/play")
	if loc := env.callback(state, nonce).Header.Get("Location"); loc != "/play?login=error" {
		t.Errorf("ошибка проверки: %q", loc)
	}
}

func TestLoginSanitizesReturnTo(t *testing.T) {
	env := newTestEnv(true)
	state, nonce := env.startLogin(t, "//evil.com/steal")
	if loc := env.callback(state, nonce).Header.Get("Location"); loc != "/" {
		t.Errorf("open redirect: %q", loc)
	}
}

func TestMeForGuest(t *testing.T) {
	env := newTestEnv(true)
	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "unknown"})
	resp := env.do(req)
	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != `{"user":null,"tracked_account_id":null}` {
		t.Errorf("/me гостя: %s", body)
	}
	if c := cookieByName(resp, auth.SessionCookie); c == nil || c.MaxAge >= 0 {
		t.Error("неизвестная сессия — cookie стирается")
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Error("ответы API по умолчанию не кэшируются")
	}
}

func TestHeroesETag(t *testing.T) {
	env := newTestEnv(true)
	resp := env.do(httptest.NewRequest(http.MethodGet, "/api/heroes", nil))
	if resp.StatusCode != http.StatusOK || resp.Header.Get("ETag") != `"abc"` ||
		!strings.Contains(resp.Header.Get("Cache-Control"), "max-age=86400") {
		t.Fatalf("heroes: %d %v", resp.StatusCode, resp.Header)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/heroes", nil)
	req.Header.Set("If-None-Match", `W/"abc"`)
	if resp := env.do(req); resp.StatusCode != http.StatusNotModified {
		t.Errorf("с совпадающим ETag ожидался 304, получен %d", resp.StatusCode)
	}

	patch := env.do(httptest.NewRequest(http.MethodGet, "/api/patch", nil))
	body, _ := io.ReadAll(patch.Body)
	if strings.TrimSpace(string(body)) != `{"number":"7.41","label":"7.41F"}` {
		t.Errorf("patch: %s", body)
	}
}

func TestHeroesNotLoaded(t *testing.T) {
	env := newTestEnv(false)
	if resp := env.do(httptest.NewRequest(http.MethodGet, "/api/heroes", nil)); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("до загрузки констант ожидался 503, получен %d", resp.StatusCode)
	}
}
