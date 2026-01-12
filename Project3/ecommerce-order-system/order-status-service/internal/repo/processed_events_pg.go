package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProcessedEventsPG struct {
	DB *pgxpool.Pool
}

func (r *ProcessedEventsPG) TryMarkProcessed(ctx context.Context, eventID, eventType, orderID string) (bool, error) {
	ct, err := r.DB.Exec(ctx, `
		insert into processed_events(event_id, event_type, order_id)
		values ($1, $2, $3)
		on conflict (event_id) do nothing
	`, eventID, eventType, orderID)
	if err != nil {
		return false, err
	}
	return ct.RowsAffected() == 0, nil
}
