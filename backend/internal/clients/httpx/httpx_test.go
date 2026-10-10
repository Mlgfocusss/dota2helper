package httpx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(retries int) (*Client, *[]time.Duration) {
	c := New(Options{
		Name:       "test",
		PerMinute:  6000,
		MaxRetries: retries,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	var sleeps []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error {
		sleeps = append(sleeps, d)
		return nil
	}
	return c, &sleeps
}

func TestRetriesOnServerErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusServiceUnavailable)
		case 2:
			w.Header().Set("Retry-After", "7")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	c, sleeps := newTestClient(3)
	var out struct{ OK bool }
	if err := c.GetJSON(context.Background(), srv.URL, nil, &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if !out.OK || calls.Load() != 3 {
		t.Fatalf("ok=%v calls=%d, ожидалось true и 3", out.OK, calls.Load())
	}
	if len(*sleeps) != 2 || (*sleeps)[1] != 7*time.Second {
		t.Errorf("паузы %v: вторая должна взять значение из Retry-After (7s)", *sleeps)
	}
}

func TestNoRetryOnClientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	c, _ := newTestClient(3)
	err := c.GetJSON(context.Background(), srv.URL, nil, nil)
	if !IsStatus(err, http.StatusForbidden) {
		t.Fatalf("ожидалась ошибка 403, получено %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("4xx не повторяется, а запросов было %d", calls.Load())
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("в ошибке должно быть тело ответа: %v", err)
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c, _ := newTestClient(2)
	if err := c.GetJSON(context.Background(), srv.URL, nil, nil); !IsStatus(err, http.StatusBadGateway) {
		t.Fatalf("ожидалась ошибка 502, получено %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("запросов %d, ожидалось 3 (1 + 2 повтора)", calls.Load())
	}
}

func TestPostJSONSendsBodyAndHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Header.Get("Authorization") != "Bearer t" || r.Header.Get("User-Agent") != "UA" {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"echo":` + string(body) + `}`))
	}))
	defer srv.Close()

	c := New(Options{Name: "test", PerMinute: 6000, UserAgent: "UA"})
	var out struct {
		Echo struct{ Q string }
	}
	h := http.Header{"Authorization": {"Bearer t"}}
	if err := c.PostJSON(context.Background(), srv.URL, h, map[string]string{"q": "hi"}, &out); err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if out.Echo.Q != "hi" {
		t.Errorf("echo %+v", out)
	}
}

func TestRedact(t *testing.T) {
	u, _ := url.Parse("https://api.example.com/x?key=SECRET&steamids=1&api_key=S2")
	got := Redact(u)
	if strings.Contains(got, "SECRET") || strings.Contains(got, "S2") || !strings.Contains(got, "steamids=1") {
		t.Errorf("Redact = %q", got)
	}

	u, _ = url.Parse("https://api.telegram.org/bot123:ABC-def/sendMessage")
	if got := Redact(u); got != "https://api.telegram.org/botREDACTED/sendMessage" {
		t.Errorf("Redact(telegram) = %q", got)
	}
}

func TestNetworkErrorHidesKey(t *testing.T) {
	c := New(Options{Name: "test", MaxRetries: 0})
	// порт 1 закрыт — сетевая ошибка сразу
	err := c.GetJSON(context.Background(), "http://127.0.0.1:1/x?key=SECRET", nil, nil)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("err = %v", err)
	}
}
