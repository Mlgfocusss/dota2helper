package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"d2helper/internal/clients/opendota"
	"d2helper/internal/dota"
	"d2helper/internal/llm"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// ----- проверка ответа модели ---------------------------------------------------

const goodAnswer = `{
  "verdict": "Хорошая линия, но три смерти без поддержки.",
  "rating": 7,
  "lane_text": "Линию выиграл.",
  "episodes": [
    {"time": "12:30", "title": "Смерть", "what_happened": "Умер у башни", "mistake": "Нет контроля карты", "better": "Отойти", "severity": "major"},
    {"time": "4:12", "title": "Убийство", "what_happened": "Добил оппонента", "mistake": null, "better": "Так держать", "severity": "good"}
  ],
  "improvements": [
    {"title": "Карта", "why": "Умирал", "drill": "Смотреть на миникарту"},
    {"title": "Фарм", "why": "Далеко", "drill": "Ставить варды"}
  ],
  "next_games_plan": ["один", "два", "три"]
}`

func TestParseAnswerOK(t *testing.T) {
	a, err := ParseAnswer(goodAnswer)
	if err != nil {
		t.Fatal(err)
	}
	if a.Rating != 7 || len(a.Episodes) != 2 || len(a.Improvements) != 2 || len(a.Plan) != 3 {
		t.Fatalf("parsed: %+v", a)
	}
	// эпизоды отсортированы по времени
	if a.Episodes[0].Time != "4:12" || a.Episodes[1].Time != "12:30" {
		t.Errorf("episodes must be sorted by time: %+v", a.Episodes)
	}
}

func TestParseAnswerExtractsJSONFromNoise(t *testing.T) {
	wrapped := map[string]string{
		"markdown fence": "```json\n" + goodAnswer + "\n```",
		"chatter":        "Конечно! Вот разбор:\n" + goodAnswer + "\nУдачи в играх!",
		"think block":    "<think>надо подумать {не json}</think>\n" + goodAnswer,
	}
	for name, raw := range wrapped {
		if _, err := ParseAnswer(raw); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestParseAnswerRejects(t *testing.T) {
	mod := func(from, to string) string { return strings.Replace(goodAnswer, from, to, 1) }
	cases := map[string]string{
		"no json":        "извини, не могу",
		"broken json":    `{"verdict": "x",`,
		"empty verdict":  mod(`"Хорошая линия, но три смерти без поддержки."`, `""`),
		"no rating":      mod(`"rating": 7,`, ``),
		"zero rating":    mod(`"rating": 7`, `"rating": 0`),
		"no episodes":    mod(`"episodes": [`, `"episodes_x": [`),
		"short plan":     mod(`["один", "два", "три"]`, `["один"]`),
		"no improvement": mod(`"improvements": [`, `"improvements_x": [`),
		"bad rating txt": mod(`"rating": 7`, `"rating": "высокая"`),
	}
	for name, raw := range cases {
		if _, err := ParseAnswer(raw); !errors.Is(err, ErrBadAnswer) {
			t.Errorf("%s: want ErrBadAnswer, got %v", name, err)
		}
	}
}

func TestParseAnswerNormalizes(t *testing.T) {
	raw := `{
	  "verdict": "ok", "rating": "8/10", "lane_text": "x",
	  "episodes": [
	    {"time": "5.3", "title": "t1", "what_happened": "w", "mistake": "ошибка у хорошего хода", "better": "b", "severity": "GOOD"},
	    {"time": "скоро", "title": "t2", "what_happened": "w", "better": "b", "severity": "major"},
	    {"time": "7:61", "title": "t3", "what_happened": "w", "better": "b", "severity": "major"},
	    {"time": "9:00", "title": "t4", "what_happened": "w", "mistake": "  ", "better": "b", "severity": "странная"}
	  ],
	  "improvements": [
	    {"title": "1", "why": "w", "drill": "d"}, {"title": "2", "why": "w", "drill": "d"},
	    {"title": "3", "why": "w", "drill": "d"}, {"title": "4", "why": "w", "drill": "d"},
	    {"title": "", "why": "w", "drill": "d"}
	  ],
	  "next_games_plan": ["a", "b", "c", "d", "e", "f", " "]
	}`
	a, err := ParseAnswer(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Rating != 8 {
		t.Errorf("rating from text: %v", a.Rating)
	}
	// эпизоды с нечитаемым временем отброшены, «5.3» → 5:30
	if len(a.Episodes) != 2 || a.Episodes[0].Time != "5:30" || a.Episodes[1].Time != "9:00" {
		t.Fatalf("episodes: %+v", a.Episodes)
	}
	if a.Episodes[0].Severity != "good" || a.Episodes[0].Mistake != nil {
		t.Errorf("good move must have no mistake: %+v", a.Episodes[0])
	}
	if a.Episodes[1].Severity != "minor" || a.Episodes[1].Mistake != nil {
		t.Errorf("unknown severity → minor, blank mistake → nil: %+v", a.Episodes[1])
	}
	if len(a.Improvements) != 3 || len(a.Plan) != 5 {
		t.Errorf("limits: %d improvements, %d plan items", len(a.Improvements), len(a.Plan))
	}
}

func TestParseAnswerClipsAndCapsRating(t *testing.T) {
	long := strings.Repeat("я", 2000)
	raw := fmt.Sprintf(`{"verdict": %q, "rating": 15, "lane_text": %q,
	  "episodes": [{"time":"1:00","title":%q,"what_happened":%q,"better":"b","severity":"minor"}],
	  "improvements": [{"title":"t","why":"w","drill":"d"}], "next_games_plan": ["a","b","c"]}`, long, long, long, long)
	a, err := ParseAnswer(raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Rating != 10 {
		t.Errorf("rating above 10 must be capped: %v", a.Rating)
	}
	if n := len([]rune(a.Verdict)); n > maxVerdict {
		t.Errorf("verdict not clipped: %d", n)
	}
	if n := len([]rune(a.Episodes[0].Title)); n > maxTitle {
		t.Errorf("title not clipped: %d", n)
	}
}

// ----- признаки игры -----------------------------------------------------------

func ints(n int, f func(i int) int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = f(i)
	}
	return out
}

func i64(v int64) *int64 { return &v }

// матч на 40 минут: я — игрок 0 (Radiant, лёгкая линия), оппонент — игрок 8 (Dire, сложная линия на той же линии).
func syntheticMatch() *opendota.ParsedMatch {
	v := 22
	players := make([]opendota.ParsedPlayer, 10)
	for i := range players {
		radiant := i < 5
		players[i] = opendota.ParsedPlayer{
			PlayerSlot: i, HeroID: 100 + i, IsRadiant: radiant,
			NetWorth: 20000 - i*1000, Lane: 2, LaneRole: 2,
			GoldT: ints(41, func(m int) int { return m * (300 + i) }),
			XPT:   ints(41, func(m int) int { return m * 400 }),
		}
	}
	me := &players[0]
	me.AccountID = i64(111)
	me.Lane, me.LaneRole = 1, 1
	me.Kills, me.Deaths, me.Assists = 9, 3, 11
	me.GPM, me.XPM, me.LastHits, me.Denies, me.Level = 612, 701, 341, 18, 27
	me.HeroDamage, me.TeamfightParticipation = 24380, 0.71
	me.GoldT = ints(41, func(m int) int { return m * 500 })
	me.LHT = ints(41, func(m int) int { return m * 6 })
	me.DNT = ints(41, func(m int) int { return m })
	me.XPT = ints(41, func(m int) int { return m * 450 })
	me.DeathsLog = []opendota.DeathEntry{
		{Time: 200, Key: "npc_dota_hero_sand_king", GoldLost: 80, TimeDead: 20},       // до 5-й минуты, не в драке
		{Time: 880, Key: "npc_dota_hero_slark", GoldLost: 214, TimeDead: 30},          // в драке
		{Time: 1500, Key: "npc_dota_hero_unknown_thing", GoldLost: 300, TimeDead: 40}, // вне драки
	}
	me.PurchaseLog = []opendota.LogEntry{
		{Time: -89, Key: "branches"}, {Time: 600, Key: "blink"}, {Time: 700, Key: "recipe_blink"},
		{Time: 800, Key: "tpscroll"}, {Time: 1320, Key: "black_king_bar"}, {Time: 900, Key: "unknown_item"},
	}
	me.BuybackLog = []opendota.LogEntry{{Time: 1875}}
	me.Item0, me.Item1 = 1, 2
	me.Benchmarks = map[string]opendota.Benchmark{
		"gold_per_min": {Raw: 612.04, Pct: 0.79}, "xp_per_min": {Raw: 701, Pct: 0.54}, "kills_per_min": {Raw: 0.2, Pct: 0.4},
	}
	eff := 81.0
	me.LaneEffPct = &eff

	opp := &players[8]
	opp.Lane, opp.LaneRole = 1, 3
	opp.GoldT = ints(41, func(m int) int { return m * 459 }) // на 5-й минуте 2295 против 2500, на 10-й 4590 против 5000
	opp.XPT = ints(41, func(m int) int { return m * 400 })

	// другая линия, чтобы не спутать оппонента
	players[7].Lane, players[7].LaneRole = 3, 1

	tf := func(start, end, deaths int) opendota.Teamfight {
		f := opendota.Teamfight{Start: start, End: end, Deaths: deaths, Players: make([]opendota.TeamfightPlayer, 10)}
		return f
	}
	fight := tf(870, 910, 4)
	fight.Players[0] = opendota.TeamfightPlayer{
		Deaths: 1, Damage: 3100, Healing: 0, GoldDelta: -120,
		DeathsPos: map[string]map[string]int{"96": {"112": 2}, "bad": {"x": 1}},
	}
	quiet := tf(2000, 2010, 0) // драка без смертей в отчёт не попадает

	return &opendota.ParsedMatch{
		MatchID: 1, Version: &v, Duration: 2400, StartTime: time.Now().Unix(), RadiantWin: true,
		RadiantScore: 40, DireScore: 31, Teamfights: []opendota.Teamfight{fight, quiet}, Players: players,
	}
}

func testCatalog() Catalog {
	return Catalog{
		HeroName: func(id int) string { return fmt.Sprintf("Hero%d", id) },
		HeroByKey: func(key string) string {
			return map[string]string{"npc_dota_hero_sand_king": "Sand King", "npc_dota_hero_slark": "Slark"}[key]
		},
		ItemByKey: map[string]opendota.ItemConstant{
			"branches": {ID: 1, DName: "Iron Branch", Cost: 50}, "blink": {ID: 2, DName: "Blink Dagger", Cost: 2250},
			"recipe_blink": {ID: 3, DName: "Recipe", Cost: 1000}, "tpscroll": {ID: 4, DName: "TP", Cost: 1000},
			"black_king_bar": {ID: 5, DName: "Black King Bar", Cost: 4050},
		},
		ItemName: func(id int) string { return map[int]string{1: "Iron Branch", 2: "Blink Dagger"}[id] },
	}
}

func TestBuildFeatures(t *testing.T) {
	f, err := BuildFeatures(syntheticMatch(), 111, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if f.Match.Result != "win" || f.Match.Side != "radiant" || f.Match.Score != "40–31" || f.Match.Duration != "40:00" {
		t.Errorf("match: %+v", f.Match)
	}
	if f.Player.Hero != "Hero100" || f.Player.PositionGuess != 1 || f.Player.TeamfightParticipation != 71 {
		t.Errorf("player: %+v", f.Player)
	}

	// линия: добито и отдано к 10-й минуте, отрыв от оппонента на той же линии
	l := f.Lane
	if l.LH10 != 60 || l.DN10 != 10 || !l.HasOpponent || l.Opponent != "Hero108" {
		t.Errorf("lane: %+v", l)
	}
	if l.GoldDiff5 != 205 || l.GoldDiff10 != 410 || l.XPDiff10 != 500 {
		t.Errorf("lane diffs: %+v", l)
	}
	if l.Efficiency == nil || *l.Efficiency != 81 {
		t.Errorf("lane efficiency: %v", l.Efficiency)
	}

	// смерти: в драке / вне драки / до 5-й минуты / неизвестный убийца
	if len(f.Deaths) != 3 {
		t.Fatalf("deaths: %+v", f.Deaths)
	}
	if d := f.Deaths[0]; d.Time != "3:20" || d.KilledBy != "Sand King" || d.InTeamfight || !d.BeforeMinute5 {
		t.Errorf("early death: %+v", d)
	}
	if d := f.Deaths[1]; !d.InTeamfight || d.KilledBy != "Slark" || d.GoldLost != 214 {
		t.Errorf("teamfight death: %+v", d)
	}
	if d := f.Deaths[2]; d.InTeamfight || d.KilledBy == "" || d.KilledBy == "Slark" {
		t.Errorf("unknown killer must get a fallback name: %+v", d)
	}

	// драки: только со смертями
	if len(f.Teamfights) != 1 || f.Teamfights[0].Start != "14:30" || f.Teamfights[0].MyDeaths != 1 || f.Teamfights[0].MyDamage != 3100 {
		t.Errorf("teamfights: %+v", f.Teamfights)
	}

	// предметы: дорогие, без рецептов, свитков телепорта и неизвестных
	if len(f.KeyItems) != 2 || f.KeyItems[0].Item != "Blink Dagger" || f.KeyItems[0].Time != "10:00" || f.KeyItems[1].Item != "Black King Bar" {
		t.Errorf("key items: %+v", f.KeyItems)
	}
	if len(f.FinalItems) != 2 {
		t.Errorf("final items: %v", f.FinalItems)
	}

	// сравнение с игроками на герое: берутся только нужные метрики
	if len(f.Benchmarks) != 2 || f.Benchmarks[0].Metric != "gold_per_min" || f.Benchmarks[0].Percentile != 79 || f.Benchmarks[0].Value != 612 {
		t.Errorf("benchmarks: %+v", f.Benchmarks)
	}

	// золото по минутам: каждые 5 минут до конца матча
	if len(f.Networth) != 8 || f.Networth[0].Minute != 5 || f.Networth[0].Me != 2500 || f.Networth[7].Minute != 40 {
		t.Errorf("gold by minute: %+v", f.Networth)
	}
	if f.Wards != nil {
		t.Error("carry has no ward block")
	}
	if len(f.Buybacks) != 1 || f.Buybacks[0] != "31:15" {
		t.Errorf("buybacks: %v", f.Buybacks)
	}

	// точки смертей: x = (96−64)/128, y перевёрнут для SVG; мусорные клетки пропущены
	if len(f.DeathPoints) != 1 || f.DeathPoints[0].X != 0.25 || f.DeathPoints[0].Y != 0.625 || f.DeathPoints[0].N != 2 {
		t.Errorf("death points: %+v", f.DeathPoints)
	}

	// ключевые цифры считаются из данных
	if len(f.KeyStats) != 6 || f.KeyStats[0].Value != "9 / 3 / 11" || f.KeyStats[1].Value != "612 / 701" {
		t.Errorf("key stats: %+v", f.KeyStats)
	}
	if last := f.KeyStats[5]; last.Value != "2 из 3" {
		t.Errorf("deaths outside teamfights: %+v", last)
	}

	// в JSON для модели нет служебных полей
	raw, _ := json.Marshal(f)
	if strings.Contains(string(raw), "death_points") || strings.Contains(string(raw), "KeyStats") {
		t.Errorf("internal fields leaked into the prompt data: %s", raw)
	}
}

func TestBuildFeaturesSupportAndMissingPlayer(t *testing.T) {
	m := syntheticMatch()
	// делаем игрока 4 (самого бедного в команде) саппортом с вардами
	m.Players[4].AccountID = i64(222)
	m.Players[4].ObsPlaced, m.Players[4].CampsStacked = 7, 3
	m.Players[4].ObsLog = []opendota.WardEntry{{Time: 90}, {Time: 400}, {Time: 700}, {Time: 900}}
	f, err := BuildFeatures(m, 222, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if f.Player.PositionGuess != 5 || f.Wards == nil || f.Wards.Observers != 7 || len(f.Wards.FirstWards) != 3 {
		t.Errorf("support: pos=%d wards=%+v", f.Player.PositionGuess, f.Wards)
	}
	if _, err := BuildFeatures(m, 999, testCatalog()); !errors.Is(err, ErrPlayerNotInMatch) {
		t.Errorf("unknown player: %v", err)
	}
}

func TestBuildFeaturesWithoutOpponent(t *testing.T) {
	m := syntheticMatch()
	m.Players[8].Lane = 3
	f, err := BuildFeatures(m, 111, testCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if f.Lane.HasOpponent || f.Lane.GoldDiff10 != 0 {
		t.Errorf("no opponent on the lane: %+v", f.Lane)
	}
}

func TestAssembleUsesDataNotModelNumbers(t *testing.T) {
	f, _ := BuildFeatures(syntheticMatch(), 111, testCatalog())
	a, _ := ParseAnswer(goodAnswer)
	r := Assemble(a, f)
	if r.Lanes.LH10 != 60 || r.Lanes.DN10 != 10 || r.Lanes.GoldDiff10 != 410 || r.Lanes.Text != "Линию выиграл." {
		t.Errorf("lanes: %+v", r.Lanes)
	}
	if len(r.Summary.KeyStats) == 0 || r.Summary.Rating != 7 || len(r.DeathPoints) != 1 {
		t.Errorf("summary: %+v", r.Summary)
	}
	// результат сериализуется в формат, который читает фронтенд
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	_ = json.Unmarshal(raw, &back)
	for _, key := range []string{"summary", "lanes", "episodes", "improvements", "next_games_plan", "death_points"} {
		if _, ok := back[key]; !ok {
			t.Errorf("missing %q in result JSON", key)
		}
	}
}

func TestExampleFileIsValid(t *testing.T) {
	raw, err := LoadExample("../../data")
	if err != nil {
		t.Fatal(err)
	}
	var r Result
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if r.Summary.Rating < 1 || r.Summary.Rating > 10 || len(r.Improvements) != 3 || len(r.NextGamesPlan) < 3 {
		t.Errorf("example shape: %+v", r.Summary)
	}
	for _, e := range r.Episodes {
		if _, _, ok := normTime(e.Time); !ok || severities[e.Severity] == "" {
			t.Errorf("bad episode: %+v", e)
		}
	}
}

// ----- сервис ----------------------------------------------------------------

type fakeStore struct {
	mu       sync.Mutex
	items    map[string]*store.Analysis
	order    []string
	next     int
	matches  map[[2]int64]bool
	statuses []string // история статусов для проверки стадий
}

func newStore() *fakeStore {
	return &fakeStore{items: map[string]*store.Analysis{}, matches: map[[2]int64]bool{}}
}

func (f *fakeStore) CreateAnalysis(_ context.Context, userID, accountID, matchID int64) (store.Analysis, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.items {
		if a.UserID == userID && a.MatchID == matchID && a.Status != "failed" {
			return *a, store.ErrAnalysisExists
		}
	}
	f.next++
	a := &store.Analysis{ID: fmt.Sprintf("a%d", f.next), UserID: userID, AccountID: accountID, MatchID: matchID, Status: "queued", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.items[a.ID] = a
	f.order = append(f.order, a.ID)
	return *a, nil
}

func (f *fakeStore) AnalysisByID(_ context.Context, id string) (store.Analysis, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.items[id]
	if !ok {
		return store.Analysis{}, store.ErrNotFound
	}
	return *a, nil
}

func (f *fakeStore) ClaimQueuedAnalysis(context.Context) (*store.Analysis, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		if a := f.items[id]; a.Status == "queued" {
			a.Status = "parsing"
			f.statuses = append(f.statuses, "parsing")
			c := *a
			return &c, nil
		}
	}
	return nil, nil
}

func (f *fakeStore) SetAnalysisStatus(_ context.Context, id, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[id].Status = status
	f.statuses = append(f.statuses, status)
	return nil
}

func (f *fakeStore) FinishAnalysis(_ context.Context, id string, result []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[id].Status, f.items[id].Result = "done", result
	return nil
}

func (f *fakeStore) FailAnalysis(_ context.Context, id, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[id].Status, f.items[id].Error = "failed", msg
	return nil
}

func (f *fakeStore) RequeueStuckAnalyses(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n int64
	for _, a := range f.items {
		if a.Status == "parsing" || a.Status == "analyzing" {
			a.Status = "queued"
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) CountAnalysesSince(_ context.Context, userID int64, since time.Time) (int, *time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	var oldest *time.Time
	for _, a := range f.items {
		if a.UserID == userID && a.Status != "failed" && !a.CreatedAt.Before(since) {
			n++
			if oldest == nil || a.CreatedAt.Before(*oldest) {
				t := a.CreatedAt
				oldest = &t
			}
		}
	}
	return n, oldest, nil
}

func (f *fakeStore) HasPlayerMatch(_ context.Context, accountID, matchID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.matches[[2]int64{accountID, matchID}], nil
}

func (f *fakeStore) status(id string) store.Analysis {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.items[id]
}

type fakeOD struct {
	mu        sync.Mutex
	match     *opendota.ParsedMatch
	err       error
	parsedAt  int // сколько раз спросят, прежде чем реплей «разберётся» (0 — уже разобран)
	asked     int
	requested int
}

func (f *fakeOD) ParsedMatch(context.Context, int64) (*opendota.ParsedMatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked++
	if f.err != nil {
		return nil, f.err
	}
	m := *f.match
	if f.asked <= f.parsedAt {
		m.Version = nil
	}
	return &m, nil
}

func (f *fakeOD) RequestParse(context.Context, int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requested++
	return nil
}

func (f *fakeOD) ItemConstants(context.Context) (map[string]opendota.ItemConstant, error) {
	return testCatalog().ItemByKey, nil
}

type fakeHeroes struct{}

func (fakeHeroes) Heroes() ([]dota.Hero, bool) {
	return []dota.Hero{{ID: 100, Name: "npc_dota_hero_x", LocalizedName: "Hero100"}}, true
}

type fakeLLM struct {
	mu         sync.Mutex
	configured bool
	answers    []string // ответы по порядку; закончились — повторяется последний
	err        error
	requests   []llm.Request
}

func (f *fakeLLM) Configured() bool { return f.configured }

func (f *fakeLLM) Complete(_ context.Context, r llm.Request) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r)
	if f.err != nil {
		return "", f.err
	}
	i := min(len(f.requests)-1, len(f.answers)-1)
	return f.answers[i], nil
}

type rig struct {
	svc *Service
	st  *fakeStore
	od  *fakeOD
	ai  *fakeLLM
}

func newRig(cfg Config) *rig {
	st := newStore()
	st.matches[[2]int64{111, 1}] = true
	od := &fakeOD{match: syntheticMatch()}
	ai := &fakeLLM{configured: true, answers: []string{goodAnswer}}
	cfg.Enabled = true
	if cfg.PollEvery == 0 {
		cfg.PollEvery = time.Millisecond
	}
	if cfg.Tick == 0 {
		cfg.Tick = 5 * time.Millisecond
	}
	return &rig{
		svc: New(Deps{Store: st, OpenDota: od, LLM: ai, Heroes: fakeHeroes{}, Config: cfg, Logger: quiet,
			Example: json.RawMessage(`{"summary":{"verdict":"пример"}}`)}),
		st: st, od: od, ai: ai,
	}
}

func (r *rig) user() store.User { return store.User{ID: 1, AccountID: 111} }

// run прогоняет заказ через воркер и ждёт конечного статуса.
func (r *rig) run(t *testing.T, id string) store.Analysis {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.svc.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	deadline := time.After(5 * time.Second)
	for {
		if a := r.st.status(id); a.Status == "done" || a.Status == "failed" {
			return a
		}
		select {
		case <-deadline:
			t.Fatalf("analysis did not finish, status %q", r.st.status(id).Status)
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func TestCreateChecks(t *testing.T) {
	ctx := context.Background()

	r := newRig(Config{})
	r.ai.configured = false
	if _, err := r.svc.Create(ctx, r.user(), 1); !errors.Is(err, ErrUnavailable) {
		t.Errorf("model not configured: %v", err)
	}

	r = newRig(Config{})
	if _, err := r.svc.Create(ctx, r.user(), 999); !errors.Is(err, ErrNotYourMatch) {
		t.Errorf("foreign match: %v", err)
	}

	// отслеживаемый профиль важнее собственного аккаунта
	r = newRig(Config{})
	tracked := int64(555)
	u := r.user()
	u.TrackedAccountID = &tracked
	if _, err := r.svc.Create(ctx, u, 1); !errors.Is(err, ErrNotYourMatch) {
		t.Errorf("match of another profile than tracked: %v", err)
	}
	r.st.matches[[2]int64{555, 1}] = true
	v, err := r.svc.Create(ctx, u, 1)
	if err != nil || v.Status != "queued" {
		t.Fatalf("tracked profile match: %v %+v", err, v)
	}
	if r.st.status(v.ID).AccountID != 555 {
		t.Error("analysis must be made for the tracked account")
	}

	// повторный заказ того же матча отдаёт существующий разбор
	var exists *ExistsError
	if _, err := r.svc.Create(ctx, u, 1); !errors.As(err, &exists) || exists.ID != v.ID {
		t.Errorf("duplicate: %v", err)
	}
}

func TestQuota(t *testing.T) {
	ctx := context.Background()
	r := newRig(Config{PerDay: 2})
	for m := int64(1); m <= 3; m++ {
		r.st.matches[[2]int64{111, m}] = true
	}
	for m := int64(1); m <= 2; m++ {
		if _, err := r.svc.Create(ctx, r.user(), m); err != nil {
			t.Fatal(err)
		}
	}
	q, _ := r.svc.Quota(ctx, 1)
	if q.Left != 0 || q.PerDay != 2 || q.ResetAt == nil || q.ResetAt.Before(time.Now().Add(23*time.Hour)) {
		t.Errorf("exhausted quota: %+v", q)
	}
	var qe *QuotaError
	if _, err := r.svc.Create(ctx, r.user(), 3); !errors.As(err, &qe) {
		t.Fatalf("third analysis must hit the limit: %v", err)
	}
	// отказ по лимиту не оставляет заказ в очереди
	if q2, _ := r.svc.Quota(ctx, 1); q2.Left != 0 {
		t.Errorf("rejected order must not use quota or queue: %+v", q2)
	}
	if a, _ := r.st.ClaimQueuedAnalysis(ctx); a == nil || a.MatchID == 3 {
		t.Errorf("rejected order must not be queued: %+v", a)
	}

	// неудавшийся разбор лимит не тратит
	r2 := newRig(Config{PerDay: 1})
	v, _ := r2.svc.Create(ctx, r2.user(), 1)
	_ = r2.st.FailAnalysis(ctx, v.ID, "ошибка")
	if q3, _ := r2.svc.Quota(ctx, 1); q3.Left != 1 {
		t.Errorf("failed analysis must be refunded: %+v", q3)
	}
	if _, err := r2.svc.Create(ctx, r2.user(), 1); err != nil {
		t.Errorf("retry after failure: %v", err)
	}
}

func TestGetOnlyForOwner(t *testing.T) {
	ctx := context.Background()
	r := newRig(Config{})
	v, _ := r.svc.Create(ctx, r.user(), 1)
	if got, err := r.svc.Get(ctx, 1, v.ID); err != nil || got.ID != v.ID || len(got.Result) != 0 {
		t.Errorf("owner: %v %+v", err, got)
	}
	if _, err := r.svc.Get(ctx, 2, v.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("someone else's analysis: %v", err)
	}
	if _, err := r.svc.Get(ctx, 1, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id: %v", err)
	}
	if ex, ok := r.svc.ExampleView(); !ok || ex.ID != "example" || ex.Status != "done" || len(ex.Result) == 0 {
		t.Errorf("example: %+v %v", ex, ok)
	}
}

func TestProcessSuccess(t *testing.T) {
	ctx := context.Background()
	r := newRig(Config{})
	v, _ := r.svc.Create(ctx, r.user(), 1)
	a := r.run(t, v.ID)
	if a.Status != "done" {
		t.Fatalf("status %q, error %q", a.Status, a.Error)
	}
	var res Result
	if err := json.Unmarshal(a.Result, &res); err != nil {
		t.Fatal(err)
	}
	if res.Summary.Rating != 7 || res.Lanes.GoldDiff10 != 410 || len(res.Episodes) != 2 {
		t.Errorf("result: %+v", res)
	}
	// стадии: очередь → чтение реплея → анализ
	if got := strings.Join(r.st.statuses, ","); got != "parsing,analyzing" {
		t.Errorf("stages: %s", got)
	}
	if len(r.ai.requests) != 1 || !strings.Contains(r.ai.requests[0].User, `"position_guess":1`) || !r.ai.requests[0].JSON {
		t.Errorf("prompt data: %+v", r.ai.requests)
	}
	if r.od.requested != 0 {
		t.Error("parsed replay must not be requested again")
	}
	// после готовности результат отдаётся владельцу
	if got, _ := r.svc.Get(ctx, 1, v.ID); len(got.Result) == 0 || got.Status != "done" {
		t.Errorf("view: %+v", got)
	}
}

func TestProcessWaitsForReplayParsing(t *testing.T) {
	ctx := context.Background()
	r := newRig(Config{})
	r.od.parsedAt = 4 // реплей «разберётся» на пятом вопросе
	v, _ := r.svc.Create(ctx, r.user(), 1)
	a := r.run(t, v.ID)
	if a.Status != "done" {
		t.Fatalf("status %q, error %q", a.Status, a.Error)
	}
	if r.od.requested != 1 || r.od.asked < 5 {
		t.Errorf("parse must be requested once and polled: requested=%d asked=%d", r.od.requested, r.od.asked)
	}
}

func TestProcessReplayErrors(t *testing.T) {
	ctx := context.Background()

	// слишком старый нераспарсенный матч — ждать бессмысленно
	r := newRig(Config{})
	r.od.match.StartTime = time.Now().Add(-30 * 24 * time.Hour).Unix()
	r.od.parsedAt = 1 << 30
	v, _ := r.svc.Create(ctx, r.user(), 1)
	if a := r.run(t, v.ID); a.Status != "failed" || a.Error != msgReplayGone || r.od.requested != 0 {
		t.Errorf("old replay: %q %q requested=%d", a.Status, a.Error, r.od.requested)
	}

	// реплей так и не разобрался
	r = newRig(Config{ParseTimeout: 30 * time.Millisecond})
	r.od.parsedAt = 1 << 30
	v, _ = r.svc.Create(ctx, r.user(), 1)
	if a := r.run(t, v.ID); a.Status != "failed" || a.Error != msgParseTimeout {
		t.Errorf("parse timeout: %q %q", a.Status, a.Error)
	}

	// матча нет в OpenDota / OpenDota недоступна
	for err, want := range map[error]string{opendota.ErrNotFound: msgMatchNotFound, errors.New("boom"): msgOpenDota} {
		r = newRig(Config{})
		r.od.err = err
		v, _ = r.svc.Create(ctx, r.user(), 1)
		if a := r.run(t, v.ID); a.Status != "failed" || a.Error != want {
			t.Errorf("%v: %q %q", err, a.Status, a.Error)
		}
	}

	// игрока нет в матче (скрытый профиль)
	r = newRig(Config{})
	r.od.match.Players[0].AccountID = nil
	v, _ = r.svc.Create(ctx, r.user(), 1)
	if a := r.run(t, v.ID); a.Status != "failed" || a.Error != msgNotInMatch {
		t.Errorf("player not in match: %q %q", a.Status, a.Error)
	}
}

func TestProcessModelErrors(t *testing.T) {
	ctx := context.Background()

	// первый ответ плохой — второй раз модель получает подсказку и отвечает верно
	r := newRig(Config{})
	r.ai.answers = []string{"извини, не могу", goodAnswer}
	v, _ := r.svc.Create(ctx, r.user(), 1)
	if a := r.run(t, v.ID); a.Status != "done" {
		t.Fatalf("retry must succeed: %q %q", a.Status, a.Error)
	}
	if len(r.ai.requests) != 2 || !strings.Contains(r.ai.requests[1].User, "Прошлый ответ не прошёл проверку") {
		t.Errorf("second request must carry a hint: %+v", r.ai.requests)
	}

	// оба ответа плохие
	r = newRig(Config{})
	r.ai.answers = []string{"нет", "и снова нет"}
	v, _ = r.svc.Create(ctx, r.user(), 1)
	if a := r.run(t, v.ID); a.Status != "failed" || a.Error != msgBadAnswer || len(r.ai.requests) != 2 {
		t.Errorf("two bad answers: %q %q requests=%d", a.Status, a.Error, len(r.ai.requests))
	}

	// отказ по ключу и перегрузка сервиса
	for err, want := range map[error]string{llm.ErrAuth: msgAuth, llm.ErrUnavailable: msgBusy} {
		r = newRig(Config{})
		r.ai.err = fmt.Errorf("wrapped: %w", err)
		v, _ = r.svc.Create(ctx, r.user(), 1)
		a := r.run(t, v.ID)
		if a.Status != "failed" || a.Error != want {
			t.Errorf("%v: %q %q", err, a.Status, a.Error)
		}
		if len(r.ai.requests) != 1 {
			t.Errorf("%v: provider errors must not be retried here (the client already tried every model): %d", err, len(r.ai.requests))
		}
		// неудача не тратит дневной лимит
		if q, _ := r.svc.Quota(ctx, 1); q.Left != q.PerDay {
			t.Errorf("failed analysis must not use quota: %+v", q)
		}
	}
}

func TestRunRequeuesStuckAnalyses(t *testing.T) {
	ctx := context.Background()
	r := newRig(Config{})
	v, _ := r.svc.Create(ctx, r.user(), 1)
	// сервер «упал» посреди разбора
	if _, err := r.st.ClaimQueuedAnalysis(ctx); err != nil {
		t.Fatal(err)
	}
	_ = r.st.SetAnalysisStatus(ctx, v.ID, "analyzing")
	if a := r.run(t, v.ID); a.Status != "done" {
		t.Errorf("stuck analysis must be finished after restart: %q %q", a.Status, a.Error)
	}
}

func TestRunProcessesQueueInParallelWithLimit(t *testing.T) {
	ctx := context.Background()
	r := newRig(Config{MaxParallel: 2})
	var ids []string
	for m := int64(10); m < 15; m++ {
		r.st.matches[[2]int64{111, m}] = true
		r.od.match.MatchID = m
	}
	r.svc.cfg.PerDay = 10
	for m := int64(10); m < 15; m++ {
		v, err := r.svc.Create(ctx, r.user(), m)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, v.ID)
	}
	for _, id := range ids {
		if a := r.run(t, id); a.Status != "done" {
			t.Errorf("%s: %q %q", id, a.Status, a.Error)
		}
	}
}
