package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	pgrepo "github.com/victorbecerragit/project-payment-gateway/internal/storage/postgres"
)

func setupTestDatabase(t *testing.T) (string, func()) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("payment_gateway"),
		postgres.WithUsername("user"),
		postgres.WithPassword("pass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	// Run migrations/schema setup
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect for schema setup: %s", err)
	}
	defer pool.Close()

	schema := `
	CREATE TABLE IF NOT EXISTS payments (
		id TEXT PRIMARY KEY,
		transaction_id TEXT,
		customer_id TEXT NOT NULL,
		amount NUMERIC(10,2) NOT NULL,
		currency TEXT NOT NULL,
		status TEXT NOT NULL,
		idempotency_key TEXT,
		description TEXT,
		created_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_payments_idempotency_key ON payments (idempotency_key);
	CREATE INDEX IF NOT EXISTS idx_payments_transaction_id ON payments (transaction_id);
	`
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatalf("failed to execute schema: %s", err)
	}

	return connStr, func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	}
}

func TestPostgresRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dsn, cleanup := setupTestDatabase(t)
	defer cleanup()

	payment.SetSupportedCurrencies([]string{"USD", "EUR"})
	ctx := context.Background()
	repo := pgrepo.NewRepository(ctx, dsn, nil) // Pass nil for tracer in tests

	t.Run("Save and GetByID", func(t *testing.T) {
		p, _ := payment.NewPayment("pay_123", "tx_001", "cust_123", 99.99, "USD", "Test Payment", "idem_001")
		
		err := repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("failed to save payment: %v", err)
		}

		got, err := repo.GetByID(ctx, "pay_123")
		if err != nil {
			t.Fatalf("failed to get payment: %v", err)
		}

		if got.ID != p.ID {
			t.Errorf("expected ID %s, got %s", p.ID, got.ID)
		}
		if got.Amount.Value() != 99.99 {
			t.Errorf("expected amount 99.99, got %f", got.Amount.Value())
		}
		if got.Status != payment.StatusPending {
			t.Errorf("expected status pending, got %s", got.Status)
		}
	})

	t.Run("Update existing payment", func(t *testing.T) {
		p, _ := repo.GetByID(ctx, "pay_123")
		
		err := p.Transition(payment.StatusProcessing)
		if err != nil {
			t.Fatalf("failed transition: %v", err)
		}
		p.TransactionID = "tx_updated"

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("failed to update payment: %v", err)
		}

		updated, _ := repo.GetByID(ctx, "pay_123")
		if updated.Status != payment.StatusProcessing {
			t.Errorf("expected status processing, got %s", updated.Status)
		}
		if updated.TransactionID != "tx_updated" {
			t.Errorf("expected transaction_id tx_updated, got %s", updated.TransactionID)
		}
	})

	t.Run("GetByIdempotencyKey", func(t *testing.T) {
		got, err := repo.GetByIdempotencyKey(ctx, "idem_001")
		if err != nil {
			t.Fatalf("failed to get by idempotency key: %v", err)
		}
		if got.ID != "pay_123" {
			t.Errorf("expected ID pay_123, got %s", got.ID)
		}

		// Non-existent key
		none, err := repo.GetByIdempotencyKey(ctx, "non_existent")
		if err != nil {
			t.Fatalf("expected nil error for missing key, got %v", err)
		}
		if none != nil {
			t.Error("expected nil payment for missing key")
		}
	})

	t.Run("GetByProviderRef", func(t *testing.T) {
		got, err := repo.GetByProviderRef(ctx, "tx_updated")
		if err != nil {
			t.Fatalf("failed to get by provider ref: %v", err)
		}
		if got.ID != "pay_123" {
			t.Errorf("expected ID pay_123, got %s", got.ID)
		}

		// Error case: not found
		_, err = repo.GetByProviderRef(ctx, "tx_unknown")
		if err != payment.ErrPaymentNotFound {
			t.Errorf("expected ErrPaymentNotFound, got %v", err)
		}
	})

	t.Run("Currency and Amount Integrity", func(t *testing.T) {
		// Test EUR and precision
		p, _ := payment.NewPayment("pay_eur", "tx_eur", "cust_456", 1234.56, "EUR", "Precision test", "idem_eur")
if err := repo.Save(ctx, p); err != nil {
				t.Fatalf("failed to save payment: %v", err)
			}

		got, _ := repo.GetByID(ctx, "pay_eur")
		if got.Currency != "EUR" {
			t.Errorf("expected EUR, got %s", got.Currency)
		}
		if got.Amount.Value() != 1234.56 {
			t.Errorf("expected 1234.56, got %f", got.Amount.Value())
		}
	})

	t.Run("GetByID Not Found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "void")
		if err != payment.ErrPaymentNotFound {
			t.Errorf("expected ErrPaymentNotFound, got %v", err)
		}
	})
}