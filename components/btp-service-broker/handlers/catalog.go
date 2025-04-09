package handlers

import (
	"net/http"

	"github.com/go-logr/logr"
	"github.com/kyma-project/cfapi/routing"
	"github.com/kyma-project/cfapi/service_manager"
)

type Catalog struct {
	smClient service_manager.Client
}

func NewCatalog(smClient service_manager.Client) *Catalog {
	return &Catalog{smClient: smClient}
}

func (h *Catalog) get(r *http.Request) (*routing.Response, error) {
	logger := logr.FromContextOrDiscard(r.Context()).WithName("handlers.catalog.get")
	catalog, err := h.smClient.GetCatalog(r.Context())
	if err != nil {
		logger.Error(err, "failed to get catalog")
		return nil, err
	}

	return routing.NewResponse(http.StatusOK).WithBody(catalog), nil
}

func (h *Catalog) Routes() []routing.Route {
	return []routing.Route{
		{Method: "GET", Pattern: "/v2/catalog", Handler: h.get},
	}
}
