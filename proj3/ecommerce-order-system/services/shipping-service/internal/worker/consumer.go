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

type ShippingScheduledPayload struct {
	Tracking string `json:"tracking"`
}

type OrderCompletedPayload struct {
	Note string `json:"note,omitempty"`
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	c.Log.Info().Msg("shipping consumer started")
	for {
		select {
		case <-ctx.Done():
			c.Log.Info().Msg("shipping consumer stopped")
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

	if d.RoutingKey != "payment.processed" {
		c.Log.Warn().Str("rk", d.RoutingKey).Str("order_id", evt.OrderID).Msg("unexpected routing key -> ack")
		_ = d.Ack(false)
		return
	}

	sched := models.Event[ShippingScheduledPayload]{
		ID:      uuid.NewString(),
		Type:    "shipping.scheduled",
		Version: 1,
		Time:    time.Now(),
		OrderID: evt.OrderID,
		Payload: ShippingScheduledPayload{Tracking: "TRK-" + evt.OrderID[:8]},
	}
	complete := models.Event[OrderCompletedPayload]{
		ID:      uuid.NewString(),
		Type:    "order.completed",
		Version: 1,
		Time:    time.Now(),
		OrderID: evt.OrderID,
		Payload: OrderCompletedPayload{Note: "done"},
	}

	pubCtx, cancel := rabbit.WithTimeout(ctx)
	err1 := c.EventsPub.PublishJSON(pubCtx, sched.Type, sched, amqp.Table{"x-correlation-id": evt.ID})
	err2 := c.EventsPub.PublishJSON(pubCtx, complete.Type, complete, amqp.Table{"x-correlation-id": evt.ID})
	cancel()

	if err1 != nil || err2 != nil {
		c.Log.Error().Err(firstErr(err1, err2)).Str("order_id", evt.OrderID).Msg("publish shipping events failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	_ = d.Ack(false)
	c.Log.Info().Str("order_id", evt.OrderID).Msg("shipping scheduled + order completed")
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
