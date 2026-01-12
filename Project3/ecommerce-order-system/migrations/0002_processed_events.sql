create table if not exists processed_events (
                                                event_id   uuid primary key,
                                                event_type text not null,
                                                order_id   uuid not null,
                                                created_at timestamptz not null default now()
    );

create index if not exists idx_processed_events_order_id on processed_events(order_id);
