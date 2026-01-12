package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handlers struct {
	Health   http.HandlerFunc
	GetOrder http.HandlerFunc
}

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(MetricsMiddleware("order-status-service"))
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/health", h.Health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/orders/{id}", h.GetOrder)
	})

	return r
}
