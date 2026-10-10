package httpapi

import (
	"context"
	"net/http"
	"time"

	"d2helper/internal/httpapi/render"
)

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx); err != nil {
		a.logger.Warn("health: db ping failed", "err", err)
		render.Error(w, http.StatusServiceUnavailable, "db_unavailable", "База данных недоступна")
		return
	}
	render.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type publicConfig struct {
	AppName        string         `json:"app_name"`
	SupportURL     string         `json:"support_url"`
	SupportContact string         `json:"support_contact"`
	Promo          *promoConfig   `json:"promo"`
	Features       featuresConfig `json:"features"`
}

type promoConfig struct {
	ImageURL string `json:"image_url"`
	Link     string `json:"link"`
}

type featuresConfig struct {
	// DeepAnalysis — разбор включен и модель настроена
	DeepAnalysis   bool `json:"deep_analysis"`
	AnalysesPerDay int  `json:"analyses_per_day"`
}

// publicConfig отдаёт фронту настройки, которые не являются секретами.
func (a *api) publicConfig(w http.ResponseWriter, _ *http.Request) {
	resp := publicConfig{
		AppName:        a.cfg.AppName,
		SupportURL:     a.cfg.SupportURL,
		SupportContact: a.cfg.SupportContact,
		Features: featuresConfig{
			DeepAnalysis:   a.analysis.Available(),
			AnalysesPerDay: a.analysis.PerDay(),
		},
	}
	if a.cfg.PromoImageURL != "" {
		resp.Promo = &promoConfig{ImageURL: a.cfg.PromoImageURL, Link: a.cfg.PromoLink}
	}
	render.JSON(w, http.StatusOK, resp)
}
