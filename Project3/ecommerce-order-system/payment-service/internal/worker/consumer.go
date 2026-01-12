package worker

import (
	"context"
	"encoding/json"
	"math/rand"
	"os"
	"strconv"
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

type PaymentFailedPayload struct {
	Reason string `json:"reason"`
}

type InventoryReleaseRequestedPayload struct {
	Reason string `json:"reason,omitempty"`
}

type OrderCancelledPayload struct {
	Reason string `json:"reason"`
}

type DlqEnvelope struct {
	Error   string          `json:"error"`
	Routing string          `json:"routing_key"`
	Body    json.RawMessage `json:"body"`
	Time    time.Time       `json:"time"`
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	c.Log.Info().Msg("payment consumer started")
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
			c.handle(ctx, d)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, d amqp.Delivery) {
	// inventory.reserved event (payload нам не критичен — нужен OrderID)
	var inEvt models.Event[json.RawMessage]
	if err := json.Unmarshal(d.Body, &inEvt); err != nil {
		c.Log.Error().Err(err).Msg("bad json -> dlq")
		_ = c.publishDLQ(ctx, d, "bad json: "+err.Error())
		_ = d.Ack(false)
		return
	}
	if inEvt.OrderID == "" {
		c.Log.Error().Msg("missing order_id -> dlq")
		_ = c.publishDLQ(ctx, d, "missing order_id")
		_ = d.Ack(false)
		return
	}

	fail := shouldFail()
	if !fail {
		// payment.processed
		processed := models.Event[models.PaymentProcessedPayload]{
			ID:      uuid.NewString(),
			Type:    "payment.processed",
			Version: 1,
			Time:    time.Now(),
			OrderID: inEvt.OrderID,
			Payload: models.PaymentProcessedPayload{}, // не трогаем поля, чтобы не ломать компиляцию
		}

		pubCtx, cancel := rabbit.WithTimeout(ctx)
		err := c.EventsPub.PublishJSON(pubCtx, processed.Type, processed, amqp.Table{
			"x-correlation-id": inEvt.ID,
		})
		cancel()

		if err != nil {
			c.Log.Error().Err(err).Str("order_id", inEvt.OrderID).Msg("publish payment.processed failed -> retry/dlq")
			_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
			return
		}

		_ = d.Ack(false)
		c.Log.Info().Str("order_id", inEvt.OrderID).Msg("payment processed")
		return
	}

	// FAIL path => compensation
	reason := "payment failed (simulated)"

	evtFailed := models.Event[PaymentFailedPayload]{
		ID:      uuid.NewString(),
		Type:    "payment.failed",
		Version: 1,
		Time:    time.Now(),
		OrderID: inEvt.OrderID,
		Payload: PaymentFailedPayload{Reason: reason},
	}

	evtRelease := models.Event[InventoryReleaseRequestedPayload]{
		ID:      uuid.NewString(),
		Type:    "inventory.release_requested",
		Version: 1,
		Time:    time.Now(),
		OrderID: inEvt.OrderID,
		Payload: InventoryReleaseRequestedPayload{Reason: reason},
	}

	evtCancel := models.Event[OrderCancelledPayload]{
		ID:      uuid.NewString(),
		Type:    "order.cancelled",
		Version: 1,
		Time:    time.Now(),
		OrderID: inEvt.OrderID,
		Payload: OrderCancelledPayload{Reason: reason},
	}

	// публикуем 3 события; если любое упало — retry original message
	if err := c.publish(ctx, inEvt.ID, evtFailed.Type, evtFailed); err != nil {
		c.Log.Error().Err(err).Msg("publish payment.failed failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}
	if err := c.publish(ctx, inEvt.ID, evtRelease.Type, evtRelease); err != nil {
		c.Log.Error().Err(err).Msg("publish inventory.release_requested failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}
	if err := c.publish(ctx, inEvt.ID, evtCancel.Type, evtCancel); err != nil {
		c.Log.Error().Err(err).Msg("publish order.cancelled failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	_ = d.Ack(false)
	c.Log.Warn().Str("order_id", inEvt.OrderID).Msg("payment failed -> requested inventory release + cancelled order")
}

func (c *Consumer) publish(ctx context.Context, corrID string, rk string, v any) error {
	pubCtx, cancel := rabbit.WithTimeout(ctx)
	defer cancel()
	return c.EventsPub.PublishJSON(pubCtx, rk, v, amqp.Table{
		"x-correlation-id": corrID,
	})
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

// PAYMENT_FAIL_RATE=0..100 (проценты). По умолчанию 30%.
func shouldFail() bool {
	rate := 30
	if s := os.Getenv("PAYMENT_FAIL_RATE"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v >= 0 && v <= 100 {
			rate = v
		}
	}
	// детерминированней для демо — seeded once
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(100) < rate
}
