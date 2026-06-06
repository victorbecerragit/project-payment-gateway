package apppayment_test

import (
	"context"
	"testing"

	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	inmemory "github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
)

func TestProcessEvent_CompletesPayment(t *testing.T) {
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	// create payment
	p := &payment.Payment{
		Amount:         20.0,
		Currency:       "USD",
		CustomerID:     "cust-evt",
		Description:    "evt-test",
		IdempotencyKey: "evt-idem",
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	ev := &payment.PaymentEvent{
		Type:          payment.EventPaymentCompleted,
		PaymentID:     p.ID,
		TransactionID: "tx-evt-1",
	}

	if err := svc.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("process event failed: %v", err)
	}

	got, err := svc.GetPayment(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment failed: %v", err)
	}
	if got.Status != payment.StatusCompleted {
		t.Fatalf("expected completed; got %s", got.Status)
	}
	if got.TransactionID != "tx-evt-1" {
		t.Fatalf("expected transaction id set; got %s", got.TransactionID)
	}
}

func TestProcessEvent_UnknownType(t *testing.T) {
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	p := &payment.Payment{
		Amount:         5.0,
		Currency:       "USD",
		CustomerID:     "cust-unknown",
		IdempotencyKey: "idem-unknown",
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	ev := &payment.PaymentEvent{
		Type:      payment.EventType("something.else"),
		PaymentID: p.ID,
	}

	if err := svc.ProcessEvent(ctx, ev); err == nil {
		t.Fatalf("expected error for unknown event type")
	}
}
