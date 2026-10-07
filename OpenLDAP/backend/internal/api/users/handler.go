package users

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

type Service interface {
	List(ctx context.Context) ([]user.User, error)
	Get(ctx context.Context, uid user.UID) (user.User, []group.Name, error)
	Create(ctx context.Context, created user.User, password string) error
	Update(ctx context.Context, updated user.User) error
	SetPassword(ctx context.Context, uid user.UID, password string) error
	Delete(ctx context.Context, actor, uid user.UID) error
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type userResponse struct {
	UID        string   `json:"uid"`
	CommonName string   `json:"cn"`
	Surname    string   `json:"sn"`
	Emails     []string `json:"mail"`
}

type userDetailsResponse struct {
	userResponse
	Groups []string `json:"groups"`
}

type createUserRequest struct {
	UID        string   `json:"uid"`
	CommonName string   `json:"cn"`
	Surname    string   `json:"sn"`
	Emails     []string `json:"mail"`
	Password   string   `json:"password"`
}

type updateUserRequest struct {
	CommonName string   `json:"cn"`
	Surname    string   `json:"sn"`
	Emails     []string `json:"mail"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	found, err := h.service.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	response := make([]userResponse, 0, len(found))
	for _, account := range found {
		response = append(response, toUserResponse(account))
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	found, groups, err := h.service.Get(r.Context(), uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	names := make([]string, 0, len(groups))
	for _, name := range groups {
		names = append(names, string(name))
	}
	httpjson.Write(w, http.StatusOK, userDetailsResponse{userResponse: toUserResponse(found), Groups: names})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	created, err := buildUser(request.UID, request.CommonName, request.Surname, request.Emails)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Create(r.Context(), created, request.Password); err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, toUserResponse(created))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var request updateUserRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	updated, err := buildUser(r.PathValue("uid"), request.CommonName, request.Surname, request.Emails)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Update(r.Context(), updated); err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toUserResponse(updated))
}

func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	var request passwordRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.SetPassword(r.Context(), uid, request.Password); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	actor, found := sessionctx.From(r.Context())
	if !found {
		h.fail(w, session.ErrNotFound)
		return
	}
	if err := h.service.Delete(r.Context(), actor.UID, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	httpjson.WriteError(w, h.logger, err)
}

func buildUser(rawUID, commonName, surname string, emails []string) (user.User, error) {
	uid, err := user.ParseUID(rawUID)
	if err != nil {
		return user.User{}, err
	}
	return user.New(uid, commonName, surname, emails)
}

func toUserResponse(account user.User) userResponse {
	emails := account.Emails
	if emails == nil {
		emails = []string{}
	}
	return userResponse{UID: string(account.UID), CommonName: account.CommonName, Surname: account.Surname, Emails: emails}
}
