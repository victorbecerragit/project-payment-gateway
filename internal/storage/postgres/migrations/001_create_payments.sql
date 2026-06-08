-- Migration: 001_create_payments
-- Direction: UP
-- Apply with: psql $DATABASE_URL -f 001_create_payments.sql
-- Or via golang-migrate / goose once a migration runner is wired.

CREATE TABLE IF NOT EXISTS payments (
    -- Internal gateway identifier (e.g. pay_01J...)
    id               TEXT        PRIMARY KEY,

    -- External provider reference (e.g. Stripe PaymentIntent ID pi_xxx).
    -- UNIQUE enforces one provider charge per gateway payment.
    transaction_id   TEXT        UNIQUE,

    customer_id      TEXT        NOT NULL,

    -- Amount stored in the smallest currency unit (cents, pence, etc.)
    -- to avoid floating-point drift. Application divides by 100 on read.
    amount           BIGINT      NOT NULL CHECK (amount > 0),

    -- ISO 4217 currency code (e.g. USD, EUR).
    currency         CHAR(3)     NOT NULL,

    -- Domain status: pending | processing | completed | failed | cancelled
    status           TEXT        NOT NULL DEFAULT 'pending',

    -- Client-supplied idempotency key. UNIQUE at DB level (belt-and-suspenders
    -- alongside the application-layer check).
    idempotency_key  TEXT        UNIQUE,

    description      TEXT,

    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast webhook reconciliation: provider sends transaction_id, we resolve the payment.
CREATE INDEX IF NOT EXISTS idx_payments_transaction_id
    ON payments (transaction_id)
    WHERE transaction_id IS NOT NULL;

-- Fast idempotency lookup on payment creation.
CREATE INDEX IF NOT EXISTS idx_payments_idempotency_key
    ON payments (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Useful for status-based queries (e.g. processing backlog, failed retries).
CREATE INDEX IF NOT EXISTS idx_payments_status
    ON payments (status);

-- Automatically maintain updated_at on every row update.
-- Requires the moddatetime extension (ships with pg_catalog in Postgres 12+):
--   CREATE EXTENSION IF NOT EXISTS moddatetime;
-- CREATE TRIGGER set_updated_at
--     BEFORE UPDATE ON payments
--     FOR EACH ROW EXECUTE FUNCTION moddatetime(updated_at);
-- Uncomment the above if moddatetime is available; otherwise update updated_at in application code.
