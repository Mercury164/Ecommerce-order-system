package config

import (
	"fmt"

	env "github.com/caarlos0/env/v11"
)

type CommonConfig struct {
	LogLevel string `env:"COMMON_LOG_LEVEL" envDefault:"info"`
}

type HTTPConfig struct {
	Addr string `env:"HTTP_ADDR" envDefault:":8080"`
}

type PostgresConfig struct {
	DSN       string `env:"POSTGRES_DSN"`
	DSNLegacy string `env:"PG_DSN"`
}

type RabbitConfig struct {
	URL string `env:"RABBIT_URL" envDefault:"amqp://guest:guest@rabbitmq:5672/"`
}

type OrderStatusConfig struct {
	URL string `env:"ORDER_STATUS_URL" envDefault:"http://order-status-service:8090"`
}

type PaymentConfig struct {
	FailRate int `env:"PAYMENT_FAIL_RATE" envDefault:"30"`
}

type OutboxHTTPConfig struct {
	Addr string `env:"OUTBOX_HTTP_ADDR" envDefault:":8085"`
}

type Config struct {
	Common      CommonConfig
	HTTP        HTTPConfig
	Postgres    PostgresConfig
	Rabbit      RabbitConfig
	OrderStatus OrderStatusConfig
	Payment     PaymentConfig
	OutboxHTTP  OutboxHTTPConfig
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.Postgres.DSN == "" {
		cfg.Postgres.DSN = cfg.Postgres.DSNLegacy
	}
	if cfg.Postgres.DSN == "" {
		return Config{}, fmt.Errorf("postgres dsn is empty: set POSTGRES_DSN (or legacy PG_DSN)")
	}
	return cfg, nil
}
