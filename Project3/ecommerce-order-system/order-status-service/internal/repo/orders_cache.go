package repo

import (
	"context"
	"time"

	"ecommerce-order-system/shared/pkg/cache"

	"github.com/redis/go-redis/v9"
)

type OrdersCached struct {
	PG    *OrdersPG
	Redis *cache.Redis
	TTL   time.Duration
}

func statusKey(orderID string) string { return "order:" + orderID + ":status" }

func (r *OrdersCached) UpdateStatus(ctx context.Context, orderID string, status string) error {
	if err := r.PG.UpdateStatus(ctx, orderID, status); err != nil {
		return err
	}
	_ = r.Redis.SetString(ctx, statusKey(orderID), status, r.TTL)
	return nil
}

func (r *OrdersCached) GetStatus(ctx context.Context, orderID string) (string, error) {
	// 1) Redis
	s, err := r.Redis.GetString(ctx, statusKey(orderID))
	if err == nil {
		return s, nil
	}
	if err != nil && err != redis.Nil {
		// Redis временно недоступен — просто упадём на БД
	}

	// 2) Postgres
	s, err = r.PG.GetStatus(ctx, orderID)
	if err != nil {
		return "", err
	}

	// 3) backfill
	_ = r.Redis.SetString(ctx, statusKey(orderID), s, r.TTL)
	return s, nil
}
