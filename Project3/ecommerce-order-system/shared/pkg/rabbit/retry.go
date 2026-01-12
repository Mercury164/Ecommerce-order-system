package rabbit

import (
	"context"
	"errors"

	amqp "github.com/rabbitmq/amqp091-go"
)

func GetAttempts(h amqp.Table) int32 {
	if h == nil {
		return 0
	}
	v, ok := h["x-attempts"]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case int32:
		return t
	case int64:
		return int32(t)
	case int:
		return int32(t)
	case float64:
		return int32(t)
	default:
		return 0
	}
}

// RetryOrDLQ:
// - publish original body to ExchangeRetry with routing "<service>.<originalRK>" and attempts+1 OR
// - publish to ExchangeDLX with routing = dlqRoutingKey when attempts >= maxAttempts
// - ACK original delivery on success
// - if publish failed -> Nack(requeue=true)
func RetryOrDLQ(
	ctx context.Context,
	d amqp.Delivery,
	service string,
	maxAttempts int32,
	retryPub *Publisher,
	dlqPub *Publisher,
	dlqRoutingKey string,
) error {
	attempts := GetAttempts(d.Headers)

	// max -> DLQ
	if attempts >= maxAttempts {
		pubCtx, cancel := WithTimeout(ctx)
		defer cancel()

		if err := dlqPub.Publish(pubCtx, dlqRoutingKey, d.Body, amqp.Table{"x-attempts": attempts}); err != nil {
			_ = d.Nack(false, true)
			return err
		}
		_ = d.Ack(false)
		return errors.New("max attempts reached -> dlq")
	}

	// schedule retry
	pubCtx, cancel := WithTimeout(ctx)
	defer cancel()

	retryKey := service + "." + d.RoutingKey
	headers := amqp.Table{"x-attempts": attempts + 1}

	if err := retryPub.Publish(pubCtx, retryKey, d.Body, headers); err != nil {
		_ = d.Nack(false, true)
		return err
	}

	_ = d.Ack(false)
	return errors.New("scheduled retry")
}
