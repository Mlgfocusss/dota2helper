// Package stratz — клиент GraphQL API STRATZ (https://stratz.com/api).
package stratz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"d2helper/internal/clients/httpx"
)

const DefaultEndpoint = "https://api.stratz.com/graphql"

// После отказа в доступе (401/403) повторяем попытки не раньше чем через это время.
const disableFor = 6 * time.Hour

var (
	// ErrNoToken — STRATZ_TOKEN не задан.
	ErrNoToken = errors.New("stratz: STRATZ_TOKEN is not set")
	// ErrUnavailable — STRATZ отказал в доступе (неверный токен или ограничения аккаунта).
	ErrUnavailable = errors.New("stratz: access denied")
)

type Client struct {
	endpoint string
	token    string
	http     *httpx.Client
	logger   *slog.Logger
	now      func() time.Time

	mu            sync.Mutex
	disabledUntil time.Time
	reason        string
}

func New(token string, perMinute int, logger *slog.Logger) *Client {
	return &Client{
		endpoint: DefaultEndpoint,
		token:    token,
		logger:   logger,
		now:      time.Now,
		http: httpx.New(httpx.Options{
			Name:       "stratz",
			PerMinute:  perMinute,
			Timeout:    30 * time.Second,
			MaxRetries: 2,
			// без этого заголовка STRATZ отклоняет запросы
			UserAgent: "STRATZ_API",
			Logger:    logger,
		}),
	}
}

// WithEndpoint — для тестов.
func (c *Client) WithEndpoint(endpoint string) *Client {
	c.endpoint = endpoint
	return c
}

// Available — можно ли сейчас обращаться к STRATZ. Если нет — используем OpenDota.
func (c *Client) Available() bool {
	if c.token == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.now().Before(c.disabledUntil)
}

// Status — человекочитаемая причина недоступности (пусто, если всё в порядке).
func (c *Client) Status() string {
	if c.token == "" {
		return "STRATZ_TOKEN не задан"
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now().Before(c.disabledUntil) {
		return c.reason
	}
	return ""
}

type gqlError struct {
	Message string `json:"message"`
}

// GraphQLError — запрос дошёл, но GraphQL вернул ошибки (например, поле переименовали).
type GraphQLError struct{ Messages []string }

func (e *GraphQLError) Error() string {
	return "stratz graphql: " + strings.Join(e.Messages, "; ")
}

// Query выполняет GraphQL‑запрос и разбирает поле data в out.
func (c *Client) Query(ctx context.Context, query string, vars map[string]any, out any) error {
	if c.token == "" {
		return ErrNoToken
	}
	if !c.Available() {
		return fmt.Errorf("%w: %s", ErrUnavailable, c.Status())
	}

	body := map[string]any{"query": query}
	if len(vars) > 0 {
		body["variables"] = vars
	}
	var resp struct {
		Data   json.RawMessage `json:"data"`
		Errors []gqlError      `json:"errors"`
	}
	header := http.Header{"Authorization": {"Bearer " + c.token}}
	err := c.http.PostJSON(ctx, c.endpoint, header, body, &resp)
	if httpx.IsStatus(err, http.StatusUnauthorized) || httpx.IsStatus(err, http.StatusForbidden) {
		c.disable(err)
		return fmt.Errorf("%w: %s", ErrUnavailable, c.Status())
	}
	if err != nil {
		return err
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return &GraphQLError{Messages: msgs}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Data, out); err != nil {
		return fmt.Errorf("stratz: decode data: %w", err)
	}
	return nil
}

// Check проверяет доступ лёгким запросом.
func (c *Client) Check(ctx context.Context) error {
	return c.Query(ctx, `{ constants { gameVersions { id } } }`, nil, nil)
}

func (c *Client) disable(err error) {
	reason := err.Error()
	var se *httpx.StatusError
	if errors.As(err, &se) && se.Body != "" {
		reason = se.Body
	}
	c.mu.Lock()
	c.disabledUntil = c.now().Add(disableFor)
	c.reason = reason
	c.mu.Unlock()
	c.logger.Warn("STRATZ недоступен, используем OpenDota", "reason", reason, "retry_after", disableFor)
}
