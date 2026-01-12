package rabbit

import amqp "github.com/rabbitmq/amqp091-go"

type Consumer struct {
	ch *amqp.Channel
}

func NewConsumer(ch *amqp.Channel) *Consumer { return &Consumer{ch: ch} }

func (c *Consumer) Consume(queue string, prefetch int) (<-chan amqp.Delivery, error) {
	if err := c.ch.Qos(prefetch, 0, false); err != nil {
		return nil, err
	}
	return c.ch.Consume(queue, "", false, false, false, false, nil)
}
