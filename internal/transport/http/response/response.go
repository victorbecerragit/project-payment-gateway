package response

import (
	"encoding/json"
	"net/http"

	"github.com/victorbecerragit/project-payment-gateway/internal/transport/http/dto"
)

// RespondWithJSON sends a success response with a JSON body.
func RespondWithJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// RespondWithError sends an error response following the ErrorResponse schema in openapi.yaml.
func RespondWithError(w http.ResponseWriter, status int, errType string, message string) {
	resp := dto.ErrorResponse{
		Error:   errType,
		Message: message,
		Code:    status,
	}
	RespondWithJSON(w, status, resp)
}
