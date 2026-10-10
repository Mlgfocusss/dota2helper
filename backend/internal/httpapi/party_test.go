package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParsePartyQuery(t *testing.T) {
	r := httptest.NewRequest("GET",
		"/api/party?looking_for=players&rank_min=6&rank_max=2&positions=1,3&modes=ranked,turbo&goals=fun&mic=1", nil)
	q, ok := parsePartyQuery(r)
	if !ok {
		t.Fatal("query should parse")
	}
	if q.LookingFor != "players" || q.RankMin != 2 || q.RankMax != 6 {
		t.Errorf("rank range should be normalized: %+v", q)
	}
	if len(q.Positions) != 2 || len(q.Modes) != 2 || len(q.Goals) != 1 || !q.Mic || q.Discord {
		t.Errorf("lists/flags: %+v", q)
	}

	q, ok = parsePartyQuery(httptest.NewRequest("GET", "/api/party", nil))
	if !ok || q.LookingFor != "team" || q.RankMin != 0 || q.RankMax != 8 {
		t.Errorf("defaults: %+v ok=%v", q, ok)
	}

	for _, bad := range []string{"looking_for=all", "rank_min=9", "rank_max=x", "positions=1,7", "positions=a"} {
		if _, ok := parsePartyQuery(httptest.NewRequest("GET", "/api/party?"+bad, nil)); ok {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestMonthBoundsMoscow(t *testing.T) {
	// 30 сентября 22:00 UTC — в Москве уже 1 октября
	start, end := monthBounds(time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC))
	if start.Month() != time.October || start.Day() != 1 || start.Hour() != 0 {
		t.Errorf("start: %v", start)
	}
	if end.Month() != time.November || end.Day() != 1 {
		t.Errorf("end: %v", end)
	}
	if got := start.UTC(); got.Hour() != 21 || got.Day() != 30 {
		t.Errorf("start in UTC: %v", got)
	}
}
