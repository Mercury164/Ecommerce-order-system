package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Repo interface {
	GetStatus(ctx any, orderID string) (string, error) // we'll adapt below
}

type GetOrderHandler struct {
	GetStatus func(r *http.Request, orderID string) (string, error)
}

func (h *GetOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	status, err := h.GetStatus(r, id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"order_id": id,
		"status":   status,
	})
}
