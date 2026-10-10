package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"d2helper/internal/dota"
	"d2helper/internal/httpapi/render"
	"d2helper/internal/meta"
)

type metaPosition struct {
	Position int             `json:"position"`
	Heroes   []meta.HeroStat `json:"heroes"`
}

type metaResponse struct {
	Patch dota.Patch `json:"patch"`
	// ByPosition = false: STRATZ недоступен, герои распределены по позициям
	// из heroes_tags.json с общим винрейтом (в UI — «по общему винрейту»)
	ByPosition bool           `json:"by_position"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Positions  []metaPosition `json:"positions"`
}

// metaPositions — GET /api/meta/positions?limit=5: топ героев каждой позиции.
func (a *api) metaPositions(w http.ResponseWriter, r *http.Request) {
	limit := intQuery(r, "limit", 5, 1, 20)
	snap, err := a.meta.Snapshot(r.Context())
	if errors.Is(err, meta.ErrNotReady) {
		w.Header().Set("Retry-After", "60")
		render.Error(w, http.StatusServiceUnavailable, "meta_not_ready", "Собираем статистику героев. Загляни через пару минут")
		return
	}
	if err != nil {
		a.logger.Error("meta snapshot", "err", err)
		render.Internal(w)
		return
	}

	resp := metaResponse{
		Patch:      snap.Patch,
		ByPosition: snap.Source == meta.SourceStratz,
		UpdatedAt:  snap.UpdatedAt,
		Positions:  make([]metaPosition, 0, 5),
	}
	for i, heroes := range snap.ByPosition {
		top := heroes
		if len(top) > limit {
			top = top[:limit]
		}
		if top == nil {
			top = []meta.HeroStat{}
		}
		resp.Positions = append(resp.Positions, metaPosition{Position: i + 1, Heroes: top})
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	render.JSON(w, http.StatusOK, resp)
}

// intQuery — целый параметр запроса в пределах [lo, hi], иначе def.
func intQuery(r *http.Request, key string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}
