package handlers

import (
	"log/slog"
	"net/http"
	"time"

	apphealth "github.com/victorbecerragit/project-payment-gateway/internal/application/health"
	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/response"
)

type HealthHandler struct {
	service apphealth.Service
}

func NewHealthHandler(s apphealth.Service) *HealthHandler {
	return &HealthHandler{service: s}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	slog.Ctx(r.Context()).Debug("health check requested")
	status := "unhealthy"
	statusCode := http.StatusOK
	if h.service.Health() {
		status = "healthy"
	} else {
		statusCode = http.StatusServiceUnavailable
	}

	response.RespondWithJSON(w, statusCode, map[string]any{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	slog.Ctx(r.Context()).Debug("readiness check requested")
	status := "unhealthy"
	statusCode := http.StatusOK
	if h.service.Ready() {
		status = "ready"
	} else {
		statusCode = http.StatusServiceUnavailable
	}

	response.RespondWithJSON(w, statusCode, map[string]any{
		"status": status,
		"time":   time.Now().Format(time.RFC3339),
	})
}
