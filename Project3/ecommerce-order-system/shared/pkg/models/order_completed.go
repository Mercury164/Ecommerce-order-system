package models

import (
	"time"

	"github.com/google/uuid"
)

type OrderCompletedPayload struct {
	Reason string `json:"reason"`
}

func NewOrderCompleted(orderID string, reason string) Event[OrderCompletedPayload] {
	return Event[OrderCompletedPayload]{
		ID:      uuid.NewString(),
		Type:    "order.completed",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: OrderCompletedPayload{Reason: reason},
	}
}
