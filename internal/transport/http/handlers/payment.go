package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/mapper"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/response"
)

type PaymentHandler struct {
	service  payment.Service
	verifier payment.WebhookVerifier
}

func NewPaymentHandler(s payment.Service, v payment.WebhookVerifier) *PaymentHandler {
	return &PaymentHandler{
		service:  s,
		verifier: v,
	}
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
		h.handleServiceError(w, err)
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
		if errors.Is(err, payment.ErrPaymentNotFound) {
			response.RespondWithError(w, http.StatusNotFound, "Not Found", "Payment not found")
		} else {
			response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		}
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

	// Read body for verification and decoding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Error", "Could not read request body")
		return
	}

	// 1. Verify Signature
	if err := h.verifier.Verify(r.Context(), body, signature); err != nil {
		response.RespondWithError(w, http.StatusUnauthorized, "Unauthorized", "Invalid webhook signature")
		return
	}

	// 2. Decode DTO
	var payload dto.WebhookPayload
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Invalid webhook payload")
		return
	}

	// 3. Map to Domain Event and Process
	event := mapper.ToPaymentEvent(&payload)

	if err := h.service.ProcessEvent(r.Context(), event); err != nil {
		h.handleServiceError(w, err)
		return
	}

	response.RespondWithJSON(w, http.StatusOK, map[string]bool{"received": true})
}

// handleServiceError maps domain and application errors to appropriate HTTP responses
func (h *PaymentHandler) handleServiceError(w http.ResponseWriter, err error) {
	var payErr *payment.PaymentError
	if errors.As(err, &payErr) {
		statusCode := http.StatusInternalServerError
		errorTitle := "Internal Server Error"

		switch payErr.Type {
		case payment.ErrorTypeDomain:
			statusCode = http.StatusBadRequest
			errorTitle = "Bad Request"
		case payment.ErrorTypeStateMachine:
			statusCode = http.StatusConflict
			errorTitle = "Conflict"
		}

		response.RespondWithError(w, statusCode, errorTitle, payErr.Message)
		return
	}

	// Default fallback for unexpected errors
	response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
}
