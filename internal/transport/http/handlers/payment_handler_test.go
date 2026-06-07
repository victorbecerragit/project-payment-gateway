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

func (f *fakeService) CreatePayment(_ context.Context, _ *payment.Payment) error { return nil }
func (f *fakeService) GetPayment(_ context.Context, _ string) (*payment.Payment, error) {
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

type mockVerifier struct{}

func (v *mockVerifier) Verify(ctx context.Context, payload []byte, signature string) error {
	if signature == "invalid" {
		return errors.New("invalid signature")
	}
	return nil
}
