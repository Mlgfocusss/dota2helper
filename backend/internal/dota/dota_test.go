package dota

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"d2helper/internal/clients/opendota"
)

func TestSteamIDConversion(t *testing.T) {
	acc, err := AccountIDFromSteamID64("76561199174956609")
	if err != nil || acc != 1214690881 {
		t.Fatalf("AccountIDFromSteamID64 = %d, %v", acc, err)
	}
	if got := SteamID64FromAccountID(1214690881); got != "76561199174956609" {
		t.Errorf("SteamID64FromAccountID = %s", got)
	}
	for _, bad := range []string{"", "abc", "76561197960265728", "123", "765611991749566090"} {
		if _, err := AccountIDFromSteamID64(bad); err == nil {
			t.Errorf("%q должен быть некорректным", bad)
		}
	}
}

func TestHeroesFromOpenDota(t *testing.T) {
	heroes := HeroesFromOpenDota(map[string]opendota.HeroConstant{
		"2":  {ID: 2, Name: "npc_dota_hero_axe", LocalizedName: "Axe", PrimaryAttr: "str", Roles: []string{"Initiator"}},
		"1":  {ID: 1, Name: "npc_dota_hero_antimage", LocalizedName: "Anti-Mage", PrimaryAttr: "agi"},
		"99": {ID: 0, Name: "", LocalizedName: "broken"},
	})
	if len(heroes) != 2 {
		t.Fatalf("героев %d, ожидалось 2 (битая запись пропускается)", len(heroes))
	}
	am := heroes[0]
	if am.LocalizedName != "Anti-Mage" || am.ShortName != "antimage" {
		t.Errorf("первым по алфавиту должен идти Anti-Mage: %+v", am)
	}
	if am.Img != heroCDN+"/antimage.png" || am.Icon != heroCDN+"/icons/antimage.png" {
		t.Errorf("картинки: %s %s", am.Img, am.Icon)
	}
	if am.Roles == nil {
		t.Error("roles должен быть пустым массивом, а не null")
	}
}

func TestPatchLabel(t *testing.T) {
	cases := []struct{ number, env, want string }{
		{"7.41", "7.41F", "7.41F"},
		{"7.41", "", "7.41"},
		{"7.42", "7.41F", "7.42"},
		{"7.4", "7.41F", "7.4"},
		{"7.41", "7.41", "7.41"},
	}
	for _, tc := range cases {
		if got := PatchLabel(tc.number, tc.env); got != tc.want {
			t.Errorf("PatchLabel(%q, %q) = %q, ожидалось %q", tc.number, tc.env, got, tc.want)
		}
	}
	latest, ok := LatestPatch([]opendota.PatchConstant{{ID: 60, Name: "7.41"}, {ID: 59, Name: "7.40"}})
	if !ok || latest != "7.41" {
		t.Errorf("LatestPatch = %q, %v", latest, ok)
	}
}

type fakeOD struct {
	heroes map[string]opendota.HeroConstant
	err    error
	calls  int
}

func (f *fakeOD) HeroConstants(context.Context) (map[string]opendota.HeroConstant, error) {
	f.calls++
	return f.heroes, f.err
}

func (f *fakeOD) PatchConstants(context.Context) ([]opendota.PatchConstant, error) {
	return []opendota.PatchConstant{{ID: 60, Name: "7.41", Date: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}}, f.err
}

type memKV map[string][]byte

func (m memKV) GetKV(_ context.Context, key string) ([]byte, time.Time, error) {
	v, ok := m[key]
	if !ok {
		return nil, time.Time{}, errors.New("not found")
	}
	return v, time.Now(), nil
}

func (m memKV) PutKV(_ context.Context, key string, value []byte) error {
	m[key] = value
	return nil
}

func TestConstantsRefreshAndReload(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	od := &fakeOD{heroes: map[string]opendota.HeroConstant{
		"1": {ID: 1, Name: "npc_dota_hero_antimage", LocalizedName: "Anti-Mage"},
	}}
	kv := memKV{}

	c := NewConstants(od, kv, "7.41F", logger)
	if _, ok := c.Heroes(); ok {
		t.Fatal("до загрузки героев нет")
	}
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if p, _ := c.Patch(); p != (Patch{Number: "7.41", Label: "7.41F"}) {
		t.Errorf("patch = %+v", p)
	}
	body, etag, ok := c.HeroesJSON()
	if !ok || len(body) == 0 || etag == "" {
		t.Fatal("нет готового JSON с ETag")
	}

	// после перезапуска константы поднимаются из БД без запроса к OpenDota
	c2 := NewConstants(&fakeOD{err: errors.New("offline")}, kv, "7.41F", logger)
	if err := c2.LoadCached(context.Background()); err != nil {
		t.Fatalf("LoadCached: %v", err)
	}
	if h, ok := c2.Hero(1); !ok || h.ShortName != "antimage" {
		t.Errorf("Hero(1) = %+v, %v", h, ok)
	}
	_, etag2, _ := c2.HeroesJSON()
	if etag2 != etag {
		t.Error("ETag должен совпадать для тех же данных")
	}

	// свежие константы не перезагружаются
	if err := c2.RefreshIfStale(context.Background()); err != nil {
		t.Errorf("RefreshIfStale: %v", err)
	}
}
