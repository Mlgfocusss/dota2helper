// Package analysis — глубокий разбор матча: сбор признаков игры из реплея, запрос к языковой
// модели, проверка ответа, очередь заказов и воркер (SPEC 6).
package analysis

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Result — готовый разбор в формате, который показывает фронтенд.
type Result struct {
	Summary       Summary       `json:"summary"`
	Lanes         Lanes         `json:"lanes"`
	Episodes      []Episode     `json:"episodes"`
	Improvements  []Improvement `json:"improvements"`
	NextGamesPlan []string      `json:"next_games_plan"`
	// DeathPoints — где игрок умирал в драках (для схематичной мини-карты), координаты 0..1
	DeathPoints []Point `json:"death_points"`
}

type Summary struct {
	Verdict  string    `json:"verdict"`
	Rating   int       `json:"rating"`
	KeyStats []KeyStat `json:"key_stats"`
}

type KeyStat struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type Lanes struct {
	Text       string `json:"text"`
	LH10       int    `json:"lh_10"`
	DN10       int    `json:"dn_10"`
	GoldDiff10 int    `json:"gold_diff_10"`
}

type Episode struct {
	Time         string  `json:"time"`
	Title        string  `json:"title"`
	WhatHappened string  `json:"what_happened"`
	Mistake      *string `json:"mistake"`
	Better       string  `json:"better"`
	Severity     string  `json:"severity"`
}

type Improvement struct {
	Title string `json:"title"`
	Why   string `json:"why"`
	Drill string `json:"drill"`
}

// Point — точка на мини-карте; N — сколько раз игрок умер в этом месте.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	N int     `json:"n"`
}

// Answer — то, что просим у модели. Числа, которые известны точно (линия, статистика),
// в ответ не входят: их подставляем сами из данных реплея, чтобы модель их не выдумала.
type Answer struct {
	Verdict      string          `json:"verdict"`
	Rating       flexInt         `json:"rating"`
	LaneText     string          `json:"lane_text"`
	Episodes     []AnswerEpisode `json:"episodes"`
	Improvements []Improvement   `json:"improvements"`
	Plan         []string        `json:"next_games_plan"`
}

type AnswerEpisode struct {
	Time         string  `json:"time"`
	Title        string  `json:"title"`
	WhatHappened string  `json:"what_happened"`
	Mistake      *string `json:"mistake"`
	Better       string  `json:"better"`
	Severity     string  `json:"severity"`
}

// ErrBadAnswer — ответ модели не годится: не JSON или не по схеме.
var ErrBadAnswer = errors.New("analysis: bad model answer")

// flexInt принимает 7, 7.4, "7" и "7/10": модели не всегда пишут оценку одинаково.
type flexInt int

var firstNumber = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexInt(int(n + 0.5))
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("rating must be a number")
	}
	m := firstNumber.FindString(s)
	if m == "" {
		return fmt.Errorf("rating must be a number")
	}
	v, err := strconv.ParseFloat(strings.Replace(m, ",", ".", 1), 64)
	if err != nil {
		return err
	}
	*f = flexInt(int(v + 0.5))
	return nil
}

// Пределы длины текстов (в символах): защищают страницу от «простыней» и мусора.
const (
	maxVerdict  = 500
	maxLane     = 900
	maxTitle    = 140
	maxText     = 700
	maxPlanItem = 300

	maxEpisodes     = 10
	minEpisodes     = 1
	maxImprovements = 3
	minPlan         = 3
	maxPlan         = 5
)

var (
	thinkRe    = regexp.MustCompile(`(?s)<think>.*?</think>`)
	timeRe     = regexp.MustCompile(`^(\d{1,3})[:.](\d{1,2})$`)
	mdFenceRe  = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")
	severities = map[string]string{"good": "good", "minor": "minor", "major": "major"}
)

// extractJSON достаёт JSON-объект из ответа: отбрасывает рассуждения модели и markdown-обёртку.
func extractJSON(raw string) string {
	s := thinkRe.ReplaceAllString(raw, "")
	if m := mdFenceRe.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// normTime приводит «12:30», «12.30», «5:3» к виду «м:сс»; seconds нужен для сортировки.
func normTime(s string) (string, int, bool) {
	m := timeRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", 0, false
	}
	min, _ := strconv.Atoi(m[1])
	sec, _ := strconv.Atoi(m[2])
	if len(m[2]) == 1 {
		sec *= 10
	}
	if sec > 59 {
		return "", 0, false
	}
	return fmt.Sprintf("%d:%02d", min, sec), min*60 + sec, true
}

// ParseAnswer разбирает и проверяет ответ модели. Лишнее обрезается, а то, без чего
// разбор бессмыслен (вердикт, оценка, эпизоды, советы), — ошибка ErrBadAnswer.
func ParseAnswer(raw string) (*Answer, error) {
	body := extractJSON(raw)
	if body == "" {
		return nil, fmt.Errorf("%w: в ответе нет JSON-объекта", ErrBadAnswer)
	}
	var a Answer
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadAnswer, err)
	}

	a.Verdict = clip(a.Verdict, maxVerdict)
	if a.Verdict == "" {
		return nil, fmt.Errorf("%w: пустой verdict", ErrBadAnswer)
	}
	if a.Rating < 1 {
		return nil, fmt.Errorf("%w: rating должен быть числом от 1 до 10", ErrBadAnswer)
	}
	if a.Rating > 10 {
		a.Rating = 10
	}
	a.LaneText = clip(a.LaneText, maxLane)

	type timed struct {
		AnswerEpisode
		sec int
	}
	var eps []timed
	for _, e := range a.Episodes {
		tm, sec, ok := normTime(e.Time)
		e.Title, e.WhatHappened, e.Better = clip(e.Title, maxTitle), clip(e.WhatHappened, maxText), clip(e.Better, maxText)
		if !ok || e.Title == "" || e.WhatHappened == "" || e.Better == "" {
			continue // эпизод без времени или без сути не показываем
		}
		e.Time = tm
		sev, known := severities[strings.ToLower(strings.TrimSpace(e.Severity))]
		if !known {
			sev = "minor"
		}
		e.Severity = sev
		if e.Mistake != nil {
			m := clip(*e.Mistake, maxText)
			if m == "" || sev == "good" {
				e.Mistake = nil
			} else {
				e.Mistake = &m
			}
		}
		eps = append(eps, timed{e, sec})
	}
	if len(eps) < minEpisodes {
		return nil, fmt.Errorf("%w: нет ни одного корректного эпизода (нужны time «м:сс», title, what_happened, better)", ErrBadAnswer)
	}
	sort.SliceStable(eps, func(i, j int) bool { return eps[i].sec < eps[j].sec })
	if len(eps) > maxEpisodes {
		eps = eps[:maxEpisodes]
	}
	a.Episodes = a.Episodes[:0]
	for _, e := range eps {
		a.Episodes = append(a.Episodes, e.AnswerEpisode)
	}

	var imps []Improvement
	for _, im := range a.Improvements {
		im.Title, im.Why, im.Drill = clip(im.Title, maxTitle), clip(im.Why, maxText), clip(im.Drill, maxText)
		if im.Title != "" && im.Why != "" && im.Drill != "" {
			imps = append(imps, im)
		}
	}
	if len(imps) == 0 {
		return nil, fmt.Errorf("%w: нужны improvements (title, why, drill)", ErrBadAnswer)
	}
	if len(imps) > maxImprovements {
		imps = imps[:maxImprovements]
	}
	a.Improvements = imps

	var plan []string
	for _, p := range a.Plan {
		if p = clip(p, maxPlanItem); p != "" {
			plan = append(plan, p)
		}
	}
	if len(plan) < minPlan {
		return nil, fmt.Errorf("%w: next_games_plan должен содержать от %d до %d пунктов", ErrBadAnswer, minPlan, maxPlan)
	}
	if len(plan) > maxPlan {
		plan = plan[:maxPlan]
	}
	a.Plan = plan
	return &a, nil
}

// Assemble собирает итоговый результат: текст — от модели, цифры — из данных реплея.
func Assemble(a *Answer, f *Features) *Result {
	r := &Result{
		Summary: Summary{Verdict: a.Verdict, Rating: int(a.Rating), KeyStats: f.KeyStats},
		Lanes: Lanes{
			Text: a.LaneText, LH10: f.Lane.LH10, DN10: f.Lane.DN10, GoldDiff10: f.Lane.GoldDiff10,
		},
		Improvements:  a.Improvements,
		NextGamesPlan: a.Plan,
		DeathPoints:   f.DeathPoints,
	}
	if r.Summary.KeyStats == nil {
		r.Summary.KeyStats = []KeyStat{}
	}
	if r.DeathPoints == nil {
		r.DeathPoints = []Point{}
	}
	for _, e := range a.Episodes {
		r.Episodes = append(r.Episodes, Episode(e))
	}
	return r
}
