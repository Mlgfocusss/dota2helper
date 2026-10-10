package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"d2helper/internal/httpapi/render"
)

// CheckOrigin отклоняет меняющие данные запросы с чужим Origin. Вместе с
// cookie SameSite=Lax это защита от CSRF. Разрешены Origin, совпадающий с
// publicURL, и Origin с тем же хостом, что у запроса (nginx и Vite dev‑сервер
// передают исходный Host). Запросы без Origin приходят не из браузера —
// их пропускаем. Пути из exemptPrefixes (вебхуки платёжки) не проверяются.
func CheckOrigin(publicURL string, exemptPrefixes ...string) func(http.Handler) http.Handler {
	allowed := originOf(publicURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSafeMethod(r.Method) || hasAnyPrefix(r.URL.Path, exemptPrefixes) {
				next.ServeHTTP(w, r)
				return
			}
			origin := r.Header.Get("Origin")
			if origin == "" || (allowed != "" && strings.EqualFold(origin, allowed)) || sameHost(origin, r.Host) {
				next.ServeHTTP(w, r)
				return
			}
			render.Error(w, http.StatusForbidden, "bad_origin", "Запрос отклонён: неизвестный источник")
		})
	}
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func originOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func sameHost(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || host == "" {
		return false
	}
	return strings.EqualFold(u.Host, host)
}
