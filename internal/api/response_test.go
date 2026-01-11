package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
)

func TestJSONResponse_SetsContentType(t *testing.T) {
	w := httptest.NewRecorder()

	JSONResponse(w, map[string]string{"status": "ok"})

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestJSONResponse_EncodesData(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]any{
		"name":  "test",
		"count": 42,
	}
	JSONResponse(w, data)

	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", result["name"])
	}
	// JSON numbers decode as float64
	if result["count"] != float64(42) {
		t.Errorf("expected count 42, got '%v'", result["count"])
	}
}

func TestJSONError_SetsStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		wantStatus int
	}{
		{"bad request", http.StatusBadRequest, 400},
		{"not found", http.StatusNotFound, 404},
		{"internal error", http.StatusInternalServerError, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			JSONError(w, "error message", tt.status)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestJSONError_ReturnsErrorJSON(t *testing.T) {
	w := httptest.NewRecorder()

	JSONError(w, "something went wrong", http.StatusBadRequest)

	var result APIError
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Error != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got '%s'", result.Error)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestHandleError_OrcError_UsesHTTPStatus(t *testing.T) {
	tests := []struct {
		name       string
		err        *orcerrors.OrcError
		wantStatus int
		wantCode   string
	}{
		{
			name:       "task not found",
			err:        orcerrors.ErrTaskNotFound("TASK-001"),
			wantStatus: http.StatusNotFound,
			wantCode:   "TASK_NOT_FOUND",
		},
		{
			name:       "task running",
			err:        orcerrors.ErrTaskRunning("TASK-002"),
			wantStatus: http.StatusConflict,
			wantCode:   "TASK_RUNNING",
		},
		{
			name:       "not initialized",
			err:        orcerrors.ErrNotInitialized(),
			wantStatus: http.StatusBadRequest,
			wantCode:   "ORC_NOT_INITIALIZED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			HandleError(w, tt.err)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}

			var result APIError
			if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if result.Code != tt.wantCode {
				t.Errorf("expected code '%s', got '%s'", tt.wantCode, result.Code)
			}
		})
	}
}

func TestHandleError_GenericError_Returns500(t *testing.T) {
	w := httptest.NewRecorder()

	genericErr := errors.New("database connection failed")
	HandleError(w, genericErr)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}

	var result APIError
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Error != "database connection failed" {
		t.Errorf("expected error 'database connection failed', got '%s'", result.Error)
	}

	// Generic errors should not have a code
	if result.Code != "" {
		t.Errorf("expected no code for generic error, got '%s'", result.Code)
	}
}

func TestHandleOrcError_FormatsCorrectly(t *testing.T) {
	w := httptest.NewRecorder()

	err := orcerrors.ErrTaskNotFound("TASK-123")
	HandleOrcError(w, err)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	var result APIError
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Code != "TASK_NOT_FOUND" {
		t.Errorf("expected code 'TASK_NOT_FOUND', got '%s'", result.Code)
	}

	if result.Error != "task TASK-123 not found" {
		t.Errorf("expected error 'task TASK-123 not found', got '%s'", result.Error)
	}
}

func TestJSONResponseStatus_SetsStatusAndData(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"id": "TASK-001"}
	JSONResponseStatus(w, data, http.StatusCreated)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["id"] != "TASK-001" {
		t.Errorf("expected id 'TASK-001', got '%s'", result["id"])
	}
}

func TestNoContent_Returns204(t *testing.T) {
	w := httptest.NewRecorder()

	NoContent(w)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("expected empty body, got '%s'", w.Body.String())
	}
}
