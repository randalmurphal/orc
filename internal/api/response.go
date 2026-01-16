// Package api provides the REST API and SSE server for orc.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
)

// APIError is the standard error response format.
type APIError struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details any    `json:"details,omitempty"`
}

// JSONResponse writes a successful JSON response.
func JSONResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// JSONError writes a simple error response.
func JSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(APIError{Error: message})
}

// HandleError inspects error type and writes appropriate response.
// This should be the primary error handler used going forward.
func HandleError(w http.ResponseWriter, err error) {
	var orcErr *orcerrors.OrcError
	if errors.As(err, &orcErr) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(orcErr.HTTPStatus())
		_ = json.NewEncoder(w).Encode(APIError{
			Error: orcErr.What,
			Code:  string(orcErr.Code),
		})
		return
	}
	// Fallback for unknown errors
	JSONError(w, err.Error(), http.StatusInternalServerError)
}

// HandleOrcError handles an OrcError specifically (for compatibility).
func HandleOrcError(w http.ResponseWriter, err *orcerrors.OrcError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus())
	_ = json.NewEncoder(w).Encode(APIError{
		Error: err.What,
		Code:  string(err.Code),
	})
}

// JSONResponseStatus writes a JSON response with a specific status code.
func JSONResponseStatus(w http.ResponseWriter, data any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// NoContent writes a 204 No Content response.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
