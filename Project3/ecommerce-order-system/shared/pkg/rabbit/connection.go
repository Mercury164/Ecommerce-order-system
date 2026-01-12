package rabbit

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Conn struct {
	Conn *amqp.Connection
	Ch   *amqp.Channel
}

func Connect(url string) (*Conn, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Conn{Conn: conn, Ch: ch}, nil
}

func (c *Conn) Close() error {
	_ = c.Ch.Close()
	return c.Conn.Close()
}

func WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 5*time.Second)
}
