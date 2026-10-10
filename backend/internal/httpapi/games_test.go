package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"d2helper/internal/config"
	"d2helper/internal/games"
	"d2helper/internal/quiz"
	"d2helper/internal/store"
)

type fakeGames struct {
	ready   bool
	scores  map[games.Mode]int
	lastBad bool
}

func (f *fakeGames) Snapshot(_ context.Context, mode games.Mode) ([]games.HeroValue, error) {
	if !f.ready {
		return nil, games.ErrNotReady
	}
	return []games.HeroValue{{HeroID: 1, Value: 10}, {HeroID: 2, Value: 20}}, nil
}

func (f *fakeGames) RecordScore(_ context.Context, _ int64, mode games.Mode, streak int) (int, error) {
	if streak < 0 || streak > games.MaxStreak {
		return 0, games.ErrBadStreak
	}
	f.scores[mode] = max(f.scores[mode], streak)
	return f.scores[mode], nil
}

func (f *fakeGames) Records(context.Context, int64) (map[string]int, error) {
	return map[string]int{"popularity": 3, "kills": 9, "deaths": 0}, nil
}

type fakeQuiz struct{ err error }

func (fakeQuiz) PositionQuiz() quiz.PositionQuiz { return quiz.PositionQuiz{ID: "position"} }
func (fakeQuiz) HeroQuestions(p int) ([]quiz.HeroQuestion, error) {
	if p < 1 || p > 5 {
		return nil, quiz.ErrBadPosition
	}
	return []quiz.HeroQuestion{{ID: "q"}}, nil
}
func (f fakeQuiz) ScorePosition(map[string]string) (quiz.PositionResult, error) {
	return quiz.PositionResult{Primary: 4}, f.err
}
func (f fakeQuiz) RecommendHeroes(_ context.Context, p int, _ map[string][]string) ([]quiz.HeroPick, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []quiz.HeroPick{{HeroID: 7, Matched: []string{"control"}}}, nil
}
func (fakeQuiz) Info(int) quiz.PositionInfo {
	return quiz.PositionInfo{Description: "описание"}
}
func (fakeQuiz) TopMeta(context.Context, int, int) []quiz.MetaHero {
	return []quiz.MetaHero{{HeroID: 5}}
}

type fakeQuizResults struct {
	saved []string
	last  []byte
}

func (f *fakeQuizResults) SaveQuizResult(_ context.Context, _ int64, q string, _ any) error {
	f.saved = append(f.saved, q)
	return nil
}

func (f *fakeQuizResults) LastQuizResult(context.Context, int64, string) ([]byte, error) {
	if f.last == nil {
		return nil, store.ErrNotFound
	}
	return f.last, nil
}

func newGamesRouter(g *fakeGames, q fakeQuiz, qr *fakeQuizResults) http.Handler {
	return NewRouter(Deps{
		Config:      config.Config{SessionSecret: "secret"},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:          okPinger{},
		Sessions:    &fakeSessions{users: map[string]store.User{"alice": {ID: 7}}},
		Games:       g,
		Quiz:        q,
		QuizResults: qr,
	})
}

func TestHigherLowerSnapshot(t *testing.T) {
	g := &fakeGames{ready: true, scores: map[games.Mode]int{}}
	h := newGamesRouter(g, fakeQuiz{}, &fakeQuizResults{})

	w := send(h, http.MethodGet, "/api/games/higher-lower/kills", "", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hero_id":1`) {
		t.Errorf("снапшот: %d %s", w.Code, w.Body)
	}
	if w := send(h, http.MethodGet, "/api/games/higher-lower/assists", "", ""); w.Code != http.StatusNotFound {
		t.Errorf("неизвестный режим: %d", w.Code)
	}
	g.ready = false
	w = send(h, http.MethodGet, "/api/games/higher-lower/deaths", "", "")
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
		t.Errorf("снапшот не готов: %d", w.Code)
	}
}

func TestHigherLowerScore(t *testing.T) {
	g := &fakeGames{ready: true, scores: map[games.Mode]int{}}
	h := newGamesRouter(g, fakeQuiz{}, &fakeQuizResults{})

	if w := send(h, http.MethodPost, "/api/games/higher-lower/kills/score", `{"streak":5}`, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("гость: %d", w.Code)
	}
	w := send(h, http.MethodPost, "/api/games/higher-lower/kills/score", `{"streak":8}`, "alice")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"best":8`) {
		t.Errorf("счёт: %d %s", w.Code, w.Body)
	}
	for _, body := range []string{`{"streak":-4}`, `{"streak":9999}`, `bad`} {
		if w := send(h, http.MethodPost, "/api/games/higher-lower/kills/score", body, "alice"); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}

	// рекорды: гость видит нули, пользователь — свои
	var guest, user struct {
		HL map[string]int `json:"higher_lower"`
	}
	_ = json.Unmarshal(send(h, http.MethodGet, "/api/games/records", "", "").Body.Bytes(), &guest)
	_ = json.Unmarshal(send(h, http.MethodGet, "/api/games/records", "", "alice").Body.Bytes(), &user)
	if guest.HL["kills"] != 0 || len(guest.HL) != 3 || user.HL["kills"] != 9 {
		t.Errorf("рекорды: гость %v, пользователь %v", guest.HL, user.HL)
	}
}

func TestQuizEndpoints(t *testing.T) {
	qr := &fakeQuizResults{}
	h := newGamesRouter(&fakeGames{}, fakeQuiz{}, qr)

	if w := send(h, http.MethodGet, "/api/quiz/position", "", ""); w.Code != http.StatusOK {
		t.Errorf("квиз позиции: %d", w.Code)
	}
	if w := send(h, http.MethodGet, "/api/quiz/hero/3", "", ""); w.Code != http.StatusOK {
		t.Errorf("квиз героя: %d", w.Code)
	}
	for _, p := range []string{"0", "6", "abc"} {
		if w := send(h, http.MethodGet, "/api/quiz/hero/"+p, "", ""); w.Code != http.StatusNotFound {
			t.Errorf("позиция %s: %d", p, w.Code)
		}
	}

	// гость: результат считается, но не сохраняется
	w := send(h, http.MethodPost, "/api/quiz/position/result", `{"answers":{"q1":"a"}}`, "")
	var res positionResponse
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != http.StatusOK || res.Primary != 4 || res.Saved || res.Description != "описание" || len(res.Heroes) != 1 {
		t.Errorf("результат гостя: %d %s", w.Code, w.Body)
	}
	// пользователь: сохраняется
	w = send(h, http.MethodPost, "/api/quiz/position/result", `{"answers":{"q1":"a"}}`, "alice")
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if !res.Saved || len(qr.saved) != 1 || qr.saved[0] != "position" {
		t.Errorf("сохранение: %+v %v", res, qr.saved)
	}

	w = send(h, http.MethodPost, "/api/quiz/hero/result", `{"position":4,"answers":{"a":["x"]}}`, "alice")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"hero_id":7`) || len(qr.saved) != 2 {
		t.Errorf("подбор героя: %d %s", w.Code, w.Body)
	}
}

func TestQuizErrors(t *testing.T) {
	h := newGamesRouter(&fakeGames{}, fakeQuiz{err: quiz.ErrBadAnswers}, &fakeQuizResults{})
	if w := send(h, http.MethodPost, "/api/quiz/position/result", `{"answers":{}}`, ""); w.Code != http.StatusBadRequest {
		t.Errorf("позиция: %d", w.Code)
	}
	if w := send(h, http.MethodPost, "/api/quiz/hero/result", `{"position":4,"answers":{}}`, ""); w.Code != http.StatusBadRequest {
		t.Errorf("герой: %d", w.Code)
	}
	h = newGamesRouter(&fakeGames{}, fakeQuiz{err: errors.New("boom")}, &fakeQuizResults{})
	if w := send(h, http.MethodPost, "/api/quiz/hero/result", `{"position":4,"answers":{}}`, ""); w.Code != http.StatusInternalServerError {
		t.Errorf("внутренняя ошибка: %d", w.Code)
	}
}

func TestQuizPositionLast(t *testing.T) {
	qr := &fakeQuizResults{}
	h := newGamesRouter(&fakeGames{}, fakeQuiz{}, qr)

	for token, want := range map[string]string{"": `{"position":null}`, "alice": `{"position":null}`} {
		if w := send(h, http.MethodGet, "/api/quiz/position/last", "", token); strings.TrimSpace(w.Body.String()) != want {
			t.Errorf("%q: %s", token, w.Body)
		}
	}
	qr.last = []byte(`{"primary":3,"secondary":null,"shares":[0,0,0,0,0]}`)
	if w := send(h, http.MethodGet, "/api/quiz/position/last", "", "alice"); !strings.Contains(w.Body.String(), `"position":3`) {
		t.Errorf("после квиза: %s", w.Body)
	}
	if w := send(h, http.MethodGet, "/api/quiz/position/last", "", ""); !strings.Contains(w.Body.String(), `null`) {
		t.Errorf("гость всегда null: %s", w.Body)
	}
}
