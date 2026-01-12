package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrdersPG struct {
	DB *pgxpool.Pool
}

func (r *OrdersPG) UpdateStatus(ctx context.Context, orderID string, status string) error {
	_, err := r.DB.Exec(ctx, `
		update orders
		set status = $2, updated_at = now()
		where id = $1
	`, orderID, status)
	return err
}

func (r *OrdersPG) GetStatus(ctx context.Context, orderID string) (string, error) {
	var status string
	err := r.DB.QueryRow(ctx, `select status from orders where id = $1`, orderID).Scan(&status)
	return status, err
}
