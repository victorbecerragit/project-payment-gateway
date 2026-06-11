package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
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

	p, err := mapper.ToPaymentDomain(&req, idempotencyKey)
	if err != nil {
		slogext.Ctx(r.Context()).Warn("invalid create payment input", "error", err)
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", err.Error())
		return
	}

	if err := h.service.CreatePayment(r.Context(), p); err != nil { // Pass r.Context() to service
		h.handleServiceError(w, r.Context(), err) // Pass r.Context() to error handler
		return
	}

	resp := mapper.ToPaymentResponse(p)

	slogext.Ctx(r.Context()).Info("payment created successfully", "payment_id", p.ID, "status", p.Status, "duration_ms", time.Since(start).Milliseconds())
	response.RespondWithJSON(w, http.StatusCreated, resp)
}

func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" {
		switch payment.Status(status) {
		case payment.StatusPending, payment.StatusProcessing, payment.StatusCompleted, payment.StatusFailed, payment.StatusCancelled:
		default:
			response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "invalid status query parameter")
			return
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil {
			response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "invalid limit query parameter")
			return
		}
		if n < 1 || n > 200 {
			response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "limit must be between 1 and 200")
			return
		}
		limit = n
	}

	payments, err := h.service.ListPayments(r.Context(), payment.ListFilter{Status: status, Limit: limit})
	if err != nil {
		slogext.Ctx(r.Context()).Error("failed to list payments", "error", err)
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
		return
	}

	resp := make([]*dto.PaymentResponse, len(payments))
	for i, p := range payments {
		resp[i] = mapper.ToPaymentResponse(p)
	}
	response.RespondWithJSON(w, http.StatusOK, resp)
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
	// Check for webhook signature as required by OpenAPI (Stripe uses Stripe-Signature)
	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Webhook-Signature")
	}

	if signature == "" {
		slogext.Ctx(r.Context()).Warn("webhook received without signature")
		response.RespondWithError(w, http.StatusUnauthorized, "Unauthorized", "Webhook signature header is required")
		return
	}

	// Read body for verification and decoding
	body, err := io.ReadAll(r.Body)
	if err != nil {
		response.RespondWithError(w, http.StatusInternalServerError, "Internal Error", "Could not read request body")
		slogext.Ctx(r.Context()).Error("failed to read webhook request body", "error", err)
		return
	}

	start := time.Now()

	// 1 & 2. Parse and verify webhook payload via service provider delegation
	event, err := h.service.ParseWebhook(r.Context(), body, signature)
	if err != nil {
		if errors.Is(err, payment.ErrUnknownEventType) {
			slogext.Ctx(r.Context()).Info("ignoring unhandled webhook event type", "error", err)
			response.RespondWithJSON(w, http.StatusOK, map[string]bool{"received": true})
			return
		}
		slogext.Ctx(r.Context()).Warn("invalid webhook payload or signature", "signature", signature, "error", err)
		response.RespondWithError(w, http.StatusBadRequest, "Bad Request", "Invalid webhook payload or signature")
		return
	}

	// 3. Process Domain Event
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
	if errors.Is(err, payment.ErrPaymentNotFound) {
		slogext.Ctx(ctx).Warn("payment not found", "error", err)
		response.RespondWithError(w, http.StatusNotFound, "Not Found", "payment not found")
		return
	}

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
