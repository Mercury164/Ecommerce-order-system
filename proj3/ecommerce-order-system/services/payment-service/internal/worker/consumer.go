package worker

import (
	"context"
	"encoding/json"
	"math/rand"
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

	FailRate int // 0..100
}

type PaymentProcessedPayload struct {
	Note string `json:"note,omitempty"`
}

type PaymentFailedPayload struct {
	Reason string `json:"reason"`
}

type InventoryReleaseRequestedPayload struct {
	Reason string `json:"reason"`
}

type OrderCancelledPayload struct {
	Reason string `json:"reason"`
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	c.Log.Info().Msg("payment consumer started")
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-ctx.Done():
			c.Log.Info().Msg("payment consumer stopped")
			return
		case d, ok := <-deliveries:
			if !ok {
				c.Log.Info().Msg("deliveries closed")
				return
			}
			c.handle(ctx, rng, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, rng *rand.Rand, d amqp.Delivery) {
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

	if d.RoutingKey != "inventory.reserved" {
		c.Log.Warn().Str("rk", d.RoutingKey).Str("order_id", evt.OrderID).Msg("unexpected routing key -> ack")
		_ = d.Ack(false)
		return
	}

	// simulate processing
	fail := rng.Intn(100) < c.FailRate

	if fail {
		c.Log.Warn().Str("order_id", evt.OrderID).Int("fail_rate", c.FailRate).Msg("payment failed")

		pf := models.Event[PaymentFailedPayload]{
			ID:      uuid.NewString(),
			Type:    "payment.failed",
			Version: 1,
			Time:    time.Now(),
			OrderID: evt.OrderID,
			Payload: PaymentFailedPayload{Reason: "simulated failure"},
		}

		releaseReq := models.Event[InventoryReleaseRequestedPayload]{
			ID:      uuid.NewString(),
			Type:    "inventory.release_requested",
			Version: 1,
			Time:    time.Now(),
			OrderID: evt.OrderID,
			Payload: InventoryReleaseRequestedPayload{Reason: "payment failed"},
		}

		cancelEvt := models.Event[OrderCancelledPayload]{
			ID:      uuid.NewString(),
			Type:    "order.cancelled",
			Version: 1,
			Time:    time.Now(),
			OrderID: evt.OrderID,
			Payload: OrderCancelledPayload{Reason: "payment failed"},
		}

		pubCtx, cancel := rabbit.WithTimeout(ctx)
		err1 := c.EventsPub.PublishJSON(pubCtx, pf.Type, pf, amqp.Table{"x-correlation-id": evt.ID})
		err2 := c.EventsPub.PublishJSON(pubCtx, releaseReq.Type, releaseReq, amqp.Table{"x-correlation-id": evt.ID})
		err3 := c.EventsPub.PublishJSON(pubCtx, cancelEvt.Type, cancelEvt, amqp.Table{"x-correlation-id": evt.ID})
		cancel()

		if err1 != nil || err2 != nil || err3 != nil {
			c.Log.Error().
				Err(firstErr(err1, err2, err3)).
				Str("order_id", evt.OrderID).
				Msg("publish failure events failed -> retry/dlq")
			_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
			return
		}

		_ = d.Ack(false)
		c.Log.Warn().Str("order_id", evt.OrderID).Msg("payment failed -> requested inventory release + cancelled order")
		return
	}

	pp := models.Event[PaymentProcessedPayload]{
		ID:      uuid.NewString(),
		Type:    "payment.processed",
		Version: 1,
		Time:    time.Now(),
		OrderID: evt.OrderID,
		Payload: PaymentProcessedPayload{Note: "paid"},
	}

	pubCtx, cancel := rabbit.WithTimeout(ctx)
	err := c.EventsPub.PublishJSON(pubCtx, pp.Type, pp, amqp.Table{"x-correlation-id": evt.ID})
	cancel()
	if err != nil {
		c.Log.Error().Err(err).Str("order_id", evt.OrderID).Msg("publish payment.processed failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	_ = d.Ack(false)
	c.Log.Info().Str("order_id", evt.OrderID).Msg("payment processed")
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
