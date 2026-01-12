package worker

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"ecommerce-order-system/shared/pkg/models"
	"ecommerce-order-system/shared/pkg/rabbit"
)

type Repo interface {
	GetStatus(ctx context.Context, orderID string) (string, error)
	UpdateStatus(ctx context.Context, orderID string, status string) error
	TryMarkProcessed(ctx context.Context, eventID, eventType, orderID string) (bool, error)
}

type Consumer struct {
	Log zerolog.Logger

	Repo Repo

	RetryPub *rabbit.Publisher
	DLQPub   *rabbit.Publisher

	Service     string
	MaxAttempts int
	DLQKey      string
}

type DlqEnvelope struct {
	Error   string          `json:"error"`
	Routing string          `json:"routing_key"`
	Body    json.RawMessage `json:"body"`
	Time    time.Time       `json:"time"`
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	c.Log.Info().Msg("order-status consumer started")
	for {
		select {
		case <-ctx.Done():
			c.Log.Info().Msg("order-status consumer stopped")
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
	// Нам достаточно ID/Type/OrderID
	var evt models.Event[json.RawMessage]
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		c.Log.Error().Err(err).Str("rk", d.RoutingKey).Msg("bad json -> dlq")
		_ = c.publishDLQ(ctx, d, "bad json: "+err.Error())
		_ = d.Ack(false)
		return
	}
	if evt.OrderID == "" || evt.ID == "" {
		c.Log.Error().Str("rk", d.RoutingKey).Msg("missing order_id or event_id -> dlq")
		_ = c.publishDLQ(ctx, d, "missing order_id or event_id")
		_ = d.Ack(false)
		return
	}

	newStatus := mapStatus(d.RoutingKey)
	if newStatus == "" {
		c.Log.Warn().Str("rk", d.RoutingKey).Str("order_id", evt.OrderID).Msg("unknown event -> ack")
		_ = d.Ack(false)
		return
	}

	eventType := evt.Type
	if eventType == "" {
		eventType = d.RoutingKey
	}

	// Идемпотентность: если уже было — ACK
	ok, err := c.Repo.TryMarkProcessed(ctx, evt.ID, eventType, evt.OrderID)
	if err != nil {
		c.Log.Error().Err(err).Str("event_id", evt.ID).Msg("mark processed failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}
	if !ok {
		c.Log.Info().Str("event_id", evt.ID).Msg("duplicate event -> ack")
		_ = d.Ack(false)
		return
	}

	// TERMINAL GUARD: если заказ уже завершён/отменён — больше НЕ меняем статус
	cur, err := c.Repo.GetStatus(ctx, evt.OrderID)
	if err != nil {
		c.Log.Error().Err(err).Str("order_id", evt.OrderID).Msg("get current status failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}
	if isTerminal(cur) {
		c.Log.Info().
			Str("order_id", evt.OrderID).
			Str("current", cur).
			Str("incoming", newStatus).
			Str("rk", d.RoutingKey).
			Msg("terminal status locked -> skip update")
		_ = d.Ack(false)
		return
	}

	// Обновляем статус
	if err := c.Repo.UpdateStatus(ctx, evt.OrderID, newStatus); err != nil {
		c.Log.Error().Err(err).Str("order_id", evt.OrderID).Str("status", newStatus).Msg("update status failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	_ = d.Ack(false)
	c.Log.Info().Str("order_id", evt.OrderID).Str("status", newStatus).Str("rk", d.RoutingKey).Msg("status updated")
}

func mapStatus(rk string) string {
	switch rk {
	case "orders.created":
		return "created"
	case "inventory.reserved":
		return "reserved"
	case "inventory.failed":
		return "inventory_failed"
	case "payment.processed":
		return "paid"
	case "payment.failed":
		return "payment_failed"
	case "shipping.scheduled":
		return "shipping_scheduled"
	case "order.completed":
		return "completed"
	case "order.cancelled":
		return "cancelled"

	// компенсация
	case "inventory.release_requested", "inventory.released":
		return "cancelled"
	default:
		return ""
	}
}

func isTerminal(status string) bool {
	return status == "completed" || status == "cancelled"
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
