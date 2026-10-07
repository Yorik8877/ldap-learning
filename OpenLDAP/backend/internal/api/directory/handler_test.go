package directory_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apidirectory "ldap-admin/internal/api/directory"
	"ldap-admin/internal/domain/directory"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeService struct {
	requested []*directory.DN
}

func (f *fakeService) Children(_ context.Context, dn *directory.DN) ([]directory.Node, error) {
	f.requested = append(f.requested, dn)
	if dn != nil && *dn == "cn=config" {
		return nil, directory.ErrOutsideBase
	}
	return []directory.Node{{DN: "ou=people,dc=example,dc=com", RDN: "ou=people", HasChildren: true}}, nil
}

func (f *fakeService) Entry(_ context.Context, dn *directory.DN) (directory.Entry, error) {
	f.requested = append(f.requested, dn)
	return directory.Entry{
		DN:                    "dc=example,dc=com",
		Attributes:            map[string][]string{"o": {"Example"}},
		OperationalAttributes: map[string][]string{"entryUUID": {"1b6d"}},
	}, nil
}

func get(handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestChildrenWithoutDNAsksForBase(t *testing.T) {
	service := &fakeService{}
	handler := apidirectory.New(service, silentLogger)

	recorder := get(handler.Children, "/api/directory/children")

	if recorder.Code != http.StatusOK || service.requested[0] != nil {
		t.Fatalf("status = %d, requested = %v", recorder.Code, service.requested)
	}
	if !strings.Contains(recorder.Body.String(), `"objectClass":[]`) || !strings.Contains(recorder.Body.String(), `"hasChildren":true`) {
		t.Fatalf("body = %s", recorder.Body)
	}
}

func TestChildrenPassesEscapedDN(t *testing.T) {
	service := &fakeService{}
	handler := apidirectory.New(service, silentLogger)

	get(handler.Children, "/api/directory/children?dn=ou%3Dpeople%2Cdc%3Dexample%2Cdc%3Dcom")

	if service.requested[0] == nil || *service.requested[0] != "ou=people,dc=example,dc=com" {
		t.Fatalf("requested = %v", service.requested)
	}
}

func TestChildrenOutsideBaseIsBadRequest(t *testing.T) {
	handler := apidirectory.New(&fakeService{}, silentLogger)

	if recorder := get(handler.Children, "/api/directory/children?dn=cn%3Dconfig"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestEntrySplitsAttributes(t *testing.T) {
	handler := apidirectory.New(&fakeService{}, silentLogger)

	recorder := get(handler.Entry, "/api/directory/entry")

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"attributes":{"o":["Example"]}`) ||
		!strings.Contains(body, `"operationalAttributes":{"entryUUID":["1b6d"]}`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, body)
	}
}
