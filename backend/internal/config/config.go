// Package config загружает настройки приложения из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppName string
	// PublicURL — адрес сайта без слэша на конце. Пустой — адрес определяется
	// по входящему запросу (только для разработки).
	PublicURL   string
	HTTPAddr    string
	DatabaseURL string
	DataDir     string

	SessionSecret string
	CookieSecure  bool

	OpenDotaAPIKey     string
	OpenDotaRatePerMin int
	StratzToken        string
	StratzRatePerMin   int
	SteamAPIKey        string

	PatchLabel     string
	MetaBracket    string
	MetaMinMatches int

	// Коэффициенты оценки тренировки драфтера (SPEC 5.3)
	DrafterScoreScale       float64
	DrafterMetaWeight       float64
	DrafterOffPosPenalty    float64
	DrafterTrainingsPerHour int

	// Глубокий разбор матчей: бесплатный, с дневным лимитом на пользователя.
	// Модель — любая OpenAI-совместимая (OpenRouter, Ollama, Groq…): адрес, ключ и список моделей.
	FeatureDeepAnalysis bool
	LLMBaseURL          string
	LLMAPIKey           string
	LLMModel            string
	AnalysesPerDay      int
	AnalysisMaxParallel int

	SupportURL     string
	SupportContact string
	PromoImageURL  string
	PromoLink      string

	TelegramFeedbackBotToken string
	TelegramFeedbackChatID   string

	LogLevel  string
	LogFormat string

	// Warnings — некритичные замечания, которые стоит вывести в лог при старте.
	Warnings []string
}

const defaultSessionSecret = "change-me-32-bytes"

// Load читает конфигурацию из окружения и проверяет её.
func Load() (Config, error) {
	var e env
	c := Config{
		AppName:     e.str("APP_NAME", "D2Helper"),
		PublicURL:   strings.TrimRight(e.str("PUBLIC_URL", ""), "/"),
		HTTPAddr:    e.str("HTTP_ADDR", ":8080"),
		DatabaseURL: e.str("DATABASE_URL", ""),
		DataDir:     e.str("DATA_DIR", "data"),

		SessionSecret: e.str("SESSION_SECRET", ""),
		CookieSecure:  e.bool("COOKIE_SECURE", false),

		OpenDotaAPIKey:     e.str("OPENDOTA_API_KEY", ""),
		OpenDotaRatePerMin: e.int("OPENDOTA_RATE_PER_MIN", 50),
		StratzToken:        e.str("STRATZ_TOKEN", ""),
		StratzRatePerMin:   e.int("STRATZ_RATE_PER_MIN", 120),
		SteamAPIKey:        e.str("STEAM_API_KEY", ""),

		PatchLabel:     e.str("PATCH_LABEL", ""),
		MetaBracket:    e.str("META_BRACKET", "ALL"),
		MetaMinMatches: e.int("META_MIN_MATCHES", 200),

		DrafterScoreScale:       e.float("DRAFTER_SCORE_SCALE", 1.1),
		DrafterMetaWeight:       e.float("DRAFTER_META_WEIGHT", 10),
		DrafterOffPosPenalty:    e.float("DRAFTER_OFFPOS_PENALTY", 3),
		DrafterTrainingsPerHour: e.int("DRAFTER_TRAININGS_PER_HOUR", 60),

		FeatureDeepAnalysis: e.bool("FEATURE_DEEP_ANALYSIS", true),
		LLMBaseURL:          strings.TrimRight(e.str("LLM_BASE_URL", "https://openrouter.ai/api/v1"), "/"),
		LLMAPIKey:           e.str("LLM_API_KEY", ""),
		LLMModel:            e.str("LLM_MODEL", "openrouter/free"),
		AnalysesPerDay:      e.int("ANALYSES_PER_DAY", 3),
		AnalysisMaxParallel: e.int("ANALYSIS_MAX_PARALLEL", 2),

		SupportURL:     e.str("SUPPORT_URL", ""),
		SupportContact: e.str("SUPPORT_CONTACT", ""),
		PromoImageURL:  e.str("PROMO_IMAGE_URL", ""),
		PromoLink:      e.str("PROMO_LINK", ""),

		TelegramFeedbackBotToken: e.str("TELEGRAM_FEEDBACK_BOT_TOKEN", ""),
		TelegramFeedbackChatID:   e.str("TELEGRAM_FEEDBACK_CHAT_ID", ""),

		LogLevel:  strings.ToLower(e.str("LOG_LEVEL", "info")),
		LogFormat: strings.ToLower(e.str("LOG_FORMAT", "text")),
	}
	c.validate(&e)
	return c, errors.Join(e.errs...)
}

func (c *Config) validate(e *env) {
	if c.DatabaseURL == "" {
		e.fail("DATABASE_URL: не задан")
	}
	if c.PublicURL == "" {
		c.warn("PUBLIC_URL не задан: адрес сайта определяется по запросу. В продакшене задай его явно")
	} else if u, err := url.Parse(c.PublicURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		e.fail("PUBLIC_URL: ожидается адрес вида https://example.com, получено %q", c.PublicURL)
	}
	switch {
	case c.SessionSecret == "":
		e.fail("SESSION_SECRET: не задан")
	case c.SessionSecret == defaultSessionSecret || len(c.SessionSecret) < 32:
		c.warn("SESSION_SECRET слишком простой: используй случайную строку от 32 символов")
	}
	if c.MetaMinMatches < 1 {
		e.fail("META_MIN_MATCHES: должно быть больше 0")
	}
	if c.DrafterScoreScale <= 0 || c.DrafterMetaWeight < 0 || c.DrafterOffPosPenalty < 0 || c.DrafterTrainingsPerHour < 1 {
		e.fail("DRAFTER_*: коэффициенты не могут быть отрицательными, масштаб и лимит тренировок — больше 0")
	}
	if c.OpenDotaRatePerMin < 1 || c.StratzRatePerMin < 1 {
		e.fail("OPENDOTA_RATE_PER_MIN и STRATZ_RATE_PER_MIN: должны быть больше 0")
	}
	if c.AnalysesPerDay < 1 || c.AnalysisMaxParallel < 1 {
		e.fail("ANALYSES_PER_DAY и ANALYSIS_MAX_PARALLEL: должны быть больше 0")
	}
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		e.fail("LOG_LEVEL: допустимо debug, info, warn или error, получено %q", c.LogLevel)
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		e.fail("LOG_FORMAT: допустимо text или json, получено %q", c.LogFormat)
	}
	if c.StratzToken == "" {
		c.warn("STRATZ_TOKEN не задан: мета и матчапы считаются по OpenDota, синергии недоступны")
	}
	if c.FeatureDeepAnalysis && c.LLMAPIKey == "" && !localLLM(c.LLMBaseURL) {
		c.warn("LLM_API_KEY не задан: глубокий разбор матчей недоступен (бесплатный ключ — на openrouter.ai, либо LLM_BASE_URL=http://ollama:11434/v1 для локальной модели)")
	}
	if c.SteamAPIKey == "" {
		c.warn("STEAM_API_KEY не задан: поиск профиля по ссылке steamcommunity.com/id/... недоступен")
	}
}

func (c *Config) warn(msg string) { c.Warnings = append(c.Warnings, msg) }

// env читает переменные и копит ошибки разбора, чтобы показать их все разом.
type env struct{ errs []error }

func (e *env) fail(format string, args ...any) {
	e.errs = append(e.errs, fmt.Errorf(format, args...))
}

func (e *env) str(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func (e *env) bool(key string, def bool) bool {
	v := e.str(key, "")
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		e.fail("%s: ожидается true или false, получено %q", key, v)
		return def
	}
	return b
}

func (e *env) int(key string, def int) int {
	v := e.str(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		e.fail("%s: ожидается целое число, получено %q", key, v)
		return def
	}
	return n
}

func (e *env) float(key string, def float64) float64 {
	v := e.str(key, "")
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		e.fail("%s: ожидается число, получено %q", key, v)
		return def
	}
	return f
}

// localLLM — адрес локальной модели (Ollama и подобные): ключ ей не нужен.
func localLLM(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "ollama", "host.docker.internal":
		return true
	}
	return false
}
