package httpjson_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func writeError(t *testing.T, err error) (int, httpjson.ErrorBody) {
	t.Helper()
	recorder := httptest.NewRecorder()
	httpjson.WriteError(recorder, silentLogger, err)
	var body httpjson.ErrorBody
	if decodeErr := json.NewDecoder(recorder.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode body: %v", decodeErr)
	}
	return recorder.Code, body
}

func TestWriteErrorMapsDomainErrors(t *testing.T) {
	driverFailure := errors.New("dial tcp 127.0.0.1:389: connection refused")
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"invalid input keeps details", fmt.Errorf("%w: surname is required", user.ErrInvalid), 400, "invalid_input", "invalid user data: surname is required"},
		{"dn outside base", directory.ErrOutsideBase, 400, "invalid_input", "dn is outside the directory base"},
		{"malformed dn", directory.ErrInvalidDN, 400, "invalid_input", "invalid dn"},
		{"no session", session.ErrNotFound, 401, "unauthenticated", "session not found or expired"},
		{"bad credentials", session.ErrInvalidCredentials, 401, "invalid_credentials", "invalid uid or password"},
		{"not admin", session.ErrNotAdmin, 403, "not_admin", "user is not a member of the admins group"},
		{"user missing", user.ErrNotFound, 404, "not_found", "user not found"},
		{"not a member", group.ErrNotMember, 404, "not_found", "user is not a member of the group"},
		{"already member", group.ErrAlreadyMember, 409, "already_exists", "user is already a member of the group"},
		{"last member", group.ErrLastMember, 409, "last_member", "cannot remove the last member of a group"},
		{"protected group", group.ErrProtected, 409, "protected", "this group is protected"},
		{"self delete", user.ErrSelfDelete, 409, "self_delete", "you cannot delete yourself"},
		{"directory down hides driver text", fmt.Errorf("%w: %w", directory.ErrUnavailable, driverFailure), 503, "directory_unavailable", "directory unavailable"},
		{"unknown error hides details", errors.New("ldap: service account bind failed: details"), 500, "internal", "internal server error"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			status, body := writeError(t, testCase.err)
			if status != testCase.status || body.Error != testCase.code || body.Message != testCase.message {
				t.Fatalf("WriteError() = %d %+v, want %d %s %q", status, body, testCase.status, testCase.code, testCase.message)
			}
		})
	}
}

func TestWriteErrorListsSoleMemberGroups(t *testing.T) {
	status, body := writeError(t, &group.SoleMemberError{Groups: []group.Name{"solo"}})

	if status != http.StatusConflict || body.Error != "sole_member" || !slices.Equal(body.Groups, []string{"solo"}) {
		t.Fatalf("WriteError() = %d %+v", status, body)
	}
}

func TestDecodeRejectsUnknownFieldsAndBrokenJSON(t *testing.T) {
	for _, payload := range []string{`{"uid":"a","extra":1}`, `{"uid":`} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
		var target struct {
			UID string `json:"uid"`
		}
		err := httpjson.Decode(httptest.NewRecorder(), request, &target)
		if !errors.Is(err, httpjson.ErrMalformedBody) {
			t.Errorf("Decode(%s) error = %v, want ErrMalformedBody", payload, err)
		}
	}
}
