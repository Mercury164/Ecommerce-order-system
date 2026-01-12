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
	Reason string `json:"reason,omitempty"`
}

type DlqEnvelope struct {
	Error   string          `json:"error"`
	Routing string          `json:"routing_key"`
	Body    json.RawMessage `json:"body"`
	Time    time.Time       `json:"time"`
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
	// payment.processed event
	var payEvt models.Event[models.PaymentProcessedPayload]
	if err := json.Unmarshal(d.Body, &payEvt); err != nil {
		c.Log.Error().Err(err).Msg("bad json -> dlq")
		_ = c.publishDLQ(ctx, d, "bad json: "+err.Error())
		_ = d.Ack(false)
		return
	}

	// 1) shipping.scheduled
	tracking := "TRK-" + time.Now().UTC().Format("20060102-150405.000")

	scheduled := models.Event[ShippingScheduledPayload]{
		ID:      uuid.NewString(),
		Type:    "shipping.scheduled",
		Version: 1,
		Time:    time.Now(),
		OrderID: payEvt.OrderID,
		Payload: ShippingScheduledPayload{Tracking: tracking},
	}

	pubCtx1, cancel1 := rabbit.WithTimeout(ctx)
	err := c.EventsPub.PublishJSON(pubCtx1, scheduled.Type, scheduled, amqp.Table{
		"x-correlation-id": payEvt.ID,
	})
	cancel1()

	if err != nil {
		c.Log.Error().Err(err).Str("order_id", payEvt.OrderID).Msg("publish shipping.scheduled failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	// 2) order.completed
	completed := models.Event[OrderCompletedPayload]{
		ID:      uuid.NewString(),
		Type:    "order.completed",
		Version: 1,
		Time:    time.Now(),
		OrderID: payEvt.OrderID,
		Payload: OrderCompletedPayload{},
	}

	pubCtx2, cancel2 := rabbit.WithTimeout(ctx)
	err = c.EventsPub.PublishJSON(pubCtx2, completed.Type, completed, amqp.Table{
		"x-correlation-id": payEvt.ID,
	})
	cancel2()

	if err != nil {
		c.Log.Error().Err(err).Str("order_id", payEvt.OrderID).Msg("publish order.completed failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	_ = d.Ack(false)
	c.Log.Info().
		Str("order_id", payEvt.OrderID).
		Str("tracking", tracking).
		Msg("shipping scheduled + order completed")
}

func (c *Consumer) publishDLQ(ctx context.Context, d amqp.Delivery, reason string) error {
	env := DlqEnvelope{
		Error:   reason,
		Routing: d.RoutingKey,
		Body:    json.RawMessage(d.Body),
		Time:    time.Now(),
	}
	pubCtx, cancel := rabbit.WithTimeout(ctx)
	defer cancel()
	return c.DLQPub.PublishJSON(pubCtx, c.DLQKey, env, amqp.Table{
		"x-original-routing-key": d.RoutingKey,
		"x-error":                reason,
	})
}
