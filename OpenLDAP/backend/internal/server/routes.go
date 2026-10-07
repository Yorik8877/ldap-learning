package server

import (
	"log/slog"
	"net/http"
	"time"

	"ldap-admin/internal/api/auth"
	"ldap-admin/internal/api/directory"
	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/api/users"
)

type Handlers struct {
	Auth      *auth.Handler
	Users     *users.Handler
	Groups    *groups.Handler
	Directory *directory.Handler
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
		{"GET /api/users/{uid}", handlers.Users.Get},
		{"PUT /api/users/{uid}", handlers.Users.Update},
		{"PUT /api/users/{uid}/password", handlers.Users.SetPassword},
		{"DELETE /api/users/{uid}", handlers.Users.Delete},
		{"GET /api/groups", handlers.Groups.List},
		{"POST /api/groups", handlers.Groups.Create},
		{"GET /api/groups/{cn}", handlers.Groups.Get},
		{"DELETE /api/groups/{cn}", handlers.Groups.Delete},
		{"POST /api/groups/{cn}/members", handlers.Groups.AddMember},
		{"DELETE /api/groups/{cn}/members/{uid}", handlers.Groups.RemoveMember},
		{"GET /api/directory/children", handlers.Directory.Children},
		{"GET /api/directory/entry", handlers.Directory.Entry},
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
