package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// fakeAPI отвечает по сценарию: функция получает запрос и возвращает статус и тело.
type fakeAPI struct {
	mu     sync.Mutex
	calls  []chatRequest
	auth   []string
	script func(n int, req chatRequest) (int, string)
}

func (f *fakeAPI) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.calls = append(f.calls, req)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		n := len(f.calls)
		f.mu.Unlock()
		status, body := f.script(n, req)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func answer(text string) string {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": text}}}})
	return string(b)
}

func newClient(base, key, models string) *Client {
	c := New(Options{BaseURL: base, APIKey: key, Models: models, Logger: quiet, Referer: "http://site", Title: "App"})
	c.pause = 0
	return c
}

func TestConfigured(t *testing.T) {
	cases := []struct {
		name              string
		base, key, models string
		want              bool
	}{
		{"cloud with key", "https://openrouter.ai/api/v1", "k", "m", true},
		{"cloud without key", "https://openrouter.ai/api/v1", "", "m", false},
		{"local without key", "http://localhost:11434/v1", "", "qwen", true},
		{"docker ollama without key", "http://ollama:11434/v1", "", "qwen", true},
		{"no model", "http://localhost:11434/v1", "", " , ", false},
		{"no url", "", "k", "m", false},
	}
	for _, c := range cases {
		if got := New(Options{BaseURL: c.base, APIKey: c.key, Models: c.models}).Configured(); got != c.want {
			t.Errorf("%s: Configured() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCompleteOK(t *testing.T) {
	api := &fakeAPI{script: func(int, chatRequest) (int, string) { return 200, answer(`{"ok":true}`) }}
	srv := api.server(t)
	c := newClient(srv.URL+"/", "secret", "model-a")

	got, err := c.Complete(context.Background(), Request{System: "sys", User: "usr", MaxTokens: 100, JSON: true})
	if err != nil || got != `{"ok":true}` {
		t.Fatalf("got %q, %v", got, err)
	}
	req := api.calls[0]
	if req.Model != "model-a" || len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Content != "usr" {
		t.Errorf("request: %+v", req)
	}
	if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" || req.MaxTokens != 100 {
		t.Errorf("json mode / max tokens not sent: %+v", req)
	}
	if api.auth[0] != "Bearer secret" {
		t.Errorf("auth header: %q", api.auth[0])
	}
}

func TestFallsBackToNextModel(t *testing.T) {
	api := &fakeAPI{script: func(_ int, req chatRequest) (int, string) {
		if req.Model == "busy" {
			return 429, `{"error":"rate limited"}`
		}
		return 200, answer("fine")
	}}
	c := newClient(api.server(t).URL, "k", "busy, good")
	got, err := c.Complete(context.Background(), Request{User: "x"})
	if err != nil || got != "fine" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(api.calls) != 2 || api.calls[1].Model != "good" {
		t.Errorf("must try the next model after 429: %+v", api.calls)
	}
}

func TestRetriesWithoutJSONModeOn400(t *testing.T) {
	api := &fakeAPI{script: func(_ int, req chatRequest) (int, string) {
		if req.ResponseFormat != nil {
			return 400, `{"error":"response_format is not supported"}`
		}
		return 200, answer("plain")
	}}
	c := newClient(api.server(t).URL, "k", "m")
	got, err := c.Complete(context.Background(), Request{User: "x", JSON: true})
	if err != nil || got != "plain" {
		t.Fatalf("got %q, %v", got, err)
	}
	if len(api.calls) != 2 || api.calls[1].ResponseFormat != nil {
		t.Errorf("second call must go without response_format: %+v", api.calls)
	}
}

func TestEmptyAnswerAndErrorInBodyCountAsFailure(t *testing.T) {
	api := &fakeAPI{script: func(n int, _ chatRequest) (int, string) {
		switch n {
		case 1:
			return 200, `{"choices":[{"message":{"content":"   "}}]}`
		case 2:
			return 200, `{"error":{"message":"provider overloaded","code":502}}`
		}
		return 200, answer("ok")
	}}
	c := newClient(api.server(t).URL, "k", "a,b,c")
	got, err := c.Complete(context.Background(), Request{User: "x"})
	if err != nil || got != "ok" || len(api.calls) != 3 {
		t.Fatalf("got %q, %v, calls %d", got, err, len(api.calls))
	}
}

func TestAuthErrorStopsImmediately(t *testing.T) {
	api := &fakeAPI{script: func(int, chatRequest) (int, string) { return 401, `{"error":"bad key"}` }}
	c := newClient(api.server(t).URL, "wrong", "a,b")
	_, err := c.Complete(context.Background(), Request{User: "x"})
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if len(api.calls) != 1 {
		t.Errorf("a rejected key must not be retried on other models: %d calls", len(api.calls))
	}
}

func TestAllModelsFailTwoPasses(t *testing.T) {
	api := &fakeAPI{script: func(int, chatRequest) (int, string) { return 503, "down" }}
	c := newClient(api.server(t).URL, "k", "a,b")
	_, err := c.Complete(context.Background(), Request{User: "x"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("want ErrUnavailable, got %v", err)
	}
	if len(api.calls) != 4 || !strings.Contains(err.Error(), "503") {
		t.Errorf("two passes over two models expected, got %d calls, err %v", len(api.calls), err)
	}
}

func TestNotConfigured(t *testing.T) {
	c := New(Options{BaseURL: "https://openrouter.ai/api/v1", Models: "m"})
	if _, err := c.Complete(context.Background(), Request{User: "x"}); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("want ErrNotConfigured, got %v", err)
	}
}

func TestContextCancelStops(t *testing.T) {
	api := &fakeAPI{script: func(int, chatRequest) (int, string) { return 503, "down" }}
	c := newClient(api.server(t).URL, "k", "a")
	c.pause = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.Complete(ctx, Request{User: "x"})
	if err == nil || time.Since(start) > 5*time.Second {
		t.Errorf("cancel must interrupt the pause between passes: %v after %v", err, time.Since(start))
	}
}
