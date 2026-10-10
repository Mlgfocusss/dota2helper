package stratz

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestQueryDecodesData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("User-Agent") != "STRATZ_API" {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"constants":{"gameVersions":[{"id":180}]}}}`))
	}))
	defer srv.Close()

	c := New("tok", 6000, quiet).WithEndpoint(srv.URL)
	var out struct {
		Constants struct {
			GameVersions []struct{ ID int }
		}
	}
	if err := c.Query(context.Background(), `{ constants { gameVersions { id } } }`, nil, &out); err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(out.Constants.GameVersions) != 1 || out.Constants.GameVersions[0].ID != 180 {
		t.Errorf("out = %+v", out)
	}
}

func TestQueryReturnsGraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errors":[{"message":"Cannot query field \"x\""}]}`))
	}))
	defer srv.Close()

	err := New("tok", 6000, quiet).WithEndpoint(srv.URL).Query(context.Background(), `{ x }`, nil, nil)
	var gqlErr *GraphQLError
	if !errors.As(err, &gqlErr) || !strings.Contains(err.Error(), "Cannot query field") {
		t.Fatalf("ожидалась GraphQLError, получено %v", err)
	}
}

func TestForbiddenDisablesClient(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "You must have 100 public games played on your account to use our API.", http.StatusForbidden)
	}))
	defer srv.Close()

	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	c := New("tok", 6000, quiet).WithEndpoint(srv.URL)
	c.now = func() time.Time { return now }

	err := c.Check(context.Background())
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("ожидалась ErrUnavailable, получено %v", err)
	}
	if c.Available() || !strings.Contains(c.Status(), "100 public games") {
		t.Fatalf("клиент должен отключиться с причиной от STRATZ, статус %q", c.Status())
	}

	// повторные запросы не уходят в сеть, пока не истёк срок
	_ = c.Check(context.Background())
	if calls.Load() != 1 {
		t.Errorf("запросов %d, ожидался 1", calls.Load())
	}

	now = now.Add(disableFor + time.Minute)
	if !c.Available() {
		t.Error("после паузы клиент снова пробует STRATZ")
	}
}

func TestNoToken(t *testing.T) {
	c := New("", 60, quiet)
	if c.Available() || !errors.Is(c.Check(context.Background()), ErrNoToken) {
		t.Error("без токена клиент недоступен")
	}
}
