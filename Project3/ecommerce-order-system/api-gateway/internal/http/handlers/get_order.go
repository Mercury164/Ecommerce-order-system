package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

type GetOrderProxyHandler struct {
	BaseURL string // например: http://order-status-service:8090
	Client  *http.Client
	Log     zerolog.Logger
}

type orderStatusResp struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
}

func (h *GetOrderProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	if strings.TrimSpace(orderID) == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.BaseURL+"/api/v1/orders/"+orderID, nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusInternalServerError)
		return
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		h.Log.Error().Err(err).Msg("order-status request failed")
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if resp.StatusCode >= 400 {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	// просто прокидываем JSON
	w.Header().Set("Content-Type", "application/json")
	var body any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	_ = json.NewEncoder(w).Encode(body)

}

// helper
func DefaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 3 * time.Second}
}
