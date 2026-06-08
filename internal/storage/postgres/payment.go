// Package postgres provides a PostgreSQL-backed implementation of payment.Repository.
//
// Status: STUB — not wired into the application.
// Wire via cmd/api/main.go once DATABASE_URL is available in the environment
// and the migration in storage/postgres/migrations/001_create_payments.sql has run.
package postgres

import (
	"context"
	"errors"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
)

// ErrNotImplemented is returned by all stub methods until a real driver is added.
var ErrNotImplemented = errors.New("postgres repository: not yet implemented")

// repository is a placeholder that satisfies payment.Repository at compile time.
// Replace the stub bodies with real pgx / database/sql calls.
type repository struct {
	// db *pgxpool.Pool  ← uncomment when wiring
}

// NewRepository creates a Postgres-backed payment repository.
// dsn is a PostgreSQL connection string (e.g. "postgres://user:pass@host/db").
func NewRepository(dsn string) payment.Repository {
	// TODO: open connection pool and verify connectivity
	// pool, err := pgxpool.New(ctx, dsn)
	_ = dsn
	return &repository{}
}

func (r *repository) Save(_ context.Context, _ *payment.Payment) error {
	return ErrNotImplemented
}

func (r *repository) GetByID(_ context.Context, _ string) (*payment.Payment, error) {
	return nil, ErrNotImplemented
}

func (r *repository) GetByIdempotencyKey(_ context.Context, _ string) (*payment.Payment, error) {
	return nil, ErrNotImplemented
}

func (r *repository) GetByProviderRef(_ context.Context, _ string) (*payment.Payment, error) {
	return nil, ErrNotImplemented
}
