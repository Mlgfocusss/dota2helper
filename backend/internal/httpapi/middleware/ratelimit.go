package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"d2helper/internal/httpapi/render"
)

// RateLimiter ограничивает число запросов с одного ключа (обычно IP)
// алгоритмом token bucket. Неактивные ключи периодически удаляются.
type RateLimiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	limit     rate.Limit
	burst     int
	idleTTL   time.Duration
	lastSweep time.Time
	now       func() time.Time
}

type visitor struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewRateLimiter разрешает не больше n запросов за период per с одного ключа.
func NewRateLimiter(n int, per time.Duration) *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    rate.Every(per / time.Duration(n)),
		burst:    n,
		// за per корзина полностью восстанавливается, поэтому ключ,
		// простаивающий дольше, можно забыть без изменения поведения
		idleTTL: per,
		now:     time.Now,
	}
}

func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastSweep) > l.idleTTL {
		for k, v := range l.visitors {
			if now.Sub(v.seen) > l.idleTTL {
				delete(l.visitors, k)
			}
		}
		l.lastSweep = now
	}

	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{lim: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[key] = v
	}
	v.seen = now
	return v.lim.AllowN(now, 1)
}

// LimitMutations ограничивает по IP запросы, меняющие данные (POST, PUT, PATCH, DELETE).
func LimitMutations(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) || l.Allow(ClientIP(r)) {
				next.ServeHTTP(w, r)
				return
			}
			TooManyRequests(w)
		})
	}
}

// Limit ограничивает по IP все запросы — для дорогих ручек.
func Limit(l *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if l.Allow(ClientIP(r)) {
				next.ServeHTTP(w, r)
				return
			}
			TooManyRequests(w)
		})
	}
}

func TooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	render.Error(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов. Подожди немного")
}
