package middleware

import (
	"net"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// ClientIP возвращает IP клиента: из X-Real-IP, который выставляет nginx
// (см. chimw.ClientIPFromHeader в роутере), иначе — адрес соединения.
func ClientIP(r *http.Request) string {
	if ip := chimw.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
