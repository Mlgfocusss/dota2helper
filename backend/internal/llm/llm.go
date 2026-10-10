// Package llm — клиент языковой модели за интерфейсом Provider.
//
// Реализация одна — для OpenAI‑совместимых API (POST {base}/chat/completions): это
// OpenRouter (в том числе бесплатные модели), Ollama, Groq, LM Studio и Gemini. Что именно
// использовать, задают три переменные окружения: адрес, ключ и модель.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Request — один запрос к модели.
type Request struct {
	System      string
	User        string
	MaxTokens   int
	Temperature float64
	// JSON просит модель отвечать JSON‑объектом (response_format). Не все модели это умеют —
	// тогда запрос повторяется без параметра, а разбор ответа всё равно проверяется.
	JSON bool
}

// Provider — языковая модель.
type Provider interface {
	Complete(ctx context.Context, r Request) (string, error)
	// Configured — есть ли всё нужное для работы (иначе разбор недоступен).
	Configured() bool
}

var (
	// ErrNotConfigured — модель не настроена (нет адреса или ключа).
	ErrNotConfigured = errors.New("llm: not configured")
	// ErrAuth — сервис отклонил ключ.
	ErrAuth = errors.New("llm: access denied")
	// ErrUnavailable — все модели из списка не ответили (лимиты, сбои сети).
	ErrUnavailable = errors.New("llm: all models failed")
)

type Options struct {
	// BaseURL — например https://openrouter.ai/api/v1 или http://ollama:11434/v1
	BaseURL string
	APIKey  string
	// Models — модели через запятую: пробуются по порядку, пока одна не ответит.
	Models string
	// Referer и Title — необязательные заголовки OpenRouter (атрибуция приложения)
	Referer string
	Title   string
	Timeout time.Duration
	Logger  *slog.Logger
}

type Client struct {
	base    string
	key     string
	models  []string
	referer string
	title   string
	http    *http.Client
	logger  *slog.Logger
	// pause между проходами по списку моделей; в тестах — ноль
	pause time.Duration
}

func New(o Options) *Client {
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Minute
	}
	var models []string
	for _, m := range strings.Split(o.Models, ",") {
		if m = strings.TrimSpace(m); m != "" {
			models = append(models, m)
		}
	}
	logger := o.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		base: strings.TrimRight(o.BaseURL, "/"), key: o.APIKey, models: models,
		referer: o.Referer, title: o.Title,
		http: &http.Client{Timeout: timeout}, logger: logger, pause: 5 * time.Second,
	}
}

// Configured — задан адрес и модель; ключ нужен всем, кроме локальных серверов.
func (c *Client) Configured() bool {
	if c.base == "" || len(c.models) == 0 {
		return false
	}
	return c.key != "" || isLocal(c.base)
}

func isLocal(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "ollama" || host == "host.docker.internal"
}

// Models — список моделей (для логов и диагностики).
func (c *Client) Models() []string { return c.models }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// callError — ошибка одного обращения к модели; retryable — стоит пробовать следующую.
type callError struct {
	status int
	msg    string
}

func (e *callError) Error() string { return fmt.Sprintf("llm: status %d: %s", e.status, e.msg) }

// Complete отправляет запрос; при отказе или лимите пробует следующие модели списка,
// затем делает ещё один проход. Бесплатные модели часто перегружены — это нормальный случай.
func (c *Client) Complete(ctx context.Context, r Request) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	var last error
	for pass := 0; pass < 2; pass++ {
		if pass > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(c.pause):
			}
		}
		for _, model := range c.models {
			text, err := c.call(ctx, model, r, r.JSON)
			var ce *callError
			if errors.As(err, &ce) && ce.status == http.StatusBadRequest && r.JSON {
				// модель не поддерживает response_format — повторяем без него
				text, err = c.call(ctx, model, r, false)
			}
			if err == nil {
				return text, nil
			}
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if errors.As(err, &ce) && (ce.status == http.StatusUnauthorized || ce.status == http.StatusForbidden) {
				return "", fmt.Errorf("%w: %s", ErrAuth, ce.msg)
			}
			c.logger.Warn("llm: model failed", "model", model, "err", err)
			last = err
		}
	}
	return "", fmt.Errorf("%w: %v", ErrUnavailable, last)
}

func (c *Client) call(ctx context.Context, model string, r Request, jsonMode bool) (string, error) {
	body := chatRequest{
		Model:       model,
		Messages:    []chatMessage{{Role: "system", Content: r.System}, {Role: "user", Content: r.User}},
		MaxTokens:   r.MaxTokens,
		Temperature: r.Temperature,
	}
	if jsonMode {
		body.ResponseFormat = &struct {
			Type string `json:"type"`
		}{Type: "json_object"}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	if c.referer != "" {
		req.Header.Set("HTTP-Referer", c.referer)
	}
	if c.title != "" {
		req.Header.Set("X-Title", c.title)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", &callError{status: resp.StatusCode, msg: truncate(string(data), 300)}
	}
	var out chatResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	// OpenRouter иногда отвечает 200 с ошибкой внутри тела
	if out.Error != nil && len(out.Choices) == 0 {
		return "", &callError{status: http.StatusBadGateway, msg: truncate(out.Error.Message, 300)}
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", &callError{status: http.StatusBadGateway, msg: "empty answer"}
	}
	return out.Choices[0].Message.Content, nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
