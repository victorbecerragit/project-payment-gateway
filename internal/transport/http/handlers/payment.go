package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/models"
)

type PaymentHandler struct {
	service payment.Service
}

func NewPaymentHandler(s payment.Service) *PaymentHandler {
	return &PaymentHandler{service: s}
}

func (h *PaymentHandler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req models.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		h.respondWithError(w, http.StatusBadRequest, "X-Idempotency-Key header is required")
		return
	}

	// Map DTO to Domain model
	p := &models.Payment{
		Amount:      req.Amount,
		Currency:    req.Currency,
		Description: req.Description,
		CustomerID:  req.CustomerID,
		// Note: CardToken is not yet used in the simple Domain model
		IdempotencyKey: idempotencyKey,
	}

	if err := h.service.CreatePayment(r.Context(), p); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Map Domain model back to Response DTO
	resp := models.PaymentResponse{
		PaymentID:     p.ID,
		Status:        p.Status,
		Amount:        p.Amount,
		Currency:      p.Currency,
		TransactionID: p.TransactionID,
		CreatedAt:     p.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("payment_id")
	if id == "" {
		h.respondWithError(w, http.StatusBadRequest, "Payment ID is required")
		return
	}

	p, err := h.service.GetPayment(r.Context(), id)
	if err != nil {
		h.respondWithError(w, http.StatusNotFound, "Payment not found")
		return
	}

	// Map Domain model back to Response DTO
	resp := models.PaymentResponse{
		PaymentID:     p.ID,
		Status:        p.Status,
		Amount:        p.Amount,
		Currency:      p.Currency,
		TransactionID: p.TransactionID,
		CreatedAt:     p.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Check for webhook signature as required by OpenAPI
	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		h.respondWithError(w, http.StatusUnauthorized, "X-Webhook-Signature header is required")
		return
	}

	var payload models.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid webhook payload")
		return
	}

	// TODO: Validate webhook signature

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{
		"received": true,
	})
}

func (h *PaymentHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   http.StatusText(code),
		Message: message,
		Code:    code,
	})
}
