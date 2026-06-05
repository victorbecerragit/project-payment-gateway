package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/victorbecerragit/project-payment-gateway/internal/application/health"
)

type HealthHandler struct {
	service health.Service
}

func NewHealthHandler(s health.Service) *HealthHandler {
	return &HealthHandler{service: s}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := "down"
	if h.service.Health() {
		status = "up"
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.service.Ready() {
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
	}
}
