package mapper

import (
	dompayment "github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
)

// ToPaymentDomain maps a PaymentRequest DTO to a domain Payment entity.
func ToPaymentDomain(req *dto.PaymentRequest, idempotencyKey string) (*dompayment.Payment, error) {
	amount, err := dompayment.NewAmount(req.Amount)
	if err != nil {
		return nil, err
	}

	customerID, err := dompayment.NewCustomerID(req.CustomerID)
	if err != nil {
		return nil, err
	}

	return &dompayment.Payment{
		Amount:         amount,
		Currency:       dompayment.Currency(req.Currency),
		Description:    req.Description,
		CustomerID:     customerID,
		IdempotencyKey: idempotencyKey,
		PaymentMethod:  req.CardToken,
	}, nil
}

// ToPaymentResponse maps a domain Payment entity to a PaymentResponse DTO.
func ToPaymentResponse(p *dompayment.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		PaymentID:     p.ID,
		Status:        string(p.Status),
		Amount:        p.Amount.Value(),
		Currency:      string(p.Currency),
		TransactionID: p.TransactionID,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
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
