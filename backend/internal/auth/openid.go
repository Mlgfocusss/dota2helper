// Package auth — вход через Steam OpenID 2.0 и сессии пользователей.
package auth

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	SteamOpenIDEndpoint = "https://steamcommunity.com/openid/login"
	openIDNS            = "http://specs.openid.net/auth/2.0"
	identifierSelect    = "http://specs.openid.net/auth/2.0/identifier_select"
)

var (
	// ErrCancelled — пользователь отменил вход на стороне Steam.
	ErrCancelled = errors.New("openid: cancelled by user")
	// ErrInvalidResponse — ответ Steam не прошёл проверку.
	ErrInvalidResponse = errors.New("openid: invalid response")

	claimedIDRe = regexp.MustCompile(`^https?://steamcommunity\.com/openid/id/(\d{17})$`)

	// Поля, которые обязаны быть подписаны: иначе их можно подменить в ссылке.
	requiredSigned = []string{"op_endpoint", "claimed_id", "identity", "return_to", "response_nonce"}
)

// Doer — HTTP‑клиент (подменяется в тестах).
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type OpenID struct {
	endpoint string
	http     Doer
}

func NewOpenID(httpClient Doer) *OpenID {
	return &OpenID{endpoint: SteamOpenIDEndpoint, http: httpClient}
}

// LoginURL — адрес страницы входа Steam.
func (o *OpenID) LoginURL(realm, returnTo string) string {
	q := url.Values{
		"openid.ns":         {openIDNS},
		"openid.mode":       {"checkid_setup"},
		"openid.identity":   {identifierSelect},
		"openid.claimed_id": {identifierSelect},
		"openid.realm":      {realm},
		"openid.return_to":  {returnTo},
	}
	return o.endpoint + "?" + q.Encode()
}

// Verify проверяет ответ Steam на нашем колбэке и возвращает SteamID64.
// expectedReturnTo — адрес колбэка, который мы передавали в LoginURL.
func (o *OpenID) Verify(ctx context.Context, query url.Values, expectedReturnTo string) (string, error) {
	switch mode := query.Get("openid.mode"); mode {
	case "cancel":
		return "", ErrCancelled
	case "id_res":
	default:
		return "", fmt.Errorf("%w: mode %q", ErrInvalidResponse, mode)
	}
	if query.Get("openid.return_to") != expectedReturnTo {
		return "", fmt.Errorf("%w: return_to mismatch", ErrInvalidResponse)
	}
	if query.Get("openid.op_endpoint") != o.endpoint {
		return "", fmt.Errorf("%w: unexpected op_endpoint", ErrInvalidResponse)
	}
	claimed := query.Get("openid.claimed_id")
	m := claimedIDRe.FindStringSubmatch(claimed)
	if m == nil || query.Get("openid.identity") != claimed {
		return "", fmt.Errorf("%w: bad claimed_id", ErrInvalidResponse)
	}
	signed := strings.Split(query.Get("openid.signed"), ",")
	for _, field := range requiredSigned {
		if !contains(signed, field) {
			return "", fmt.Errorf("%w: %s is not signed", ErrInvalidResponse, field)
		}
	}

	// Проверка подписи у самого Steam: те же параметры, mode = check_authentication
	form := url.Values{}
	for k, v := range query {
		if strings.HasPrefix(k, "openid.") {
			form[k] = v
		}
	}
	form.Set("openid.mode", "check_authentication")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := o.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openid: check_authentication: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openid: check_authentication: HTTP %d", resp.StatusCode)
	}
	if !isValidResponse(io.LimitReader(resp.Body, 64<<10)) {
		return "", fmt.Errorf("%w: signature rejected by Steam", ErrInvalidResponse)
	}
	return m[1], nil
}

func isValidResponse(r io.Reader) bool {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "is_valid:true" {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
