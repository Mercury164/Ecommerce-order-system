create extension if not exists "uuid-ossp";

create table if not exists orders (
                                      id uuid primary key default uuid_generate_v4(),
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

create index if not exists idx_order_items_order_id on order_items(order_id);
