package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
)

// StateTTL — сколько живёт ссылка на вход: за это время нужно подтвердить вход в Steam.
const StateTTL = 10 * time.Minute

var ErrBadState = errors.New("auth: invalid or expired state")

// StateSigner подписывает state для входа через Steam: куда вернуть
// пользователя, срок действия и nonce, привязанный к cookie браузера
// (защита от подмены входа — login CSRF).
type StateSigner struct {
	key []byte
	now func() time.Time
}

func NewStateSigner(secret string) *StateSigner {
	// отдельный ключ, производный от SESSION_SECRET, чтобы не переиспользовать секрет напрямую
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("steam-openid-state"))
	return &StateSigner{key: mac.Sum(nil), now: time.Now}
}

type statePayload struct {
	ReturnTo string `json:"r"`
	Nonce    string `json:"n"`
	Expires  int64  `json:"e"`
}

func (s *StateSigner) Sign(returnTo, nonce string) string {
	payload, _ := json.Marshal(statePayload{
		ReturnTo: returnTo,
		Nonce:    nonce,
		Expires:  s.now().Add(StateTTL).Unix(),
	})
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + base64.RawURLEncoding.EncodeToString(s.sign(body))
}

// Verify проверяет подпись, срок и совпадение nonce с cookie; возвращает return_to.
func (s *StateSigner) Verify(state, nonce string) (string, error) {
	body, sig, ok := strings.Cut(state, ".")
	if !ok || nonce == "" {
		return "", ErrBadState
	}
	gotSig, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(gotSig, s.sign(body)) {
		return "", ErrBadState
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return "", ErrBadState
	}
	var p statePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", ErrBadState
	}
	if s.now().Unix() > p.Expires || subtle.ConstantTimeCompare([]byte(p.Nonce), []byte(nonce)) != 1 {
		return "", ErrBadState
	}
	return SafeReturnTo(p.ReturnTo), nil
}

func (s *StateSigner) sign(body string) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(body))
	return mac.Sum(nil)
}

// NewNonce — случайная строка для cookie входа.
func NewNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SafeReturnTo пропускает только относительные пути внутри сайта, иначе «/».
// Защита от open redirect: «//evil.com», «/\evil.com», «https://evil.com».
func SafeReturnTo(raw string) string {
	if raw == "" || len(raw) > 512 || !strings.HasPrefix(raw, "/") ||
		strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\") {
		return "/"
	}
	for _, r := range raw {
		if r < 0x20 || r == 0x7f {
			return "/"
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil {
		return "/"
	}
	return raw
}
