package quiz

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"

	"d2helper/internal/dota"
	"d2helper/internal/meta"
)

// ----- квиз позиции ----------------------------------------------------------

// PositionResult — итог квиза позиции.
type PositionResult struct {
	Primary int `json:"primary"`
	// Secondary — «дополнительные задатки»: вторая позиция, если набрала ≥ 60% от максимума
	Secondary *int `json:"secondary"`
	// Shares — доля очков по позициям 1..5, в процентах (сумма ≈ 100)
	Shares [5]float64 `json:"shares"`
}

const secondaryThreshold = 0.6

// ScorePosition считает результат. answers: id вопроса → id выбранного варианта; нужны ответы на все вопросы.
func (d *Data) ScorePosition(answers map[string]string) (PositionResult, error) {
	var scores [5]int
	firstChoice := [5]int{} // веса варианта, выбранного в первом вопросе — для разрешения ничьей
	for i, q := range d.Position.Questions {
		optID, ok := answers[q.ID]
		if !ok {
			return PositionResult{}, ErrBadAnswers
		}
		var chosen *PositionOption
		for j := range q.Options {
			if q.Options[j].ID == optID {
				chosen = &q.Options[j]
			}
		}
		if chosen == nil {
			return PositionResult{}, ErrBadAnswers
		}
		for pos, w := range chosen.Weights {
			n, _ := strconv.Atoi(pos)
			scores[n-1] += w
			if i == 0 {
				firstChoice[n-1] = w
			}
		}
	}
	if len(answers) != len(d.Position.Questions) {
		return PositionResult{}, ErrBadAnswers
	}

	// порядок: больше очков, затем выше вес в ответе на 1-й вопрос, затем меньший номер
	order := []int{0, 1, 2, 3, 4}
	sort.SliceStable(order, func(a, b int) bool {
		i, j := order[a], order[b]
		if scores[i] != scores[j] {
			return scores[i] > scores[j]
		}
		return firstChoice[i] > firstChoice[j]
	})

	res := PositionResult{Primary: order[0] + 1}
	if top := scores[order[0]]; top > 0 && float64(scores[order[1]]) >= secondaryThreshold*float64(top) {
		second := order[1] + 1
		res.Secondary = &second
	}
	total := 0
	for _, s := range scores {
		total += s
	}
	if total > 0 {
		for i, s := range scores {
			res.Shares[i] = math.Round(float64(s)/float64(total)*1000) / 10
		}
	}
	return res, nil
}

// ----- подбор героя ----------------------------------------------------------

// HeroPick — рекомендованный герой.
type HeroPick struct {
	HeroID     int      `json:"hero_id"`
	Score      float64  `json:"score"`
	Winrate    *float64 `json:"winrate"`
	Complexity int      `json:"complexity"`
	// Matched — теги, совпавшие с ответами (для чипов и объяснения «почему подходит»)
	Matched []string `json:"matched"`
}

// MetaSource — мета по позициям: нужна для бонуса за винрейт.
type MetaSource interface {
	Snapshot(ctx context.Context) (*meta.Snapshot, error)
}

const (
	topHeroes      = 3
	metaBonusScale = 20
	metaBonusCap   = 3
)

// RecommendHeroes подбирает топ-3 героя. answers: id вопроса → выбранные варианты (1–2 штуки).
func (d *Data) RecommendHeroes(ctx context.Context, position int, answers map[string][]string, tags *dota.HeroTagSet, ms MetaSource) ([]HeroPick, error) {
	qs, ok := d.heroQuestions(position)
	if !ok {
		return nil, ErrBadPosition
	}
	if len(answers) != len(qs) {
		return nil, ErrBadAnswers
	}

	weights := map[string]int{}
	var wantHigh, wantLow bool
	for _, q := range qs {
		chosen := answers[q.ID]
		if len(chosen) < 1 || len(chosen) > 2 || (len(chosen) == 2 && chosen[0] == chosen[1]) {
			return nil, ErrBadAnswers
		}
		for _, id := range chosen {
			opt := findHeroOption(q, id)
			if opt == nil {
				return nil, ErrBadAnswers
			}
			for tag, w := range opt.Tags {
				weights[tag] += w
			}
			wantHigh = wantHigh || opt.ComplexityPref == "high"
			wantLow = wantLow || opt.ComplexityPref == "low"
		}
	}
	// оба варианта сложности — без фильтра
	if wantHigh && wantLow {
		wantHigh, wantLow = false, false
	}

	winrates := map[int]float64{}
	if snap, err := ms.Snapshot(ctx); err == nil {
		for _, h := range snap.ByPosition[position-1] {
			winrates[h.HeroID] = h.Winrate
		}
	} else if !errors.Is(err, meta.ErrNotReady) {
		return nil, err
	}

	var picks []HeroPick
	for _, id := range tags.HeroesForPosition(position) {
		h, _ := tags.Get(id)
		pick := HeroPick{HeroID: id, Complexity: h.Complexity}
		for tag, w := range weights {
			if heroHas(h, tag) {
				pick.Score += float64(w)
				pick.Matched = append(pick.Matched, tag)
			}
		}
		sort.Strings(pick.Matched)
		switch {
		case wantLow && h.Complexity == 1:
			pick.Score += 2
		case wantLow && h.Complexity == 3:
			pick.Score -= 3
		case wantHigh && h.Complexity == 3:
			pick.Score += 2
		}
		if wr, ok := winrates[id]; ok {
			pick.Score += math.Max(-metaBonusCap, math.Min(metaBonusCap, (wr-0.5)*metaBonusScale))
			w := wr
			pick.Winrate = &w
		}
		picks = append(picks, pick)
	}

	sort.Slice(picks, func(i, j int) bool {
		if picks[i].Score != picks[j].Score {
			return picks[i].Score > picks[j].Score
		}
		wi, wj := winrateOf(picks[i]), winrateOf(picks[j])
		if wi != wj {
			return wi > wj
		}
		return picks[i].HeroID < picks[j].HeroID
	})
	if len(picks) > topHeroes {
		picks = picks[:topHeroes]
	}
	return picks, nil
}

func winrateOf(p HeroPick) float64 {
	if p.Winrate == nil {
		return 0
	}
	return *p.Winrate
}

func findHeroOption(q HeroQuestion, id string) *HeroOption {
	for i := range q.Options {
		if q.Options[i].ID == id {
			return &q.Options[i]
		}
	}
	return nil
}

// heroHas: тег стиля или тайминг силы (early/mid/late).
func heroHas(h dota.HeroTags, tag string) bool {
	if h.Has(tag) {
		return true
	}
	for _, t := range h.Timing {
		if t == tag {
			return true
		}
	}
	return false
}

// ----- рекомендации героев для результата квиза позиции ---------------------

// MetaHero — сильный герой позиции для экрана результата.
type MetaHero struct {
	HeroID  int     `json:"hero_id"`
	Winrate float64 `json:"winrate"`
	// Difficulty: "easy", "hard" или пусто (средний)
	Difficulty string `json:"difficulty,omitempty"`
}

// TopMetaHeroes — n лучших героев позиции по мете с пометкой сложности из heroes_tags.json.
func TopMetaHeroes(ctx context.Context, ms MetaSource, tags *dota.HeroTagSet, position, n int) []MetaHero {
	out := []MetaHero{}
	snap, err := ms.Snapshot(ctx)
	if err != nil || position < 1 || position > 5 {
		return out
	}
	for _, h := range snap.ByPosition[position-1] {
		if len(out) == n {
			break
		}
		m := MetaHero{HeroID: h.HeroID, Winrate: h.Winrate}
		if t, ok := tags.Get(h.HeroID); ok {
			switch t.Complexity {
			case 1:
				m.Difficulty = "easy"
			case 3:
				m.Difficulty = "hard"
			}
		}
		out = append(out, m)
	}
	return out
}
