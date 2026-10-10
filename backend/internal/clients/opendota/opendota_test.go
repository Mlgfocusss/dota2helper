package opendota

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func newTestServer(t *testing.T) (*Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("api_key") != "KEY" {
			http.Error(w, "no key", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/players/1214690881":
			_, _ = w.Write([]byte(`{"profile":{"account_id":1214690881,"personaname":"Mlgfocus","avatarfull":"https://a/b.jpg"},"rank_tier":51}`))
		case "/constants/patch":
			_, _ = w.Write([]byte(`[{"id":60,"name":"7.41","date":"2026-03-24T00:50:59.580Z"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New("KEY", 6000, logger).WithBaseURL(srv.URL), &calls
}

func TestPlayerIsParsedAndCached(t *testing.T) {
	c, calls := newTestServer(t)
	for range 3 {
		p, err := c.Player(context.Background(), 1214690881)
		if err != nil {
			t.Fatalf("Player: %v", err)
		}
		if p.Profile == nil || p.Profile.PersonaName != "Mlgfocus" || p.RankTier == nil || *p.RankTier != 51 {
			t.Fatalf("player = %+v", p)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("запросов %d: повторные должны браться из кэша", calls.Load())
	}
}

func TestNotFound(t *testing.T) {
	c, calls := newTestServer(t)
	for range 2 {
		if _, err := c.Player(context.Background(), 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("ожидалась ErrNotFound, получено %v", err)
		}
	}
	if calls.Load() != 2 {
		t.Errorf("ошибки не кэшируются: запросов %d, ожидалось 2", calls.Load())
	}
}

func TestPatchConstants(t *testing.T) {
	c, _ := newTestServer(t)
	patches, err := c.PatchConstants(context.Background())
	if err != nil || len(patches) != 1 || patches[0].Name != "7.41" || patches[0].Date.Year() != 2026 {
		t.Fatalf("PatchConstants = %+v, %v", patches, err)
	}
}

func TestParsedMatchDecodesRealShapes(t *testing.T) {
	// формы полей взяты из настоящего ответа OpenDota: дробные координаты вардов, пустые и отсутствующие поля
	raw := `{"match_id":1,"version":22,"duration":2400,"radiant_win":true,"teamfights":[{"start":10,"end":20,"deaths":2,"players":[{"deaths":1,"damage":100,"deaths_pos":{"96":{"112":2}}}]}],
		"players":[{"account_id":5,"hero_id":1,"isRadiant":true,"lane_efficiency_pct":81,"teamfight_participation":0.7,
		"obs_log":[{"time":90,"x":174.4,"y":120.5,"z":0,"type":"obs_log"}],"deaths_log":[{"time":5,"key":"npc_dota_hero_slark","gold_lost":80,"time_dead":20}],
		"benchmarks":{"gold_per_min":{"raw":582,"pct":0.79}},"purchase_log":[{"time":-89,"key":"branches"}],"buyback_log":[{"time":1,"slot":0}]},{"account_id":null,"hero_id":2}]}`
	var m ParsedMatch
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if !m.Parsed() || len(m.Players) != 2 || m.Players[0].ObsLog[0].X != 174.4 || m.Players[1].AccountID != nil {
		t.Errorf("decoded: %+v", m)
	}
	var unparsed ParsedMatch
	_ = json.Unmarshal([]byte(`{"match_id":2,"version":null,"players":[]}`), &unparsed)
	if unparsed.Parsed() {
		t.Error("version null means the replay is not parsed yet")
	}
}
