package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	C *redis.Client
}

func New(addr string) *Redis {
	return &Redis{
		C: redis.NewClient(&redis.Options{Addr: addr}),
	}
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.C.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.C.Close()
}

func (r *Redis) GetString(ctx context.Context, key string) (string, error) {
	return r.C.Get(ctx, key).Result()
}

func (r *Redis) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.C.Set(ctx, key, value, ttl).Err()
}
