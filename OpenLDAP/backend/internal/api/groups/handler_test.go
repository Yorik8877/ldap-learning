package groups_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeService struct {
	created   []group.Group
	removeErr error
	added     []user.UID
}

func (f *fakeService) List(_ context.Context) ([]group.Group, error) {
	return []group.Group{{Name: "admins", Members: []group.Member{{UID: "alice"}}}}, nil
}

func (f *fakeService) Get(_ context.Context, name group.Name) (group.Group, error) {
	return group.Group{Name: name, Members: []group.Member{{DN: "uid=alice,ou=people,dc=example,dc=com", UID: "alice"}}}, nil
}

func (f *fakeService) Create(_ context.Context, created group.Group) (group.Group, error) {
	f.created = append(f.created, created)
	return created, nil
}

func (f *fakeService) Delete(_ context.Context, name group.Name) error {
	if name == "admins" {
		return group.ErrProtected
	}
	return nil
}

func (f *fakeService) AddMember(_ context.Context, _ group.Name, uid user.UID) error {
	f.added = append(f.added, uid)
	return nil
}

func (f *fakeService) RemoveMember(_ context.Context, _ group.Name, _ user.UID) error {
	return f.removeErr
}

func serve(handler http.HandlerFunc, method, name, uid, payload string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/groups", strings.NewReader(payload))
	request.SetPathValue("cn", name)
	request.SetPathValue("uid", uid)
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

func TestListReturnsMemberCount(t *testing.T) {
	handler := groups.New(&fakeService{}, silentLogger)

	recorder := serve(handler.List, http.MethodGet, "", "", "")

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"memberCount":1`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestGetReturnsMembersWithDN(t *testing.T) {
	handler := groups.New(&fakeService{}, silentLogger)

	recorder := serve(handler.Get, http.MethodGet, "admins", "", "")

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"dn":"uid=alice,ou=people,dc=example,dc=com"`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestCreate(t *testing.T) {
	service := &fakeService{}
	handler := groups.New(service, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "", "", `{"cn":"team","description":"Team","members":["alice","bob"]}`)

	if recorder.Code != http.StatusCreated || len(service.created) != 1 || len(service.created[0].Members) != 2 {
		t.Fatalf("status = %d, created = %+v", recorder.Code, service.created)
	}
}

func TestCreateRejectsInvalidGroups(t *testing.T) {
	payloads := []string{
		`{"cn":"team","members":[]}`,
		`{"cn":"team","members":["alice","alice"]}`,
		`{"cn":"Team","members":["alice"]}`,
		`{"cn":"team","members":["Alice"]}`,
	}
	for _, payload := range payloads {
		service := &fakeService{}
		recorder := serve(groups.New(service, silentLogger).Create, http.MethodPost, "", "", payload)
		if recorder.Code != http.StatusBadRequest || len(service.created) != 0 {
			t.Errorf("payload %s: status = %d, created = %v", payload, recorder.Code, service.created)
		}
	}
}

func TestDeleteProtectedGroup(t *testing.T) {
	handler := groups.New(&fakeService{}, silentLogger)

	if recorder := serve(handler.Delete, http.MethodDelete, "admins", "", ""); recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if recorder := serve(handler.Delete, http.MethodDelete, "team", "", ""); recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

func TestMembers(t *testing.T) {
	service := &fakeService{removeErr: group.ErrLastMember}
	handler := groups.New(service, silentLogger)

	added := serve(handler.AddMember, http.MethodPost, "team", "", `{"uid":"bob"}`)
	removed := serve(handler.RemoveMember, http.MethodDelete, "team", "alice", "")

	if added.Code != http.StatusNoContent || len(service.added) != 1 || service.added[0] != "bob" {
		t.Errorf("add: status = %d, added = %v", added.Code, service.added)
	}
	if removed.Code != http.StatusConflict || !strings.Contains(removed.Body.String(), "last_member") {
		t.Errorf("remove: status = %d, body = %s", removed.Code, removed.Body)
	}
}
