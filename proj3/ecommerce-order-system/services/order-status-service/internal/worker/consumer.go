package worker

import (
	"context"
	"encoding/json"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"ecommerce-order-system/services/order-status-service/internal/repo"
	"ecommerce-order-system/shared/pkg/models"
	"ecommerce-order-system/shared/pkg/rabbit"
)

type Consumer struct {
	Log  zerolog.Logger
	Repo *repo.OrdersPG

	RetryPub *rabbit.Publisher
	DLQPub   *rabbit.Publisher

	Service     string
	MaxAttempts int
	DLQKey      string
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	c.Log.Info().Msg("status consumer started")
	for {
		select {
		case <-ctx.Done():
			c.Log.Info().Msg("status consumer stopped")
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
	if evt.ID == "" || evt.OrderID == "" {
		c.Log.Error().Str("rk", d.RoutingKey).Msg("missing id/order_id -> dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, 0, c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	ok, err := c.Repo.TryMarkProcessed(ctx, evt.ID)
	if err != nil {
		c.Log.Error().Err(err).Str("order_id", evt.OrderID).Msg("try mark processed failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}
	if !ok {
		_ = d.Ack(false)
		c.Log.Debug().Str("event_id", evt.ID).Msg("duplicate event ignored")
		return
	}

	status := mapRoutingKeyToStatus(d.RoutingKey)
	if status == "" {
		_ = d.Ack(false)
		return
	}

	if err := c.Repo.UpdateStatus(ctx, evt.OrderID, status); err != nil {
		c.Log.Error().Err(err).Str("order_id", evt.OrderID).Str("status", status).Msg("update status failed -> retry/dlq")
		_ = rabbit.RetryOrDLQ(ctx, d, c.Service, int32(c.MaxAttempts), c.RetryPub, c.DLQPub, c.DLQKey)
		return
	}

	_ = d.Ack(false)
	c.Log.Info().Str("order_id", evt.OrderID).Str("status", status).Str("rk", d.RoutingKey).Msg("status updated")
}

func mapRoutingKeyToStatus(rk string) string {
	rk = strings.ToLower(rk)
	switch rk {
	case "orders.created":
		return "created"
	case "inventory.reserved":
		return "reserved"
	case "payment.processed":
		return "paid"
	case "shipping.scheduled":
		return "shipping_scheduled"
	case "order.completed":
		return "completed"
	case "inventory.failed", "payment.failed", "order.cancelled":
		return "cancelled"
	default:
		return ""
	}
}
