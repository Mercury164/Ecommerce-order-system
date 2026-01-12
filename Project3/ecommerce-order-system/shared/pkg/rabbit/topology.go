package rabbit

import amqp "github.com/rabbitmq/amqp091-go"

const (
	ExchangeEvents = "orders.events"
	ExchangeDLX    = "orders.dlx"
	ExchangeRetry  = "orders.retry"
)

func DeclareBase(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(ExchangeDLX, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(ExchangeRetry, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	return nil
}

type QueueSpec struct {
	Name     string
	BindKeys []string // routing keys (from ExchangeEvents)
	DLQ      string   // dlq routing key / queue name
	Prefetch int
}

func DeclareQueueWithDLQ(ch *amqp.Channel, q QueueSpec) error {
	args := amqp.Table{}
	if q.DLQ != "" {
		args["x-dead-letter-exchange"] = ExchangeDLX
		args["x-dead-letter-routing-key"] = q.DLQ
	}

	qq, err := ch.QueueDeclare(q.Name, true, false, false, false, args)
	if err != nil {
		return err
	}

	for _, key := range q.BindKeys {
		if err := ch.QueueBind(qq.Name, key, ExchangeEvents, false, nil); err != nil {
			return err
		}
	}

	// DLQ queue + bind to DLX by routing key = q.DLQ
	if q.DLQ != "" {
		dlq, err := ch.QueueDeclare(q.DLQ, true, false, false, false, nil)
		if err != nil {
			return err
		}
		if err := ch.QueueBind(dlq.Name, q.DLQ, ExchangeDLX, false, nil); err != nil {
			return err
		}
	}

	return nil
}
