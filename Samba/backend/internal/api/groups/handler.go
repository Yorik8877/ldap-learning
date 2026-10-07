// Package groups — управление группами безопасности. {name} в пути — cn группы, {login} — sAMAccountName.
package groups

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

// List: GET /api/groups → 200 []SummaryResponse.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Create: POST /api/groups, тело CreateRequest → 201 DetailsResponse.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Get: GET /api/groups/{name} → 200 DetailsResponse.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Delete: DELETE /api/groups/{name} → 204.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// AddMember: PUT /api/groups/{name}/members/{login} → 204.
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// RemoveMember: DELETE /api/groups/{name}/members/{login} → 204.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}
