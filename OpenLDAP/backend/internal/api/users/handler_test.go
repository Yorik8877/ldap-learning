package users_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/api/users"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeService struct {
	users       []user.User
	created     []user.User
	createErr   error
	deleteErr   error
	deleteCalls [][2]user.UID
}

func (f *fakeService) List(_ context.Context) ([]user.User, error) { return f.users, nil }

func (f *fakeService) Get(_ context.Context, uid user.UID) (user.User, []group.Name, error) {
	for _, stored := range f.users {
		if stored.UID == uid {
			return stored, []group.Name{"admins"}, nil
		}
	}
	return user.User{}, nil, user.ErrNotFound
}

func (f *fakeService) Create(_ context.Context, created user.User, _ string) error {
	f.created = append(f.created, created)
	return f.createErr
}

func (f *fakeService) Update(_ context.Context, _ user.User) error { return nil }

func (f *fakeService) SetPassword(_ context.Context, _ user.UID, _ string) error { return nil }

func (f *fakeService) Delete(_ context.Context, actor, uid user.UID) error {
	f.deleteCalls = append(f.deleteCalls, [2]user.UID{actor, uid})
	return f.deleteErr
}

func serve(handler http.HandlerFunc, method, uid, payload string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/users", strings.NewReader(payload))
	request.SetPathValue("uid", uid)
	request = request.WithContext(sessionctx.With(request.Context(), session.Session{UID: "alice"}))
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

func TestListReturnsEmptyArrayNotNull(t *testing.T) {
	handler := users.New(&fakeService{}, silentLogger)

	recorder := serve(handler.List, http.MethodGet, "", "")

	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestGetReturnsGroups(t *testing.T) {
	service := &fakeService{users: []user.User{{UID: "jdoe", CommonName: "John", Surname: "Doe", Emails: []string{}}}}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Get, http.MethodGet, "jdoe", "")

	var body map[string]any
	_ = json.NewDecoder(recorder.Body).Decode(&body)
	if recorder.Code != http.StatusOK || body["uid"] != "jdoe" || body["groups"] == nil {
		t.Fatalf("status = %d, body = %v", recorder.Code, body)
	}
	if missing := serve(handler.Get, http.MethodGet, "ghost", ""); missing.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d, want 404", missing.Code)
	}
}

func TestCreate(t *testing.T) {
	service := &fakeService{}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "",
		`{"uid":"jdoe","cn":"Doe, John","sn":"Doe","mail":["jdoe@example.com"],"password":"correct-horse"}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	if len(service.created) != 1 || service.created[0].CommonName != "Doe, John" {
		t.Fatalf("created = %+v", service.created)
	}
}

func TestCreateRejectsInvalidInputBeforeService(t *testing.T) {
	service := &fakeService{}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "", `{"uid":"John","cn":"John","sn":"Doe","password":"correct-horse"}`)

	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_input") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	if len(service.created) != 0 {
		t.Fatalf("service was called with invalid input")
	}
}

func TestCreateDuplicateIsConflict(t *testing.T) {
	handler := users.New(&fakeService{createErr: user.ErrAlreadyExists}, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "", `{"uid":"jdoe","cn":"John","sn":"Doe","password":"correct-horse"}`)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
}

func TestDeletePassesActorFromSession(t *testing.T) {
	service := &fakeService{}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Delete, http.MethodDelete, "jdoe", "")

	if recorder.Code != http.StatusNoContent || len(service.deleteCalls) != 1 || service.deleteCalls[0] != [2]user.UID{"alice", "jdoe"} {
		t.Fatalf("status = %d, calls = %v", recorder.Code, service.deleteCalls)
	}
}

func TestDeleteSoleMemberListsGroups(t *testing.T) {
	handler := users.New(&fakeService{deleteErr: &group.SoleMemberError{Groups: []group.Name{"solo"}}}, silentLogger)

	recorder := serve(handler.Delete, http.MethodDelete, "jdoe", "")

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"groups":["solo"]`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}
