package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"d2helper/internal/auth"
	"d2helper/internal/coach"
	"d2helper/internal/config"
	"d2helper/internal/store"
)

type fakeCoach struct {
	err     error
	lastID  int64
	lastCur string
	filter  string
}

func (f *fakeCoach) Resolve(_ context.Context, input string) (*coach.Resolved, error) {
	if f.err != nil {
		return nil, f.err
	}
	name := "Nick"
	return &coach.Resolved{AccountID: 1214690881, PersonaName: &name}, nil
}

func (f *fakeCoach) Summary(_ context.Context, id int64) (*coach.Summary, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	return &coach.Summary{Player: coach.Player{AccountID: id}, HasHistory: true, HeroesInForm: []coach.FormPeriod{}}, nil
}

func (f *fakeCoach) Matches(_ context.Context, id int64, filter, cursor string, _ int) (*coach.MatchesPage, error) {
	f.lastID, f.filter, f.lastCur = id, filter, cursor
	if f.err != nil {
		return nil, f.err
	}
	return &coach.MatchesPage{Items: []coach.MatchRow{}}, nil
}

func (f *fakeCoach) Refresh(_ context.Context, id int64) error { f.lastID = id; return f.err }

func (f *fakeCoach) Match(_ context.Context, matchID, accountID int64) (*coach.MatchDetail, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &coach.MatchDetail{MatchID: matchID, Players: []coach.MatchPlayer{}}, nil
}

type fakeTracked struct{ user, account int64 }

func (f *fakeTracked) SetTrackedAccount(_ context.Context, userID, accountID int64) error {
	f.user, f.account = userID, accountID
	return nil
}

func newCoachRouter(c *fakeCoach, tr *fakeTracked) http.Handler {
	return NewRouter(Deps{
		Config:   config.Config{SessionSecret: "secret"},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:       okPinger{},
		Sessions: &fakeSessions{users: map[string]store.User{"alice": {ID: 7, PersonaName: "Alice"}}},
		Coach:    c,
		Tracked:  tr,
	})
}

func send(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestCoachRoutesPassParameters(t *testing.T) {
	c := &fakeCoach{}
	h := newCoachRouter(c, &fakeTracked{})

	if w := send(h, http.MethodGet, "/api/coach/1214690881/summary", "", ""); w.Code != http.StatusOK || c.lastID != 1214690881 {
		t.Errorf("summary: %d %s", w.Code, w.Body)
	}
	if w := send(h, http.MethodGet, "/api/coach/5/matches?filter=win&cursor=1:2", "", ""); w.Code != http.StatusOK || c.filter != "win" || c.lastCur != "1:2" {
		t.Errorf("matches: %d filter=%q cursor=%q", w.Code, c.filter, c.lastCur)
	}
	if w := send(h, http.MethodPost, "/api/coach/5/refresh", "", ""); w.Code != http.StatusNoContent {
		t.Errorf("refresh: %d", w.Code)
	}
	if w := send(h, http.MethodGet, "/api/matches/77?account_id=5", "", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"match_id":77`) {
		t.Errorf("match: %d %s", w.Code, w.Body)
	}
	for _, path := range []string{"/api/coach/abc/summary", "/api/coach/0/summary", "/api/coach/99999999999/matches", "/api/coach/-5/refresh"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "refresh") {
			method = http.MethodPost
		}
		if w := send(h, method, path, "", ""); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d, ожидался 400", path, w.Code)
		}
	}
}

func TestCoachErrorMapping(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{coach.ErrPlayerNotFound, http.StatusNotFound, "player_not_found"},
		{coach.ErrBadInput, http.StatusBadRequest, "bad_profile_input"},
		{coach.ErrNoSteamKey, http.StatusUnprocessableEntity, "vanity_unavailable"},
		{coach.ErrMatchNotFound, http.StatusNotFound, "match_not_found"},
		{coach.ErrBadFilter, http.StatusBadRequest, "bad_request"},
		{&coach.TooSoonError{RetryAfter: 40 * time.Second}, http.StatusTooManyRequests, "refresh_too_soon"},
		{context.DeadlineExceeded, http.StatusBadGateway, "upstream_unavailable"},
	}
	for _, c := range cases {
		h := newCoachRouter(&fakeCoach{err: c.err}, &fakeTracked{})
		w := send(h, http.MethodPost, "/api/coach/5/refresh", "", "")
		if w.Code != c.status || !strings.Contains(w.Body.String(), c.code) {
			t.Errorf("%v → %d %s, ожидалось %d %s", c.err, w.Code, w.Body, c.status, c.code)
		}
		if c.status == http.StatusTooManyRequests && w.Header().Get("Retry-After") != "41" {
			t.Errorf("Retry-After = %q", w.Header().Get("Retry-After"))
		}
	}
}

func TestProfileResolve(t *testing.T) {
	h := newCoachRouter(&fakeCoach{}, &fakeTracked{})
	w := send(h, http.MethodPost, "/api/profile/resolve", `{"input":"1214690881"}`, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"account_id":1214690881`) {
		t.Errorf("resolve: %d %s", w.Code, w.Body)
	}
	long := `{"input":"` + strings.Repeat("a", 400) + `"}`
	if w := send(h, http.MethodPost, "/api/profile/resolve", long, ""); w.Code != http.StatusBadRequest {
		t.Errorf("слишком длинный ввод: %d", w.Code)
	}
	if w := send(h, http.MethodPost, "/api/profile/resolve", `not json`, ""); w.Code != http.StatusBadRequest {
		t.Errorf("не JSON: %d", w.Code)
	}
}

func TestProfileTracked(t *testing.T) {
	tr := &fakeTracked{}
	h := newCoachRouter(&fakeCoach{}, tr)

	if w := send(h, http.MethodPut, "/api/profile/tracked", `{"account_id":1214690881}`, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("гость: %d", w.Code)
	}
	if w := send(h, http.MethodPut, "/api/profile/tracked", `{"account_id":1214690881}`, "alice"); w.Code != http.StatusOK || tr.user != 7 || tr.account != 1214690881 {
		t.Errorf("смена профиля: %d user=%d account=%d", w.Code, tr.user, tr.account)
	}
	for _, body := range []string{`{"account_id":0}`, `{"account_id":-3}`, `{"account_id":99999999999}`, `{}`} {
		if w := send(h, http.MethodPut, "/api/profile/tracked", body, "alice"); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
}
