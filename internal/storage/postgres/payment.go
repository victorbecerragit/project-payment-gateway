// Package postgres provides a PostgreSQL-backed implementation of payment.Repository.
//
// Status: STUB — not wired into the application.
// Wire via cmd/api/main.go once DATABASE_URL is available in the environment
// and the migration in storage/postgres/migrations/001_create_payments.sql has run.
package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// ErrNotImplemented is returned by all stub methods until a real driver is added.
var ErrNotImplemented = errors.New("postgres repository: not yet implemented")

// repository is a placeholder that satisfies payment.Repository at compile time.
// Replace the stub bodies with real pgx / database/sql calls.
type repository struct {
	// db *pgxpool.Pool  ← uncomment when wiring
	db *pgxpool.Pool
}

// NewRepository creates a Postgres-backed payment repository.
// dsn is a PostgreSQL connection string (e.g. "postgres://user:pass@host/db").
func NewRepository(ctx context.Context, dsn string) payment.Repository {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		slog.Error("unable to create connection pool", "error", err)
		os.Exit(1)
	}

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		slog.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}

	if err := runMigrations(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	return &repository{db: pool}
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Simple migration runner that applies embedded SQL files in order.
	// Since the SQL uses 'IF NOT EXISTS', direct execution is idempotent and safe for startup.
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, entry := range entries {
		content, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}

		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

const paymentFields = "id, transaction_id, customer_id, amount, currency, status, idempotency_key, description, created_at, updated_at"

func (r *repository) Save(ctx context.Context, p *payment.Payment) error {
	query := `
		INSERT INTO payments (
			id, transaction_id, customer_id, amount, currency, status, idempotency_key, description, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE SET
			transaction_id = EXCLUDED.transaction_id,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at;
	`

	// Convert domain float amount (dollars) to BIGINT (cents) for DB storage
	amountCents := int64(p.Amount.Value() * 100)

	_, err := r.db.Exec(ctx, query,
		p.ID,
		nullString(p.TransactionID),
		p.CustomerID.Value(),
		amountCents,
		string(p.Currency),
		string(p.Status),
		nullString(p.IdempotencyKey),
		nullString(p.Description),
		p.CreatedAt,
		p.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to save payment: %w", err)
	}
	return nil
}

func (r *repository) GetByID(ctx context.Context, id string) (*payment.Payment, error) {
	query := fmt.Sprintf("SELECT %s FROM payments WHERE id = $1", paymentFields)
	return r.scanRow(r.db.QueryRow(ctx, query, id))
}

func (r *repository) GetByIdempotencyKey(ctx context.Context, key string) (*payment.Payment, error) {
	query := fmt.Sprintf("SELECT %s FROM payments WHERE idempotency_key = $1", paymentFields)
	return r.scanRow(r.db.QueryRow(ctx, query, key))
}

func (r *repository) GetByProviderRef(ctx context.Context, providerRef string) (*payment.Payment, error) {
	query := fmt.Sprintf("SELECT %s FROM payments WHERE transaction_id = $1", paymentFields)
	return r.scanRow(r.db.QueryRow(ctx, query, providerRef))
}

// scanRow is a helper to map a database row back into a domain Payment entity.
func (r *repository) scanRow(row pgx.Row) (*payment.Payment, error) {
	var (
		p              payment.Payment
		amountCents    int64
		customerIDStr  string
		currencyStr    string
		statusStr      string
		transactionID  *string
		idempotencyKey *string
		description    *string
	)

	err := row.Scan(
		&p.ID,
		&transactionID,
		&customerIDStr,
		&amountCents,
		&currencyStr,
		&statusStr,
		&idempotencyKey,
		&description,
		&p.CreatedAt,
		&p.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, payment.ErrPaymentNotFound
		}
		return nil, err
	}

	// Rehydrate Value Objects and Enums
	p.Amount = payment.MustNewAmount(float64(amountCents) / 100.0)
	p.CustomerID = payment.MustNewCustomerID(customerIDStr)
	p.Currency = payment.Currency(currencyStr)
	p.Status = payment.Status(statusStr)

	if transactionID != nil { p.TransactionID = *transactionID }
	if idempotencyKey != nil { p.IdempotencyKey = *idempotencyKey }
	if description != nil { p.Description = *description }

	return &p, nil
}

func nullString(s string) *string {
	if s == "" { return nil }
	return &s
}

// Close closes the underlying database connection pool.
func (r *repository) Close() {
	if r.db != nil {
		r.db.Close()
	}
}
