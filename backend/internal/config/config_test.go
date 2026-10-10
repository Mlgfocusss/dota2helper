package config

import (
	"strings"
	"testing"
)

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://d2:d2@localhost:5432/d2")
	t.Setenv("SESSION_SECRET", strings.Repeat("x", 32))
	t.Setenv("PUBLIC_URL", "https://example.com/")
	t.Setenv("LLM_API_KEY", "key")
	t.Setenv("STRATZ_TOKEN", "token")
	t.Setenv("STEAM_API_KEY", "key")
}

func TestLoadDefaults(t *testing.T) {
	setBaseEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.PublicURL != "https://example.com" {
		t.Errorf("PublicURL = %q, слэш на конце должен обрезаться", c.PublicURL)
	}
	if c.MetaMinMatches != 200 || c.HTTPAddr != ":8080" || c.AppName != "D2Helper" {
		t.Errorf("неожиданные значения по умолчанию: %+v", c)
	}
	if len(c.Warnings) != 0 {
		t.Errorf("не ожидались предупреждения: %v", c.Warnings)
	}
}

func TestLoadCollectsAllErrors(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DATABASE_URL", "")
	t.Setenv("COOKIE_SECURE", "yes please")
	t.Setenv("META_MIN_MATCHES", "много")
	t.Setenv("ANALYSES_PER_DAY", "0")

	_, err := Load()
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	for _, key := range []string{"DATABASE_URL", "COOKIE_SECURE", "META_MIN_MATCHES", "ANALYSES_PER_DAY"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("в ошибке нет %s: %v", key, err)
		}
	}
}

func TestLoadRejectsBadPublicURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("PUBLIC_URL", "example.com")
	if _, err := Load(); err == nil {
		t.Fatal("ожидалась ошибка для PUBLIC_URL без схемы")
	}
}

func TestLoadWarnsAboutWeakSecret(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("SESSION_SECRET", defaultSessionSecret)
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Warnings) == 0 {
		t.Error("ожидалось предупреждение о слабом SESSION_SECRET")
	}
}

func TestLoadLLMDefaults(t *testing.T) {
	setBaseEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.LLMBaseURL != "https://openrouter.ai/api/v1" || c.LLMModel != "openrouter/free" {
		t.Errorf("OpenRouter с бесплатным роутером — по умолчанию: %q %q", c.LLMBaseURL, c.LLMModel)
	}
	if c.AnalysesPerDay != 3 || c.AnalysisMaxParallel != 2 {
		t.Errorf("лимиты по умолчанию: %d в сутки, %d параллельно", c.AnalysesPerDay, c.AnalysisMaxParallel)
	}
}

func TestLoadWarnsWithoutLLMKey(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("LLM_API_KEY", "")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Warnings) != 1 || !strings.Contains(c.Warnings[0], "LLM_API_KEY") {
		t.Errorf("облачной модели нужен ключ — ожидалось предупреждение: %v", c.Warnings)
	}

	// локальной модели (Ollama) ключ не нужен
	t.Setenv("LLM_BASE_URL", "http://ollama:11434/v1")
	c, err = Load()
	if err != nil || len(c.Warnings) != 0 {
		t.Errorf("локальная модель без ключа: %v %v", err, c.Warnings)
	}

	// разбор выключен — ключ тоже не нужен
	t.Setenv("LLM_BASE_URL", "https://openrouter.ai/api/v1")
	t.Setenv("FEATURE_DEEP_ANALYSIS", "false")
	if c, _ = Load(); len(c.Warnings) != 0 {
		t.Errorf("при выключенном разборе предупреждения не нужны: %v", c.Warnings)
	}
}
