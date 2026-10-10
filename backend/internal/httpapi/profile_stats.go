package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"d2helper/internal/auth"
	"d2helper/internal/games"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/store"
)

// DrafterStats — сводка по драфтеру для профиля.
type DrafterStats interface {
	DrafterStatsForUser(ctx context.Context, userID int64, monthStart, monthEnd time.Time) (store.DrafterStats, error)
}

type heroQuizStats struct {
	Position int   `json:"position"`
	HeroIDs  []int `json:"hero_ids"`
}

type drafterStatsBody struct {
	Trainings int      `json:"trainings"`
	BestScore *float64 `json:"best_score"`
	MonthRank *int     `json:"month_rank"`
}

type profileStatsBody struct {
	HigherLower map[string]int `json:"higher_lower"`
	Quiz        struct {
		Position *int           `json:"position"`
		Hero     *heroQuizStats `json:"hero"`
	} `json:"quiz"`
	Drafter drafterStatsBody `json:"drafter"`
}

// profileStats — GET /api/profile/stats: рекорды игр, результаты квизов и статистика драфтера.
func (a *api) profileStats(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFrom(r.Context())
	var body profileStatsBody

	records, err := a.games.Records(r.Context(), user.ID)
	if err != nil {
		a.logger.Error("profile stats: records", "err", err)
		render.Internal(w)
		return
	}
	body.HigherLower = map[string]int{}
	for _, m := range games.Modes {
		body.HigherLower[string(m)] = records[string(m)]
	}

	// результаты квизов хранятся как JSON: битый или устаревший просто не показываем
	raw, err := a.quizResults.LastQuizResult(r.Context(), user.ID, "position")
	switch {
	case errors.Is(err, store.ErrNotFound):
	case err != nil:
		a.logger.Error("profile stats: position quiz", "err", err)
		render.Internal(w)
		return
	default:
		var res struct {
			Primary int `json:"primary"`
		}
		if json.Unmarshal(raw, &res) == nil && res.Primary >= 1 && res.Primary <= 5 {
			body.Quiz.Position = &res.Primary
		}
	}

	raw, err = a.quizResults.LastQuizResult(r.Context(), user.ID, "hero")
	switch {
	case errors.Is(err, store.ErrNotFound):
	case err != nil:
		a.logger.Error("profile stats: hero quiz", "err", err)
		render.Internal(w)
		return
	default:
		var res struct {
			Position int `json:"position"`
			Heroes   []struct {
				HeroID int `json:"hero_id"`
			} `json:"heroes"`
		}
		if json.Unmarshal(raw, &res) == nil && len(res.Heroes) > 0 {
			hero := &heroQuizStats{Position: res.Position}
			for _, h := range res.Heroes {
				hero.HeroIDs = append(hero.HeroIDs, h.HeroID)
			}
			body.Quiz.Hero = hero
		}
	}

	start, end := monthBounds(time.Now())
	ds, err := a.drafterStats.DrafterStatsForUser(r.Context(), user.ID, start, end)
	if err != nil {
		a.logger.Error("profile stats: drafter", "err", err)
		render.Internal(w)
		return
	}
	body.Drafter = drafterStatsBody{Trainings: ds.Trainings, BestScore: ds.BestScore, MonthRank: ds.MonthRank}

	render.JSON(w, http.StatusOK, body)
}
