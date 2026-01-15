package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/models"
)

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// ReadyHandler handles readiness check requests
func ReadyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ready",
		"time":   time.Now().Format(time.RFC3339),
	})
}

// PaymentHandler handles payment creation and retrieval
func PaymentHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodPost:
		handleCreatePayment(w, r)
	case http.MethodGet:
		handleGetPayment(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCreatePayment(w http.ResponseWriter, r *http.Request) {
	var req models.PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Amount <= 0 {
		errorResponse(w, "Amount must be greater than zero", http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		errorResponse(w, "Currency is required", http.StatusBadRequest)
		return
	}
	if req.CustomerID == "" {
		errorResponse(w, "Customer ID is required", http.StatusBadRequest)
		return
	}

	// Simulate payment processing
	response := models.PaymentResponse{
		PaymentID:     generatePaymentID(),
		Status:        "pending",
		Amount:        req.Amount,
		Currency:      req.Currency,
		TransactionID: generateTransactionID(),
		CreatedAt:     time.Now(),
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func handleGetPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := r.URL.Query().Get("payment_id")
	if paymentID == "" {
		errorResponse(w, "payment_id parameter is required", http.StatusBadRequest)
		return
	}

	// Simulate payment retrieval
	response := models.PaymentResponse{
		PaymentID:     paymentID,
		Status:        "completed",
		Amount:        100.00,
		Currency:      "USD",
		TransactionID: generateTransactionID(),
		CreatedAt:     time.Now().Add(-1 * time.Hour),
	}

	json.NewEncoder(w).Encode(response)
}

// PaymentStatusHandler handles payment status queries
func PaymentStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	paymentID := r.URL.Query().Get("payment_id")
	if paymentID == "" {
		errorResponse(w, "payment_id parameter is required", http.StatusBadRequest)
		return
	}

	status := models.PaymentStatus{
		PaymentID:     paymentID,
		Status:        "completed",
		TransactionID: generateTransactionID(),
		UpdatedAt:     time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// WebhookHandler handles webhook notifications from payment providers
func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var webhook models.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		errorResponse(w, "Invalid webhook payload", http.StatusBadRequest)
		return
	}

	// Process webhook (in real implementation, would update payment status)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "received",
		"message": "Webhook processed successfully",
	})
}

func errorResponse(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(models.ErrorResponse{
		Error:   http.StatusText(code),
		Message: message,
		Code:    code,
	})
}

func generatePaymentID() string {
	return "pay_" + time.Now().Format("20060102150405")
}

func generateTransactionID() string {
	return "txn_" + time.Now().Format("20060102150405")
}
