package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OrdersPG struct{ DB *pgxpool.Pool }

func (r *OrdersPG) GetStatus(ctx context.Context, orderID string) (string, error) {
	var s string
	err := r.DB.QueryRow(ctx, `select status from orders where id = $1`, orderID).Scan(&s)
	return s, err
}
