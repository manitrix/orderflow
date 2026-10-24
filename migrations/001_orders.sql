BEGIN;
CREATE SCHEMA IF NOT EXISTS orders;
CREATE TABLE IF NOT EXISTS orders.orders (
    id text PRIMARY KEY,
    customer_id text NOT NULL CHECK (length(customer_id) BETWEEN 1 AND 128),
    status text NOT NULL CHECK (status IN ('PENDING', 'RESERVED', 'CONFIRMED', 'FAILED', 'CANCELLED')),
    items jsonb NOT NULL CHECK (jsonb_typeof(items) = 'array' AND jsonb_array_length(items) BETWEEN 1 AND 100),
    total_cents bigint NOT NULL CHECK (total_cents BETWEEN 1 AND 1000000000000),
    created_at timestamptz NOT NULL
);
COMMIT;
