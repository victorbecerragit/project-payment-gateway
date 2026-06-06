package handlers

import (
	"encoding/json"
	"net/http"

	apppayment "github.com/victorbecerragit/project-payment-gateway/internal/application/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/mapper"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/response"
)

type PaymentHandler struct {
	service apppayment.Service
}

func NewPaymentHandler(s apppayment.Service) *PaymentHandler {
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

	p := mapper.ToPaymentDomain(&req, idempotencyKey)

	if err := h.service.CreatePayment(r.Context(), p); err != nil {
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	resp := mapper.ToPaymentResponse(p)

	response.RespondWithJSON(w, http.StatusCreated, resp)
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := r.PathValue("payment_id")
	if paymentID == "" {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "payment_id path parameter is required")
		return
	}

	p, err := h.service.GetPayment(r.Context(), paymentID)
	if err != nil {
		response.RespondWithError(w, http.StatusNotFound, "Not Found", "Payment not found")
		return
	}

	resp := mapper.ToPaymentResponse(p)

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

	// TODO: Add signature verification logic here

	// TODO: Add webhook processing logic here

	response.RespondWithJSON(w, http.StatusOK, map[string]bool{"received": true})
}
