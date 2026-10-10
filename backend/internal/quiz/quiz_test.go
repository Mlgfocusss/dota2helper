package quiz

import (
	"context"
	"errors"
	"testing"

	"d2helper/internal/dota"
	"d2helper/internal/meta"
)

const dataDir = "../../data"

func load(t *testing.T) (*Data, *dota.HeroTagSet) {
	t.Helper()
	d, err := Load(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := dota.LoadHeroTags(dataDir + "/heroes_tags.json")
	if err != nil {
		t.Fatal(err)
	}
	return d, tags
}

func TestLoadRealData(t *testing.T) {
	d, _ := load(t)
	if len(d.Position.Questions) != 5 {
		t.Errorf("вопросов позиции: %d", len(d.Position.Questions))
	}
	for pos := 1; pos <= 5; pos++ {
		qs, err := d.HeroQuestionsForClient(pos)
		if err != nil || len(qs) != 4 {
			t.Errorf("позиция %d: %d вопросов, %v", pos, len(qs), err)
		}
	}
	if _, err := d.HeroQuestionsForClient(6); !errors.Is(err, ErrBadPosition) {
		t.Errorf("позиция 6: %v", err)
	}
}

func TestClientViewHasNoWeights(t *testing.T) {
	d, _ := load(t)
	for _, q := range d.PositionForClient().Questions {
		for _, o := range q.Options {
			if o.Weights != nil {
				t.Fatal("веса не должны уходить клиенту")
			}
		}
	}
	qs, _ := d.HeroQuestionsForClient(1)
	for _, q := range qs {
		for _, o := range q.Options {
			if o.Tags != nil || o.ComplexityPref != "" {
				t.Fatal("теги не должны уходить клиенту")
			}
		}
	}
}

func answersAll(option string) map[string]string {
	return map[string]string{"q1": option, "q2": option, "q3": option, "q4": option, "q5": option}
}

func TestScorePosition(t *testing.T) {
	d, _ := load(t)

	// все «a»: 1 (3+3+3+3+3=15) против 2 (1+0+1+0+0)
	res, err := d.ScorePosition(answersAll("a"))
	if err != nil || res.Primary != 1 || res.Secondary != nil {
		t.Fatalf("все a: %+v %v", res, err)
	}
	// все «d»: пятёрка 12 очков, роумер 6 (50% < 60%) — дополнительных задатков нет
	res, _ = d.ScorePosition(answersAll("d"))
	if res.Primary != 5 || res.Secondary != nil {
		t.Errorf("все d: %+v", res)
	}
	var sum float64
	for _, s := range res.Shares {
		sum += s
	}
	if sum < 99 || sum > 101 {
		t.Errorf("сумма долей = %v", sum)
	}
}

func TestScorePositionTieUsesFirstAnswer(t *testing.T) {
	d, _ := load(t)
	// q1=c (p3:3,p4:1), q2=c (p3:3,p4:1), q3=b (p4:2,p3:1,p5:1), q4=d (p5:3,p4:1), q5=d (p5:2,p4:2)
	// p3 = 3+3+1+0+0 = 7; p4 = 1+1+2+1+2 = 7; p5 = 0+0+1+3+2 = 6 → ничья 3 и 4, в q1 у p3 вес больше
	res, err := d.ScorePosition(map[string]string{"q1": "c", "q2": "c", "q3": "b", "q4": "d", "q5": "d"})
	if err != nil || res.Primary != 3 {
		t.Errorf("ничья: %+v %v", res, err)
	}
	if res.Secondary == nil || *res.Secondary != 4 {
		t.Errorf("задатки: %+v", res.Secondary)
	}
}

func TestScorePositionRejectsBadAnswers(t *testing.T) {
	d, _ := load(t)
	bad := []map[string]string{
		{},
		{"q1": "a"},
		{"q1": "a", "q2": "a", "q3": "a", "q4": "a", "q5": "z"},
		{"q1": "a", "q2": "a", "q3": "a", "q4": "a", "q5": "a", "extra": "a"},
	}
	for _, a := range bad {
		if _, err := d.ScorePosition(a); !errors.Is(err, ErrBadAnswers) {
			t.Errorf("%v: %v", a, err)
		}
	}
}

type fakeMeta struct{ snap *meta.Snapshot }

func (f fakeMeta) Snapshot(context.Context) (*meta.Snapshot, error) {
	if f.snap == nil {
		return nil, meta.ErrNotReady
	}
	return f.snap, nil
}

func TestRecommendHeroes(t *testing.T) {
	d, tags := load(t)
	ctx := context.Background()

	// позиция 4: зависимость от уровня, контроль, любым сложным/простым, в середине игры
	answers := map[string][]string{
		"roam1": {"a"}, "roam2": {"a"}, "complexity": {"high", "low"}, "timing": {"mid"},
	}
	picks, err := d.RecommendHeroes(ctx, 4, answers, tags, fakeMeta{})
	if err != nil || len(picks) != 3 {
		t.Fatalf("%v %v", picks, err)
	}
	for _, p := range picks {
		h, _ := tags.Get(p.HeroID)
		onPos := false
		for _, pos := range h.Positions {
			onPos = onPos || pos == 4
		}
		if !onPos {
			t.Errorf("герой %d не играется на позиции 4", p.HeroID)
		}
		if len(p.Matched) == 0 {
			t.Errorf("у героя %d нет совпавших тегов", p.HeroID)
		}
	}
	if picks[0].Score < picks[1].Score || picks[1].Score < picks[2].Score {
		t.Errorf("порядок по очкам: %+v", picks)
	}
}

func TestRecommendComplexityPreference(t *testing.T) {
	d, tags := load(t)
	ctx := context.Background()
	base := map[string][]string{"carry1": {"a"}, "carry2": {"a"}, "timing": {"late"}}

	easy := map[string][]string{"carry1": base["carry1"], "carry2": base["carry2"], "timing": base["timing"], "complexity": {"low"}}
	picks, err := d.RecommendHeroes(ctx, 1, easy, tags, fakeMeta{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range picks {
		if p.Complexity == 3 {
			t.Errorf("при «хочу попроще» не должно быть сложных героев в топе: %+v", p)
		}
	}
}

func TestRecommendUsesMetaBonus(t *testing.T) {
	d, tags := load(t)
	ctx := context.Background()
	answers := map[string][]string{"roam1": {"a"}, "roam2": {"a"}, "complexity": {"high", "low"}, "timing": {"mid"}}

	plain, _ := d.RecommendHeroes(ctx, 4, answers, tags, fakeMeta{})
	last := plain[2]

	// герою, оказавшемуся в топе с минимальным числом очков, даём огромный винрейт — он должен подняться
	snap := &meta.Snapshot{}
	snap.ByPosition[3] = []meta.HeroStat{{HeroID: last.HeroID, Winrate: 0.9, Matches: 5000}}
	boosted, _ := d.RecommendHeroes(ctx, 4, answers, tags, fakeMeta{snap: snap})
	var found *HeroPick
	for i := range boosted {
		if boosted[i].HeroID == last.HeroID {
			found = &boosted[i]
		}
	}
	if found == nil || found.Winrate == nil || found.Score-last.Score != metaBonusCap {
		t.Errorf("бонус меты ограничен ±3: %+v (было %v)", found, last.Score)
	}
}

func TestRecommendRejectsBadAnswers(t *testing.T) {
	d, tags := load(t)
	ctx := context.Background()
	good := map[string][]string{"roam1": {"a"}, "roam2": {"a"}, "complexity": {"low"}, "timing": {"mid"}}

	if _, err := d.RecommendHeroes(ctx, 9, good, tags, fakeMeta{}); !errors.Is(err, ErrBadPosition) {
		t.Errorf("позиция: %v", err)
	}
	cases := []map[string][]string{
		{},
		{"roam1": {"a"}, "roam2": {"a"}, "complexity": {"low"}},
		{"roam1": {"a"}, "roam2": {"zzz"}, "complexity": {"low"}, "timing": {"mid"}},
		{"roam1": {"a", "a"}, "roam2": {"a"}, "complexity": {"low"}, "timing": {"mid"}},
		{"roam1": {"a", "b", "c"}, "roam2": {"a"}, "complexity": {"low"}, "timing": {"mid"}},
		{"roam1": {}, "roam2": {"a"}, "complexity": {"low"}, "timing": {"mid"}},
		{"carry1": {"a"}, "carry2": {"a"}, "complexity": {"low"}, "timing": {"mid"}}, // вопросы чужой позиции
	}
	for _, c := range cases {
		if _, err := d.RecommendHeroes(ctx, 4, c, tags, fakeMeta{}); !errors.Is(err, ErrBadAnswers) {
			t.Errorf("%v: %v", c, err)
		}
	}
}

func TestTopMetaHeroesMarksDifficulty(t *testing.T) {
	_, tags := load(t)
	snap := &meta.Snapshot{}
	snap.ByPosition[0] = []meta.HeroStat{{HeroID: 1, Winrate: 0.55}, {HeroID: 2, Winrate: 0.54}, {HeroID: 3, Winrate: 0.53}}
	got := TopMetaHeroes(context.Background(), fakeMeta{snap: snap}, tags, 1, 2)
	if len(got) != 2 || got[0].HeroID != 1 {
		t.Errorf("%+v", got)
	}
	if got := TopMetaHeroes(context.Background(), fakeMeta{}, tags, 1, 2); len(got) != 0 {
		t.Errorf("без меты список пуст: %+v", got)
	}
}
