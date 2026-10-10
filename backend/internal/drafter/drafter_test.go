package drafter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"testing"
	"time"

	"d2helper/internal/dota"
	"d2helper/internal/matchups"
	"d2helper/internal/meta"
	"d2helper/internal/store"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// matrix: герои 1..12. vs(a,b) = a − b (антисимметрично), with(a,b) = 1 для соседей по id.
func testMatrix(withSynergy bool) *matchups.Matrix {
	var ids []int
	for i := 1; i <= 12; i++ {
		ids = append(ids, i)
	}
	var rows []store.MatchupRow
	for _, a := range ids {
		for _, b := range ids {
			if a == b {
				continue
			}
			rows = append(rows, store.MatchupRow{HeroID: a, OtherID: b, Kind: "vs", Matches: 500, Advantage: float32(a - b)})
			if withSynergy && (a-b == 1 || b-a == 1) {
				rows = append(rows, store.MatchupRow{HeroID: a, OtherID: b, Kind: "with", Matches: 500, Advantage: 1})
			}
		}
	}
	return matchups.BuildMatrix(ids, rows, time.Now())
}

func slots(ids ...int) []Slot {
	out := make([]Slot, len(ids))
	for i, id := range ids {
		out[i] = Slot{HeroID: id, Position: i + 1}
	}
	return out
}

func TestAnalyze(t *testing.T) {
	m := testMatrix(true)
	a := Analyze(m, slots(10, 11), slots(1, 2))

	// герой 10: vs(10,1)+vs(10,2) = 9+8 = 17, синергия 0.5×with(10,11) = 0.5
	if got := a.Radiant.Heroes[0]; got.HeroID != 10 || got.Score != 17.5 {
		t.Errorf("hero 10 score: %+v", got)
	}
	// герой 11: 10+9 + 0.5 = 19.5
	if got := a.Radiant.Heroes[1].Score; got != 19.5 {
		t.Errorf("hero 11 score: %v", got)
	}
	if a.Radiant.Total != 37 {
		t.Errorf("radiant total: %v", a.Radiant.Total)
	}
	// герои 1, 2: −9−10 +0.5 и −8−9 +0.5
	if a.Dire.Total != -(17+19)+1 {
		t.Errorf("dire total: %v", a.Dire.Total)
	}
	if a.BestPair == nil || a.BestPair.Value != 1 {
		t.Errorf("best pair: %+v", a.BestPair)
	}
	if a.BestMatchup == nil || a.BestMatchup.A != 11 || a.BestMatchup.B != 1 || a.BestMatchup.Value != 10 {
		t.Errorf("best matchup: %+v", a.BestMatchup)
	}
	if a.WorstMatchup == nil || a.WorstMatchup.A != 1 || a.WorstMatchup.B != 11 || a.WorstMatchup.Value != -10 {
		t.Errorf("worst matchup: %+v", a.WorstMatchup)
	}
}

func TestAnalyzeWithoutSynergyHidesBestPair(t *testing.T) {
	a := Analyze(testMatrix(false), slots(10, 11), slots(1))
	if a.BestPair != nil {
		t.Errorf("no STRATZ synergy → no best pair, got %+v", a.BestPair)
	}
	if a.Radiant.Heroes[0].Score != 9 { // только vs
		t.Errorf("score without synergy: %v", a.Radiant.Heroes[0].Score)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	a := Analyze(testMatrix(true), nil, nil)
	if a.Radiant.Total != 0 || a.BestMatchup != nil || a.WorstMatchup != nil || len(a.Radiant.Heroes) != 0 {
		t.Errorf("empty draft: %+v", a)
	}
	// одна команда без врагов: оценка = только синергия
	a = Analyze(testMatrix(true), slots(5, 6), nil)
	if a.Radiant.Heroes[0].Score != 0.5 || a.BestMatchup != nil {
		t.Errorf("no enemies: %+v", a)
	}
}

type fakeTags struct{ byPos map[int][]int }

func (f fakeTags) Get(int) (dota.HeroTags, bool) { return dota.HeroTags{}, false }
func (f fakeTags) HeroesForPosition(pos int) []int {
	return f.byPos[pos]
}

func snapshot(source meta.Source, byPos [5][]meta.HeroStat) *meta.Snapshot {
	return &meta.Snapshot{Source: source, ByPosition: byPos}
}

func TestBuildPositions(t *testing.T) {
	tags := fakeTags{byPos: map[int][]int{1: {1, 2}, 2: {3}, 3: {4}, 4: {5}, 5: {6}}}
	snap := snapshot(meta.SourceStratz, [5][]meta.HeroStat{{{HeroID: 7, Winrate: 0.55, Matches: 900}}})

	pd := BuildPositions(snap, tags)
	if !pd.IsPlayable(7, 1) || pd.Winrate[0][7] != 0.55 || pd.Weight[0][7] != 900 {
		t.Errorf("stratz position data: %+v", pd.Playable[0])
	}
	if pd.IsPlayable(1, 1) {
		t.Error("with STRATZ meta, tags must not widen the pool")
	}

	// мета без реальных позиций (OpenDota) — позиции из тегов тоже годятся
	pd = BuildPositions(snapshot(meta.SourceOpenDota, [5][]meta.HeroStat{}), tags)
	if !pd.IsPlayable(1, 1) || !pd.IsPlayable(2, 1) || pd.IsPlayable(1, 2) {
		t.Errorf("tags fallback: %+v", pd.Playable)
	}
	// меты нет вообще
	if pd = BuildPositions(nil, tags); !pd.IsPlayable(6, 5) {
		t.Error("nil snapshot should fall back to tags")
	}
	if pd.IsPlayable(1, 0) || pd.IsPlayable(1, 6) {
		t.Error("out-of-range positions are never playable")
	}
}

func pool() PositionData {
	var pd PositionData
	for i := range pd.Playable {
		pd.Playable[i] = map[int]bool{}
		pd.Winrate[i] = map[int]float64{}
		pd.Weight[i] = map[int]int{}
	}
	// у героев 1–3 общий пул на всех позициях — проверяем, что повторов нет
	for pos := 0; pos < 5; pos++ {
		for _, id := range []int{1, 2, 3, 4, 5, 6, 7} {
			pd.Playable[pos][id] = true
			pd.Weight[pos][id] = id * 100
		}
	}
	return pd
}

func TestGenerateEnemies(t *testing.T) {
	pd := pool()
	for seed := range uint64(200) {
		enemies, err := GenerateEnemies(pd, rand.New(rand.NewPCG(seed, 1)))
		if err != nil {
			t.Fatal(err)
		}
		if len(enemies) != 5 {
			t.Fatalf("want 5 enemies, got %d", len(enemies))
		}
		seen := map[int]bool{}
		for i, e := range enemies {
			if e.Position != i+1 || seen[e.HeroID] || !pd.IsPlayable(e.HeroID, e.Position) {
				t.Fatalf("seed %d: bad enemies %+v", seed, enemies)
			}
			seen[e.HeroID] = true
		}
	}
}

func TestGenerateEnemiesWeightsByPopularity(t *testing.T) {
	pd := pool()
	// на позиции 1 только герои 1 и 7 (вес 100 и 700), остальные позиции их не трогают
	pd.Playable[0] = map[int]bool{1: true, 7: true}
	for pos := 1; pos < 5; pos++ {
		pd.Playable[pos] = map[int]bool{2: true, 3: true, 4: true, 5: true, 6: true}
	}
	counts := map[int]int{}
	rnd := rand.New(rand.NewPCG(5, 5))
	for range 2000 {
		enemies, err := GenerateEnemies(pd, rnd)
		if err != nil {
			t.Fatal(err)
		}
		counts[enemies[0].HeroID]++
	}
	// ожидаем ~7:1 в пользу популярного героя
	if counts[7] < 5*counts[1] {
		t.Errorf("popular hero should be picked far more often: %v", counts)
	}
}

func TestGenerateEnemiesEmptyPool(t *testing.T) {
	pd := pool()
	pd.Playable[2] = map[int]bool{}
	if _, err := GenerateEnemies(pd, rand.New(rand.NewPCG(1, 1))); err == nil {
		t.Error("empty pool must be an error")
	}
}

func TestScoreTeam(t *testing.T) {
	m := testMatrix(true)
	pd := pool()
	pd.Winrate[0][10] = 0.6 // герой 10 на позиции 1: +0.1 к винрейту
	pd.Playable[0][10] = true
	for pos := 0; pos < 5; pos++ {
		for _, id := range []int{8, 9, 10, 11, 12} {
			pd.Playable[pos][id] = true
		}
	}
	c := Coefficients{Scale: 1, MetaWeight: 10, OffPosPenalty: 3}
	enemies := slots(1, 2, 3, 4, 5)

	score, bd := ScoreTeam(m, pd, c, slots(8, 9, 10, 11, 12), enemies)
	// vs: Σ_h Σ_e (h − e) = 5×(8+9+10+11+12) − 5×(1+2+3+4+5) = 250 − 75 = 175
	// слишком большой разрыв — счёт упирается в 100, но разбор считается без обрезки
	if score != 100 {
		t.Errorf("huge advantage must clamp to 100, got %v", score)
	}
	if bd.Contrib.VS != 175 {
		t.Errorf("vs contribution: %v", bd.Contrib.VS)
	}
	// синергия: пары соседей (8,9),(9,10),(10,11),(11,12) → у каждого героя 0.5×(число соседей)
	// соседей всего 8 «в обе стороны» → 0.5×8 = 4
	if bd.Contrib.Synergy != 4 {
		t.Errorf("synergy contribution: %v", bd.Contrib.Synergy)
	}
	// мета: герой 10 на позиции 3 без данных → 0.5 → 0; винрейт 0.6 есть только для позиции 1
	if bd.Contrib.Meta != 0 || bd.OffPosition != 0 {
		t.Errorf("meta/off: %+v", bd)
	}

	// герой 10 на позиции 1 даёт +10×0.1 = +1 к мете
	_, bd = ScoreTeam(m, pd, c, []Slot{{10, 1}, {8, 2}, {9, 3}, {11, 4}, {12, 5}}, enemies)
	if bd.Contrib.Meta != 1 {
		t.Errorf("meta contribution: %v", bd.Contrib.Meta)
	}

	// герой вне пула позиции: штраф за каждого
	pd.Playable[1] = map[int]bool{}
	_, bd = ScoreTeam(m, pd, c, slots(8, 9, 10, 11, 12), enemies)
	if bd.OffPosition != 1 || bd.Contrib.Penalty != 3 || !bd.Heroes[1].OffPosition {
		t.Errorf("off-position penalty: %+v", bd)
	}
}

func TestScoreTeamBalanced(t *testing.T) {
	// равные команды дают около 50
	m := testMatrix(false)
	pd := pool()
	c := Coefficients{Scale: 1, MetaWeight: 10, OffPosPenalty: 3}
	score, _ := ScoreTeam(m, pd, c, slots(1, 2, 3, 4, 5), slots(1, 2, 3, 4, 5))
	if score != 50 {
		t.Errorf("mirror draft should score 50, got %v", score)
	}
	// заведомо проигрышный состав — счёт ограничен снизу нулём
	score, _ = ScoreTeam(m, pd, Coefficients{Scale: 5, MetaWeight: 10, OffPosPenalty: 3}, slots(1, 2, 3, 4, 5), slots(8, 9, 10, 11, 12))
	if score != 0 {
		t.Errorf("hopeless draft should clamp to 0, got %v", score)
	}
}

func TestValidateTeam(t *testing.T) {
	m := testMatrix(false)
	enemies := slots(1, 2, 3, 4, 5)
	good := slots(6, 7, 8, 9, 10)
	if err := validateTeam(m, good, enemies); err != nil {
		t.Fatal(err)
	}
	bad := map[string][]Slot{
		"too few":        good[:4],
		"enemy hero":     {{6, 1}, {7, 2}, {8, 3}, {9, 4}, {1, 5}},
		"duplicate hero": {{6, 1}, {7, 2}, {8, 3}, {9, 4}, {6, 5}},
		"dup position":   {{6, 1}, {7, 1}, {8, 3}, {9, 4}, {10, 5}},
		"bad position":   {{6, 1}, {7, 2}, {8, 3}, {9, 4}, {10, 6}},
		"unknown hero":   {{6, 1}, {7, 2}, {8, 3}, {9, 4}, {99, 5}},
	}
	for name, team := range bad {
		if err := validateTeam(m, team, enemies); !errors.Is(err, ErrBadTeam) {
			t.Errorf("%s: want ErrBadTeam, got %v", name, err)
		}
	}
}

func TestMatrixPayload(t *testing.T) {
	m := testMatrix(true)
	m.VS[0][1] = 3.14159
	m.VS[0][2] = -0.04
	body := encodeMatrix(m, pool())
	var got struct {
		IDs      []int       `json:"ids"`
		VS       [][]float64 `json:"vs"`
		With     [][]float64 `json:"with"`
		Playable [][]int     `json:"playable"`
		HasWith  bool        `json:"has_with"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, body)
	}
	if len(got.IDs) != 12 || len(got.VS) != 12 || len(got.VS[0]) != 12 || !got.HasWith || len(got.Playable) != 5 {
		t.Fatalf("shape: ids=%d vs=%d has_with=%v playable=%d", len(got.IDs), len(got.VS), got.HasWith, len(got.Playable))
	}
	if got.VS[0][1] != 3.1 || got.VS[0][2] != 0 {
		t.Errorf("rounding to tenths: %v %v", got.VS[0][1], got.VS[0][2])
	}

	// без синергий поле with — null: не гоняем лишние килобайты
	var noWith struct {
		With    any  `json:"with"`
		HasWith bool `json:"has_with"`
	}
	if err := json.Unmarshal(encodeMatrix(testMatrix(false), pool()), &noWith); err != nil || noWith.With != nil || noWith.HasWith {
		t.Errorf("no synergy payload: %+v %v", noWith, err)
	}
}

func TestRankScores(t *testing.T) {
	rows := []store.MonthUserScores{
		{UserID: 1, PersonaName: "a", Scores: []float64{90, 80, 70, 60, 50, 40, 30}},      // топ‑5 = 350
		{UserID: 2, PersonaName: "b", Scores: []float64{99}},                              // всего 99
		{UserID: 3, PersonaName: "c", Scores: []float64{70, 70, 70, 70, 70}},              // 350, как у первого
		{UserID: 4, PersonaName: "d", Scores: []float64{50, 40}, AvatarURL: "http://x/y"}, // 90
		{UserID: 5, PersonaName: "e", Scores: []float64{10, 99.94, 20, 30, 40, 50, 60}},   // 99.94+60+50+40+30 = 279.9 (округление)
	}
	got := RankScores(rows)
	order := []int64{1, 3, 5, 2, 4}
	for i, id := range order {
		if got[i].UserID != id {
			t.Fatalf("order at %d: want user %d, got %d (%+v)", i, id, got[i].UserID, got)
		}
	}
	// у первого и третьего равный счёт — место общее, у первого больше тренировок (выше в списке)
	if got[0].Rank != 1 || got[1].Rank != 1 || got[2].Rank != 3 {
		t.Errorf("ranks: %d %d %d", got[0].Rank, got[1].Rank, got[2].Rank)
	}
	if got[0].Score != 350 || got[0].Drafts != 7 {
		t.Errorf("score/drafts: %+v", got[0])
	}
	if got[2].Score != 279.9 {
		t.Errorf("rounding: %v", got[2].Score)
	}
	if got[4].Avatar == nil || *got[4].Avatar != "http://x/y" || got[0].Avatar != nil {
		t.Errorf("avatars: %+v %+v", got[4].Avatar, got[0].Avatar)
	}
	// входной порядок оценок не важен
	if RankScores([]store.MonthUserScores{{UserID: 9, Scores: []float64{10, 90, 20}}})[0].Score != 120 {
		t.Error("scores must be sorted before summing the best five")
	}
}

func TestParseMonth(t *testing.T) {
	// 30 сентября 22:00 UTC — в Москве уже 1 октября
	now := time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)
	label, from, to, err := parseMonth("", now)
	if err != nil || label != "2026-10" || from.Day() != 1 || to.Month() != time.November {
		t.Errorf("current month: %s %v %v %v", label, from, to, err)
	}
	if got := from.UTC(); got.Day() != 30 || got.Hour() != 21 {
		t.Errorf("month start in UTC: %v", got)
	}
	if label, _, _, err = parseMonth("2026-09", now); err != nil || label != "2026-09" {
		t.Errorf("past month: %s %v", label, err)
	}
	for _, bad := range []string{"2026-11", "2024-12", "2026-13", "октябрь", "2026-9-1"} {
		if _, _, _, err := parseMonth(bad, now); !errors.Is(err, ErrBadMonth) {
			t.Errorf("%q: want ErrBadMonth, got %v", bad, err)
		}
	}
}

// ----- сервис с фейками ----------------------------------------------------------

type fakeMatrix struct{ m *matchups.Matrix }

func (f fakeMatrix) Matrix(context.Context) (*matchups.Matrix, error) {
	if f.m == nil {
		return nil, matchups.ErrNotReady
	}
	return f.m, nil
}

type fakeMeta struct{}

func (fakeMeta) Snapshot(context.Context) (*meta.Snapshot, error) { return nil, meta.ErrNotReady }

type fakeStore struct {
	trainings map[string]*store.Training
	next      int
	scores    []store.MonthUserScores
	submitted map[int64]int
}

func newFakeStore() *fakeStore {
	return &fakeStore{trainings: map[string]*store.Training{}, submitted: map[int64]int{}}
}

func (f *fakeStore) CreateTraining(_ context.Context, userID *int64, enemies []byte) (string, error) {
	if userID != nil { // новая тренировка закрывает прежнюю открытую
		for id, t := range f.trainings {
			if t.UserID != nil && *t.UserID == *userID && t.Status == "open" {
				delete(f.trainings, id)
			}
		}
	}
	f.next++
	id := "t" + string(rune('0'+f.next))
	f.trainings[id] = &store.Training{ID: id, UserID: userID, Enemies: enemies, Status: "open", CreatedAt: time.Now()}
	return id, nil
}

func (f *fakeStore) TrainingByID(_ context.Context, id string) (store.Training, error) {
	t, ok := f.trainings[id]
	if !ok {
		return store.Training{}, store.ErrNotFound
	}
	return *t, nil
}

func (f *fakeStore) SubmitTraining(_ context.Context, id string, userID int64, team []byte, score float64, bd []byte) error {
	t, ok := f.trainings[id]
	if !ok || t.Status != "open" || (t.UserID != nil && *t.UserID != userID) {
		return store.ErrTrainingClosed
	}
	now := time.Now()
	t.UserID, t.Team, t.Score, t.Breakdown, t.Status, t.SubmittedAt = &userID, team, &score, bd, "submitted", &now
	f.submitted[userID]++
	f.scores = append(f.scores, store.MonthUserScores{UserID: userID, Scores: []float64{score}})
	return nil
}

func (f *fakeStore) CountSubmittedSince(_ context.Context, userID int64, _ time.Time) (int, error) {
	return f.submitted[userID], nil
}

func (f *fakeStore) TrainingHistory(_ context.Context, userID int64, before *time.Time, limit int) ([]store.Training, error) {
	var out []store.Training
	for _, t := range f.trainings {
		if t.Status == "submitted" && *t.UserID == userID && (before == nil || t.SubmittedAt.Before(*before)) {
			out = append(out, *t)
		}
	}
	// новые первыми
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j].SubmittedAt.After(*out[i].SubmittedAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out[:min(len(out), limit)], nil
}

func (f *fakeStore) MonthScores(context.Context, time.Time, time.Time) ([]store.MonthUserScores, error) {
	merged := map[int64]*store.MonthUserScores{}
	for _, s := range f.scores {
		if m, ok := merged[s.UserID]; ok {
			m.Scores = append(m.Scores, s.Scores...)
		} else {
			c := s
			merged[s.UserID] = &c
		}
	}
	var out []store.MonthUserScores
	for _, m := range merged {
		out = append(out, *m)
	}
	return out, nil
}

func (f *fakeStore) DeleteStaleTrainings(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

func newService(f *fakeStore, perHour int) *Service {
	tags := fakeTags{byPos: map[int][]int{
		1: {1, 2, 3, 4, 5, 6}, 2: {1, 2, 3, 4, 5, 6}, 3: {1, 2, 3, 4, 5, 6},
		4: {1, 2, 3, 4, 5, 6}, 5: {1, 2, 3, 4, 5, 6},
	}}
	for pos := range tags.byPos { // герои 7–12 тоже годятся на всех позициях
		tags.byPos[pos] = append(tags.byPos[pos], 7, 8, 9, 10, 11, 12)
	}
	return New(Deps{
		Matrix: fakeMatrix{m: testMatrix(true)}, Meta: fakeMeta{}, Tags: tags, Store: f,
		Coef:             Coefficients{Scale: 1, MetaWeight: 10, OffPosPenalty: 3},
		TrainingsPerHour: perHour, Logger: quiet,
	})
}

func TestTrainingFlow(t *testing.T) {
	ctx := context.Background()
	f := newFakeStore()
	svc := newService(f, 60)

	// гость начинает тренировку, а оценивает уже после входа: тренировка привязывается к пользователю
	id, enemies, err := svc.NewTraining(ctx, nil)
	if err != nil || len(enemies) != 5 {
		t.Fatalf("new training: %v %v", err, enemies)
	}
	taken := map[int]bool{}
	for _, e := range enemies {
		taken[e.HeroID] = true
	}
	var team []Slot
	for hero, pos := 1, 1; pos <= 5; hero++ {
		if !taken[hero] {
			team = append(team, Slot{HeroID: hero, Position: pos})
			pos++
		}
	}

	// герой из вражеской команды недопустим
	bad := append([]Slot(nil), team...)
	bad[0].HeroID = enemies[0].HeroID
	if _, err := svc.Submit(ctx, 7, id, bad); !errors.Is(err, ErrBadTeam) {
		t.Fatalf("enemy hero in team: %v", err)
	}

	res, err := svc.Submit(ctx, 7, id, team)
	if err != nil {
		t.Fatal(err)
	}
	if res.Score < 0 || res.Score > 100 || len(res.Breakdown.Heroes) != 5 {
		t.Errorf("result: %+v", res)
	}
	if res.MonthRank == nil || *res.MonthRank != 1 {
		t.Errorf("month rank after first submit: %v", res.MonthRank)
	}
	if f.trainings[id].UserID == nil || *f.trainings[id].UserID != 7 {
		t.Error("guest training must be bound to the user")
	}

	// повторная оценка той же тренировки и чужая тренировка
	if _, err := svc.Submit(ctx, 7, id, team); !errors.Is(err, ErrTrainingClosed) {
		t.Errorf("resubmit: %v", err)
	}
	id2, _, _ := svc.NewTraining(ctx, int64p(8))
	if _, err := svc.Submit(ctx, 7, id2, team); !errors.Is(err, ErrTrainingClosed) {
		t.Errorf("foreign training: %v", err)
	}
	if _, err := svc.Submit(ctx, 7, "nope", team); !errors.Is(err, ErrTrainingMissing) {
		t.Errorf("missing training: %v", err)
	}
}

func int64p(v int64) *int64 { return &v }

func TestNewTrainingClosesPreviousOpen(t *testing.T) {
	ctx := context.Background()
	f := newFakeStore()
	svc := newService(f, 60)
	first, _, _ := svc.NewTraining(ctx, int64p(1))
	second, _, _ := svc.NewTraining(ctx, int64p(1))
	if _, ok := f.trainings[first]; ok || second == first {
		t.Error("a new training must close the previous open one")
	}
}

func TestSubmitRateLimit(t *testing.T) {
	ctx := context.Background()
	f := newFakeStore()
	svc := newService(f, 2)
	f.submitted[5] = 2 // за последний час уже оценено максимум
	id, enemies, _ := svc.NewTraining(ctx, int64p(5))
	_ = enemies
	var tooMany *TooManyError
	if _, err := svc.Submit(ctx, 5, id, nil); !errors.As(err, &tooMany) {
		t.Errorf("want TooManyError, got %v", err)
	}
}

func TestNotReady(t *testing.T) {
	svc := newService(newFakeStore(), 60)
	svc.d.Matrix = fakeMatrix{}
	if _, _, err := svc.NewTraining(context.Background(), nil); !errors.Is(err, ErrNotReady) {
		t.Errorf("new training: %v", err)
	}
	if _, _, err := svc.MatrixPayload(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Errorf("matrix payload: %v", err)
	}
	if _, err := svc.Analyze(context.Background(), nil, nil, nil); !errors.Is(err, ErrNotReady) {
		t.Errorf("analyze: %v", err)
	}
}

func TestAnalyzeValidation(t *testing.T) {
	svc := newService(newFakeStore(), 60)
	ctx := context.Background()
	if _, err := svc.Analyze(ctx, slots(1, 2), slots(3, 4), []int{5}); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		r, d []Slot
		bans []int
	}{
		"hero twice":    {slots(1, 2), slots(2), nil},
		"banned picked": {slots(1), slots(2), []int{1}},
		"unknown":       {slots(99), nil, nil},
		"too many bans": {nil, nil, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		"bad position":  {[]Slot{{1, 9}}, nil, nil},
	}
	for name, c := range cases {
		if _, err := svc.Analyze(ctx, c.r, c.d, c.bans); !errors.Is(err, ErrBadTeam) {
			t.Errorf("%s: want ErrBadTeam, got %v", name, err)
		}
	}
}

func TestHistoryPaging(t *testing.T) {
	ctx := context.Background()
	f := newFakeStore()
	svc := newService(f, 1000)
	base := time.Now().Add(-time.Hour)
	for i := range HistoryPageSize + 3 {
		ts := base.Add(time.Duration(i) * time.Minute)
		score := float64(i)
		uid := int64(1)
		id := "h" + string(rune('A'+i))
		f.trainings[id] = &store.Training{
			ID: id, UserID: &uid, Status: "submitted", SubmittedAt: &ts, Score: &score,
			Enemies: []byte(`[{"hero_id":1,"position":1}]`), Team: []byte(`[{"hero_id":2,"position":1}]`),
			Breakdown: []byte(`{"contrib":{"vs":1},"heroes":[]}`),
		}
	}
	page, err := svc.History(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != HistoryPageSize || page.NextCursor == nil {
		t.Fatalf("first page: %d items, cursor %v", len(page.Items), page.NextCursor)
	}
	if page.Items[0].Score != float64(HistoryPageSize+2) || page.Items[0].Breakdown == nil || len(page.Items[0].Team) != 1 {
		t.Errorf("newest first with details: %+v", page.Items[0])
	}
	next, err := svc.History(ctx, 1, *page.NextCursor)
	if err != nil || len(next.Items) != 3 || next.NextCursor != nil {
		t.Fatalf("second page: %v %+v", err, next)
	}
	if _, err := svc.History(ctx, 1, "abc"); !errors.Is(err, ErrBadCursor) {
		t.Errorf("bad cursor: %v", err)
	}
	empty, _ := svc.History(ctx, 99, "")
	if len(empty.Items) != 0 || empty.NextCursor != nil {
		t.Errorf("empty history: %+v", empty)
	}
}

func TestLeaderboardTop100AndMe(t *testing.T) {
	ctx := context.Background()
	f := newFakeStore()
	for i := 1; i <= 120; i++ {
		f.scores = append(f.scores, store.MonthUserScores{UserID: int64(i), Scores: []float64{float64(i)}})
	}
	svc := newService(f, 60)
	b, err := svc.Leaderboard(ctx, "", 3) // у пользователя 3 — 118-е место
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != LeaderboardSize || b.Items[0].UserID != 120 {
		t.Fatalf("top: %d items, first %d", len(b.Items), b.Items[0].UserID)
	}
	if b.Me == nil || b.Me.Rank != 118 {
		t.Errorf("own row outside top 100: %+v", b.Me)
	}
	if b, _ = svc.Leaderboard(ctx, "", 0); b.Me != nil {
		t.Error("guest has no own row")
	}
	if b, _ = svc.Leaderboard(ctx, "", 999); b.Me != nil {
		t.Error("user without trainings has no own row")
	}
	if _, err := svc.Leaderboard(ctx, "2099-01", 0); !errors.Is(err, ErrBadMonth) {
		t.Errorf("future month: %v", err)
	}
}
