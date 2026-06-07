package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
)

type fakeService struct {
	received *payment.PaymentEvent
	err      error
}

func (f *fakeService) CreatePayment(_ context.Context, _ *payment.Payment) error { return f.err }
func (f *fakeService) GetPayment(_ context.Context, paymentID string) (*payment.Payment, error) {
	// Simulate not found error for a specific ID or if f.err is set
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}
func (f *fakeService) ProcessEvent(_ context.Context, e *payment.PaymentEvent) error {
	f.received = e
	return f.err
}

func webhookPayload(t *testing.T, paymentID string) []byte {
	t.Helper()
	p := dto.WebhookPayload{EventType: string(payment.EventPaymentCompleted), PaymentID: paymentID, TransactionID: "tx-1", Timestamp: time.Now()}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestHandleWebhook(t *testing.T) {
	tests := []struct {
		name            string
		signatureHeader string
		serviceErr      error
		wantStatus      int
		wantPaymentID   string
	}{
		{name: "missing signature returns 401", wantStatus: 401},
		{name: "valid request dispatches event", signatureHeader: "sig", wantStatus: 200, wantPaymentID: "p-1"},
		{name: "invalid signature returns 401", signatureHeader: "invalid", wantStatus: 401},
		{name: "domain logic error returns 400", signatureHeader: "sig", serviceErr: &payment.PaymentError{Type: payment.ErrorTypeDomain, Message: "invalid amount"}, wantStatus: 400},
		{name: "state machine error returns 409", signatureHeader: "sig", serviceErr: &payment.PaymentError{Type: payment.ErrorTypeStateMachine, Message: "invalid transition"}, wantStatus: 409},
		{name: "service error returns 500", signatureHeader: "sig", serviceErr: errors.New("boom"), wantStatus: 500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{err: tc.serviceErr}
			verifier := &mockVerifier{}
			h := NewPaymentHandler(svc, verifier)

			req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(webhookPayload(t, "p-1")))
			if tc.signatureHeader != "" {
				req.Header.Set("X-Webhook-Signature", tc.signatureHeader)
			}
			rr := httptest.NewRecorder()
			h.HandleWebhook(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("status: want %d got %d", tc.wantStatus, rr.Code)
			}
			if tc.wantPaymentID != "" {
				if svc.received == nil { t.Fatal("no event received") }
				if svc.received.PaymentID != tc.wantPaymentID { t.Fatalf("id: want %q got %q", tc.wantPaymentID, svc.received.PaymentID) }
			}
		})
	}
}

func TestCreatePayment(t *testing.T) {
	tests := []struct {
		name           string
		idempotencyKey string
		serviceErr     error
		wantStatus     int
	}{
		{name: "missing idempotency key returns 400", wantStatus: 400},
		{name: "valid request returns 201", idempotencyKey: "key-1", wantStatus: 201},
		{name: "domain logic error returns 400", idempotencyKey: "key-1", serviceErr: &payment.PaymentError{Type: payment.ErrorTypeDomain, Message: "validation failed"}, wantStatus: 400},
		{name: "state machine error returns 409", idempotencyKey: "key-1", serviceErr: &payment.PaymentError{Type: payment.ErrorTypeStateMachine, Message: "conflict"}, wantStatus: 409},
		{name: "generic error returns 500", idempotencyKey: "key-1", serviceErr: errors.New("internal"), wantStatus: 500},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{err: tc.serviceErr}
			h := NewPaymentHandler(svc, nil)

			payload := dto.PaymentRequest{
				Amount:      100,
				Currency:    "USD",
				CustomerID:  "cust_1",
				Description: "test",
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewReader(body))
			if tc.idempotencyKey != "" {
				req.Header.Set("X-Idempotency-Key", tc.idempotencyKey)
			}
			rr := httptest.NewRecorder()
			h.CreatePayment(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status: want %d got %d", tc.wantStatus, rr.Code)
			}
		})
	}
}

func TestGetPayment(t *testing.T) {
	tests := []struct {
		name        string
		paymentID   string
		serviceErr  error
		wantStatus  int
	}{
		{name: "missing payment ID returns 400", paymentID: "", wantStatus: 400},
		{name: "payment not found returns 404", paymentID: "non-existent", serviceErr: payment.ErrPaymentNotFound, wantStatus: 404},
		{name: "generic service error returns 500", paymentID: "some-id", serviceErr: errors.New("database error"), wantStatus: 500},
		// Add a test case for successful retrieval if the fakeService can be extended to return a payment
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{err: tc.serviceErr}
			h := NewPaymentHandler(svc, nil)

			var req *http.Request
			if tc.paymentID == "" {
				req = httptest.NewRequest(http.MethodGet, "/api/v1/payments/", nil)
			} else {
				req = httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+tc.paymentID, nil)
				req = req.WithPathValue("payment_id", tc.paymentID)
			}

			rr := httptest.NewRecorder()
			h.GetPayment(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status: want %d got %d", tc.wantStatus, rr.Code)
			}
		})
	}
}

type mockVerifier struct{}

func (v *mockVerifier) Verify(ctx context.Context, payload []byte, signature string) error {
	if signature == "invalid" {
		return errors.New("invalid signature")
	}
	return nil
}
