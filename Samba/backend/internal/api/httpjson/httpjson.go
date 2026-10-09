// Package httpjson читает и пишет JSON и переводит доменные ошибки в HTTP-ответы.
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"samba-admin/internal/domain/group"
	"samba-admin/internal/domain/session"
	"samba-admin/internal/domain/user"
)

const maxBodyBytes = 1 << 20

var ErrMalformedBody = errors.New("malformed request body")

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorRule struct {
	target error
	status int
	code   string
	// detailed — в ответ уходит полный текст ошибки, а не только текст sentinel.
	// Только для ошибок ввода: в их тексте нет деталей драйвера LDAP.
	detailed bool
}

// errorRules — таблица «доменная ошибка → ответ». Новая доменная ошибка — новая строка.
// Ошибка не из таблицы становится 500 internal: её текст пишется в лог, клиенту не уходит.
var errorRules = []errorRule{
	{target: ErrMalformedBody, status: http.StatusBadRequest, code: "invalid_request", detailed: true},
	{target: user.ErrNotFound, status: http.StatusNotFound, code: "not_found", detailed: false},
	{target: user.ErrWrongLoginOrPassword, status: http.StatusUnauthorized, code: "unauthorized", detailed: false},
	{target: user.ErrNoAdminPrivilege, status: http.StatusForbidden, code: "forbidden", detailed: false},
	{target: session.ErrNotFound, status: http.StatusUnauthorized, code: "unauthorized", detailed: false},
	{target: session.ErrExpired, status: http.StatusUnauthorized, code: "unauthorized", detailed: false},
	{target: group.ErrNotFound, status: http.StatusNotFound, code: "not_found", detailed: false},
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
	for _, rule := range errorRules {
		if !errors.Is(err, rule.target) {
			continue
		}
		if rule.status >= http.StatusInternalServerError {
			logger.Error("request failed", "error", err)
		}
		Write(w, rule.status, ErrorBody{Code: rule.code, Message: messageFor(rule, err)})
		return
	}
	logger.Error("unexpected error", "error", err)
	Write(w, http.StatusInternalServerError, ErrorBody{Code: "internal", Message: "internal server error"})
}

func NotImplemented(w http.ResponseWriter) {
	Write(w, http.StatusNotImplemented, ErrorBody{Code: "not_implemented", Message: "not implemented yet"})
}

func messageFor(rule errorRule, err error) string {
	if rule.detailed {
		return err.Error()
	}
	return rule.target.Error()
}
