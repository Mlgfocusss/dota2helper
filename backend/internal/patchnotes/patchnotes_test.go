package patchnotes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"d2helper/internal/clients/dota2"
)

type fakeSource struct {
	calls    int
	namesErr error
}

func (f *fakeSource) Patch(_ context.Context, version string) (*dota2.Patch, error) {
	f.calls++
	if version != "7.41f" {
		return nil, dota2.ErrNotFound
	}
	return &dota2.Patch{
		Name:      "7.41f",
		Timestamp: 1789455600,
		General:   []dota2.GeneralNotes{{Title: "Общее", Notes: []dota2.Note{{Indent: 1, Text: "Правка"}}}},
		Items: []dota2.Entry{
			{AbilityID: -1, Title: "Предметы"},
			{AbilityID: 265, Notes: []dota2.Note{{Indent: 1, Text: "Бонус уменьшен"}, {Indent: 1, Text: "<br>"}}},
		},
		Heroes: []dota2.HeroNotes{
			{HeroID: 1, Abilities: []dota2.Entry{{AbilityID: 5003, Notes: []dota2.Note{{Indent: 2, Text: "Урон увеличен"}}}}},
			{HeroID: 2}, // без изменений — в ответ не попадает
		},
	}, nil
}

func (f *fakeSource) AbilityNames(context.Context) (map[int]string, error) {
	if f.namesErr != nil {
		return nil, f.namesErr
	}
	return map[int]string{265: "Boots of Bearing", 5003: "Mana Void"}, nil
}

func newService(src Source) *Service {
	return New(src, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestGetConvertsAndCaches(t *testing.T) {
	src := &fakeSource{}
	svc := newService(src)

	p, err := svc.Get(context.Background(), " 7.41F ")
	if err != nil {
		t.Fatal(err)
	}
	if p.Number != "7.41f" || len(p.Heroes) != 1 || p.Heroes[0].HeroID != 1 {
		t.Fatalf("patch = %+v", p)
	}
	if got := p.Heroes[0].Abilities[0]; got.Title != "Mana Void" || got.Notes[0].Indent != 2 {
		t.Errorf("ability = %+v", got)
	}
	if len(p.Items) != 2 || p.Items[0].Title != "Предметы" || p.Items[1].Title != "Boots of Bearing" {
		t.Errorf("items = %+v", p.Items)
	}
	if len(p.Items[1].Notes) != 1 {
		t.Errorf("служебная строка <br> должна пропадать: %+v", p.Items[1].Notes)
	}

	if _, err := svc.Get(context.Background(), "7.41f"); err != nil || src.calls != 1 {
		t.Errorf("повторный запрос должен идти из кэша: err=%v calls=%d", err, src.calls)
	}
}

func TestGetRejectsBadVersion(t *testing.T) {
	src := &fakeSource{}
	svc := newService(src)
	for _, v := range []string{"", "abc", "7.4", "../etc", "7.41f&language=x", "7.41ff"} {
		if _, err := svc.Get(context.Background(), v); !errors.Is(err, ErrNotFound) {
			t.Errorf("%q: err = %v", v, err)
		}
	}
	if src.calls != 0 {
		t.Error("мусорные версии не должны доходить до dota2.com")
	}
	if _, err := svc.Get(context.Background(), "9.99"); !errors.Is(err, ErrNotFound) {
		t.Errorf("неизвестная версия: %v", err)
	}
}

func TestGetWorksWithoutAbilityNames(t *testing.T) {
	svc := newService(&fakeSource{namesErr: errors.New("boom")})
	p, err := svc.Get(context.Background(), "7.41f")
	if err != nil {
		t.Fatal(err)
	}
	if p.Heroes[0].Abilities[0].Title != "Способность 5003" {
		t.Errorf("title = %q", p.Heroes[0].Abilities[0].Title)
	}
}

func TestVersionFromTitle(t *testing.T) {
	cases := map[string]string{
		"7.41f Gameplay Patch":                  "7.41f",
		"Gameplay Patch 7.41e and Summer Scrub": "7.41e",
		"7.41 Gameplay Patch":                   "7.41",
		"Dota 2 Update - 7/1/2026":              "",
		"Inside the Trove of Terror":            "",
		"The International 2026":                "",
		"Patch notes: 2026 recap":               "",
	}
	for title, want := range cases {
		if got := VersionFromTitle(title); got != want {
			t.Errorf("%q → %q, ожидалось %q", title, got, want)
		}
	}
}
