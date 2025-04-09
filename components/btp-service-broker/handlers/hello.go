package handlers

import (
	"net/http"

	"github.com/kyma-project/cfapi/routing"
)

type Hello struct{}

func NewHello() *Hello {
	return &Hello{}
}

func (h *Hello) get(r *http.Request) (*routing.Response, error) {
	return routing.NewResponse(http.StatusOK).WithBody("CFAPI default BTP Service Broker!"), nil
}

func (h *Hello) Routes() []routing.Route {
	return []routing.Route{
		{Method: "GET", Pattern: "/", Handler: h.get},
	}
}
