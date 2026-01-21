package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_SetsHeaders(t *testing.T) {
	t.Parallel()
	handler := CORS(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT, DELETE, OPTIONS" {
		t.Errorf("Access-Control-Allow-Methods = %q, want %q", got, "GET, POST, PUT, DELETE, OPTIONS")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("Access-Control-Allow-Headers = %q, want %q", got, "Content-Type")
	}
}

func TestCORS_OptionsRequest_Returns200(t *testing.T) {
	t.Parallel()
	handlerCalled := false
	handler := CORS(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if handlerCalled {
		t.Error("wrapped handler should not be called for OPTIONS request")
	}
}

func TestCORS_PassesThroughToHandler(t *testing.T) {
	t.Parallel()
	handlerCalled := false
	handler := CORS(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if !handlerCalled {
		t.Error("wrapped handler should be called for non-OPTIONS request")
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("status code = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestCORS_AllMethods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		method string
	}{
		{http.MethodGet},
		{http.MethodPost},
		{http.MethodPut},
		{http.MethodDelete},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			handlerCalled := false
			handler := CORS(func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			rec := httptest.NewRecorder()

			handler(rec, req)

			if !handlerCalled {
				t.Errorf("wrapped handler should be called for %s request", tt.method)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
			}
		})
	}
}
