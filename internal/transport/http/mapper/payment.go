package mapper

import (
	dompayment "github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
)

// ToPaymentDomain maps a PaymentRequest DTO to a domain Payment entity.
func ToPaymentDomain(req *dto.PaymentRequest, idempotencyKey string) *dompayment.Payment {
	return &dompayment.Payment{
		Amount:         req.Amount,
		Currency:       req.Currency,
		Description:    req.Description,
		CustomerID:     req.CustomerID,
		IdempotencyKey: idempotencyKey,
	}
}

// ToPaymentResponse maps a domain Payment entity to a PaymentResponse DTO.
func ToPaymentResponse(p *dompayment.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		PaymentID:     p.ID,
		Status:        string(p.Status),
		Amount:        p.Amount,
		Currency:      p.Currency,
		TransactionID: p.TransactionID,
		CreatedAt:     p.CreatedAt,
	}
}

// ToPaymentEvent maps a WebhookPayload DTO to a domain PaymentEvent.
func ToPaymentEvent(payload *dto.WebhookPayload) *dompayment.PaymentEvent {
	return &dompayment.PaymentEvent{
		Type:          dompayment.EventType(payload.EventType),
		PaymentID:     payload.PaymentID,
		TransactionID: payload.TransactionID,
		Timestamp:     payload.Timestamp,
	}
}
