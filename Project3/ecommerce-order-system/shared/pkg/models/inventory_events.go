package models

import (
	"time"

	"github.com/google/uuid"
)

type InventoryReservedPayload struct {
	Reserved bool `json:"reserved"`
}

type InventoryFailedPayload struct {
	Reason string `json:"reason"`
}

func NewInventoryReservedEvent(orderID string) Event[InventoryReservedPayload] {
	return Event[InventoryReservedPayload]{
		ID:      uuid.NewString(),
		Type:    "inventory.reserved",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: InventoryReservedPayload{Reserved: true},
	}
}

func NewInventoryFailedEvent(orderID, reason string) Event[InventoryFailedPayload] {
	return Event[InventoryFailedPayload]{
		ID:      uuid.NewString(),
		Type:    "inventory.failed",
		Version: 1,
		Time:    time.Now(),
		OrderID: orderID,
		Payload: InventoryFailedPayload{Reason: reason},
	}
}
