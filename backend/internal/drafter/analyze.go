// Package drafter — расчёты драфта: оценка героев в «Анализе», тренировка с серверной
// оценкой, история и топ драфтеров за месяц (SPEC 5).
package drafter

import (
	"math"

	"d2helper/internal/matchups"
)

// synergyWeight — вес синергии с союзниками в оценке героя (SPEC 5.2).
const synergyWeight = 0.5

// Slot — герой в команде и позиция, на которой он играет.
type Slot struct {
	HeroID   int `json:"hero_id"`
	Position int `json:"position"`
}

// HeroScore — оценка героя в драфте.
type HeroScore struct {
	HeroID int     `json:"hero_id"`
	Score  float64 `json:"score"`
}

// TeamAnalysis — оценки героев команды и итог.
type TeamAnalysis struct {
	Total  float64     `json:"total"`
	Heroes []HeroScore `json:"heroes"`
}

// Pair — пара героев и значение (синергия или преимущество в п. п.).
type Pair struct {
	A     int     `json:"a"`
	B     int     `json:"b"`
	Value float64 `json:"value"`
}

// Analysis — результат «Анализа» (формат совпадает с DraftAnalysis на фронте).
type Analysis struct {
	Radiant      TeamAnalysis `json:"radiant"`
	Dire         TeamAnalysis `json:"dire"`
	BestPair     *Pair        `json:"best_pair"`
	BestMatchup  *Pair        `json:"best_matchup"`
	WorstMatchup *Pair        `json:"worst_matchup"`
}

// round1 — округление до одного знака; ноль без знака («−0.0» не бывает).
func round1(v float64) float64 {
	r := math.Round(v*10) / 10
	if r == 0 {
		return 0
	}
	return r
}

// heroScore — score(h) = Σ_{e∈E} vs(h,e) + 0.5 × Σ_{a∈T, a≠h} with(h,a), округление до 0.1.
func heroScore(m *matchups.Matrix, h int, team, enemies []Slot) float64 {
	var sum float64
	for _, e := range enemies {
		sum += m.Vs(h, e.HeroID)
	}
	for _, a := range team {
		if a.HeroID != h {
			sum += synergyWeight * m.Synergy(h, a.HeroID)
		}
	}
	return round1(sum)
}

func analyzeTeam(m *matchups.Matrix, team, enemies []Slot) TeamAnalysis {
	out := TeamAnalysis{Heroes: make([]HeroScore, 0, len(team))}
	for _, s := range team {
		sc := heroScore(m, s.HeroID, team, enemies)
		out.Heroes = append(out.Heroes, HeroScore{HeroID: s.HeroID, Score: sc})
		out.Total += sc
	}
	out.Total = round1(out.Total)
	return out
}

// Analyze оценивает обе команды. Баны на оценку не влияют, поэтому сюда не передаются.
func Analyze(m *matchups.Matrix, radiant, dire []Slot) Analysis {
	a := Analysis{
		Radiant: analyzeTeam(m, radiant, dire),
		Dire:    analyzeTeam(m, dire, radiant),
	}
	if m.HasWith {
		a.BestPair = bestPair(m, radiant, dire)
	}
	a.BestMatchup, a.WorstMatchup = extremeMatchups(m, radiant, dire)
	return a
}

// pairSynergy — синергия пары: среднее значений в обе стороны.
func pairSynergy(m *matchups.Matrix, a, b int) float64 {
	return (m.Synergy(a, b) + m.Synergy(b, a)) / 2
}

// bestPair — сильнейшая пара внутри одной из команд; nil, если пар нет.
func bestPair(m *matchups.Matrix, teams ...[]Slot) *Pair {
	var best *Pair
	for _, team := range teams {
		for i := 0; i < len(team); i++ {
			for j := i + 1; j < len(team); j++ {
				v := round1(pairSynergy(m, team[i].HeroID, team[j].HeroID))
				if best == nil || v > best.Value {
					best = &Pair{A: team[i].HeroID, B: team[j].HeroID, Value: v}
				}
			}
		}
	}
	return best
}

// extremeMatchups — лучший и худший матчап: a из одной команды против b из другой
// (значение — преимущество a над b). Оба nil, если одной из команд нет.
func extremeMatchups(m *matchups.Matrix, x, y []Slot) (best, worst *Pair) {
	consider := func(from, against []Slot) {
		for _, a := range from {
			for _, b := range against {
				v := round1(m.Vs(a.HeroID, b.HeroID))
				if best == nil || v > best.Value {
					best = &Pair{A: a.HeroID, B: b.HeroID, Value: v}
				}
				if worst == nil || v < worst.Value {
					worst = &Pair{A: a.HeroID, B: b.HeroID, Value: v}
				}
			}
		}
	}
	consider(x, y)
	consider(y, x)
	return best, worst
}
