package models

import (
	"time"

	"github.com/google/uuid"
)

type OrderItemPayload struct {
	SKU        string `json:"sku"`
	Qty        int    `json:"qty"`
	PriceCents int    `json:"price_cents"`
}

type OrderCreatedPayload struct {
	UserID     string             `json:"user_id"`
	Email      string             `json:"email"`
	TotalCents int                `json:"total_cents"`
	Items      []OrderItemPayload `json:"items"`
}

func NewOrderCreatedEvent(orderID, userID, email string, total int, items []struct {
	SKU        string `json:"sku"`
	Qty        int    `json:"qty"`
	PriceCents int    `json:"price_cents"`
}) Event[OrderCreatedPayload] {
	outItems := make([]OrderItemPayload, 0, len(items))
	for _, it := range items {
		outItems = append(outItems, OrderItemPayload{
			SKU: it.SKU, Qty: it.Qty, PriceCents: it.PriceCents,
		})
	}
	return Event[OrderCreatedPayload]{
		ID:      uuid.NewString(),
		Type:    "orders.created",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: OrderCreatedPayload{
			UserID:     userID,
			Email:      email,
			TotalCents: total,
			Items:      outItems,
		},
	}
}
