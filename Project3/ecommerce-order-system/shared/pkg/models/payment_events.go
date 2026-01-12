package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentProcessedPayload struct {
	Provider    string `json:"provider"`
	PaymentID   string `json:"payment_id"`
	AmountCents int    `json:"amount_cents"`
}

type PaymentFailedPayload struct {
	Reason string `json:"reason"`
}

func NewPaymentProcessedEvent(orderID string, amountCents int) Event[PaymentProcessedPayload] {
	return Event[PaymentProcessedPayload]{
		ID:      uuid.NewString(),
		Type:    "payment.processed",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: PaymentProcessedPayload{
			Provider:    "mock",
			PaymentID:   uuid.NewString(),
			AmountCents: amountCents,
		},
	}
}

func NewPaymentFailedEvent(orderID, reason string) Event[PaymentFailedPayload] {
	return Event[PaymentFailedPayload]{
		ID:      uuid.NewString(),
		Type:    "payment.failed",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: PaymentFailedPayload{Reason: reason},
	}
}
