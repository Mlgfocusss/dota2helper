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

	"d2helper/internal/auth"
	"d2helper/internal/config"
	"d2helper/internal/patchnotes"
	"d2helper/internal/store"
)

type fakeComments struct {
	mu    sync.Mutex
	items []store.Comment
	next  int64
}

func (f *fakeComments) ListComments(_ context.Context, gid string, before int64, limit int) ([]store.Comment, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var all, page []store.Comment
	for i := len(f.items) - 1; i >= 0; i-- { // от новых к старым
		c := f.items[i]
		if c.GID != gid {
			continue
		}
		all = append(all, c)
		if (before == 0 || c.ID < before) && len(page) < limit {
			page = append(page, c)
		}
	}
	return page, len(all), nil
}

func (f *fakeComments) CreateComment(_ context.Context, gid string, userID int64, text string) (store.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if gid != "1" {
		return store.Comment{}, store.ErrNotFound
	}
	f.next++
	c := store.Comment{ID: f.next, GID: gid, UserID: userID, AuthorName: "Mlgfocus", Text: text, CreatedAt: time.Now()}
	f.items = append(f.items, c)
	return c, nil
}

func (f *fakeComments) DeleteComment(_ context.Context, gid string, id, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, c := range f.items {
		if c.ID == id && c.GID == gid && c.UserID == userID {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

type fakePatches struct{}

func (fakePatches) Get(_ context.Context, version string) (*patchnotes.Patch, error) {
	if version != "7.41f" {
		return nil, patchnotes.ErrNotFound
	}
	return &patchnotes.Patch{Number: "7.41f", General: []patchnotes.Section{}, Heroes: []patchnotes.Hero{{HeroID: 1}}}, nil
}

type commentsEnv struct {
	handler  http.Handler
	comments *fakeComments
}

func newCommentsEnv() *commentsEnv {
	sessions := &fakeSessions{users: map[string]store.User{
		"alice": {ID: 1, PersonaName: "Alice"},
		"bob":   {ID: 2, PersonaName: "Bob"},
	}}
	comments := &fakeComments{}
	h := NewRouter(Deps{
		Config:   config.Config{SessionSecret: "secret"},
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:       okPinger{},
		Sessions: sessions,
		News: fakeNews{items: []store.News{
			{GID: "1", Title: "7.41f Gameplay Patch", Date: time.Now(), HTML: "<p>hi</p>"},
			{GID: "2", Title: "The International", Date: time.Now(), HTML: "<p>ti</p>"},
		}},
		Comments: comments,
		Patches:  fakePatches{},
	})
	return &commentsEnv{handler: h, comments: comments}
}

// as отправляет запрос от имени пользователя (token) или гостя (пустой token).
func (e *commentsEnv) as(token, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	}
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, req)
	return w
}

func TestCommentsRequireLogin(t *testing.T) {
	e := newCommentsEnv()
	if w := e.as("", http.MethodPost, "/api/news/1/comments", `{"text":"привет"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("гость не может писать: %d", w.Code)
	}
	if w := e.as("", http.MethodGet, "/api/news/1/comments", ""); w.Code != http.StatusOK {
		t.Errorf("гость может читать: %d", w.Code)
	}
}

func TestCommentsCreateListDelete(t *testing.T) {
	e := newCommentsEnv()

	w := e.as("alice", http.MethodPost, "/api/news/1/comments", `{"text":"  Нерф Axe — это больно  "}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("создание: %d %s", w.Code, w.Body)
	}
	var created commentJSON
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if created.Text != "Нерф Axe — это больно" || !created.Mine {
		t.Errorf("созданный комментарий: %+v", created)
	}

	// чужой комментарий: виден, но mine=false и удалить нельзя
	var list commentsResponse
	w = e.as("bob", http.MethodGet, "/api/news/1/comments", "")
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].Mine || list.Total != 1 {
		t.Errorf("список для другого пользователя: %s", w.Body)
	}
	if w := e.as("bob", http.MethodDelete, "/api/news/1/comments/1", ""); w.Code != http.StatusNotFound {
		t.Errorf("чужой комментарий удалить нельзя: %d", w.Code)
	}
	if w := e.as("alice", http.MethodDelete, "/api/news/1/comments/1", ""); w.Code != http.StatusNoContent {
		t.Errorf("свой комментарий удаляется: %d", w.Code)
	}
	if len(e.comments.items) != 0 {
		t.Error("комментарий не удалён")
	}
}

func TestCommentsValidation(t *testing.T) {
	e := newCommentsEnv()
	cases := map[string]int{
		`{"text":"   "}`: http.StatusBadRequest,
		`{"text":"` + strings.Repeat("я", 1001) + `"}`: http.StatusBadRequest,
		`not json`: http.StatusBadRequest,
	}
	for body, want := range cases {
		if w := e.as("alice", http.MethodPost, "/api/news/1/comments", body); w.Code != want {
			t.Errorf("%.20s…: %d, ожидался %d", body, w.Code, want)
		}
	}
	if w := e.as("alice", http.MethodPost, "/api/news/404/comments", `{"text":"привет"}`); w.Code != http.StatusNotFound {
		t.Errorf("новости нет: %d", w.Code)
	}
}

func TestCommentsPagination(t *testing.T) {
	e := newCommentsEnv()
	for range commentsPageSize + 5 {
		e.comments.CreateComment(context.Background(), "1", 1, "текст")
	}
	var first commentsResponse
	w := e.as("", http.MethodGet, "/api/news/1/comments", "")
	_ = json.Unmarshal(w.Body.Bytes(), &first)
	if len(first.Items) != commentsPageSize || first.NextBefore == nil || first.Total != commentsPageSize+5 {
		t.Fatalf("первая страница: %d шт., next=%v, total=%d", len(first.Items), first.NextBefore, first.Total)
	}
	var second commentsResponse
	w = e.as("", http.MethodGet, "/api/news/1/comments?before="+itoa(*first.NextBefore), "")
	_ = json.Unmarshal(w.Body.Bytes(), &second)
	if len(second.Items) != 5 || second.NextBefore != nil {
		t.Errorf("вторая страница: %d шт., next=%v", len(second.Items), second.NextBefore)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestPatchEndpointAndNewsLink(t *testing.T) {
	e := newCommentsEnv()
	if w := e.as("", http.MethodGet, "/api/patches/7.41f", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hero_id":1`) {
		t.Errorf("патч: %d %s", w.Code, w.Body)
	}
	if w := e.as("", http.MethodGet, "/api/patches/9.99", ""); w.Code != http.StatusNotFound {
		t.Errorf("неизвестный патч: %d", w.Code)
	}

	var patchNews, plainNews newsItem
	_ = json.Unmarshal(e.as("", http.MethodGet, "/api/news/1", "").Body.Bytes(), &patchNews)
	_ = json.Unmarshal(e.as("", http.MethodGet, "/api/news/2", "").Body.Bytes(), &plainNews)
	if patchNews.Patch != "7.41f" || plainNews.Patch != "" {
		t.Errorf("связь новости с патчем: %q / %q", patchNews.Patch, plainNews.Patch)
	}
}
