package repo

import (
	"context"
)

type Combined struct {
	Orders interface {
		UpdateStatus(ctx context.Context, orderID string, status string) error
	}
	Events interface {
		TryMarkProcessed(ctx context.Context, eventID, eventType, orderID string) (bool, error)
	}
}

func (c *Combined) UpdateStatus(ctx context.Context, orderID string, status string) error {
	return c.Orders.UpdateStatus(ctx, orderID, status)
}

func (c *Combined) TryMarkProcessed(ctx context.Context, eventID, eventType, orderID string) (bool, error) {
	return c.Events.TryMarkProcessed(ctx, eventID, eventType, orderID)
}
