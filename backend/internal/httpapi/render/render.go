// Package render пишет JSON‑ответы API в едином формате.
package render

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody — формат ошибок API: { "error": { "code": "...", "message": "..." } }.
type ErrorBody struct {
	Error ErrorInfo `json:"error"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Fields — ошибки по полям формы: имя поля → текст (только для validation_failed)
	Fields map[string]string `json:"fields,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write json response", "err", err)
	}
}

// Error отвечает ошибкой; message показывается пользователю как есть.
func Error(w http.ResponseWriter, status int, code, message string) {
	JSON(w, status, ErrorBody{Error: ErrorInfo{Code: code, Message: message}})
}

// ValidationFailed отвечает 400 с ошибками по полям формы.
func ValidationFailed(w http.ResponseWriter, fields map[string]string) {
	JSON(w, http.StatusBadRequest, ErrorBody{Error: ErrorInfo{
		Code:    "validation_failed",
		Message: "Проверь заполнение анкеты",
		Fields:  fields,
	}})
}

func NotFound(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusNotFound, "not_found", "Не найдено")
}

func MethodNotAllowed(w http.ResponseWriter, _ *http.Request) {
	Error(w, http.StatusMethodNotAllowed, "method_not_allowed", "Метод не поддерживается")
}

func Internal(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, "internal", "Что-то пошло не так. Попробуй ещё раз")
}
