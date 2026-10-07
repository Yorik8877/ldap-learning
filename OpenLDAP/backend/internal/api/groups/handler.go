package groups

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type Service interface {
	List(ctx context.Context) ([]group.Group, error)
	Get(ctx context.Context, name group.Name) (group.Group, error)
	Create(ctx context.Context, created group.Group) (group.Group, error)
	Delete(ctx context.Context, name group.Name) error
	AddMember(ctx context.Context, name group.Name, uid user.UID) error
	RemoveMember(ctx context.Context, name group.Name, uid user.UID) error
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type groupSummaryResponse struct {
	Name        string `json:"cn"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}

type memberResponse struct {
	DN  string `json:"dn"`
	UID string `json:"uid"`
}

type groupResponse struct {
	Name        string           `json:"cn"`
	Description string           `json:"description"`
	Members     []memberResponse `json:"members"`
}

type createGroupRequest struct {
	Name        string   `json:"cn"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

type memberRequest struct {
	UID string `json:"uid"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	found, err := h.service.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	response := make([]groupSummaryResponse, 0, len(found))
	for _, stored := range found {
		response = append(response, groupSummaryResponse{
			Name: string(stored.Name), Description: stored.Description, MemberCount: len(stored.Members),
		})
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	found, err := h.service.Get(r.Context(), name)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toGroupResponse(found))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createGroupRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	draft, err := buildGroup(request)
	if err != nil {
		h.fail(w, err)
		return
	}
	created, err := h.service.Create(r.Context(), draft)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, toGroupResponse(created))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), name); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	var request memberRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	uid, err := user.ParseUID(request.UID)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.AddMember(r.Context(), name, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.RemoveMember(r.Context(), name, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	httpjson.WriteError(w, h.logger, err)
}

func buildGroup(request createGroupRequest) (group.Group, error) {
	name, err := group.ParseName(request.Name)
	if err != nil {
		return group.Group{}, err
	}
	members := make([]group.Member, 0, len(request.Members))
	for _, rawUID := range request.Members {
		uid, err := user.ParseUID(rawUID)
		if err != nil {
			return group.Group{}, err
		}
		members = append(members, group.Member{UID: uid})
	}
	return group.New(name, request.Description, members)
}

func toGroupResponse(stored group.Group) groupResponse {
	members := make([]memberResponse, 0, len(stored.Members))
	for _, member := range stored.Members {
		members = append(members, memberResponse{DN: string(member.DN), UID: string(member.UID)})
	}
	return groupResponse{Name: string(stored.Name), Description: stored.Description, Members: members}
}
