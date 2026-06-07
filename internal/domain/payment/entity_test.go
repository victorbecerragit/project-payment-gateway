package payment

import (
	"testing"
)

func TestCanTransitionTo(t *testing.T) {
	p := &Payment{Status: StatusPending}

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
	p := NewPayment("id-1", "tx-1", "cust-1", 10.0, "USD", "desc", "")

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
