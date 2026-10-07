package server

import (
	"log/slog"
	"net/http"
	"time"

	"samba-admin/internal/api/auth"
	"samba-admin/internal/api/groups"
	"samba-admin/internal/api/users"
)

type Handlers struct {
	Auth   *auth.Handler
	Users  *users.Handler
	Groups *groups.Handler
}

type route struct {
	pattern string
	handler http.HandlerFunc
}

func NewHandler(handlers Handlers, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", handlers.Auth.Login)
	for _, protected := range protectedRoutes(handlers) {
		mux.Handle(protected.pattern, handlers.Auth.RequireSession(protected.handler))
	}
	return logRequests(logger, mux)
}

func protectedRoutes(handlers Handlers) []route {
	return []route{
		{"POST /api/auth/logout", handlers.Auth.Logout},
		{"GET /api/auth/me", handlers.Auth.Me},
		{"GET /api/users", handlers.Users.List},
		{"POST /api/users", handlers.Users.Create},
		{"GET /api/users/{login}", handlers.Users.Get},
		{"PUT /api/users/{login}", handlers.Users.Update},
		{"PUT /api/users/{login}/password", handlers.Users.SetPassword},
		{"PUT /api/users/{login}/enabled", handlers.Users.SetEnabled},
		{"DELETE /api/users/{login}", handlers.Users.Delete},
		{"GET /api/groups", handlers.Groups.List},
		{"POST /api/groups", handlers.Groups.Create},
		{"GET /api/groups/{name}", handlers.Groups.Get},
		{"DELETE /api/groups/{name}", handlers.Groups.Delete},
		{"PUT /api/groups/{name}/members/{login}", handlers.Groups.AddMember},
		{"DELETE /api/groups/{name}/members/{login}", handlers.Groups.RemoveMember},
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("request",
			"method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration", time.Since(started))
	})
}
