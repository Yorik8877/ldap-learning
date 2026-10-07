// Package auth — HTTP-вход: выдача cookie сессии и middleware, пускающий дальше
// только с действующей сессией.
package auth

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/domain/session"
)

const (
	CookieName = "ldap_admin_session"
	cookiePath = "/api"
)

type Service interface {
	Login(ctx context.Context, uid, password string) (session.Session, error)
	Authenticate(ctx context.Context, id string) (session.Session, error)
	Logout(ctx context.Context, id string) error
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type loginRequest struct {
	UID      string `json:"uid"`
	Password string `json:"password"`
}

type currentUserResponse struct {
	UID        string `json:"uid"`
	CommonName string `json:"cn"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	started, err := h.service.Login(r.Context(), request.UID, request.Password)
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	http.SetCookie(w, sessionCookie(started))
	httpjson.Write(w, http.StatusOK, toCurrentUser(started))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	current, found := sessionctx.From(r.Context())
	if !found {
		httpjson.WriteError(w, h.logger, session.ErrNotFound)
		return
	}
	if err := h.service.Logout(r.Context(), current.ID); err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	http.SetCookie(w, expiredCookie())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	current, found := sessionctx.From(r.Context())
	if !found {
		httpjson.WriteError(w, h.logger, session.ErrNotFound)
		return
	}
	httpjson.Write(w, http.StatusOK, toCurrentUser(current))
}

func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil {
			httpjson.WriteError(w, h.logger, session.ErrNotFound)
			return
		}
		current, err := h.service.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			httpjson.WriteError(w, h.logger, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(sessionctx.With(r.Context(), current)))
	})
}

// sessionCookie недоступна скриптам страницы (HttpOnly) и не уходит с запросами
// с чужих сайтов (SameSite=Strict) — этого достаточно вместо отдельной CSRF-защиты.
func sessionCookie(started session.Session) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    started.ID,
		Path:     cookiePath,
		Expires:  started.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func expiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Path:     cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func toCurrentUser(current session.Session) currentUserResponse {
	return currentUserResponse{UID: string(current.UID), CommonName: current.CommonName}
}
