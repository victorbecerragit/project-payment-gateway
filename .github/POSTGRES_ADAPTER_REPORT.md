# Postgres Adapter — Implementation Report

## Status: STUB (compile-checked, not wired)

The Postgres storage adapter has been scaffolded and satisfies the `payment.Repository`
interface at compile time. It is not wired into the application entrypoint.
Wire it once `DATABASE_URL` is available in the runtime environment.

---

## Files Delivered

| File | Purpose |
|------|---------|
| `internal/storage/postgres/payment.go` | Stub repository — all four interface methods present, returns `ErrNotImplemented` |
| `internal/storage/postgres/migrations/001_create_payments.sql` | Production-ready schema with indexes and design rationale |

---

## Repository Contract Change

**`internal/domain/payment/service.go`** — one method added to `Repository`:

```go
GetByProviderRef(ctx context.Context, providerRef string) (*Payment, error)
```

**Why**: Stripe webhooks carry the PaymentIntent ID (`pi_xxx`), not our internal `payment_id`,
when metadata is absent (Dashboard retry, metadata stripped by provider update).
Without this method, `ProcessEvent` has no fallback and silently fails.

**`internal/storage/inmemory/payment.go`** — linear scan by `p.TransactionID`, returns
`ErrPaymentNotFound` (not `nil, nil`) so callers can distinguish "not found" from "no key".

**`internal/application/payment/service.go`** — `ProcessEvent` now tries `GetByID` first,
then falls back to `GetByProviderRef(e.TransactionID)`:

```go
p, err := s.repo.GetByID(ctx, e.PaymentID)
if err != nil && e.TransactionID != "" {
    p, err = s.repo.GetByProviderRef(ctx, e.TransactionID)
}
if err != nil {
    return fmt.Errorf("payment not found by id %q or provider ref %q: %w", ...)
}
```

---

## Schema Design — `001_create_payments.sql`

```sql
CREATE TABLE payments (
    id               TEXT        PRIMARY KEY,
    transaction_id   TEXT        UNIQUE,      -- Stripe PI ID; UNIQUE = one charge per payment
    customer_id      TEXT        NOT NULL,
    amount           BIGINT      NOT NULL CHECK (amount > 0),   -- cents, no float drift
    currency         CHAR(3)     NOT NULL,
    status           TEXT        NOT NULL DEFAULT 'pending',    -- TEXT not enum (no DDL on new status)
    idempotency_key  TEXT        UNIQUE,
    description      TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payments_transaction_id  ON payments (transaction_id)   WHERE transaction_id IS NOT NULL;
CREATE INDEX idx_payments_idempotency_key ON payments (idempotency_key)  WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_payments_status          ON payments (status);
```

Key decisions:

- `transaction_id UNIQUE` — DB-level webhook deduplication; one provider charge per gateway payment.
- `idempotency_key UNIQUE` — belt-and-suspenders alongside the application-layer guard.
- `amount BIGINT` (cents) — eliminates floating-point precision drift on read/write round-trips.
- `status TEXT` (not Postgres enum) — new statuses (e.g. `refunded`, `captured`) require no DDL migration.
- Partial indexes on nullable columns avoid index bloat for rows where the column is NULL.

---

## Storage Package Structure

```
internal/storage/
  inmemory/
    payment.go                        ← GetByProviderRef added
  postgres/
    payment.go                        ← stub, compile-checked, ErrNotImplemented
    migrations/
      001_create_payments.sql         ← schema ready to apply
```

---

## Wiring Checklist (future session)

1. Add `DATABASE_URL` to `internal/platform/config/config.go`.
2. Add `pgxpool` (or `database/sql` + `pgx/v5/stdlib`) to `go.mod`.
3. Implement `Save`, `GetByID`, `GetByIdempotencyKey`, `GetByProviderRef` in `postgres/payment.go`.
4. In `cmd/api/main.go` select the repository implementation based on config:
   ```go
   if cfg.DatabaseURL != "" {
       repo = postgres.NewRepository(cfg.DatabaseURL)
   } else {
       repo = inmemory.NewRepository()
   }
   ```
5. Add `DATABASE_URL` secret to `k8s/deployment.yaml` (reference a Kubernetes Secret).
6. Run migration before application start (init container or pre-deploy job).

---

## Test Status at Time of Delivery

```
ok  internal/application/payment     0.005s
ok  internal/domain/payment          (cached)
ok  internal/provider/stripe         (cached)
ok  internal/transport/http          0.012s
ok  internal/transport/http/handlers (cached)
```

All packages build clean. Zero test failures. HTTP handlers unchanged.
