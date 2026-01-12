create table if not exists outbox_events (
                                             id uuid primary key,
                                             aggregate_id uuid not null,
                                             event_type text not null,
                                             payload jsonb not null,

                                             created_at timestamptz not null default now(),
    sent_at timestamptz,
    attempts int not null default 0,
    next_attempt_at timestamptz not null default now(),
    last_error text
    );

create index if not exists idx_outbox_pending
    on outbox_events (sent_at, next_attempt_at)
    where sent_at is null;

create index if not exists idx_outbox_aggregate on outbox_events(aggregate_id);
