package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"ecommerce-order-system/services/outbox-worker/internal/metrics"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	DB *pgxpool.Pool
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/outbox/pending", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		var n int
		if err := s.DB.QueryRow(ctx, `select count(*) from outbox_events where sent_at is null`).Scan(&n); err != nil {
			http.Error(w, "db error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		metrics.OutboxPending.Set(float64(n))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]int{"pending": n})
	})

	return mux
}
