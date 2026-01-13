package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"ecommerce-order-system/services/api-gateway/internal/repo"
	"ecommerce-order-system/shared/pkg/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type CreateOrderHandler struct {
	DB     *pgxpool.Pool
	Outbox *repo.OutboxPG
	Log    zerolog.Logger
}

type createOrderReq struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Items  []struct {
		SKU        string `json:"sku"`
		Qty        int    `json:"qty"`
		PriceCents int    `json:"price_cents"`
	} `json:"items"`
}

type createOrderResp struct {
	ID string `json:"id"`
}

func (h *CreateOrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req createOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.UserID == "" || req.Email == "" || len(req.Items) == 0 {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	for _, it := range req.Items {
		if it.SKU == "" || it.Qty <= 0 || it.PriceCents < 0 {
			http.Error(w, "invalid items", http.StatusBadRequest)
			return
		}
	}

	orderID := uuid.NewString()
	total := 0
	for _, it := range req.Items {
		total += it.PriceCents * it.Qty
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := h.DB.Begin(ctx)
	if err != nil {
		h.Log.Error().Err(err).Msg("begin tx failed")
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		insert into orders(id, user_id, email, status, total_cents)
		values ($1, $2, $3, $4, $5)
	`, orderID, req.UserID, req.Email, "created", total)
	if err != nil {
		h.Log.Error().Err(err).Msg("insert order failed")
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}

	for _, it := range req.Items {
		_, err = tx.Exec(ctx, `
			insert into order_items(order_id, sku, qty, price_cents)
			values ($1, $2, $3, $4)
		`, orderID, it.SKU, it.Qty, it.PriceCents)
		if err != nil {
			h.Log.Error().Err(err).Msg("insert order_items failed")
			http.Error(w, "failed to create order", http.StatusInternalServerError)
			return
		}
	}

	itemsArg := make([]struct {
		SKU        string `json:"sku"`
		Qty        int    `json:"qty"`
		PriceCents int    `json:"price_cents"`
	}, 0, len(req.Items))
	for _, it := range req.Items {
		itemsArg = append(itemsArg, struct {
			SKU        string `json:"sku"`
			Qty        int    `json:"qty"`
			PriceCents int    `json:"price_cents"`
		}{SKU: it.SKU, Qty: it.Qty, PriceCents: it.PriceCents})
	}

	evt := models.NewOrderCreatedEvent(orderID, req.UserID, req.Email, total, itemsArg)

	if err := h.Outbox.Enqueue(ctx, tx, evt.ID, evt.OrderID, evt.Type, evt); err != nil {
		h.Log.Error().Err(err).Msg("outbox enqueue failed")
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		h.Log.Error().Err(err).Msg("commit failed")
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(createOrderResp{ID: orderID})
}
