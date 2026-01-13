package repo

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

type OutboxPG struct{}

// Enqueue writes an event into outbox_events within the given transaction.
func (o *OutboxPG) Enqueue(ctx context.Context, tx pgx.Tx, eventID string, orderID string, eventType string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into outbox_events(
			id, order_id, event_type, payload,
			attempts, next_attempt_at, created_at
		)
		values ($1::uuid, $2::uuid, $3, $4::jsonb, 0, now(), now())
	`, eventID, orderID, eventType, string(b))
	return err
}
