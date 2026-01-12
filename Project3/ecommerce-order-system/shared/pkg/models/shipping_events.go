package models

import (
	"time"

	"github.com/google/uuid"
)

type ShippingScheduledPayload struct {
	TrackingNumber string `json:"tracking_number"`
	Carrier        string `json:"carrier"`
}

func NewShippingScheduledEvent(orderID string) Event[ShippingScheduledPayload] {
	return Event[ShippingScheduledPayload]{
		ID:      uuid.NewString(),
		Type:    "shipping.scheduled",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: ShippingScheduledPayload{
			TrackingNumber: uuid.NewString(),
			Carrier:        "mock-carrier",
		},
	}
}
