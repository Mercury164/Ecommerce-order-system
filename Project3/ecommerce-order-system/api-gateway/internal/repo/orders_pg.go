package repo

import (
	"context"

	"ecommerce-order-system/shared/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrdersPG struct {
	DB *pgxpool.Pool
}

func (r *OrdersPG) Create(ctx context.Context, o models.Order, items []models.OrderItem) (string, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var orderID string
	err = tx.QueryRow(ctx, `
		insert into orders (user_id, email, status, total_cents)
		values ($1, $2, $3, $4)
		returning id
	`, o.UserID, o.Email, string(o.Status), o.TotalCents).Scan(&orderID)
	if err != nil {
		return "", err
	}

	for _, it := range items {
		_, err = tx.Exec(ctx, `
			insert into order_items (order_id, sku, qty, price_cents)
			values ($1, $2, $3, $4)
		`, orderID, it.SKU, it.Qty, it.PriceCents)
		if err != nil {
			return "", err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return orderID, nil
}
