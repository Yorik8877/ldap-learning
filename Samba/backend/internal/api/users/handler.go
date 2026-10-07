// Package users — управление пользователями домена. {login} в пути — sAMAccountName.
package users

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

// List: GET /api/users → 200 []SummaryResponse.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Create: POST /api/users, тело CreateRequest → 201 DetailsResponse.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Get: GET /api/users/{login} → 200 DetailsResponse.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Update: PUT /api/users/{login}, тело UpdateRequest → 200 DetailsResponse.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// SetPassword: PUT /api/users/{login}/password, тело PasswordRequest → 204.
func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// SetEnabled: PUT /api/users/{login}/enabled, тело EnabledRequest → 204.
func (h *Handler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Delete: DELETE /api/users/{login} → 204.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}
