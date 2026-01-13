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

// TryMarkProcessed returns true if inserted (new), false if already processed.
func (r *OrdersPG) TryMarkProcessed(ctx context.Context, eventID string) (bool, error) {
	ct, err := r.DB.Exec(ctx, `insert into processed_events(event_id) values ($1) on conflict do nothing`, eventID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 1, nil
}

// UpdateStatus applies terminal guard: do not override completed/cancelled.
func (r *OrdersPG) UpdateStatus(ctx context.Context, orderID string, status string) error {
	_, err := r.DB.Exec(ctx, `
		update orders
		set status = $2,
		    updated_at = now()
		where id = $1
		  and status not in ('completed','cancelled')
	`, orderID, status)
	return err
}
