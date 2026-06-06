package apppayment_test

import (
	"context"
	"testing"

	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	inmemory "github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
)

func TestCreatePayment_Idempotency(t *testing.T) {
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	p := &payment.Payment{
		Amount:         50.0,
		Currency:       "USD",
		CustomerID:     "cust-1",
		Description:    "test",
		IdempotencyKey: "idem-123",
	}

	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if p.ID == "" {
		t.Fatalf("expected payment ID to be set after create")
	}

	firstID := p.ID

	// Second create with same idempotency key should return existing
	p2 := &payment.Payment{
		Amount:         50.0,
		Currency:       "USD",
		CustomerID:     "cust-1",
		Description:    "test",
		IdempotencyKey: "idem-123",
	}

	if err := svc.CreatePayment(ctx, p2); err != nil {
		t.Fatalf("second create failed: %v", err)
	}

	if p2.ID != firstID {
		t.Fatalf("expected idempotent create to return same ID; got %s vs %s", p2.ID, firstID)
	}
}
