package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
})

func TestCheckOrigin(t *testing.T) {
	h := CheckOrigin("https://d2.example", "/api/payments/webhook/")(okHandler)

	cases := []struct {
		name   string
		method string
		path   string
		host   string
		origin string
		want   int
	}{
		{"GET без проверки", http.MethodGet, "/api/me", "d2.example", "https://evil.example", http.StatusNoContent},
		{"POST с PUBLIC_URL", http.MethodPost, "/api/feedback", "backend:8080", "https://d2.example", http.StatusNoContent},
		{"POST с тем же хостом", http.MethodPost, "/api/feedback", "localhost:5173", "http://localhost:5173", http.StatusNoContent},
		{"POST без Origin", http.MethodPost, "/api/feedback", "d2.example", "", http.StatusNoContent},
		{"POST с чужим Origin", http.MethodPost, "/api/feedback", "d2.example", "https://evil.example", http.StatusForbidden},
		{"POST с Origin null", http.MethodPost, "/api/feedback", "d2.example", "null", http.StatusForbidden},
		{"DELETE с чужим Origin", http.MethodDelete, "/api/party/me", "d2.example", "https://evil.example", http.StatusForbidden},
		{"вебхук без проверки", http.MethodPost, "/api/payments/webhook/yookassa", "d2.example", "https://pay.example", http.StatusNoContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Errorf("статус %d, ожидался %d", w.Code, tc.want)
			}
		})
	}
}

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	l := NewRateLimiter(3, time.Minute)
	l.now = func() time.Time { return now }

	for i := range 3 {
		if !l.Allow("1.1.1.1") {
			t.Fatalf("запрос %d должен пройти", i+1)
		}
	}
	if l.Allow("1.1.1.1") {
		t.Fatal("4-й запрос за минуту должен быть отклонён")
	}
	if !l.Allow("2.2.2.2") {
		t.Fatal("у другого IP своя квота")
	}

	now = now.Add(20 * time.Second) // восстановился один токен
	if !l.Allow("1.1.1.1") {
		t.Fatal("через 20 секунд должен пройти ещё один запрос")
	}

	now = now.Add(5 * time.Minute)
	l.Allow("3.3.3.3") // запускает очистку
	if _, ok := l.visitors["2.2.2.2"]; ok {
		t.Error("неактивный ключ должен удаляться")
	}
}

func TestLimitMutationsSkipsGET(t *testing.T) {
	l := NewRateLimiter(1, time.Minute)
	h := LimitMutations(l)(okHandler)

	for range 3 {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/me", nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("GET не должен ограничиваться, статус %d", w.Code)
		}
	}

	codes := make([]int, 2)
	for i := range codes {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/feedback", nil))
		codes[i] = w.Code
	}
	if codes[0] != http.StatusNoContent || codes[1] != http.StatusTooManyRequests {
		t.Errorf("статусы POST %v, ожидались [204 429]", codes)
	}
}
