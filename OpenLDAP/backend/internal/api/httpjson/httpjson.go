// Package httpjson пишет JSON-ответы и переводит доменные ошибки в HTTP-коды.
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

const maxBodyBytes = 1 << 20

var ErrMalformedBody = errors.New("malformed request body")

type ErrorBody struct {
	Error   string   `json:"error"`
	Message string   `json:"message"`
	Groups  []string `json:"groups,omitempty"`
}

type errorRule struct {
	target error
	status int
	code   string
	// detailed — в ответ уходит полный текст ошибки, а не только текст sentinel.
	// Так только для ошибок ввода: их текст собирает домен, деталей драйвера в нём нет.
	detailed bool
}

var errorRules = []errorRule{
	{target: user.ErrInvalid, status: http.StatusBadRequest, code: "invalid_input", detailed: true},
	{target: group.ErrInvalid, status: http.StatusBadRequest, code: "invalid_input", detailed: true},
	{target: ErrMalformedBody, status: http.StatusBadRequest, code: "invalid_input", detailed: true},
	{target: directory.ErrOutsideBase, status: http.StatusBadRequest, code: "invalid_input"},
	{target: directory.ErrInvalidDN, status: http.StatusBadRequest, code: "invalid_input"},
	{target: session.ErrNotFound, status: http.StatusUnauthorized, code: "unauthenticated"},
	{target: session.ErrInvalidCredentials, status: http.StatusUnauthorized, code: "invalid_credentials"},
	{target: session.ErrNotAdmin, status: http.StatusForbidden, code: "not_admin"},
	{target: user.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: group.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: directory.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: group.ErrNotMember, status: http.StatusNotFound, code: "not_found"},
	{target: user.ErrAlreadyExists, status: http.StatusConflict, code: "already_exists"},
	{target: group.ErrAlreadyExists, status: http.StatusConflict, code: "already_exists"},
	{target: group.ErrAlreadyMember, status: http.StatusConflict, code: "already_exists"},
	{target: group.ErrLastMember, status: http.StatusConflict, code: "last_member"},
	{target: group.ErrProtected, status: http.StatusConflict, code: "protected"},
	{target: user.ErrSelfDelete, status: http.StatusConflict, code: "self_delete"},
	{target: directory.ErrUnavailable, status: http.StatusServiceUnavailable, code: "directory_unavailable"},
}

func Write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func Decode(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedBody, err)
	}
	return nil
}

func WriteError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var soleMember *group.SoleMemberError
	if errors.As(err, &soleMember) {
		Write(w, http.StatusConflict, ErrorBody{Error: "sole_member", Message: err.Error(), Groups: groupNames(soleMember.Groups)})
		return
	}
	for _, rule := range errorRules {
		if !errors.Is(err, rule.target) {
			continue
		}
		if rule.status >= http.StatusInternalServerError {
			logger.Error("request failed", "error", err)
		}
		Write(w, rule.status, ErrorBody{Error: rule.code, Message: messageFor(rule, err)})
		return
	}
	logger.Error("unexpected error", "error", err)
	Write(w, http.StatusInternalServerError, ErrorBody{Error: "internal", Message: "internal server error"})
}

func messageFor(rule errorRule, err error) string {
	if rule.detailed {
		return err.Error()
	}
	return rule.target.Error()
}

func groupNames(names []group.Name) []string {
	converted := make([]string, 0, len(names))
	for _, name := range names {
		converted = append(converted, string(name))
	}
	return converted
}
