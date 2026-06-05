package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/response"
)

type PaymentHandler struct {
	service payment.Service
}

func NewPaymentHandler(s payment.Service) *PaymentHandler {
	return &PaymentHandler{service: s}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req dto.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "X-Idempotency-Key header is required")
		return
	}

	// Map DTO to Domain model
	p := &payment.Payment{
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		CustomerID:  req.CustomerID,
		// Note: CardToken is not yet used in the simple Domain model
		IdempotencyKey: idempotencyKey,
	}

	if err := h.service.CreatePayment(r.Context(), p); err != nil {
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	// Map Domain model back to Response DTO
	resp := dto.PaymentResponse{
		PaymentID:     p.ID,
		Status:        string(p.Status),
		Amount:        p.Amount,
		Currency:      p.Currency,
		TransactionID: p.TransactionID,
		CreatedAt:     p.CreatedAt,
	}

	response.RespondWithJSON(w, http.StatusCreated, resp)
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := r.PathValue("payment_id")
	if paymentID == "" {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Payment ID is required")
		return
	}

	p, err := h.service.GetPayment(r.Context(), paymentID)
	if err != nil {
		response.RespondWithError(w, http.StatusNotFound, "Not Found", "Payment not found")
		return
	}

	// Map Domain model back to Response DTO
	resp := dto.PaymentStatusResponse{
		PaymentID:     p.ID,
		Status:        string(p.Status),
		TransactionID: p.TransactionID,
		Timestamp:     p.UpdatedAt,
		UpdatedAt:     p.UpdatedAt,
	}

	response.RespondWithJSON(w, http.StatusOK, resp)
}

func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Check for webhook signature as required by OpenAPI
	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		response.RespondWithError(w, http.StatusUnauthorized, "Unauthorized", "X-Webhook-Signature header is required")
		return
	}

	var payload dto.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Invalid webhook payload")
		return
	}

	// TODO: Validate webhook signature

	event := &payment.PaymentEvent{
		Type:          payment.EventType(payload.EventType),
		PaymentID:     payload.PaymentID,
		TransactionID: payload.TransactionID,
		Timestamp:     payload.Timestamp,
	}

	if err := h.service.ProcessEvent(r.Context(), event); err != nil {
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", "Failed to process webhook")
		return
	}

	response.RespondWithJSON(w, http.StatusOK, map[string]bool{
		"received": true,
	})
}
