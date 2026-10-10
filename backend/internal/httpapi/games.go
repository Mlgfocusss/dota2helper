package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"d2helper/internal/auth"
	"d2helper/internal/games"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/quiz"
	"d2helper/internal/store"
)

// GamesService — снапшот и рекорды «Больше / Меньше».
type GamesService interface {
	Snapshot(ctx context.Context, mode games.Mode) ([]games.HeroValue, error)
	RecordScore(ctx context.Context, userID int64, mode games.Mode, streak int) (int, error)
	Records(ctx context.Context, userID int64) (map[string]int, error)
}

// QuizService — квизы позиции и героя.
type QuizService interface {
	PositionQuiz() quiz.PositionQuiz
	HeroQuestions(position int) ([]quiz.HeroQuestion, error)
	ScorePosition(answers map[string]string) (quiz.PositionResult, error)
	RecommendHeroes(ctx context.Context, position int, answers map[string][]string) ([]quiz.HeroPick, error)
	Info(position int) quiz.PositionInfo
	TopMeta(ctx context.Context, position, n int) []quiz.MetaHero
}

// QuizResults — сохранённые результаты квизов вошедших пользователей.
type QuizResults interface {
	SaveQuizResult(ctx context.Context, userID int64, quiz string, result any) error
	LastQuizResult(ctx context.Context, userID int64, quiz string) ([]byte, error)
}

func modeParam(w http.ResponseWriter, r *http.Request) (games.Mode, bool) {
	mode, ok := games.ParseMode(chi.URLParam(r, "mode"))
	if !ok {
		render.Error(w, http.StatusNotFound, "unknown_mode", "Такого режима нет")
	}
	return mode, ok
}

// higherLowerSnapshot — GET /api/games/higher-lower/{mode}: [{hero_id, value}].
func (a *api) higherLowerSnapshot(w http.ResponseWriter, r *http.Request) {
	mode, ok := modeParam(w, r)
	if !ok {
		return
	}
	rows, err := a.games.Snapshot(r.Context(), mode)
	if errors.Is(err, games.ErrNotReady) {
		w.Header().Set("Retry-After", "60")
		render.Error(w, http.StatusServiceUnavailable, "snapshot_not_ready",
			"Этот режим пока недоступен: данные ещё собираются. Загляни позже")
		return
	}
	if err != nil {
		a.logger.Error("higher-lower snapshot", "err", err)
		render.Internal(w)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=600")
	render.JSON(w, http.StatusOK, rows)
}

type scoreRequest struct {
	Streak int `json:"streak"`
}

// higherLowerScore — POST /api/games/higher-lower/{mode}/score { streak } → { best }.
func (a *api) higherLowerScore(w http.ResponseWriter, r *http.Request) {
	mode, ok := modeParam(w, r)
	if !ok {
		return
	}
	var req scoreRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	best, err := a.games.RecordScore(r.Context(), auth.UserFrom(r.Context()).ID, mode, req.Streak)
	if errors.Is(err, games.ErrBadStreak) {
		render.Error(w, http.StatusBadRequest, "bad_streak", "Некорректный результат")
		return
	}
	if err != nil {
		a.logger.Error("higher-lower score", "err", err)
		render.Internal(w)
		return
	}
	render.JSON(w, http.StatusOK, map[string]int{"best": best})
}

// gameRecords — GET /api/games/records: рекорды «Больше / Меньше» (у гостя — нули).
func (a *api) gameRecords(w http.ResponseWriter, r *http.Request) {
	records := map[string]int{}
	for _, m := range games.Modes {
		records[string(m)] = 0
	}
	if u := auth.UserFrom(r.Context()); u != nil {
		got, err := a.games.Records(r.Context(), u.ID)
		if err != nil {
			a.logger.Error("game records", "err", err)
			render.Internal(w)
			return
		}
		records = got
	}
	render.JSON(w, http.StatusOK, map[string]any{"higher_lower": records})
}

// ----- квизы -----------------------------------------------------------------

func (a *api) quizPosition(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=3600")
	render.JSON(w, http.StatusOK, a.quiz.PositionQuiz())
}

func positionParam(w http.ResponseWriter, raw int) bool {
	if raw < 1 || raw > 5 {
		render.Error(w, http.StatusNotFound, "bad_position", "Такой позиции нет")
		return false
	}
	return true
}

// quizHero — GET /api/quiz/hero/{position}: 4 вопроса для позиции.
func (a *api) quizHero(w http.ResponseWriter, r *http.Request) {
	pos := intParam(r, "position")
	if !positionParam(w, pos) {
		return
	}
	qs, err := a.quiz.HeroQuestions(pos)
	if err != nil {
		render.Error(w, http.StatusNotFound, "bad_position", "Такой позиции нет")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	render.JSON(w, http.StatusOK, map[string]any{"id": "hero", "position": pos, "questions": qs})
}

type positionAnswers struct {
	Answers map[string]string `json:"answers"`
}

type positionResponse struct {
	quiz.PositionResult
	Description string          `json:"description"`
	Heroes      []quiz.MetaHero `json:"heroes"`
	Saved       bool            `json:"saved"`
}

// quizPositionResult — POST /api/quiz/position/result { answers: {q1: "a", …} }.
func (a *api) quizPositionResult(w http.ResponseWriter, r *http.Request) {
	var req positionAnswers
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := a.quiz.ScorePosition(req.Answers)
	if errors.Is(err, quiz.ErrBadAnswers) {
		render.Error(w, http.StatusBadRequest, "bad_answers", "Ответь на все вопросы")
		return
	}
	if err != nil {
		a.logger.Error("score position", "err", err)
		render.Internal(w)
		return
	}
	resp := positionResponse{
		PositionResult: res,
		Description:    a.quiz.Info(res.Primary).Description,
		Heroes:         a.quiz.TopMeta(r.Context(), res.Primary, 4),
	}
	if u := auth.UserFrom(r.Context()); u != nil {
		if err := a.quizResults.SaveQuizResult(r.Context(), u.ID, "position", res); err != nil {
			a.logger.Warn("save quiz result", "err", err)
		} else {
			resp.Saved = true
		}
	}
	render.JSON(w, http.StatusOK, resp)
}

// quizPositionLast — GET /api/quiz/position/last: последняя позиция из квиза (у гостя — null).
func (a *api) quizPositionLast(w http.ResponseWriter, r *http.Request) {
	var position *int
	if u := auth.UserFrom(r.Context()); u != nil {
		raw, err := a.quizResults.LastQuizResult(r.Context(), u.ID, "position")
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			a.logger.Error("last quiz result", "err", err)
			render.Internal(w)
			return
		default:
			var res quiz.PositionResult
			if json.Unmarshal(raw, &res) == nil && res.Primary >= 1 && res.Primary <= 5 {
				position = &res.Primary
			}
		}
	}
	render.JSON(w, http.StatusOK, map[string]*int{"position": position})
}

type heroAnswers struct {
	Position int                 `json:"position"`
	Answers  map[string][]string `json:"answers"`
}

type heroResponse struct {
	Position int             `json:"position"`
	Heroes   []quiz.HeroPick `json:"heroes"`
	Saved    bool            `json:"saved"`
}

// quizHeroResult — POST /api/quiz/hero/result { position, answers: {qid: ["a"]} }.
func (a *api) quizHeroResult(w http.ResponseWriter, r *http.Request) {
	var req heroAnswers
	if !decodeJSON(w, r, &req) {
		return
	}
	picks, err := a.quiz.RecommendHeroes(r.Context(), req.Position, req.Answers)
	switch {
	case errors.Is(err, quiz.ErrBadPosition):
		render.Error(w, http.StatusBadRequest, "bad_position", "Такой позиции нет")
		return
	case errors.Is(err, quiz.ErrBadAnswers):
		render.Error(w, http.StatusBadRequest, "bad_answers", "Ответь на все вопросы: выбери один или два варианта")
		return
	case err != nil:
		a.logger.Error("recommend heroes", "err", err)
		render.Internal(w)
		return
	}
	resp := heroResponse{Position: req.Position, Heroes: picks}
	if u := auth.UserFrom(r.Context()); u != nil {
		if err := a.quizResults.SaveQuizResult(r.Context(), u.ID, "hero", resp); err != nil {
			a.logger.Warn("save quiz result", "err", err)
		} else {
			resp.Saved = true
		}
	}
	render.JSON(w, http.StatusOK, resp)
}

// intParam — целое из пути; 0, если не число.
func intParam(r *http.Request, name string) int {
	n, err := strconv.Atoi(chi.URLParam(r, name))
	if err != nil {
		return 0
	}
	return n
}
