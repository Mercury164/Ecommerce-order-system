package rabbit

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeEvents = "orders.events"
	ExchangeRetry  = "orders.retry"
	ExchangeDLX    = "orders.dlx"
)

type Conn struct {
	Conn *amqp.Connection
	Ch   *amqp.Channel
}

func Connect(url string) (*Conn, error) {
	c, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := c.Channel()
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	return &Conn{Conn: c, Ch: ch}, nil
}

func (c *Conn) Close() error {
	if c.Ch != nil {
		_ = c.Ch.Close()
	}
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}

func DeclareBase(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(ExchangeEvents, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(ExchangeRetry, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.ExchangeDeclare(ExchangeDLX, "topic", true, false, false, false, nil); err != nil {
		return err
	}
	return nil
}

type QueueSpec struct {
	Name     string
	BindKeys []string
	DLQKey   string
	Prefetch int
}

func DeclareQueueWithDLQ(ch *amqp.Channel, spec QueueSpec) error {
	if spec.Prefetch > 0 {
		_ = ch.Qos(spec.Prefetch, 0, false)
	}

	// DLQ queue
	dlqName := spec.Name + ".dlq"
	if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
		return err
	}
	if err := ch.QueueBind(dlqName, spec.DLQKey, ExchangeDLX, false, nil); err != nil {
		return err
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    ExchangeDLX,
		"x-dead-letter-routing-key": spec.DLQKey,
	}

	if _, err := ch.QueueDeclare(spec.Name, true, false, false, false, args); err != nil {
		return err
	}

	for _, key := range spec.BindKeys {
		if err := ch.QueueBind(spec.Name, key, ExchangeEvents, false, nil); err != nil {
			return err
		}
	}
	return nil
}

// DeclareRetryQueue creates a queue bound to ExchangeRetry with retryRoutingKey.
// After TTL it dead-letters back to ExchangeEvents with originalRoutingKey.
func DeclareRetryQueue(ch *amqp.Channel, name string, retryRoutingKey string, originalRoutingKey string, ttlMs int) error {
	args := amqp.Table{
		"x-message-ttl":             int32(ttlMs),
		"x-dead-letter-exchange":    ExchangeEvents,
		"x-dead-letter-routing-key": originalRoutingKey,
	}
	if _, err := ch.QueueDeclare(name, true, false, false, false, args); err != nil {
		return err
	}
	return ch.QueueBind(name, retryRoutingKey, ExchangeRetry, false, nil)
}

type Publisher struct {
	ch       *amqp.Channel
	exchange string
}

func NewPublisher(ch *amqp.Channel, exchange string) *Publisher {
	return &Publisher{ch: ch, exchange: exchange}
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, body []byte, headers amqp.Table) error {
	return p.ch.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
		Headers:     headers,
		Timestamp:   time.Now(),
	})
}

func (p *Publisher) PublishJSON(ctx context.Context, routingKey string, v any, headers amqp.Table) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return p.Publish(ctx, routingKey, b, headers)
}

type Consumer struct{ ch *amqp.Channel }

func NewConsumer(ch *amqp.Channel) *Consumer { return &Consumer{ch: ch} }

func (c *Consumer) Consume(queue string, prefetch int) (<-chan amqp.Delivery, error) {
	if prefetch > 0 {
		_ = c.ch.Qos(prefetch, 0, false)
	}
	return c.ch.Consume(queue, "", false, false, false, false, nil)
}

func WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 5*time.Second)
}

// RetryOrDLQ republish message to retry-exchange or dlx based on attempts.
func RetryOrDLQ(ctx context.Context, d amqp.Delivery, service string, maxAttempts int32, retryPub, dlqPub *Publisher, dlqKey string) error {
	var attempts int32 = 0
	if v, ok := d.Headers["x-attempts"]; ok {
		switch t := v.(type) {
		case int32:
			attempts = t
		case int64:
			attempts = int32(t)
		case int:
			attempts = int32(t)
		}
	}
	attempts++

	h := amqp.Table{}
	for k, v := range d.Headers {
		h[k] = v
	}
	h["x-attempts"] = attempts

	pubCtx, cancel := WithTimeout(ctx)
	defer cancel()

	if attempts <= maxAttempts {
		retryRK := service + "." + d.RoutingKey
		_ = d.Ack(false)
		return retryPub.Publish(pubCtx, retryRK, d.Body, h)
	}

	_ = d.Ack(false)
	return dlqPub.Publish(pubCtx, dlqKey, d.Body, h)
}
