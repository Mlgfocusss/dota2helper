package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"d2helper/internal/analysis"
	"d2helper/internal/coach"
	"d2helper/internal/config"
	"d2helper/internal/store"
)

type fakeAnalysis struct {
	available bool
	createErr error
	getErr    error
	view      *analysis.View
	quota     analysis.Quota
	gotMatch  int64
	gotUser   int64
	example   bool
}

func (f *fakeAnalysis) Available() bool { return f.available }
func (f *fakeAnalysis) PerDay() int     { return 3 }

func (f *fakeAnalysis) Quota(context.Context, int64) (analysis.Quota, error) { return f.quota, nil }

func (f *fakeAnalysis) Create(_ context.Context, u store.User, matchID int64) (*analysis.View, error) {
	f.gotMatch, f.gotUser = matchID, u.ID
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.view, nil
}

func (f *fakeAnalysis) Get(_ context.Context, userID int64, id string) (*analysis.View, error) {
	f.gotUser = userID
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.view, nil
}

func (f *fakeAnalysis) ExampleView() (*analysis.View, bool) {
	return &analysis.View{ID: "example", Status: "done", Result: json.RawMessage(`{"summary":{}}`)}, f.example
}

type fakeRefs struct {
	refs  map[int64]store.AnalysisRef
	list  []store.AnalysisListItem
	calls int
}

func (f *fakeRefs) ListAnalyses(context.Context, int64, int) ([]store.AnalysisListItem, error) {
	return f.list, nil
}

func (f *fakeRefs) AnalysesForMatches(_ context.Context, _ int64, ids []int64) (map[int64]store.AnalysisRef, error) {
	f.calls++
	return f.refs, nil
}

func newAnalysisRouter(a *fakeAnalysis, c *fakeCoach, refs *fakeRefs) http.Handler {
	return NewRouter(Deps{
		Config:       config.Config{SessionSecret: "secret"},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:           okPinger{},
		Sessions:     &fakeSessions{users: map[string]store.User{"alice": {ID: 7, PersonaName: "Alice"}}},
		Analysis:     a,
		AnalysisRefs: refs,
		Coach:        c,
	})
}

func TestAnalysisCreateStatuses(t *testing.T) {
	view := &analysis.View{ID: "abc", MatchID: 99, Status: "queued"}
	cases := []struct {
		name string
		err  error
		want int
		code string
	}{
		{"new", nil, http.StatusCreated, ""},
		{"exists", &analysis.ExistsError{ID: "abc"}, http.StatusOK, ""},
		{"unavailable", analysis.ErrUnavailable, http.StatusServiceUnavailable, "analysis_unavailable"},
		{"foreign match", analysis.ErrNotYourMatch, http.StatusForbidden, "match_not_yours"},
		{"quota", &analysis.QuotaError{ResetAt: time.Now().Add(5 * time.Hour)}, http.StatusTooManyRequests, "analysis_quota"},
		{"internal", errors.New("boom"), http.StatusInternalServerError, "internal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeAnalysis{createErr: c.err, view: view, available: true}
			h := newAnalysisRouter(f, &fakeCoach{}, &fakeRefs{})
			rec := send(h, http.MethodPost, "/api/analyses", `{"match_id": 99}`, "alice")
			if rec.Code != c.want {
				t.Fatalf("status %d, want %d: %s", rec.Code, c.want, rec.Body)
			}
			if f.gotMatch != 99 || f.gotUser != 7 {
				t.Errorf("service got match %d for user %d", f.gotMatch, f.gotUser)
			}
			if c.code != "" {
				var body struct{ Error struct{ Code string } }
				_ = json.Unmarshal(rec.Body.Bytes(), &body)
				if body.Error.Code != c.code {
					t.Errorf("error code %q, want %q", body.Error.Code, c.code)
				}
			}
			if c.want == http.StatusTooManyRequests && rec.Header().Get("Retry-After") == "" {
				t.Error("429 must carry Retry-After")
			}
			if c.want == http.StatusCreated || c.want == http.StatusOK {
				var v analysis.View
				if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || v.ID != "abc" {
					t.Errorf("body: %v %s", err, rec.Body)
				}
			}
		})
	}
}

func TestAnalysisCreateValidatesAndRequiresLogin(t *testing.T) {
	h := newAnalysisRouter(&fakeAnalysis{available: true, view: &analysis.View{}}, &fakeCoach{}, &fakeRefs{})
	if rec := send(h, http.MethodPost, "/api/analyses", `{"match_id": 99}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest: %d", rec.Code)
	}
	for _, body := range []string{`{}`, `{"match_id": 0}`, `{"match_id": -5}`, `not json`} {
		if rec := send(h, http.MethodPost, "/api/analyses", body, "alice"); rec.Code != http.StatusBadRequest {
			t.Errorf("%q: %d", body, rec.Code)
		}
	}
}

func TestAnalysisGetAndQuotaAndExample(t *testing.T) {
	f := &fakeAnalysis{available: true, view: &analysis.View{ID: "abc", Status: "done"}, example: true,
		quota: analysis.Quota{PerDay: 3, Left: 2}}
	h := newAnalysisRouter(f, &fakeCoach{}, &fakeRefs{})

	if rec := send(h, http.MethodGet, "/api/analyses/abc", "", "alice"); rec.Code != http.StatusOK || f.gotUser != 7 {
		t.Errorf("get: %d user %d", rec.Code, f.gotUser)
	}
	if rec := send(h, http.MethodGet, "/api/analyses/abc", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest get: %d", rec.Code)
	}
	f.getErr = analysis.ErrNotFound
	if rec := send(h, http.MethodGet, "/api/analyses/abc", "", "alice"); rec.Code != http.StatusNotFound {
		t.Errorf("someone else's analysis must look like missing: %d", rec.Code)
	}

	rec := send(h, http.MethodGet, "/api/analyses/quota", "", "alice")
	var q analysis.Quota
	if err := json.Unmarshal(rec.Body.Bytes(), &q); err != nil || rec.Code != http.StatusOK || q.Left != 2 || q.PerDay != 3 {
		t.Errorf("quota: %d %v %+v", rec.Code, err, q)
	}
	if rec := send(h, http.MethodGet, "/api/analyses/quota", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest quota: %d", rec.Code)
	}

	// пример доступен гостям и не путается с {id}
	rec = send(h, http.MethodGet, "/api/analyses/example", "", "")
	var v analysis.View
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || rec.Code != http.StatusOK || v.ID != "example" {
		t.Errorf("example: %d %v %s", rec.Code, err, rec.Body)
	}
	f.example = false
	if rec := send(h, http.MethodGet, "/api/analyses/example", "", ""); rec.Code != http.StatusNotFound {
		t.Errorf("no example: %d", rec.Code)
	}
}

func TestPublicConfigReportsAnalysisAvailability(t *testing.T) {
	for _, available := range []bool{true, false} {
		h := newAnalysisRouter(&fakeAnalysis{available: available}, &fakeCoach{}, &fakeRefs{})
		rec := send(h, http.MethodGet, "/api/config", "", "")
		var cfg struct {
			Features struct {
				DeepAnalysis   bool `json:"deep_analysis"`
				AnalysesPerDay int  `json:"analyses_per_day"`
			}
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
			t.Fatal(err)
		}
		if cfg.Features.DeepAnalysis != available || cfg.Features.AnalysesPerDay != 3 {
			t.Errorf("available=%v: %+v", available, cfg.Features)
		}
	}
}

// список матчей «Тренера» общий для всех пользователей: статусы разборов вешаются на копию
type sharedCoach struct {
	fakeCoach
	page    *coach.MatchesPage
	summary *coach.Summary
}

func (c *sharedCoach) Matches(context.Context, int64, string, string, int) (*coach.MatchesPage, error) {
	return c.page, nil
}

func (c *sharedCoach) Summary(context.Context, int64) (*coach.Summary, error) { return c.summary, nil }

func TestMatchListsGetAnalysisStatusWithoutMutatingSharedData(t *testing.T) {
	shared := &sharedCoach{
		page: &coach.MatchesPage{Items: []coach.MatchRow{{MatchID: 1}, {MatchID: 2}}},
		summary: &coach.Summary{
			HasHistory: true, HeroesInForm: []coach.FormPeriod{}, LastMatch: &coach.MatchRow{MatchID: 1},
		},
	}
	refs := &fakeRefs{refs: map[int64]store.AnalysisRef{1: {ID: "x", Status: "done"}}}
	h := newAnalysisRouter(&fakeAnalysis{}, &fakeCoach{}, refs)
	// подменяем сервис «Тренера» на общий
	h = NewRouter(Deps{
		Config:       config.Config{SessionSecret: "secret"},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:           okPinger{},
		Sessions:     &fakeSessions{users: map[string]store.User{"alice": {ID: 7}}},
		Analysis:     &fakeAnalysis{},
		AnalysisRefs: refs,
		Coach:        shared,
	})

	var page struct {
		Items []struct {
			MatchID  int64 `json:"match_id"`
			Analysis *struct {
				ID, Status string
			} `json:"analysis"`
		}
	}
	rec := send(h, http.MethodGet, "/api/coach/123/matches", "", "alice")
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("matches: %d %v", rec.Code, err)
	}
	if page.Items[0].Analysis == nil || page.Items[0].Analysis.Status != "done" || page.Items[1].Analysis != nil {
		t.Errorf("statuses must be attached per match: %+v", page.Items)
	}
	if shared.page.Items[0].Analysis != nil {
		t.Fatal("shared (cached) page was mutated — statuses would leak to other users")
	}

	var sum struct {
		LastMatch *struct {
			Analysis *struct{ Status string } `json:"analysis"`
		} `json:"last_match"`
	}
	rec = send(h, http.MethodGet, "/api/coach/123/summary", "", "alice")
	if err := json.Unmarshal(rec.Body.Bytes(), &sum); err != nil || sum.LastMatch == nil || sum.LastMatch.Analysis == nil || sum.LastMatch.Analysis.Status != "done" {
		t.Errorf("last match status: %v %s", err, rec.Body)
	}
	if shared.summary.LastMatch.Analysis != nil {
		t.Fatal("shared summary was mutated")
	}

	// гость: статусов нет и в базу за ними не ходим
	before := refs.calls
	rec = send(h, http.MethodGet, "/api/coach/123/matches", "", "")
	var guestPage struct {
		Items []struct {
			Analysis *struct{ Status string } `json:"analysis"`
		}
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &guestPage)
	if guestPage.Items[0].Analysis != nil || refs.calls != before {
		t.Errorf("guest must not get statuses (calls %d→%d): %+v", before, refs.calls, guestPage.Items)
	}
}

func TestAnalysisListExtractsPlan(t *testing.T) {
	hero := 14
	refs := &fakeRefs{list: []store.AnalysisListItem{
		{ID: "a1", MatchID: 1, Status: "done", HeroID: &hero,
			Result: []byte(`{"summary":{"rating":6},"improvements":[{"title":"T","why":"W","drill":"D"}],"next_games_plan":["p1","p2"],"episodes":[{"time":"1:00"}]}`)},
		{ID: "a2", MatchID: 2, Status: "failed", Error: "boom"},
	}}
	h := newAnalysisRouter(&fakeAnalysis{available: true}, &fakeCoach{}, refs)

	if rec := send(h, http.MethodGet, "/api/analyses", "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("guest: %d", rec.Code)
	}
	rec := send(h, http.MethodGet, "/api/analyses", "", "alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var body struct{ Items []analysisListItem }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || len(body.Items) != 2 {
		t.Fatalf("body: %v %s", err, rec.Body)
	}
	done, failed := body.Items[0], body.Items[1]
	if done.Rating == nil || *done.Rating != 6 || len(done.Improvements) != 1 || len(done.NextGamesPlan) != 2 {
		t.Errorf("done item: %+v", done)
	}
	if failed.Rating != nil || failed.Error != "boom" {
		t.Errorf("failed item: %+v", failed)
	}
}
