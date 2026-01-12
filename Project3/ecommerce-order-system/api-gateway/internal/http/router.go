package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	r.Use(MetricsMiddleware("api-gateway"))
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/health", h.Health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/orders", h.CreateOrder)
		r.Get("/orders/{id}", h.GetOrder)
	})

	return r
}
