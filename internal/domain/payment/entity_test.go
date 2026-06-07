package payment

import (
	"errors"
	"testing"
)

func TestCanTransitionTo(t *testing.T) {
	p := &Payment{Status: StatusPending, CustomerID: MustNewCustomerID("cust_test")}

	if !p.CanTransitionTo(StatusProcessing) {
		t.Fatalf("expected pending -> processing to be allowed")
	}
	if p.CanTransitionTo(StatusCompleted) {
		t.Fatalf("expected pending -> completed to be disallowed (must go through processing)")
	}

	p.Status = StatusCompleted
	if p.CanTransitionTo(StatusPending) {
		t.Fatalf("expected completed -> pending to be disallowed")
	}
}

func TestTransition(t *testing.T) {
	originalSupportedCurrencies := globalSupportedCurrencies
	SetSupportedCurrencies([]string{"USD"})
	defer func() { globalSupportedCurrencies = originalSupportedCurrencies }()
	p, err := NewPayment("id-1", "tx-1", "cust_1", 10.0, "USD", "desc", "")
	if err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	if err := p.Transition(StatusProcessing); err != nil {
		t.Fatalf("unexpected error transitioning to processing: %v", err)
	}
	if p.Status != StatusProcessing {
		t.Fatalf("expected status processing; got %s", p.Status)
	}

	// invalid transition from completed back to pending
	p.Status = StatusCompleted
	if err := p.Transition(StatusPending); err == nil {
		t.Fatalf("expected error for invalid transition completed -> pending")
	}
}

func TestNewPayment_Validation(t *testing.T) {
	originalSupportedCurrencies := globalSupportedCurrencies
	SetSupportedCurrencies([]string{"USD", "EUR", "GBP"}) // Set a default set for tests
	defer func() { globalSupportedCurrencies = originalSupportedCurrencies }()
	tests := []struct {
		name        string
		customerID  string
		amount      float64
		currency    string
		description string
		wantErr     bool
	}{
		{
			name:        "valid payment",
			customerID:  "cust_1",
			amount:      10.0,
			currency:    "USD",
			description: "desc-1",
			wantErr:     false,
		},
		{
			name:        "invalid prefix customer ID",
			customerID:  "user_123",
			amount:      10.0,
			currency:    "USD",
			description: "desc-1",
			wantErr:     true,
		},
		{
			name:        "empty description",
			customerID:  "cust_1",
			amount:      10.0,
			currency:    "USD",
			description: "",
			wantErr:     true,
		},
		{
			name:        "zero amount",
			customerID:  "cust_1",
			amount:      0.0,
			currency:    "USD",
			description: "desc-1",
			wantErr:     true,
		},
		{
			name:        "negative amount",
			customerID:  "cust_1",
			amount:      -5.0,
			currency:    "USD",
			description: "desc-1",
			wantErr:     true,
		},
		{
			name:        "invalid currency length",
			customerID:  "cust_1",
			amount:      10.0,
			currency:    "US",
			description: "desc-1",
			wantErr:     true,
		},
		{
			name:        "unsupported currency",
			customerID:  "cust_1",
			amount:      10.0,
			currency:    "XYZ",
			description: "desc-1",
			wantErr:     true,
		},
		{
			name:        "empty currency",
			customerID:  "cust_1",
			amount:      10.0,
			currency:    "",
			description: "desc-1",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPayment("id-1", "tx-1", tt.customerID, tt.amount, tt.currency, tt.description, "idem-1")
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPayment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSetSupportedCurrencies(t *testing.T) {
	// Save current state to restore after test
	original := globalSupportedCurrencies
	defer func() { globalSupportedCurrencies = original }()

	t.Run("empty input results in empty map", func(t *testing.T) {
		SetSupportedCurrencies([]string{})
		if globalSupportedCurrencies == nil {
			t.Fatal("expected map to be initialized, got nil")
		}
		if len(globalSupportedCurrencies) != 0 {
			t.Errorf("expected empty map, got length %d", len(globalSupportedCurrencies))
		}
	})

	t.Run("duplicate entries are handled correctly", func(t *testing.T) {
		SetSupportedCurrencies([]string{"USD", "EUR", "USD", "GBP", "EUR"})
		expectedLen := 3
		if len(globalSupportedCurrencies) != expectedLen {
			t.Errorf("expected map length %d, got %d", expectedLen, len(globalSupportedCurrencies))
		}
		for _, c := range []string{"USD", "EUR", "GBP"} {
			if !globalSupportedCurrencies[Currency(c)] {
				t.Errorf("expected %s to be supported", c)
			}
		}
	})
}

func TestNewCustomerID_Validation(t *testing.T) {
	_, err := NewCustomerID("invalid_prefix")
	if err == nil {
		t.Fatal("expected error for invalid prefix")
	}

	cid, err := NewCustomerID("cust_valid")
	if err != nil {
		t.Fatalf("unexpected error for valid CustomerID: %v", err)
	}
	if cid.Value() != "cust_valid" {
		t.Errorf("expected cust_valid, got %s", cid.Value())
	}
}

func TestNewAmount_Validation(t *testing.T) {
	_, err := NewAmount(-10.0)
	if err == nil {
		t.Fatal("expected error for negative amount")
	}

	amt, err := NewAmount(100.50)
	if err != nil {
		t.Fatalf("unexpected error for valid amount: %v", err)
	}
	if amt.Value() != 100.50 {
		t.Errorf("expected 100.50, got %f", amt.Value())
	}
}

func TestPaymentError_TypeDistinction(t *testing.T) {
	originalSupportedCurrencies := globalSupportedCurrencies
	SetSupportedCurrencies([]string{"USD"})
	defer func() { globalSupportedCurrencies = originalSupportedCurrencies }()

	t.Run("state machine violation error type", func(t *testing.T) {
		p := &Payment{Status: StatusCompleted}
		err := p.Transition(StatusProcessing)

		var payErr *PaymentError
		if !errors.As(err, &payErr) {
			t.Fatalf("expected PaymentError, got %T", err)
		}
		if payErr.Type != ErrorTypeStateMachine {
			t.Errorf("expected error type %s, got %s", ErrorTypeStateMachine, payErr.Type)
		}
	})

	t.Run("domain logic error type", func(t *testing.T) {
		_, err := NewPayment("id", "tx", "cust_valid", -10.0, "USD", "desc", "idem")

		var payErr *PaymentError
		if !errors.As(err, &payErr) {
			t.Fatalf("expected PaymentError, got %T", err)
		}
		if payErr.Type != ErrorTypeDomain {
			t.Errorf("expected error type %s, got %s", ErrorTypeDomain, payErr.Type)
		}
	})
}

func TestTransition_TerminalStateError(t *testing.T) {
	originalSupportedCurrencies := globalSupportedCurrencies
	SetSupportedCurrencies([]string{"USD"})
	defer func() { globalSupportedCurrencies = originalSupportedCurrencies }()

	terminalStatuses := []Status{StatusCompleted, StatusFailed, StatusCancelled}
	// Attempt to transition to a status that is otherwise valid in the state machine
	nextStatus := StatusProcessing

	for _, status := range terminalStatuses {
		t.Run(string(status), func(t *testing.T) {
			p := &Payment{Status: status}

			err := p.Transition(nextStatus)
			if err == nil {
				t.Errorf("expected error when transitioning from terminal state %s to %s, got nil", status, nextStatus)
			}
		})
	}
}
