package apppayment_test

import (
	"context"
	"testing"

	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/tracing"
	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/provider"
	inmemory "github.com/victorbecerragit/project-payment-gateway/internal/storage/inmemory"
)

func TestProcessEvent_CompletesPayment(t *testing.T) {
	originalSupportedCurrencies := payment.GetSupportedCurrencies()
	originalCurrencyStrs := make([]string, 0, len(originalSupportedCurrencies))
	for k := range originalSupportedCurrencies {
		originalCurrencyStrs = append(originalCurrencyStrs, string(k))
	}
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.SetSupportedCurrencies(originalCurrencyStrs) }()
	repo := inmemory.NewRepository(tracing.NewNoOpTracer())
	svc := apppayment.NewService(repo, provider.NewMockProvider(tracing.NewNoOpTracer()), tracing.NewNoOpTracer(), nil)
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
	originalSupportedCurrencies := payment.GetSupportedCurrencies()
	originalCurrencyStrs := make([]string, 0, len(originalSupportedCurrencies))
	for k := range originalSupportedCurrencies {
		originalCurrencyStrs = append(originalCurrencyStrs, string(k))
	}
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.SetSupportedCurrencies(originalCurrencyStrs) }()
	repo := inmemory.NewRepository(tracing.NewNoOpTracer())
	svc := apppayment.NewService(repo, provider.NewMockProvider(tracing.NewNoOpTracer()), tracing.NewNoOpTracer(), nil)
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
	originalSupportedCurrencies := payment.GetSupportedCurrencies()
	originalCurrencyStrs := make([]string, 0, len(originalSupportedCurrencies))
	for k := range originalSupportedCurrencies {
		originalCurrencyStrs = append(originalCurrencyStrs, string(k))
	}
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.SetSupportedCurrencies(originalCurrencyStrs) }()
	repo := inmemory.NewRepository(tracing.NewNoOpTracer())
	svc := apppayment.NewService(repo, provider.NewMockProvider(tracing.NewNoOpTracer()), tracing.NewNoOpTracer(), nil)
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
	originalSupportedCurrencies := payment.GetSupportedCurrencies()
	originalCurrencyStrs := make([]string, 0, len(originalSupportedCurrencies))
	for k := range originalSupportedCurrencies {
		originalCurrencyStrs = append(originalCurrencyStrs, string(k))
	}
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.SetSupportedCurrencies(originalCurrencyStrs) }()
	repo := inmemory.NewRepository(tracing.NewNoOpTracer())
	svc := apppayment.NewService(repo, provider.NewMockProvider(tracing.NewNoOpTracer()), tracing.NewNoOpTracer(), nil)
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
	originalSupportedCurrencies := payment.GetSupportedCurrencies()
	originalCurrencyStrs := make([]string, 0, len(originalSupportedCurrencies))
	for k := range originalSupportedCurrencies {
		originalCurrencyStrs = append(originalCurrencyStrs, string(k))
	}
	payment.SetSupportedCurrencies([]string{"USD"})
	defer func() { payment.SetSupportedCurrencies(originalCurrencyStrs) }()
	repo := inmemory.NewRepository(tracing.NewNoOpTracer())
	svc := apppayment.NewService(repo, provider.NewMockProvider(tracing.NewNoOpTracer()), tracing.NewNoOpTracer(), nil)
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

// setupPayment is a test helper that creates and saves a payment, then optionally
// advances it to a given status via ProcessEvent to reduce boilerplate.
// It uses t.Name() as the idempotency key so each subtest gets a distinct payment.
func setupPayment(t *testing.T, svc apppayment.Service, repo payment.Repository, advanceTo payment.Status) *payment.Payment {
	t.Helper()
	ctx := context.Background()
	p := &payment.Payment{
		Amount:         payment.MustNewAmount(10.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_setup"),
		Description:    "setup payment",
		IdempotencyKey: "setup-" + t.Name(),
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("setupPayment: create failed: %v", err)
	}
	if advanceTo == payment.StatusPending {
		return p
	}
	// Advance to processing unconditionally first.
	switch advanceTo {
	case payment.StatusCompleted:
		ev := &payment.PaymentEvent{Type: payment.EventPaymentCompleted, PaymentID: p.ID, TransactionID: "tx-setup"}
		if err := svc.ProcessEvent(ctx, ev); err != nil {
			t.Fatalf("setupPayment: advance to completed failed: %v", err)
		}
	case payment.StatusFailed:
		ev := &payment.PaymentEvent{Type: payment.EventPaymentFailed, PaymentID: p.ID, TransactionID: "tx-setup"}
		if err := svc.ProcessEvent(ctx, ev); err != nil {
			t.Fatalf("setupPayment: advance to failed failed: %v", err)
		}
	case payment.StatusCancelled:
		ev := &payment.PaymentEvent{Type: payment.EventPaymentCancelled, PaymentID: p.ID, TransactionID: "tx-setup"}
		if err := svc.ProcessEvent(ctx, ev); err != nil {
			t.Fatalf("setupPayment: advance to cancelled failed: %v", err)
		}
	case payment.StatusProcessing:
		// ProcessEvent does not expose a "processing" target event; use direct repo Save.
		fetched, _ := repo.GetByID(ctx, p.ID)
		_ = fetched.Transition(payment.StatusProcessing)
		_ = repo.Save(ctx, fetched)
	}
	got, _ := repo.GetByID(ctx, p.ID)
	return got
}

func newSvc(t *testing.T) (apppayment.Service, payment.Repository) {
	t.Helper()
	payment.SetSupportedCurrencies([]string{"USD"})
	payment.SetSupportedCurrencies([]string{"USD"}) // Ensure currencies are set for domain validation
	repo := inmemory.NewRepository(tracing.NewNoOpTracer())
	return apppayment.NewService(repo, provider.NewMockProvider(tracing.NewNoOpTracer()), tracing.NewNoOpTracer(), nil), repo
}

// TestProcessEvent_TerminalSameEvent verifies that a duplicate terminal webhook is
// a safe no-op (idempotent delivery).
func TestProcessEvent_TerminalSameEvent(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name   payment.Status
		evType payment.EventType
	}{
		{payment.StatusCompleted, payment.EventPaymentCompleted},
		{payment.StatusFailed, payment.EventPaymentFailed},
		{payment.StatusCancelled, payment.EventPaymentCancelled},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			p := setupPayment(t, svc, repo, tc.name)
			ev := &payment.PaymentEvent{Type: tc.evType, PaymentID: p.ID, TransactionID: "tx-dup"}
			if err := svc.ProcessEvent(ctx, ev); err != nil {
				t.Fatalf("duplicate terminal event must be a no-op, got error: %v", err)
			}
			got, _ := svc.GetPayment(ctx, p.ID)
			if got.Status != tc.name {
				t.Fatalf("status changed unexpectedly: want %s, got %s", tc.name, got.Status)
			}
		})
	}
}

// TestProcessEvent_TerminalDifferentEvent verifies that the domain state machine
// rejects an event that would cause an illegal transition from a terminal state.
// e.g. a "completed" payment must not become "failed" on a subsequent webhook.
func TestProcessEvent_TerminalDifferentEvent(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()

	cases := []struct {
		name      string
		startAt   payment.Status
		incomingEv payment.EventType
	}{
		{"completed→failed", payment.StatusCompleted, payment.EventPaymentFailed},
		{"completed→cancelled", payment.StatusCompleted, payment.EventPaymentCancelled},
		{"failed→completed", payment.StatusFailed, payment.EventPaymentCompleted},
		{"failed→cancelled", payment.StatusFailed, payment.EventPaymentCancelled},
		{"cancelled→completed", payment.StatusCancelled, payment.EventPaymentCompleted},
		{"cancelled→failed", payment.StatusCancelled, payment.EventPaymentFailed},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := setupPayment(t, svc, repo, tc.startAt)
			ev := &payment.PaymentEvent{Type: tc.incomingEv, PaymentID: p.ID, TransactionID: "tx-illegal"}
			err := svc.ProcessEvent(ctx, ev)
			if err == nil {
				t.Fatalf("expected state machine error for %s, got nil", tc.name)
			}
			// Status must be unchanged.
			got, _ := svc.GetPayment(ctx, p.ID)
			if got.Status != tc.startAt {
				t.Fatalf("status mutated despite error: want %s, got %s", tc.startAt, got.Status)
			}
		})
	}
}

// TestProcessEvent_ProcessingReceivesTerminalEvent verifies that a payment already
// in the "processing" state can move directly to a terminal state without the
// pending→processing bridge (the bridge is only for pending payments).
func TestProcessEvent_ProcessingReceivesTerminalEvent(t *testing.T) {
	svc, repo := newSvc(t)
	ctx := context.Background()

	for _, tc := range []struct {
		evType  payment.EventType
		wantStatus payment.Status
	}{
		{payment.EventPaymentCompleted, payment.StatusCompleted},
		{payment.EventPaymentFailed, payment.StatusFailed},
		{payment.EventPaymentCancelled, payment.StatusCancelled},
	} {
		t.Run(string(tc.wantStatus), func(t *testing.T) {
			p := setupPayment(t, svc, repo, payment.StatusProcessing)
			ev := &payment.PaymentEvent{Type: tc.evType, PaymentID: p.ID, TransactionID: "tx-from-proc"}
			if err := svc.ProcessEvent(ctx, ev); err != nil {
				t.Fatalf("processing→%s must succeed, got error: %v", tc.wantStatus, err)
			}
			got, _ := svc.GetPayment(ctx, p.ID)
			if got.Status != tc.wantStatus {
				t.Fatalf("want %s, got %s", tc.wantStatus, got.Status)
			}
		})
	}
}

// TestProcessEvent_FallbackByProviderRef verifies that ProcessEvent can resolve a
// payment using TransactionID when PaymentID is missing from the webhook event.
// This covers the Stripe Dashboard retry path where metadata is absent.
func TestProcessEvent_FallbackByProviderRef(t *testing.T) {
	svc, _ := newSvc(t)
	ctx := context.Background()

	p := &payment.Payment{
		Amount:         payment.MustNewAmount(25.0),
		Currency:       payment.Currency("USD"),
		CustomerID:     payment.MustNewCustomerID("cust_provref"),
		Description:    "provider-ref fallback test",
		IdempotencyKey: "idem-provref",
	}
	if err := svc.CreatePayment(ctx, p); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	// The payment now has p.TransactionID set by the mock provider.
	providerRef := p.TransactionID
	if providerRef == "" {
		t.Fatal("expected mock provider to set a TransactionID")
	}

	// Simulate a webhook that carries only the provider reference, not our internal ID.
	ev := &payment.PaymentEvent{
		Type:          payment.EventPaymentCompleted,
		PaymentID:     "", // absent — simulates missing metadata
		TransactionID: providerRef,
	}
	if err := svc.ProcessEvent(ctx, ev); err != nil {
		t.Fatalf("fallback by provider ref failed: %v", err)
	}

	got, err := svc.GetPayment(ctx, p.ID)
	if err != nil {
		t.Fatalf("get payment failed: %v", err)
	}
	if got.Status != payment.StatusCompleted {
		t.Fatalf("expected completed; got %s", got.Status)
	}
}
