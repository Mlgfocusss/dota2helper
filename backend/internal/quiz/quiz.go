// Package quiz — квизы «Какая твоя позиция?» и «Подбор идеального героя»: загрузка вопросов и подсчёт результата.
package quiz

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"d2helper/internal/dota"
)

// ----- вопросы ---------------------------------------------------------------

type PositionOption struct {
	ID      string         `json:"id"`
	Icon    string         `json:"icon"`
	Text    string         `json:"text"`
	Weights map[string]int `json:"weights,omitempty"`
}

type PositionQuestion struct {
	ID      string           `json:"id"`
	Text    string           `json:"text"`
	Options []PositionOption `json:"options"`
}

type PositionQuiz struct {
	ID        string             `json:"id"`
	Questions []PositionQuestion `json:"questions"`
}

type HeroOption struct {
	ID   string         `json:"id"`
	Text string         `json:"text"`
	Tags map[string]int `json:"tags,omitempty"`
	// ComplexityPref: "high" или "low" — предпочтение по сложности героя
	ComplexityPref string `json:"complexity_pref,omitempty"`
}

type HeroQuestion struct {
	ID      string       `json:"id"`
	Text    string       `json:"text"`
	Options []HeroOption `json:"options"`
}

type HeroQuiz struct {
	ID         string                    `json:"id"`
	ByPosition map[string][]HeroQuestion `json:"byPosition"`
	Common     []HeroQuestion            `json:"common"`
}

type PositionInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Data — все загруженные данные квизов.
type Data struct {
	Position  PositionQuiz
	Hero      HeroQuiz
	Positions map[int]PositionInfo
}

var (
	ErrBadAnswers  = errors.New("quiz: bad answers")
	ErrBadPosition = errors.New("quiz: bad position")
)

// Load читает и проверяет файлы квизов. Ошибка в наших данных — ошибка запуска.
func Load(dir string) (*Data, error) {
	var d Data
	if err := readJSON(filepath.Join(dir, "quiz_position.json"), &d.Position); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "quiz_hero.json"), &d.Hero); err != nil {
		return nil, err
	}
	raw := map[string]PositionInfo{}
	if err := readJSON(filepath.Join(dir, "positions.json"), &raw); err != nil {
		return nil, err
	}
	d.Positions = make(map[int]PositionInfo, len(raw))
	for k, v := range raw {
		n, err := strconv.Atoi(k)
		if err != nil || n < 1 || n > 5 {
			return nil, fmt.Errorf("positions.json: bad key %q", k)
		}
		d.Positions[n] = v
	}
	if err := d.validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

func readJSON(path string, out any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

// Теги, допустимые в вопросах: теги героев и тайминг силы.
var heroQuizTags = func() map[string]bool {
	m := map[string]bool{"early": true, "mid": true, "late": true}
	for _, t := range dota.AllowedTags {
		m[t] = true
	}
	return m
}()

func (d *Data) validate() error {
	if len(d.Position.Questions) == 0 {
		return errors.New("quiz_position.json: нет вопросов")
	}
	seenQ := map[string]bool{}
	for _, q := range d.Position.Questions {
		if seenQ[q.ID] || len(q.Options) < 2 {
			return fmt.Errorf("quiz_position.json: вопрос %q повторяется или имеет меньше двух вариантов", q.ID)
		}
		seenQ[q.ID] = true
		seenO := map[string]bool{}
		for _, o := range q.Options {
			if seenO[o.ID] || len(o.Weights) == 0 {
				return fmt.Errorf("quiz_position.json: %s/%s: нет id или весов", q.ID, o.ID)
			}
			seenO[o.ID] = true
			for pos := range o.Weights {
				if n, err := strconv.Atoi(pos); err != nil || n < 1 || n > 5 {
					return fmt.Errorf("quiz_position.json: %s/%s: позиция %q", q.ID, o.ID, pos)
				}
			}
		}
	}
	for pos := 1; pos <= 5; pos++ {
		if _, ok := d.Positions[pos]; !ok {
			return fmt.Errorf("positions.json: нет позиции %d", pos)
		}
		if len(d.Hero.ByPosition[strconv.Itoa(pos)]) != 2 {
			return fmt.Errorf("quiz_hero.json: для позиции %d нужно ровно 2 вопроса", pos)
		}
		for _, q := range d.Hero.ByPosition[strconv.Itoa(pos)] {
			if err := validateHeroQuestion(q); err != nil {
				return err
			}
		}
	}
	if len(d.Hero.Common) != 2 {
		return errors.New("quiz_hero.json: нужно ровно 2 общих вопроса")
	}
	for _, q := range d.Hero.Common {
		if err := validateHeroQuestion(q); err != nil {
			return err
		}
	}
	return nil
}

func validateHeroQuestion(q HeroQuestion) error {
	if len(q.Options) < 2 {
		return fmt.Errorf("quiz_hero.json: вопрос %q: меньше двух вариантов", q.ID)
	}
	seen := map[string]bool{}
	for _, o := range q.Options {
		if seen[o.ID] {
			return fmt.Errorf("quiz_hero.json: %s: повтор варианта %q", q.ID, o.ID)
		}
		seen[o.ID] = true
		if len(o.Tags) == 0 && o.ComplexityPref == "" {
			return fmt.Errorf("quiz_hero.json: %s/%s: нет тегов", q.ID, o.ID)
		}
		for tag := range o.Tags {
			if !heroQuizTags[tag] {
				return fmt.Errorf("quiz_hero.json: %s/%s: неизвестный тег %q", q.ID, o.ID, tag)
			}
		}
		if p := o.ComplexityPref; p != "" && p != "high" && p != "low" {
			return fmt.Errorf("quiz_hero.json: %s/%s: complexity_pref %q", q.ID, o.ID, p)
		}
	}
	return nil
}

// ----- что отдаём клиенту (без весов) ----------------------------------------

// PositionForClient — вопросы квиза позиции без весов: подсчёт идёт только на сервере.
func (d *Data) PositionForClient() PositionQuiz {
	out := PositionQuiz{ID: d.Position.ID}
	for _, q := range d.Position.Questions {
		cq := PositionQuestion{ID: q.ID, Text: q.Text}
		for _, o := range q.Options {
			cq.Options = append(cq.Options, PositionOption{ID: o.ID, Icon: o.Icon, Text: o.Text})
		}
		out.Questions = append(out.Questions, cq)
	}
	return out
}

// HeroQuestionsForClient — 4 вопроса подбора героя для позиции (2 свои + 2 общих), без тегов.
func (d *Data) HeroQuestionsForClient(position int) ([]HeroQuestion, error) {
	qs, ok := d.heroQuestions(position)
	if !ok {
		return nil, ErrBadPosition
	}
	out := make([]HeroQuestion, 0, len(qs))
	for _, q := range qs {
		cq := HeroQuestion{ID: q.ID, Text: q.Text}
		for _, o := range q.Options {
			cq.Options = append(cq.Options, HeroOption{ID: o.ID, Text: o.Text})
		}
		out = append(out, cq)
	}
	return out, nil
}

func (d *Data) heroQuestions(position int) ([]HeroQuestion, bool) {
	own, ok := d.Hero.ByPosition[strconv.Itoa(position)]
	if !ok {
		return nil, false
	}
	qs := make([]HeroQuestion, 0, len(own)+len(d.Hero.Common))
	qs = append(qs, own...)
	return append(qs, d.Hero.Common...), true
}
