package worker

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"ecommerce-order-system/shared/pkg/models"
	"ecommerce-order-system/shared/pkg/rabbit"

	"github.com/google/uuid"
)

type Consumer struct {
	Log zerolog.Logger

	EventsPub *rabbit.Publisher
	RetryPub  *rabbit.Publisher
	DLQPub    *rabbit.Publisher

	Service     string
	MaxAttempts int
	DLQKey      string
}

type InventoryReservedPayload struct {
	Note string `json:"note,omitempty"`
}

type InventoryReleasedPayload struct {
	Note string `json:"note,omitempty"`
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	c.Log.Info().Msg("inventory consumer started")
	for {
		select {
		case <-ctx.Done():
			c.Log.Info().Msg("inventory consumer stopped")
			return
		case d, ok := <-deliveries:
			if !ok {
				c.Log.Info().Msg("deliveries closed")
				return
			}
			c.handle(ctx, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) {
	var evt models.Event[json.RawMessage]
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		c.Log.Error().Err(err).Str("rk", d.RoutingKey).Msg("bad json -> dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, 0, c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}
	if evt.OrderID == "" || evt.ID == "" {
		c.Log.Error().Str("rk", d.RoutingKey).Msg("missing order_id/event_id -> dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, 0, c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	switch d.RoutingKey {
	case "orders.created":
		reserved := models.Event[InventoryReservedPayload]{
			ID:      uuid.NewString(),
			Type:    "inventory.reserved",
			Version: 1,
			Time:    time.Now(),
			OrderID: evt.OrderID,
			Payload: InventoryReservedPayload{Note: "reserved"},
		}
		pubCtx, cancel := rabbit.WithTimeout(ctx)
		err := c.EventsPub.PublishJSON(pubCtx, reserved.Type, reserved, amqp.Table{"x-correlation-id": evt.ID})
		cancel()
		if err != nil {
			c.Log.Error().Err(err).Str("order_id", evt.OrderID).Msg("publish inventory.reserved failed -> retry/dlq")
			_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
			return
		}
		_ = d.Ack(false)
		c.Log.Info().Str("order_id", evt.OrderID).Msg("inventory reserved")
		return

	case "inventory.release_requested":
		released := models.Event[InventoryReleasedPayload]{
			ID:      uuid.NewString(),
			Type:    "inventory.released",
			Version: 1,
			Time:    time.Now(),
			OrderID: evt.OrderID,
			Payload: InventoryReleasedPayload{Note: "released"},
		}
		pubCtx, cancel := rabbit.WithTimeout(ctx)
		err := c.EventsPub.PublishJSON(pubCtx, released.Type, released, amqp.Table{"x-correlation-id": evt.ID})
		cancel()
		if err != nil {
			c.Log.Error().Err(err).Str("order_id", evt.OrderID).Msg("publish inventory.released failed -> retry/dlq")
			_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
			return
		}
		_ = d.Ack(false)
		c.Log.Info().Str("order_id", evt.OrderID).Msg("inventory released (compensation)")
		return

	default:
		c.Log.Warn().Str("rk", d.RoutingKey).Str("order_id", evt.OrderID).Msg("unexpected routing key -> ack")
		_ = d.Ack(false)
		return
	}
}
