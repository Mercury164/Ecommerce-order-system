-- 001_init.sql

create table if not exists orders (
  id uuid primary key,
  user_id text not null,
  email text not null,
  status text not null,
  total_cents int not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists order_items (
  id bigserial primary key,
  order_id uuid not null references orders(id) on delete cascade,
  sku text not null,
  qty int not null,
  price_cents int not null
);

create table if not exists outbox_events (
  id uuid primary key,
  order_id uuid not null references orders(id) on delete cascade,
  event_type text not null,
  payload jsonb not null,
  attempts int not null default 0,
  next_attempt_at timestamptz not null default now(),
  last_error text,
  created_at timestamptz not null default now(),
  sent_at timestamptz
);

create index if not exists outbox_events_pending_idx
  on outbox_events (next_attempt_at)
  where sent_at is null;

create index if not exists outbox_events_unsent_idx
  on outbox_events (created_at)
  where sent_at is null;

create table if not exists processed_events (
  event_id text primary key,
  processed_at timestamptz not null default now()
);
