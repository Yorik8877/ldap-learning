// Package auth — вход, выход и проверка сессии.
package auth

import (
	"log/slog"
	"net/http"

	"samba-admin/internal/api/httpjson"
)

type Handler struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// Login: POST /api/auth/login, тело LoginRequest → 200 CurrentUserResponse + cookie сессии.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Logout: POST /api/auth/logout → 204.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Me: GET /api/auth/me → 200 CurrentUserResponse.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// RequireSession пропускает запрос дальше только с действующей сессией, иначе 401 unauthorized.
// TODO: сейчас пропускает всех.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return next
}
