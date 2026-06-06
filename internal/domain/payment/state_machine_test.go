package payment

import (
	"testing"
)

// legalTransitions lists every edge in the valid state machine.
var legalTransitions = []struct {
	from Status
	to   Status
}{
	{StatusPending, StatusProcessing},
	{StatusPending, StatusCancelled},
	{StatusProcessing, StatusCompleted},
	{StatusProcessing, StatusFailed},
	{StatusProcessing, StatusCancelled},
}

// illegalTransitions lists every edge that MUST be rejected.
var illegalTransitions = []struct {
	from Status
	to   Status
}{
	// pending cannot skip processing
	{StatusPending, StatusCompleted},
	{StatusPending, StatusFailed},
	// pending cannot stay pending
	{StatusPending, StatusPending},
	// processing cannot go back to pending
	{StatusProcessing, StatusPending},
	// processing cannot stay processing
	{StatusProcessing, StatusProcessing},
	// terminal -> any
	{StatusCompleted, StatusPending},
	{StatusCompleted, StatusProcessing},
	{StatusCompleted, StatusFailed},
	{StatusCompleted, StatusCancelled},
	{StatusCompleted, StatusCompleted},
	{StatusFailed, StatusPending},
	{StatusFailed, StatusProcessing},
	{StatusFailed, StatusCompleted},
	{StatusFailed, StatusCancelled},
	{StatusFailed, StatusFailed},
	{StatusCancelled, StatusPending},
	{StatusCancelled, StatusProcessing},
	{StatusCancelled, StatusCompleted},
	{StatusCancelled, StatusFailed},
	{StatusCancelled, StatusCancelled},
}

func paymentAt(s Status) *Payment {
	p := &Payment{ID: "pay_test", Status: s}
	return p
}

func TestCanTransitionTo_LegalTransitions(t *testing.T) {
	for _, tc := range legalTransitions {
		p := paymentAt(tc.from)
		if !p.CanTransitionTo(tc.to) {
			t.Errorf("expected %s->%s to be LEGAL, but was rejected", tc.from, tc.to)
		}
	}
}

func TestCanTransitionTo_IllegalTransitions(t *testing.T) {
	for _, tc := range illegalTransitions {
		p := paymentAt(tc.from)
		if p.CanTransitionTo(tc.to) {
			t.Errorf("expected %s->%s to be ILLEGAL, but was allowed", tc.from, tc.to)
		}
	}
}

func TestTransition_LegalPath(t *testing.T) {
	p := paymentAt(StatusPending)
	if err := p.Transition(StatusProcessing); err != nil {
		t.Fatalf("pending->processing: unexpected error: %v", err)
	}
	if p.Status != StatusProcessing {
		t.Fatalf("expected status processing, got %s", p.Status)
	}
	if err := p.Transition(StatusCompleted); err != nil {
		t.Fatalf("processing->completed: unexpected error: %v", err)
	}
	if p.Status != StatusCompleted {
		t.Fatalf("expected status completed, got %s", p.Status)
	}
}

func TestTransition_FailurePath(t *testing.T) {
	p := paymentAt(StatusPending)
	_ = p.Transition(StatusProcessing)
	if err := p.Transition(StatusFailed); err != nil {
		t.Fatalf("processing->failed: unexpected error: %v", err)
	}
	if p.Status != StatusFailed {
		t.Fatalf("expected status failed, got %s", p.Status)
	}
}

func TestTransition_CancelFromPending(t *testing.T) {
	p := paymentAt(StatusPending)
	if err := p.Transition(StatusCancelled); err != nil {
		t.Fatalf("pending->cancelled: unexpected error: %v", err)
	}
	if p.Status != StatusCancelled {
		t.Fatalf("expected status cancelled, got %s", p.Status)
	}
}

func TestTransition_CancelFromProcessing(t *testing.T) {
	p := paymentAt(StatusPending)
	_ = p.Transition(StatusProcessing)
	if err := p.Transition(StatusCancelled); err != nil {
		t.Fatalf("processing->cancelled: unexpected error: %v", err)
	}
}

func TestTransition_SkipProcessingIsIllegal(t *testing.T) {
	p := paymentAt(StatusPending)
	if err := p.Transition(StatusCompleted); err == nil {
		t.Error("expected error skipping processing, got nil")
	}
	if p.Status != StatusPending {
		t.Errorf("status should remain pending after rejected transition, got %s", p.Status)
	}
}

func TestTransition_TerminalStatesAreImmutable(t *testing.T) {
	terminals := []Status{StatusCompleted, StatusFailed, StatusCancelled}
	next := []Status{StatusPending, StatusProcessing, StatusCompleted, StatusFailed, StatusCancelled}
	for _, from := range terminals {
		for _, to := range next {
			p := paymentAt(from)
			if err := p.Transition(to); err == nil {
				t.Errorf("terminal %s should not transition to %s", from, to)
			}
			if p.Status != from {
				t.Errorf("status should remain %s, got %s", from, p.Status)
			}
		}
	}
}

func TestTransition_UpdatedAtIsSet(t *testing.T) {
	p := paymentAt(StatusPending)
	before := p.UpdatedAt
	_ = p.Transition(StatusProcessing)
	if !p.UpdatedAt.After(before) {
		t.Error("UpdatedAt should advance after a transition")
	}
}
