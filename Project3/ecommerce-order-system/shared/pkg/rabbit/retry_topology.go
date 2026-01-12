package rabbit

import amqp "github.com/rabbitmq/amqp091-go"

// Retry-queue:
// - bind to ExchangeRetry with bindKey (обычно "<service>.<originalRK>")
// - message sits ttlMs
// - then dead-letters to ExchangeEvents with deadRoutingKey (originalRK)
func DeclareRetryQueue(ch *amqp.Channel, name string, bindKey string, deadRoutingKey string, ttlMs int) error {
	args := amqp.Table{
		"x-message-ttl":             ttlMs,
		"x-dead-letter-exchange":    ExchangeEvents,
		"x-dead-letter-routing-key": deadRoutingKey,
	}

	q, err := ch.QueueDeclare(name, true, false, false, false, args)
	if err != nil {
		return err
	}

	return ch.QueueBind(q.Name, bindKey, ExchangeRetry, false, nil)
}
