package models

import (
	"time"

	"github.com/google/uuid"
)

type InventoryReleaseRequestedPayload struct {
	Reason string `json:"reason"`
}

type InventoryReleasedPayload struct {
	Released bool `json:"released"`
}

type OrderCancelledPayload struct {
	Reason string `json:"reason"`
}

func NewInventoryReleaseRequested(orderID, reason string) Event[InventoryReleaseRequestedPayload] {
	return Event[InventoryReleaseRequestedPayload]{
		ID:      uuid.NewString(),
		Type:    "inventory.release_requested",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: InventoryReleaseRequestedPayload{Reason: reason},
	}
}

func NewInventoryReleased(orderID string) Event[InventoryReleasedPayload] {
	return Event[InventoryReleasedPayload]{
		ID:      uuid.NewString(),
		Type:    "inventory.released",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: InventoryReleasedPayload{Released: true},
	}
}

func NewOrderCancelled(orderID, reason string) Event[OrderCancelledPayload] {
	return Event[OrderCancelledPayload]{
		ID:      uuid.NewString(),
		Type:    "order.cancelled",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: OrderCancelledPayload{Reason: reason},
	}
}
