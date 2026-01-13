package outbox

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	"ecommerce-order-system/services/outbox-worker/internal/metrics"
	"ecommerce-order-system/shared/pkg/rabbit"
)

type Runner struct {
	Log zerolog.Logger
	DB  *pgxpool.Pool

	EventsPub *rabbit.Publisher

	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	BackoffMax   time.Duration
}

type EventRow struct {
	ID        string
	EventType string
	Payload   []byte
	Attempts  int
}

func (r *Runner) Run(ctx context.Context) {
	t := time.NewTicker(r.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			r.Log.Info().Msg("outbox runner stopped")
			return
		case <-t.C:
			if err := r.tick(ctx); err != nil {
				r.Log.Error().Err(err).Msg("outbox tick failed")
			}
		}
	}
}

func (r *Runner) tick(ctx context.Context) error {
	// update pending gauge
	_ = r.updatePending(ctx)

	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		select id, event_type, payload::text, attempts
		from outbox_events
		where sent_at is null and next_attempt_at <= now()
		order by created_at
		limit $1
		for update skip locked
	`, r.BatchSize)
	if err != nil {
		return err
	}
	defer rows.Close()

	var batch []EventRow
	for rows.Next() {
		var e EventRow
		var payloadText string
		if err := rows.Scan(&e.ID, &e.EventType, &payloadText, &e.Attempts); err != nil {
			return err
		}
		e.Payload = []byte(payloadText)
		batch = append(batch, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, e := range batch {
		if e.Attempts >= r.MaxAttempts {
			_, _ = tx.Exec(ctx, `update outbox_events set last_error=$2, sent_at=now() where id=$1`, e.ID, "max attempts reached")
			r.Log.Warn().Str("id", e.ID).Int("attempts", e.Attempts).Msg("outbox drop (max attempts), marked sent")
			continue
		}

		pubCtx, cancel := rabbit.WithTimeout(ctx)
		err := r.EventsPub.Publish(pubCtx, e.EventType, e.Payload, amqp.Table{
			"x-outbox-id": e.ID,
			"x-attempts":  int32(e.Attempts),
		})
		cancel()

		if err == nil {
			metrics.OutboxSentTotal.Inc()
			_, err2 := tx.Exec(ctx, `update outbox_events set sent_at=now(), last_error=null where id=$1`, e.ID)
			if err2 != nil {
				return err2
			}
			continue
		}

		metrics.OutboxPublishErrorsTotal.Inc()
		next := time.Now().Add(backoff(e.Attempts+1, r.BackoffMax))
		_, err2 := tx.Exec(ctx, `
			update outbox_events
			set attempts = attempts + 1,
			    next_attempt_at = $2,
			    last_error = $3
			where id = $1
		`, e.ID, next, err.Error())
		if err2 != nil {
			return err2
		}
		r.Log.Error().Err(err).Str("id", e.ID).Str("type", e.EventType).Int("attempts", e.Attempts+1).Time("next", next).Msg("publish failed -> retry scheduled")
	}

	return tx.Commit(ctx)
}

func (r *Runner) updatePending(ctx context.Context) error {
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var n int
	if err := r.DB.QueryRow(ctx2, `select count(*) from outbox_events where sent_at is null`).Scan(&n); err != nil {
		return err
	}
	metrics.OutboxPending.Set(float64(n))
	return nil
}

func backoff(attempt int, max time.Duration) time.Duration {
	sec := math.Pow(2, float64(attempt))
	d := time.Duration(sec) * time.Second
	if d > max {
		return max
	}
	if d < time.Second {
		return time.Second
	}
	return d
}
