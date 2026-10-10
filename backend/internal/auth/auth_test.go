package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"d2helper/internal/store"
)

// --- state ---------------------------------------------------------------

func TestStateRoundTrip(t *testing.T) {
	s := NewStateSigner("secret")
	state := s.Sign("/coach?tab=1", "nonce-1")
	got, err := s.Verify(state, "nonce-1")
	if err != nil || got != "/coach?tab=1" {
		t.Fatalf("Verify = %q, %v", got, err)
	}
}

func TestStateRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	s := NewStateSigner("secret")
	s.now = func() time.Time { return now }
	state := s.Sign("/coach", "n")

	cases := map[string]func() (string, error){
		"чужой nonce (cookie другого браузера)": func() (string, error) { return s.Verify(state, "other") },
		"пустой nonce": func() (string, error) { return s.Verify(state, "") },
		"подделанная подпись": func() (string, error) {
			return s.Verify(state[:len(state)-2]+"xx", "n")
		},
		"другой секрет": func() (string, error) { return NewStateSigner("other").Verify(state, "n") },
		"мусор":         func() (string, error) { return s.Verify("garbage", "n") },
	}
	for name, fn := range cases {
		if _, err := fn(); !errors.Is(err, ErrBadState) {
			t.Errorf("%s: ожидалась ErrBadState, получено %v", name, err)
		}
	}

	now = now.Add(StateTTL + time.Second)
	if _, err := s.Verify(state, "n"); !errors.Is(err, ErrBadState) {
		t.Error("просроченный state должен отклоняться")
	}
}

func TestSafeReturnTo(t *testing.T) {
	cases := map[string]string{
		"/coach":                       "/coach",
		"/tools/drafter?tab=a#x":       "/tools/drafter?tab=a#x",
		"":                             "/",
		"coach":                        "/",
		"//evil.com":                   "/",
		"/\\evil.com":                  "/",
		"https://evil.com/":            "/",
		"/ok\nSet-Cookie: x":           "/",
		"/" + strings.Repeat("a", 600): "/",
	}
	for in, want := range cases {
		if got := SafeReturnTo(in); got != want {
			t.Errorf("SafeReturnTo(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

// --- OpenID --------------------------------------------------------------

type fakeDoer struct {
	body    string
	status  int
	lastReq *http.Request
	form    url.Values
}

func (f *fakeDoer) Do(r *http.Request) (*http.Response, error) {
	f.lastReq = r
	raw, _ := io.ReadAll(r.Body)
	f.form, _ = url.ParseQuery(string(raw))
	status := f.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

const callback = "http://localhost:5173/api/auth/steam/callback?state=abc"

func steamResponse() url.Values {
	return url.Values{
		"state":                 {"abc"},
		"openid.ns":             {openIDNS},
		"openid.mode":           {"id_res"},
		"openid.op_endpoint":    {SteamOpenIDEndpoint},
		"openid.claimed_id":     {"https://steamcommunity.com/openid/id/76561199174956609"},
		"openid.identity":       {"https://steamcommunity.com/openid/id/76561199174956609"},
		"openid.return_to":      {callback},
		"openid.response_nonce": {"2026-10-09T12:00:00Zabc"},
		"openid.assoc_handle":   {"1234567890"},
		"openid.signed":         {"signed,op_endpoint,claimed_id,identity,return_to,response_nonce,assoc_handle"},
		"openid.sig":            {"c2lnbmF0dXJl"},
	}
}

func TestLoginURL(t *testing.T) {
	o := NewOpenID(&fakeDoer{})
	u, err := url.Parse(o.LoginURL("http://localhost:5173", callback))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "steamcommunity.com" || q.Get("openid.mode") != "checkid_setup" ||
		q.Get("openid.return_to") != callback || q.Get("openid.realm") != "http://localhost:5173" ||
		q.Get("openid.claimed_id") != identifierSelect || q.Get("openid.ns") != openIDNS {
		t.Errorf("LoginURL = %s", u)
	}
}

func TestVerifySuccess(t *testing.T) {
	doer := &fakeDoer{body: "ns:" + openIDNS + "\nis_valid:true\n"}
	id, err := NewOpenID(doer).Verify(context.Background(), steamResponse(), callback)
	if err != nil || id != "76561199174956609" {
		t.Fatalf("Verify = %q, %v", id, err)
	}
	if doer.form.Get("openid.mode") != "check_authentication" {
		t.Errorf("mode = %q", doer.form.Get("openid.mode"))
	}
	if doer.form.Get("openid.sig") != "c2lnbmF0dXJl" || doer.form.Has("state") {
		t.Errorf("в Steam уходят только openid.* параметры: %v", doer.form)
	}
	if doer.lastReq.Method != http.MethodPost || doer.lastReq.URL.String() != SteamOpenIDEndpoint {
		t.Errorf("запрос: %s %s", doer.lastReq.Method, doer.lastReq.URL)
	}
}

func TestVerifyFailures(t *testing.T) {
	valid := "is_valid:true\n"
	cases := []struct {
		name   string
		mutate func(url.Values)
		body   string
		want   error
	}{
		{"отмена", func(q url.Values) { q.Set("openid.mode", "cancel") }, valid, ErrCancelled},
		{"не тот return_to", func(q url.Values) { q.Set("openid.return_to", "http://evil/cb") }, valid, ErrInvalidResponse},
		{"чужой op_endpoint", func(q url.Values) { q.Set("openid.op_endpoint", "https://evil/openid") }, valid, ErrInvalidResponse},
		{"кривой claimed_id", func(q url.Values) {
			q.Set("openid.claimed_id", "https://evil.com/openid/id/76561199174956609")
		}, valid, ErrInvalidResponse},
		{"identity ≠ claimed_id", func(q url.Values) {
			q.Set("openid.identity", "https://steamcommunity.com/openid/id/76561197960287930")
		}, valid, ErrInvalidResponse},
		{"claimed_id не подписан", func(q url.Values) {
			q.Set("openid.signed", "signed,op_endpoint,identity,return_to,response_nonce")
		}, valid, ErrInvalidResponse},
		{"Steam не подтвердил подпись", func(url.Values) {}, "is_valid:false\n", ErrInvalidResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := steamResponse()
			tc.mutate(q)
			_, err := NewOpenID(&fakeDoer{body: tc.body}).Verify(context.Background(), q, callback)
			if !errors.Is(err, tc.want) {
				t.Errorf("ожидалась %v, получено %v", tc.want, err)
			}
		})
	}
}

// --- sessions ------------------------------------------------------------

type memSessions struct {
	items    map[string]time.Time
	extended int
}

func (m *memSessions) CreateSession(_ context.Context, id string, _ int64, exp time.Time) error {
	m.items[id] = exp
	return nil
}

func (m *memSessions) SessionUser(_ context.Context, id string) (store.User, time.Time, error) {
	exp, ok := m.items[id]
	if !ok {
		return store.User{}, time.Time{}, store.ErrNotFound
	}
	return store.User{ID: 7}, exp, nil
}

func (m *memSessions) ExtendSession(_ context.Context, id string, exp time.Time) error {
	m.items[id] = exp
	m.extended++
	return nil
}

func (m *memSessions) DeleteSession(_ context.Context, id string) error {
	delete(m.items, id)
	return nil
}

func TestSessions(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	ms := &memSessions{items: map[string]time.Time{}}
	s := NewSessions(ms)
	s.now = func() time.Time { return now }

	token, exp, err := s.Create(context.Background(), 7)
	if err != nil || !exp.Equal(now.Add(SessionTTL)) {
		t.Fatalf("Create: %v, exp %v", err, exp)
	}
	if _, stored := ms.items[token]; stored {
		t.Fatal("в БД должен храниться хеш токена, а не сам токен")
	}

	u, extended, err := s.Resolve(context.Background(), token)
	if err != nil || u.ID != 7 || !extended.IsZero() {
		t.Fatalf("свежую сессию не продлеваем: %v %v %v", u, extended, err)
	}

	now = now.Add(3 * 24 * time.Hour)
	_, extended, _ = s.Resolve(context.Background(), token)
	if !extended.Equal(now.Add(SessionTTL)) || ms.extended != 1 {
		t.Errorf("через 3 дня сессия продлевается на 30 дней: %v", extended)
	}

	if err := s.Delete(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Resolve(context.Background(), token); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("после выхода сессии нет: %v", err)
	}
}
