// Package directory — HTTP-просмотр дерева каталога. DN передаётся query-параметром:
// в нём есть запятые и знаки «=», в пути URL он был бы неудобен.
package directory

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/domain/directory"
)

const dnParameter = "dn"

type Service interface {
	Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error)
	Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error)
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type nodeResponse struct {
	DN            string   `json:"dn"`
	RDN           string   `json:"rdn"`
	ObjectClasses []string `json:"objectClass"`
	HasChildren   bool     `json:"hasChildren"`
}

type entryResponse struct {
	DN                    string              `json:"dn"`
	Attributes            map[string][]string `json:"attributes"`
	OperationalAttributes map[string][]string `json:"operationalAttributes"`
}

func (h *Handler) Children(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.service.Children(r.Context(), requestedDN(r))
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	response := make([]nodeResponse, 0, len(nodes))
	for _, node := range nodes {
		response = append(response, nodeResponse{
			DN:            string(node.DN),
			RDN:           node.RDN,
			ObjectClasses: append([]string{}, node.ObjectClasses...),
			HasChildren:   node.HasChildren,
		})
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) Entry(w http.ResponseWriter, r *http.Request) {
	entry, err := h.service.Entry(r.Context(), requestedDN(r))
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	httpjson.Write(w, http.StatusOK, entryResponse{
		DN:                    string(entry.DN),
		Attributes:            entry.Attributes,
		OperationalAttributes: entry.OperationalAttributes,
	})
}

// requestedDN различает «параметра нет» (nil — корень каталога) и «параметр пустой»:
// пустой DN — это ввод клиента, и его проверит репозиторий.
func requestedDN(r *http.Request) *directory.DN {
	query := r.URL.Query()
	if !query.Has(dnParameter) {
		return nil
	}
	dn := directory.DN(query.Get(dnParameter))
	return &dn
}
