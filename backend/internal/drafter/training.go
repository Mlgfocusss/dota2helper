package drafter

import (
	"errors"
	"math"
	"math/rand/v2"
	"slices"

	"d2helper/internal/dota"
	"d2helper/internal/matchups"
	"d2helper/internal/meta"
)

// TagSource — теги героев: на какие позиции герой годится, если меты ещё нет.
type TagSource interface {
	Get(heroID int) (dota.HeroTags, bool)
	HeroesForPosition(pos int) []int
}

// PositionData — кто играется на каких позициях и с каким винрейтом.
type PositionData struct {
	// [pos-1] → hero_id → …
	Playable [5]map[int]bool
	Winrate  [5]map[int]float64
	// Weight — популярность героя на позиции (число матчей), для выбора врагов
	Weight [5]map[int]int
}

// BuildPositions собирает данные из меты (snap может быть nil) и тегов героев.
// Мета по реальным позициям (STRATZ) — основной источник: «играется» значит ≥1% игр
// позиции и ≥30% игр героя. Без неё герой годится на все позиции из тегов.
func BuildPositions(snap *meta.Snapshot, tags TagSource) PositionData {
	var pd PositionData
	for i := range pd.Playable {
		pd.Playable[i] = map[int]bool{}
		pd.Winrate[i] = map[int]float64{}
		pd.Weight[i] = map[int]int{}
	}
	if snap != nil {
		for i, list := range snap.ByPosition {
			for _, h := range list {
				pd.Playable[i][h.HeroID] = true
				pd.Winrate[i][h.HeroID] = h.Winrate
				pd.Weight[i][h.HeroID] = h.Matches
			}
		}
	}
	if snap == nil || snap.Source != meta.SourceStratz {
		for pos := 1; pos <= 5; pos++ {
			for _, id := range tags.HeroesForPosition(pos) {
				pd.Playable[pos-1][id] = true
			}
		}
	}
	return pd
}

// IsPlayable — играется ли герой на позиции.
func (pd PositionData) IsPlayable(heroID, pos int) bool {
	return pos >= 1 && pos <= 5 && pd.Playable[pos-1][heroID]
}

// PlayableList — id героев позиции по возрастанию.
func (pd PositionData) PlayableList(pos int) []int {
	ids := make([]int, 0, len(pd.Playable[pos-1]))
	for id := range pd.Playable[pos-1] {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// GenerateEnemies — по герою на каждую позицию из пула «играбельных», с весами по
// популярности, без повторов (SPEC 5.3). rnd — источник случайности (в тестах фиксированный).
func GenerateEnemies(pd PositionData, rnd *rand.Rand) ([]Slot, error) {
	order := rnd.Perm(5)
	taken := map[int]bool{}
	picked := make(map[int]int, 5) // позиция → герой
	for _, idx := range order {
		pos := idx + 1
		ids := pd.PlayableList(pos)
		var total float64
		var cand []int
		var weights []float64
		for _, id := range ids {
			if taken[id] {
				continue
			}
			w := float64(pd.Weight[idx][id])
			if w <= 0 {
				w = 1
			}
			cand = append(cand, id)
			weights = append(weights, w)
			total += w
		}
		if len(cand) == 0 {
			return nil, errors.New("drafter: no heroes for position")
		}
		r := rnd.Float64() * total
		choice := cand[len(cand)-1]
		for i, w := range weights {
			if r < w {
				choice = cand[i]
				break
			}
			r -= w
		}
		taken[choice] = true
		picked[pos] = choice
	}
	out := make([]Slot, 0, 5)
	for pos := 1; pos <= 5; pos++ {
		out = append(out, Slot{HeroID: picked[pos], Position: pos})
	}
	return out, nil
}

// Coefficients — коэффициенты оценки тренировки (конфиг).
type Coefficients struct {
	Scale         float64
	MetaWeight    float64
	OffPosPenalty float64
}

// Contributions — вклад слагаемых в итоговый счёт, в очках (уже умножены на Scale).
type Contributions struct {
	VS      float64 `json:"vs"`
	Synergy float64 `json:"synergy"`
	Meta    float64 `json:"meta"`
	Penalty float64 `json:"penalty"`
}

// HeroBreakdown — вклад одного героя.
type HeroBreakdown struct {
	HeroID      int     `json:"hero_id"`
	Position    int     `json:"position"`
	VS          float64 `json:"vs"`
	Synergy     float64 `json:"synergy"`
	Meta        float64 `json:"meta"`
	OffPosition bool    `json:"off_position"`
}

// Breakdown — разбор оценки тренировки.
type Breakdown struct {
	Contrib      Contributions   `json:"contrib"`
	OffPosition  int             `json:"off_position"`
	Heroes       []HeroBreakdown `json:"heroes"`
	BestMatchup  *Pair           `json:"best_matchup"`
	WorstMatchup *Pair           `json:"worst_matchup"`
	BestPair     *Pair           `json:"best_pair"`
}

// ScoreTeam считает счёт 0–100 (SPEC 5.3):
//
//	raw = Σ_h [ Σ_e vs(h,e) + 0.5 × Σ_{a≠h} with(h,a) + MetaWeight × (wr_pos(h) − 0.5) ] − OffPosPenalty × (героев не на своей позиции)
//	score = clamp(50 + raw × Scale, 0, 100)
func ScoreTeam(m *matchups.Matrix, pd PositionData, c Coefficients, team, enemies []Slot) (float64, Breakdown) {
	var bd Breakdown
	var vs, syn, metaSum float64
	for _, s := range team {
		hv := 0.0
		for _, e := range enemies {
			hv += m.Vs(s.HeroID, e.HeroID)
		}
		hs := 0.0
		for _, a := range team {
			if a.HeroID != s.HeroID {
				hs += synergyWeight * m.Synergy(s.HeroID, a.HeroID)
			}
		}
		wr, hasWR := pd.Winrate[s.Position-1][s.HeroID]
		if !hasWR {
			wr = 0.5
		}
		hm := c.MetaWeight * (wr - 0.5)
		off := !pd.IsPlayable(s.HeroID, s.Position)
		if off {
			bd.OffPosition++
		}
		vs += hv
		syn += hs
		metaSum += hm
		bd.Heroes = append(bd.Heroes, HeroBreakdown{
			HeroID: s.HeroID, Position: s.Position,
			VS: round1(hv * c.Scale), Synergy: round1(hs * c.Scale), Meta: round1(hm * c.Scale),
			OffPosition: off,
		})
	}
	penalty := c.OffPosPenalty * float64(bd.OffPosition)
	raw := vs + syn + metaSum - penalty
	bd.Contrib = Contributions{
		VS: round1(vs * c.Scale), Synergy: round1(syn * c.Scale),
		Meta: round1(metaSum * c.Scale), Penalty: round1(penalty * c.Scale),
	}
	if m.HasWith {
		bd.BestPair = bestPair(m, team)
	}
	bd.BestMatchup, bd.WorstMatchup = oneWayMatchups(m, team, enemies)
	score := round1(math.Max(0, math.Min(100, 50+raw*c.Scale)))
	return score, bd
}

// oneWayMatchups — лучший и худший матчап героев команды против врагов.
func oneWayMatchups(m *matchups.Matrix, team, enemies []Slot) (best, worst *Pair) {
	for _, a := range team {
		for _, b := range enemies {
			v := round1(m.Vs(a.HeroID, b.HeroID))
			if best == nil || v > best.Value {
				best = &Pair{A: a.HeroID, B: b.HeroID, Value: v}
			}
			if worst == nil || v < worst.Value {
				worst = &Pair{A: a.HeroID, B: b.HeroID, Value: v}
			}
		}
	}
	return best, worst
}
