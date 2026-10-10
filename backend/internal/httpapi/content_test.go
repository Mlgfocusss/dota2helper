package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"d2helper/internal/config"
	"d2helper/internal/dota"
	"d2helper/internal/meta"
	"d2helper/internal/news"
	"d2helper/internal/store"
)

type fakeMeta struct {
	snap *meta.Snapshot
	err  error
}

func (f fakeMeta) Snapshot(context.Context) (*meta.Snapshot, error) { return f.snap, f.err }

type fakeNews struct{ items []store.News }

func (f fakeNews) List(_ context.Context, limit int) ([]store.News, error) {
	return f.items[:min(limit, len(f.items))], nil
}

func (f fakeNews) Get(_ context.Context, gid string) (store.News, error) {
	for _, n := range f.items {
		if n.GID == gid {
			return n, nil
		}
	}
	return store.News{}, news.ErrNotFound
}

type fakeFeedback struct {
	mu   sync.Mutex
	text []string
}

func (f *fakeFeedback) CreateFeedback(_ context.Context, _ *int64, text, _ string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.text = append(f.text, text)
	return int64(len(f.text)), nil
}

type fakeNotifier struct{ sent chan string }

func (f fakeNotifier) Enabled() bool { return true }

func (f fakeNotifier) Send(_ context.Context, text string) error {
	f.sent <- text
	return nil
}

func newContentRouter(m MetaSource, fb *fakeFeedback, notifier Notifier) http.Handler {
	return NewRouter(Deps{
		Config:   config.Config{SessionSecret: "secret"},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:       okPinger{},
		Sessions: &fakeSessions{users: map[string]store.User{}},
		Meta:     m,
		News: fakeNews{items: []store.News{
			{GID: "1", Title: "7.41f Gameplay Patch", Date: time.Now(), HTML: "<p>hi</p>", Excerpt: "hi"},
		}},
		Feedback: fb,
		Notifier: notifier,
	})
}

func doJSON(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestMetaPositions(t *testing.T) {
	snap := &meta.Snapshot{Patch: dota.Patch{Number: "7.41", Label: "7.41F"}, Source: meta.SourceOpenDota}
	for i := range 7 {
		snap.ByPosition[0] = append(snap.ByPosition[0], meta.HeroStat{HeroID: i + 1, Winrate: 0.5, Matches: 300})
	}
	h := newContentRouter(fakeMeta{snap: snap}, &fakeFeedback{}, nil)

	w := doJSON(h, http.MethodGet, "/api/meta/positions?limit=3", "")
	var resp metaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if len(resp.Positions) != 5 || len(resp.Positions[0].Heroes) != 3 || resp.ByPosition {
		t.Errorf("ответ: %+v", resp)
	}
	if resp.Positions[4].Heroes == nil {
		t.Error("пустая позиция — пустой массив, а не null")
	}

	notReady := newContentRouter(fakeMeta{err: meta.ErrNotReady}, &fakeFeedback{}, nil)
	if w := doJSON(notReady, http.MethodGet, "/api/meta/positions", ""); w.Code != http.StatusServiceUnavailable {
		t.Errorf("до сбора меты ожидался 503, получен %d", w.Code)
	}
}

func TestNewsEndpoints(t *testing.T) {
	h := newContentRouter(fakeMeta{}, &fakeFeedback{}, nil)
	if w := doJSON(h, http.MethodGet, "/api/news?limit=5", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"excerpt":"hi"`) {
		t.Errorf("список: %d %s", w.Code, w.Body)
	}
	var item newsItem
	w := doJSON(h, http.MethodGet, "/api/news/1", "")
	if err := json.Unmarshal(w.Body.Bytes(), &item); err != nil || item.HTML != "<p>hi</p>" {
		t.Errorf("новость: %s", w.Body)
	}
	if w := doJSON(h, http.MethodGet, "/api/news/404", ""); w.Code != http.StatusNotFound {
		t.Errorf("нет такой новости: %d", w.Code)
	}
}

func TestFeedback(t *testing.T) {
	fb := &fakeFeedback{}
	notifier := fakeNotifier{sent: make(chan string, 1)}
	h := newContentRouter(fakeMeta{}, fb, notifier)

	cases := map[string]int{
		`{"text":" "}`: http.StatusBadRequest,
		`{"text":"` + strings.Repeat("я", 2001) + `"}`: http.StatusBadRequest,
		`not json`: http.StatusBadRequest,
	}
	for body, want := range cases {
		if w := doJSON(h, http.MethodPost, "/api/feedback", body); w.Code != want {
			t.Errorf("%.30s…: статус %d, ожидался %d", body, w.Code, want)
		}
	}

	w := doJSON(h, http.MethodPost, "/api/feedback", `{"text":"  Классный сайт  ","contact":"@me"}`)
	if w.Code != http.StatusCreated || len(fb.text) != 1 || fb.text[0] != "Классный сайт" {
		t.Fatalf("отзыв: %d %v", w.Code, fb.text)
	}
	select {
	case msg := <-notifier.sent:
		if !strings.Contains(msg, "Классный сайт") || !strings.Contains(msg, "@me") {
			t.Errorf("сообщение в Telegram: %q", msg)
		}
	case <-time.After(time.Second):
		t.Error("отзыв не отправлен в Telegram")
	}
}
