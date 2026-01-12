package worker

import (
	"context"
	"encoding/json"

	"ecommerce-order-system/shared/pkg/models"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

type Consumer struct {
	Log zerolog.Logger
}

func (c *Consumer) Run(ctx context.Context, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			c.Log.Info().Msg("notification consumer stopped")
			return
		case d, ok := <-deliveries:
			if !ok {
				c.Log.Info().Msg("deliveries closed")
				return
			}
			c.handle(d)
		}
	}
}

func (c *Consumer) handle(d amqp.Delivery) {
	var evt models.EventRaw
	if err := json.Unmarshal(d.Body, &evt); err != nil {
		c.Log.Error().Err(err).Msg("bad json -> dlq")
		_ = d.Nack(false, false)
		return
	}

	c.Log.Info().
		Str("order_id", evt.OrderID).
		Str("type", evt.Type).
		Msg("notify event")

	_ = d.Ack(false)
}
