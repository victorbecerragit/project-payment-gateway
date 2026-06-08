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
	"regexp"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

var migrationFilePattern = regexp.MustCompile(`^\d{3}_[a-z0-9_]+\.sql$`)

var requiredPaymentColumns = []string{
	"id",
	"transaction_id",
	"customer_id",
	"amount",
	"currency",
	"status",
	"idempotency_key",
	"description",
	"created_at",
	"updated_at",
}

// ErrNotImplemented is returned by all stub methods until a real driver is added.
var ErrNotImplemented = errors.New("postgres repository: not yet implemented")

// repository is a placeholder that satisfies payment.Repository at compile time.
// Replace the stub bodies with real pgx / database/sql calls.
type repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a Postgres-backed payment repository.
// dsn is a PostgreSQL connection string (e.g. "postgres://user:pass@host/db").
func NewRepository(ctx context.Context, dsn string, _ any) payment.Repository {
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

	if err := verifySchemaReadiness(ctx, pool); err != nil {
		slog.Error("database schema is not ready", "error", err)
		os.Exit(1)
	}

	return &repository{db: pool}
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Startup bootstrap only runs idempotent forward migrations embedded in the binary.
	// CI should separately validate reversible up/down migrations before merge.
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	migrationNames, err := validatedMigrationNames(entries)
	if err != nil {
		return err
	}

	for _, name := range migrationNames {
		content, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", name, err)
		}

		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", name, err)
		}
	}
	return nil
}

func validatedMigrationNames(entries []os.DirEntry) ([]string, error) {
	if len(entries) == 0 {
		return nil, errors.New("no migration files found")
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("unexpected migration subdirectory %q", entry.Name())
		}
		if !migrationFilePattern.MatchString(entry.Name()) {
			return nil, fmt.Errorf("invalid migration filename %q: expected NNN_description.sql", entry.Name())
		}
		names = append(names, entry.Name())
	}

	sort.Strings(names)
	for index := 1; index < len(names); index++ {
		if names[index] == names[index-1] {
			return nil, fmt.Errorf("duplicate migration filename %q", names[index])
		}
	}

	return names, nil
}

func verifySchemaReadiness(ctx context.Context, pool *pgxpool.Pool) error {
	var paymentsTableExists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.payments') IS NOT NULL`).Scan(&paymentsTableExists); err != nil {
		return fmt.Errorf("failed to verify payments table existence: %w", err)
	}
	if !paymentsTableExists {
		return errors.New("payments table is missing after migrations")
	}

	for _, columnName := range requiredPaymentColumns {
		var columnExists bool
		query := `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = 'public'
				  AND table_name = 'payments'
				  AND column_name = $1
			)
		`
		if err := pool.QueryRow(ctx, query, columnName).Scan(&columnExists); err != nil {
			return fmt.Errorf("failed to verify payments.%s column existence: %w", columnName, err)
		}
		if !columnExists {
			return fmt.Errorf("payments schema is missing required column %q", columnName)
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
		slog.Error("failed to save payment", "payment_id", p.ID, "error", err)
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
	p, err := r.scanRow(r.db.QueryRow(ctx, query, key))
	if errors.Is(err, payment.ErrPaymentNotFound) {
		return nil, nil
	}
	return p, err
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
		slog.Info("postgres connection pool closed")
	}
}
