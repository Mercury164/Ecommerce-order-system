package rabbit

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	ch       *amqp.Channel
	exchange string
}

func NewPublisher(ch *amqp.Channel, exchange string) *Publisher {
	return &Publisher{ch: ch, exchange: exchange}
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, body []byte, headers amqp.Table) error {
	return p.ch.PublishWithContext(ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			Headers:      headers,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

func (p *Publisher) PublishJSON(ctx context.Context, routingKey string, v any, headers amqp.Table) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.Publish(ctx, routingKey, b, headers)
}
