package dota

import "testing"

func TestParseProfileInput(t *testing.T) {
	cases := []struct {
		in   string
		want ProfileRef
	}{
		{"76561199174956609", ProfileRef{SteamID64: "76561199174956609"}},
		{"1214690881", ProfileRef{AccountID: 1214690881}},
		{"  1214690881  ", ProfileRef{AccountID: 1214690881}},
		{"https://steamcommunity.com/profiles/76561199174956609", ProfileRef{SteamID64: "76561199174956609"}},
		{"https://steamcommunity.com/profiles/76561199174956609/", ProfileRef{SteamID64: "76561199174956609"}},
		{"steamcommunity.com/id/my_nick", ProfileRef{Vanity: "my_nick"}},
		{"http://www.steamcommunity.com/id/my-nick/?l=russian", ProfileRef{Vanity: "my-nick"}},
		{"https://www.opendota.com/players/1214690881", ProfileRef{AccountID: 1214690881}},
		{"https://stratz.com/players/1214690881/matches", ProfileRef{AccountID: 1214690881}},
		{"https://www.dotabuff.com/players/1214690881", ProfileRef{AccountID: 1214690881}},
	}
	for _, c := range cases {
		got, err := ParseProfileInput(c.in)
		if err != nil || got != c.want {
			t.Errorf("%q → %+v, %v; ожидалось %+v", c.in, got, err, c.want)
		}
	}
}

func TestParseProfileInputRejectsGarbage(t *testing.T) {
	for _, in := range []string{
		"", "   ", "привет", "0", "99999999999", "76561190000000000", "123456789012345678",
		"https://example.com/players/123", "https://steamcommunity.com/id/", "https://steamcommunity.com/id/a b",
		"https://steamcommunity.com/groups/dota2", "javascript:alert(1)", "https://opendota.com/matches/123",
	} {
		if got, err := ParseProfileInput(in); err == nil {
			t.Errorf("%q должен отклоняться, получено %+v", in, got)
		}
	}
}
