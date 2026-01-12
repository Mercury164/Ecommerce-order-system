package config

import "github.com/caarlos0/env/v11"

type RabbitConfig struct {
	URL string `env:"RABBIT_URL,required"`
}

type PostgresConfig struct {
	DSN       string `env:"POSTGRES_DSN"`
	DSNLegacy string `env:"PG_DSN"`
}

type RedisConfig struct {
	Addr string `env:"REDIS_ADDR"`
}

type HTTPConfig struct {
	Addr string `env:"HTTP_ADDR" envDefault:":8080"`
}

type Common struct {
	ServiceName string `env:"SERVICE_NAME" envDefault:"service"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
}

type Config struct {
	Common   Common
	Rabbit   RabbitConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	HTTP     HTTPConfig
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	if cfg.Postgres.DSN == "" {
		cfg.Postgres.DSN = cfg.Postgres.DSNLegacy
	}

	return cfg, nil
}
