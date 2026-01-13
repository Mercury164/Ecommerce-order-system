package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ecommerce-order-system/shared/pkg/metrics"
)

type Handlers struct {
	Health      http.HandlerFunc
	CreateOrder http.HandlerFunc
	GetOrder    http.HandlerFunc
}

func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()
	r.Use(metrics.Middleware("api-gateway"))
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/health", h.Health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/orders", h.CreateOrder)
		r.Get("/orders/{id}", h.GetOrder)
	})
	return r
}
