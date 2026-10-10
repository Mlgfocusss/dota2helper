// Package telegram — отправка сообщений ботом (дублирование отзывов в чат).
package telegram

import (
	"context"
	"log/slog"
	"time"

	"d2helper/internal/clients/httpx"
)

const apiBase = "https://api.telegram.org"

type Client struct {
	token  string
	chatID string
	base   string
	http   *httpx.Client
}

func New(token, chatID string, logger *slog.Logger) *Client {
	return &Client{
		token:  token,
		chatID: chatID,
		base:   apiBase,
		http: httpx.New(httpx.Options{
			Name:       "telegram",
			PerMinute:  20,
			Timeout:    10 * time.Second,
			MaxRetries: 2,
			Logger:     logger,
		}),
	}
}

// WithBaseURL — для тестов.
func (c *Client) WithBaseURL(base string) *Client {
	c.base = base
	return c
}

// Enabled — заданы ли токен бота и чат.
func (c *Client) Enabled() bool { return c.token != "" && c.chatID != "" }

// Send отправляет текст в настроенный чат.
func (c *Client) Send(ctx context.Context, text string) error {
	body := map[string]any{
		"chat_id":                  c.chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	return c.http.PostJSON(ctx, c.base+"/bot"+c.token+"/sendMessage", nil, body, nil)
}
