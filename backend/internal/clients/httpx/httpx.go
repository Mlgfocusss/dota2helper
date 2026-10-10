// Package httpx — общий HTTP‑клиент для внешних API: лимит запросов (token bucket),
// повторы с экспоненциальной задержкой на 429/5xx и сетевых ошибках, разбор JSON.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	maxBodyBytes  = 64 << 20
	maxErrorBytes = 512
	baseBackoff   = time.Second
	maxBackoff    = 15 * time.Second
	maxRetryAfter = 30 * time.Second
)

type Options struct {
	// Name — имя API для логов
	Name       string
	PerMinute  int
	Timeout    time.Duration
	MaxRetries int
	UserAgent  string
	Logger     *slog.Logger
	HTTPClient *http.Client
}

type Client struct {
	name       string
	http       *http.Client
	limiter    *rate.Limiter
	maxRetries int
	userAgent  string
	logger     *slog.Logger
	sleep      func(ctx context.Context, d time.Duration) error
}

func New(o Options) *Client {
	if o.PerMinute <= 0 {
		o.PerMinute = 60
	}
	if o.Timeout <= 0 {
		o.Timeout = 20 * time.Second
	}
	if o.MaxRetries < 0 {
		o.MaxRetries = 0
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	hc := o.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: o.Timeout}
	}
	// небольшой запас на всплески, но не больше пятой части минутного лимита
	burst := max(1, o.PerMinute/5)
	return &Client{
		name:       o.Name,
		http:       hc,
		limiter:    rate.NewLimiter(rate.Every(time.Minute/time.Duration(o.PerMinute)), burst),
		maxRetries: o.MaxRetries,
		userAgent:  o.UserAgent,
		logger:     o.Logger,
		sleep:      sleepCtx,
	}
}

// StatusError — ответ с неуспешным статусом после всех повторов.
type StatusError struct {
	API    string
	Status int
	URL    string
	Body   string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %s → HTTP %d: %s", e.API, e.URL, e.Status, e.Body)
}

// IsStatus проверяет, что err — StatusError с указанным кодом.
func IsStatus(err error, status int) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Status == status
}

// Do выполняет запрос с ожиданием лимитера и повторами. newReq вызывается на
// каждую попытку: тело запроса нельзя прочитать дважды. Неуспешный статус
// возвращается как *StatusError, тело ответа в этом случае уже закрыто.
func (c *Client) Do(ctx context.Context, newReq func(ctx context.Context) (*http.Request, error)) (*http.Response, error) {
	for attempt := 0; ; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		req, err := newReq(ctx)
		if err != nil {
			return nil, err
		}
		if c.userAgent != "" {
			req.Header.Set("User-Agent", c.userAgent)
		}

		start := time.Now()
		resp, err := c.http.Do(req)
		link := Redact(req.URL)
		if err != nil {
			// *url.Error печатает полный адрес вместе с ключами
			var ue *url.Error
			if errors.As(err, &ue) {
				ue.URL = link
			}
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt >= c.maxRetries {
				return nil, fmt.Errorf("%s: %s: %w", c.name, link, err)
			}
			delay := backoff(attempt)
			c.logger.Warn("external api: network error, retrying", "api", c.name, "url", link, "err", err, "in", delay)
			if err := c.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		c.logger.Debug("external api", "api", c.name, "url", link, "status", resp.StatusCode, "dur", time.Since(start))
		if resp.StatusCode < 300 {
			return resp, nil
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBytes))
		_ = resp.Body.Close()
		statusErr := &StatusError{
			API:    c.name,
			Status: resp.StatusCode,
			URL:    link,
			Body:   strings.TrimSpace(string(body)),
		}
		if !retryable(resp.StatusCode) || attempt >= c.maxRetries {
			return nil, statusErr
		}
		delay := backoff(attempt)
		if ra := retryAfter(resp.Header.Get("Retry-After")); ra > 0 {
			delay = ra
		}
		c.logger.Warn("external api: retrying", "api", c.name, "url", link, "status", resp.StatusCode, "in", delay)
		if err := c.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
}

// GetJSON выполняет GET и разбирает JSON‑ответ в out.
func (c *Client) GetJSON(ctx context.Context, rawURL string, header http.Header, out any) error {
	resp, err := c.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		copyHeader(req.Header, header)
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	return c.decode(resp, out)
}

// PostJSON отправляет body как JSON и разбирает JSON‑ответ в out.
func (c *Client) PostJSON(ctx context.Context, rawURL string, header http.Header, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%s: encode request: %w", c.name, err)
	}
	resp, err := c.Do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		copyHeader(req.Header, header)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return req, nil
	})
	if err != nil {
		return err
	}
	return c.decode(resp, out)
}

func (c *Client) decode(resp *http.Response, out any) error {
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(out); err != nil {
		return fmt.Errorf("%s: decode %s: %w", c.name, Redact(resp.Request.URL), err)
	}
	return nil
}

func copyHeader(dst, src http.Header) {
	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff: 1с, 2с, 4с… до 15с плюс случайные до 250 мс, чтобы повторы не совпадали.
func backoff(attempt int) time.Duration {
	d := baseBackoff << attempt
	if d > maxBackoff || d <= 0 {
		d = maxBackoff
	}
	return d + time.Duration(rand.Int64N(int64(250*time.Millisecond)))
}

func retryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs <= 0 {
		return 0
	}
	return min(time.Duration(secs)*time.Second, maxRetryAfter)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// botTokenRe — токен Telegram стоит в пути: /bot<token>/sendMessage.
var botTokenRe = regexp.MustCompile(`^/bot[^/]+/`)

// Redact убирает из адреса ключи API, чтобы они не попали в логи и ошибки.
func Redact(u *url.URL) string {
	if u == nil {
		return ""
	}
	cp := *u
	if botTokenRe.MatchString(cp.Path) {
		cp.Path = botTokenRe.ReplaceAllString(cp.Path, "/botREDACTED/")
		cp.RawPath = ""
	}
	q := cp.Query()
	changed := false
	for _, k := range []string{"key", "api_key", "token"} {
		if q.Has(k) {
			q.Set(k, "***")
			changed = true
		}
	}
	if changed {
		cp.RawQuery = q.Encode()
	}
	return cp.String()
}
