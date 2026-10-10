package analysis

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"d2helper/internal/clients/opendota"
)

// ErrPlayerNotInMatch — игрока с таким account_id нет в матче (или он скрыл профиль).
var ErrPlayerNotInMatch = errors.New("analysis: player not found in match")

// Catalog — справочники для читаемых названий в признаках.
type Catalog struct {
	// HeroName — имя героя по id
	HeroName func(id int) string
	// HeroByKey — имя героя по ключу из логов («npc_dota_hero_sand_king»)
	HeroByKey func(key string) string
	// ItemByKey — предметы по короткому имени («blink»)
	ItemByKey map[string]opendota.ItemConstant
	// ItemName — название предмета по id (слоты инвентаря)
	ItemName func(id int) string
}

// Features — компактное описание игры одного игрока. Уходит в языковую модель как JSON:
// модель разбирает только то, что здесь есть.
type Features struct {
	Match      MatchInfo       `json:"match"`
	Player     PlayerInfo      `json:"player"`
	Lane       LaneInfo        `json:"lane"`
	Benchmarks []BenchmarkInfo `json:"benchmarks,omitempty"`
	Deaths     []DeathInfo     `json:"deaths"`
	Teamfights []FightInfo     `json:"teamfights,omitempty"`
	KeyItems   []ItemTiming    `json:"key_item_purchases,omitempty"`
	FinalItems []string        `json:"final_items,omitempty"`
	Networth   []NetworthPoint `json:"gold_by_minute,omitempty"`
	Wards      *WardInfo       `json:"wards,omitempty"`
	Buybacks   []string        `json:"buybacks,omitempty"`

	// не уходят в модель: подставляются в результат сами
	KeyStats    []KeyStat `json:"-"`
	DeathPoints []Point   `json:"-"`
}

type MatchInfo struct {
	Duration string `json:"duration"`
	Result   string `json:"result"` // win / loss
	Side     string `json:"side"`   // radiant / dire
	Score    string `json:"score"`  // «наши – чужие» по убийствам
}

type PlayerInfo struct {
	Hero                   string  `json:"hero"`
	PositionGuess          int     `json:"position_guess"`
	Kills                  int     `json:"kills"`
	Deaths                 int     `json:"deaths"`
	Assists                int     `json:"assists"`
	GPM                    int     `json:"gpm"`
	XPM                    int     `json:"xpm"`
	LastHits               int     `json:"last_hits"`
	Denies                 int     `json:"denies"`
	Level                  int     `json:"level"`
	HeroDamage             int     `json:"hero_damage"`
	TowerDamage            int     `json:"tower_damage"`
	HeroHealing            int     `json:"hero_healing"`
	TeamfightParticipation float64 `json:"teamfight_participation_pct,omitempty"`
}

type LaneInfo struct {
	LH10 int `json:"last_hits_at_10"`
	DN10 int `json:"denies_at_10"`
	// разница с оппонентом по линии (отрицательная — проигрыш); HasOpponent=false — оппонента определить не удалось
	HasOpponent bool     `json:"has_opponent"`
	Opponent    string   `json:"opponent_hero,omitempty"`
	GoldDiff5   int      `json:"gold_diff_at_5"`
	GoldDiff10  int      `json:"gold_diff_at_10"`
	XPDiff5     int      `json:"xp_diff_at_5"`
	XPDiff10    int      `json:"xp_diff_at_10"`
	Efficiency  *float64 `json:"lane_efficiency_pct,omitempty"`
}

type BenchmarkInfo struct {
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Percentile int     `json:"percentile"` // 0..100, выше — лучше среди игроков на этом герое
}

type DeathInfo struct {
	Time          string `json:"time"`
	KilledBy      string `json:"killed_by"`
	GoldLost      int    `json:"gold_lost"`
	RespawnSec    int    `json:"respawn_sec"`
	InTeamfight   bool   `json:"in_teamfight"`
	BeforeMinute5 bool   `json:"before_minute_5,omitempty"`
}

type FightInfo struct {
	Start       string `json:"start"`
	TotalDeaths int    `json:"total_deaths"`
	MyDeaths    int    `json:"my_deaths"`
	MyDamage    int    `json:"my_damage"`
	MyHealing   int    `json:"my_healing"`
	GoldDelta   int    `json:"my_gold_delta"`
}

type ItemTiming struct {
	Time string `json:"time"`
	Item string `json:"item"`
}

type NetworthPoint struct {
	Minute   int `json:"minute"`
	Me       int `json:"me"`
	TeamAvg  int `json:"team_avg"`
	EnemyAvg int `json:"enemy_avg"`
}

type WardInfo struct {
	Observers  int      `json:"observers"`
	Sentries   int      `json:"sentries"`
	FirstWards []string `json:"first_ward_times,omitempty"`
	CampsStack int      `json:"camps_stacked"`
}

// Пределы: данные должны умещаться в контекст небольшой бесплатной модели.
const (
	maxDeaths     = 15
	maxFights     = 10
	maxKeyItems   = 14
	keyItemCost   = 1000
	networthStep  = 5
	fightPadding  = 5 // секунд до начала и после конца драки, когда смерть ещё считается «в драке»
	mapOrigin     = 64.0
	mapSize       = 128.0
	maxDeathCells = 40
)

func clock(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}

// at — значение минутного ряда на минуте i (последнее известное, если ряд короче).
func at(series []int, i int) int {
	if len(series) == 0 {
		return 0
	}
	return series[min(i, len(series)-1)]
}

func avg(vals []int) int {
	if len(vals) == 0 {
		return 0
	}
	sum := 0
	for _, v := range vals {
		sum += v
	}
	return int(math.Round(float64(sum) / float64(len(vals))))
}

// guessPosition оценивает позицию 1–5: три самых богатых героя команды — кор (линия решает,
// какой именно), остальные двое — саппорты (богаче — четвёрка).
func guessPosition(m *opendota.ParsedMatch, me int) int {
	team := []int{}
	for i, p := range m.Players {
		if p.IsRadiant == m.Players[me].IsRadiant {
			team = append(team, i)
		}
	}
	sort.SliceStable(team, func(a, b int) bool { return m.Players[team[a]].NetWorth > m.Players[team[b]].NetWorth })
	rank := 0
	for i, idx := range team {
		if idx == me {
			rank = i
		}
	}
	if rank >= 3 {
		return rank + 1 // 4 или 5
	}
	switch role := m.Players[me].LaneRole; {
	case role >= 1 && role <= 3:
		return role
	}
	return rank + 1
}

// laneOpponent — герой другой команды на той же линии на противоположной роли
// (лёгкая линия против сложной; мид против мида). nil — не нашли.
func laneOpponent(m *opendota.ParsedMatch, me int) *opendota.ParsedPlayer {
	p := m.Players[me]
	var wantRole int
	switch p.LaneRole {
	case 1:
		wantRole = 3
	case 3:
		wantRole = 1
	case 2:
		wantRole = 2
	default:
		return nil
	}
	var best *opendota.ParsedPlayer
	for i := range m.Players {
		o := &m.Players[i]
		if o.IsRadiant == p.IsRadiant || o.Lane != p.Lane || o.LaneRole != wantRole || o.IsRoaming {
			continue
		}
		if best == nil || at(o.GoldT, 10) > at(best.GoldT, 10) {
			best = o
		}
	}
	return best
}

func inFight(fights []opendota.Teamfight, t int) bool {
	for _, f := range fights {
		if t >= f.Start-fightPadding && t <= f.End+fightPadding {
			return true
		}
	}
	return false
}

// BuildFeatures собирает признаки игры игрока accountID из распарсенного матча.
func BuildFeatures(m *opendota.ParsedMatch, accountID int64, cat Catalog) (*Features, error) {
	me := -1
	for i, p := range m.Players {
		if p.AccountID != nil && *p.AccountID == accountID {
			me = i
		}
	}
	if me < 0 {
		return nil, ErrPlayerNotInMatch
	}
	p := m.Players[me]
	f := &Features{Deaths: []DeathInfo{}}

	// матч
	myKills, enemyKills := m.RadiantScore, m.DireScore
	if !p.IsRadiant {
		myKills, enemyKills = enemyKills, myKills
	}
	f.Match = MatchInfo{
		Duration: clock(m.Duration), Side: map[bool]string{true: "radiant", false: "dire"}[p.IsRadiant],
		Result: map[bool]string{true: "win", false: "loss"}[p.IsRadiant == m.RadiantWin],
		Score:  fmt.Sprintf("%d–%d", myKills, enemyKills),
	}

	position := guessPosition(m, me)
	f.Player = PlayerInfo{
		Hero: cat.HeroName(p.HeroID), PositionGuess: position,
		Kills: p.Kills, Deaths: p.Deaths, Assists: p.Assists, GPM: p.GPM, XPM: p.XPM,
		LastHits: p.LastHits, Denies: p.Denies, Level: p.Level,
		HeroDamage: p.HeroDamage, TowerDamage: p.TowerDamage, HeroHealing: p.HeroHealing,
	}
	if p.TeamfightParticipation > 0 {
		f.Player.TeamfightParticipation = math.Round(p.TeamfightParticipation * 100)
	}

	// линия
	f.Lane = LaneInfo{LH10: at(p.LHT, 10), DN10: at(p.DNT, 10), Efficiency: p.LaneEffPct}
	if opp := laneOpponent(m, me); opp != nil && len(p.GoldT) > 5 && len(opp.GoldT) > 5 {
		f.Lane.HasOpponent = true
		f.Lane.Opponent = cat.HeroName(opp.HeroID)
		f.Lane.GoldDiff5 = at(p.GoldT, 5) - at(opp.GoldT, 5)
		f.Lane.GoldDiff10 = at(p.GoldT, 10) - at(opp.GoldT, 10)
		f.Lane.XPDiff5 = at(p.XPT, 5) - at(opp.XPT, 5)
		f.Lane.XPDiff10 = at(p.XPT, 10) - at(opp.XPT, 10)
	}

	// сравнение с другими игроками на этом герое
	for _, name := range []string{"gold_per_min", "xp_per_min", "last_hits_per_min", "hero_damage_per_min", "hero_healing_per_min", "tower_damage"} {
		if b, ok := p.Benchmarks[name]; ok {
			f.Benchmarks = append(f.Benchmarks, BenchmarkInfo{
				Metric: name, Value: math.Round(b.Raw*10) / 10, Percentile: int(math.Round(b.Pct * 100)),
			})
		}
	}

	// смерти и драки
	teamfightDeaths := 0
	for _, d := range p.DeathsLog {
		if len(f.Deaths) >= maxDeaths {
			break
		}
		in := inFight(m.Teamfights, d.Time)
		if in {
			teamfightDeaths++
		}
		killer := cat.HeroByKey(d.Key)
		if killer == "" {
			killer = "нейтралы или башня"
		}
		f.Deaths = append(f.Deaths, DeathInfo{
			Time: clock(d.Time), KilledBy: killer, GoldLost: d.GoldLost, RespawnSec: d.TimeDead,
			InTeamfight: in, BeforeMinute5: d.Time < 5*60,
		})
	}
	cells := map[[2]int]int{}
	for _, tf := range m.Teamfights {
		if me >= len(tf.Players) {
			continue
		}
		mine := tf.Players[me]
		if tf.Deaths > 0 && len(f.Teamfights) < maxFights {
			f.Teamfights = append(f.Teamfights, FightInfo{
				Start: clock(tf.Start), TotalDeaths: tf.Deaths, MyDeaths: mine.Deaths,
				MyDamage: mine.Damage, MyHealing: mine.Healing, GoldDelta: mine.GoldDelta,
			})
		}
		for xs, row := range mine.DeathsPos {
			for ys, n := range row {
				var x, y int
				if _, err := fmt.Sscan(xs, &x); err != nil {
					continue
				}
				if _, err := fmt.Sscan(ys, &y); err != nil {
					continue
				}
				cells[[2]int{x, y}] += n
			}
		}
	}
	f.DeathPoints = deathPoints(cells)

	// предметы
	for _, e := range p.PurchaseLog {
		it, ok := cat.ItemByKey[e.Key]
		if !ok || e.Time < 0 || it.Cost < keyItemCost || strings.HasPrefix(e.Key, "recipe_") || e.Key == "tpscroll" {
			continue
		}
		if len(f.KeyItems) < maxKeyItems {
			f.KeyItems = append(f.KeyItems, ItemTiming{Time: clock(e.Time), Item: it.DName})
		}
	}
	for _, id := range p.Items() {
		if id != 0 {
			if name := cat.ItemName(id); name != "" {
				f.FinalItems = append(f.FinalItems, name)
			}
		}
	}

	// золото по минутам: я, союзники, враги
	var allies, enemies []opendota.ParsedPlayer
	for i, o := range m.Players {
		switch {
		case i == me:
		case o.IsRadiant == p.IsRadiant:
			allies = append(allies, o)
		default:
			enemies = append(enemies, o)
		}
	}
	for minute := networthStep; minute*60 <= m.Duration && minute <= len(p.GoldT)-1; minute += networthStep {
		pt := NetworthPoint{Minute: minute, Me: at(p.GoldT, minute)}
		var a, e []int
		for _, o := range allies {
			a = append(a, at(o.GoldT, minute))
		}
		for _, o := range enemies {
			e = append(e, at(o.GoldT, minute))
		}
		pt.TeamAvg, pt.EnemyAvg = avg(a), avg(e)
		f.Networth = append(f.Networth, pt)
	}

	// варды и стаки — только саппортам
	if position >= 4 {
		w := &WardInfo{Observers: p.ObsPlaced, Sentries: p.SenPlaced, CampsStack: p.CampsStacked}
		for i, e := range p.ObsLog {
			if i < 3 {
				w.FirstWards = append(w.FirstWards, clock(e.Time))
			}
		}
		f.Wards = w
	}
	for _, b := range p.BuybackLog {
		f.Buybacks = append(f.Buybacks, clock(b.Time))
	}

	f.KeyStats = keyStats(p, teamfightDeaths, len(f.Deaths))
	return f, nil
}

// deathPoints переводит клетки карты в точки 0..1 (ось y — сверху вниз, как в SVG).
func deathPoints(cells map[[2]int]int) []Point {
	type cell struct {
		xy [2]int
		n  int
	}
	list := make([]cell, 0, len(cells))
	for xy, n := range cells {
		list = append(list, cell{xy, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].xy[0]*1000+list[i].xy[1] < list[j].xy[0]*1000+list[j].xy[1]
	})
	if len(list) > maxDeathCells {
		list = list[:maxDeathCells]
	}
	norm := func(v int) float64 {
		return math.Round(math.Min(1, math.Max(0, (float64(v)-mapOrigin)/mapSize))*1000) / 1000
	}
	out := make([]Point, 0, len(list))
	for _, c := range list {
		out = append(out, Point{X: norm(c.xy[0]), Y: 1 - norm(c.xy[1]), N: c.n})
	}
	return out
}

// keyStats — главные цифры матча для карточки вердикта (считаются, а не пишутся моделью).
func keyStats(p opendota.ParsedPlayer, teamfightDeaths, loggedDeaths int) []KeyStat {
	stats := []KeyStat{
		{"Убийства / смерти / помощь", fmt.Sprintf("%d / %d / %d", p.Kills, p.Deaths, p.Assists)},
		{"Золото и опыт в минуту", fmt.Sprintf("%d / %d", p.GPM, p.XPM)},
		{"Добито / отдано", fmt.Sprintf("%d / %d", p.LastHits, p.Denies)},
		{"Урон по героям", fmt.Sprintf("%d", p.HeroDamage)},
	}
	if p.TeamfightParticipation > 0 {
		stats = append(stats, KeyStat{"Участие в драках", fmt.Sprintf("%d%%", int(math.Round(p.TeamfightParticipation*100)))})
	}
	if loggedDeaths > 0 {
		stats = append(stats, KeyStat{"Смертей вне драк", fmt.Sprintf("%d из %d", loggedDeaths-teamfightDeaths, loggedDeaths)})
	}
	return stats
}
