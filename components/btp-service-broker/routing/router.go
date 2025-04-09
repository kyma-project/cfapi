package routing

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

var URLParam = chi.URLParam

type Route struct {
	Method  string
	Pattern string
	Handler Handler
}
type Routable interface {
	Routes() []Route
}

type RouterBuilder struct {
	routes      []Route
	middlewares []func(http.Handler) http.Handler
}

func NewRouterBuilder() *RouterBuilder {
	return &RouterBuilder{}
}

func (b *RouterBuilder) LoadRoutes(routable Routable) {
	b.routes = append(b.routes, routable.Routes()...)
}

func (b *RouterBuilder) Build() *chi.Mux {
	router := chi.NewRouter()
	setupRouter(router, b.middlewares, b.routes)
	return router
}

func setupRouter(router chi.Router, middlewares []func(http.Handler) http.Handler, routes []Route) {
	for _, middleware := range middlewares {
		router.Use(middleware)
	}
	for _, route := range routes {
		router.Method(route.Method, route.Pattern, route.Handler)
	}
}

func (b *RouterBuilder) UseMiddleware(middleware ...func(http.Handler) http.Handler) {
	b.middlewares = append(b.middlewares, middleware...)
}
