// Package auth — вход, выход и проверка сессии.
package auth

import (
	"log/slog"
	"net/http"

	"samba-admin/internal/api/httpjson"
	"samba-admin/internal/domain/session"
)

type authService interface {
	Current(id string) (session.Session, error)
	Login(login string, password string) (session.Session, error)
	Logout(id string) error
}

type Handler struct {
	logger *slog.Logger
	svc    authService
}

func New(logger *slog.Logger, service authService) *Handler {
	return &Handler{logger: logger, svc: service}
}

// Login: POST /api/auth/login, тело LoginRequest → 200 CurrentUserResponse + cookie сессии.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}

	opened, err := h.svc.Login(request.Login, request.Password)
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}

	// Cookie — это заголовок: его нужно поставить до первой записи статуса или тела.
	setSessionCookie(w, opened)
	httpjson.Write(w, http.StatusOK, currentUserResponse(opened))
}

// Logout: POST /api/auth/logout → 204. Маршрут защищён RequireSession.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	current, found := sessionFromContext(r.Context())
	if !found {
		httpjson.WriteError(w, h.logger, session.ErrNotFound)
		return
	}

	if err := h.svc.Logout(current.ID); err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}

	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// Me: GET /api/auth/me → 200 CurrentUserResponse. Маршрут защищён RequireSession.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	current, found := sessionFromContext(r.Context())
	if !found {
		httpjson.WriteError(w, h.logger, session.ErrNotFound)
		return
	}

	httpjson.Write(w, http.StatusOK, currentUserResponse(current))
}

// RequireSession пропускает запрос дальше только с действующей сессией, иначе 401 unauthorized.
// Найденная сессия кладётся в контекст запроса: обработчики берут её через sessionFromContext.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			httpjson.WriteError(w, h.logger, session.ErrNotFound)
			return
		}

		current, err := h.svc.Current(cookie.Value)
		if err != nil {
			httpjson.WriteError(w, h.logger, err)
			return
		}

		next.ServeHTTP(w, r.WithContext(contextWithSession(r.Context(), current)))
	})
}

func currentUserResponse(current session.Session) CurrentUserResponse {
	return CurrentUserResponse{Login: current.Login, DisplayName: current.DisplayName}
}
