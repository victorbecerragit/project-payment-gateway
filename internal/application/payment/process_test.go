package apppayment_test

import (
	"context"
	"testing"

	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	inmemory "github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
)

func TestProcessEvent_CompletesPayment(t *testing.T) {
	originalSupportedCurrencies := payment.globalSupportedCurrencies
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.globalSupportedCurrencies = originalSupportedCurrencies }()
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	// create payment
	p := &payment.Payment{
		Amount:         payment.MustNewAmount(20.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_evt"),
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
	originalSupportedCurrencies := payment.globalSupportedCurrencies
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.globalSupportedCurrencies = originalSupportedCurrencies }()
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	p := &payment.Payment{
		Amount:         payment.MustNewAmount(5.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_unknown"),
		Description:    "unknown-test",
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

func TestProcessEvent_Idempotency(t *testing.T) {
	originalSupportedCurrencies := payment.globalSupportedCurrencies
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.globalSupportedCurrencies = originalSupportedCurrencies }()
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	p := &payment.Payment{
		Amount:         payment.MustNewAmount(30.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_idem"),
		Description:    "idempotency-test",
		IdempotencyKey: "idem-event",
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	ev := &payment.PaymentEvent{
		Type:          payment.EventPaymentCompleted,
		PaymentID:     p.ID,
		TransactionID: "tx-idem-1",
	}

	// First time: should succeed and transition through processing to completed
	if err := svc.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("first process event failed: %v", err)
	}

	// Second time: should return nil (idempotent) and not error
	if err := svc.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("second process event failed: %v", err)
	}

	got, err := svc.GetPayment(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment failed: %v", err)
	}
	if got.Status != payment.StatusCompleted {
		t.Fatalf("expected completed; got %s", got.Status)
	}
}

func TestProcessEvent_FailsPaymentFromPending(t *testing.T) {
	originalSupportedCurrencies := payment.globalSupportedCurrencies
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.globalSupportedCurrencies = originalSupportedCurrencies }()
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	// create payment in pending state
	p := &payment.Payment{
		Amount:         payment.MustNewAmount(40.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_fail"),
		Description:    "fail-test",
		IdempotencyKey: "fail-idem",
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// create a failed event
	ev := &payment.PaymentEvent{
		Type:          payment.EventPaymentFailed,
		PaymentID:     p.ID,
		TransactionID: "tx-fail-1",
	}

	// process the failed event
	if err := svc.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("process event failed: %v", err)
	}

	// verify the payment status is now failed
	got, err := svc.GetPayment(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment failed: %v", err)
	}
	if got.Status != payment.StatusFailed {
		t.Fatalf("expected failed; got %s", got.Status)
	}
	if got.TransactionID != "tx-fail-1" {
		t.Fatalf("expected transaction id set; got %s", got.TransactionID)
	}
}

func TestProcessEvent_CancelsPaymentFromPending(t *testing.T) {
	originalSupportedCurrencies := payment.globalSupportedCurrencies
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.globalSupportedCurrencies = originalSupportedCurrencies }()
	repo := inmemory.NewRepository()
	svc := apppayment.NewService(repo)
	ctx := context.Background()

	// Create payment in pending state
	p := &payment.Payment{
		Amount:         payment.MustNewAmount(50.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_cancel"),
		Description:    "cancel-test",
		IdempotencyKey: "cancel-idem",
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Create a cancelled event
	ev := &payment.PaymentEvent{
		Type:      payment.EventPaymentCancelled,
		PaymentID: p.ID,
	}

	// Process the cancelled event
	if err := svc.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("process event failed: %v", err)
	}

	got, err := svc.GetPayment(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment failed: %v", err)
	}
	if got.Status != payment.StatusCancelled {
		t.Fatalf("expected cancelled; got %s", got.Status)
	}
}
