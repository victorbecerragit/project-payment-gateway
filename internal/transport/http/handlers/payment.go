package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/domain/payment"
	"github.com/victorbecerragit/project-payment-gateway/internal/platform/slogext"
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
	start := time.Now() // Initialize start here
	var req dto.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		slogext.Ctx(r.Context()).Error("invalid request body for CreatePayment", "error", err)
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Invalid request body")
		return
	}

	slogext.Ctx(r.Context()).Info("received CreatePayment request", "customer_id", req.CustomerID, "amount", req.Amount, "currency", req.Currency)
	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "X-Idempotency-Key header is required")
		return
	}

	p := mapper.ToPaymentDomain(&req, idempotencyKey)

	if err := h.service.CreatePayment(r.Context(), p); err != nil { // Pass r.Context() to service
		h.handleServiceError(w, r.Context(), err) // Pass r.Context() to error handler
		return
	}

	resp := mapper.ToPaymentResponse(p)

	slogext.Ctx(r.Context()).Info("payment created successfully", "payment_id", p.ID, "status", p.Status, "duration_ms", time.Since(start).Milliseconds())
	response.RespondWithJSON(w, http.StatusCreated, resp)
}

func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	paymentID := r.PathValue("payment_id")
	if paymentID == "" {
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "payment_id path parameter is required")
		return
	}

	p, err := h.service.GetPayment(r.Context(), paymentID)
	slogext.Ctx(r.Context()).Info("received GetPayment request", "payment_id", paymentID)
	if err != nil {
		if errors.Is(err, payment.ErrPaymentNotFound) {
			slogext.Ctx(r.Context()).Warn("payment not found", "payment_id", paymentID)
			response.RespondWithError(w, http.StatusNotFound, "Not Found", "Payment not found")
		} else {
			slogext.Ctx(r.Context()).Error("failed to get payment", "payment_id", paymentID, "error", err)
			response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		}
		return
	}

	resp := mapper.ToPaymentResponse(p)
	slogext.Ctx(r.Context()).Info("payment details retrieved", "payment_id", p.ID, "status", p.Status, "duration_ms", time.Since(start).Milliseconds())

	response.RespondWithJSON(w, http.StatusOK, resp)
}

func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	// Check for webhook signature as required by OpenAPI
	signature := r.Header.Get("X-Webhook-Signature")
	if signature == "" {
		slogext.Ctx(r.Context()).Warn("webhook received without signature")
		response.RespondWithError(w, http.StatusUnauthorized, "Unauthorized", "X-Webhook-Signature header is required")
		return
	}

	// Read body for verification and decoding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Error", "Could not read request body")
		slogext.Ctx(r.Context()).Error("failed to read webhook request body", "error", err)
		return
	}

	// 1. Verify Signature
	if err := h.verifier.Verify(r.Context(), body, signature); err != nil {
		slogext.Ctx(r.Context()).Warn("invalid webhook signature", "signature", signature, "error", err)
		response.RespondWithError(w, http.StatusUnauthorized, "Unauthorized", "Invalid webhook signature")
		return
	}

	// 2. Decode DTO
	var payload dto.WebhookPayload
	start := time.Now()
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		slogext.Ctx(r.Context()).Error("invalid webhook payload format", "error", err, "payload", string(body))
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Invalid webhook payload")
		return
	}

	// 3. Map to Domain Event and Process
	event := mapper.ToPaymentEvent(&payload)
	slogext.Ctx(r.Context()).Info("processing webhook event", "event_type", event.Type, "payment_id", event.PaymentID, "transaction_id", event.TransactionID)

	if err := h.service.ProcessEvent(r.Context(), event); err != nil {
		h.handleServiceError(w, r.Context(), err) // Pass r.Context() to error handler
		return
	}

	response.RespondWithJSON(w, http.StatusOK, map[string]bool{"received": true})
	slogext.Ctx(r.Context()).Info("webhook processed successfully", "event_type", event.Type, "payment_id", event.PaymentID, "duration_ms", time.Since(start).Milliseconds())
}

// handleServiceError maps domain and application errors to appropriate HTTP responses
func (h *PaymentHandler) handleServiceError(w http.ResponseWriter, ctx context.Context, err error) {
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

		slogext.Ctx(ctx).Warn("service error during request", "error_type", payErr.Type, "message", payErr.Message, "status_code", statusCode)
		response.RespondWithError(w, statusCode, errorTitle, payErr.Message) // Respond after logging
		return
	}

	// Default fallback for unexpected errors
	slogext.Ctx(ctx).Error("unexpected internal server error", "error", err)
	response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error()) // Respond after logging
}
