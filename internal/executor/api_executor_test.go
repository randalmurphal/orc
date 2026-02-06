// Tests for TASK-007: APIPhaseExecutor that makes HTTP requests with
// status validation and response capture.
//
// Coverage mapping:
//   SC-5:  TestAPI_SuccessStatusMatch
//   SC-6:  TestAPI_StatusMismatchError
//   SC-7:  TestAPI_NetworkError
//   SC-8:  TestAPI_Defaults
//   SC-9:  TestAPI_VariableInterpolation
//   SC-10: TestAPI_OutputVar
//   SC-12: TestAPI_ZeroCostAndDuration
//
// Edge cases:
//   TestAPI_EmptyBody, TestAPI_DefaultSuccessStatus, TestAPI_LargeResponse
//
// Failure modes:
//   TestAPI_StatusMismatchError, TestAPI_NetworkError, TestAPI_EmptyURL
package executor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-5: HTTP request with configured method, URL, headers, body
// =============================================================================

func TestAPI_SuccessStatusMatch(t *testing.T) {
	t.Parallel()

	var gotMethod, gotBody, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id": "deploy-123"}`)
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		Method:        "POST",
		URL:           server.URL + "/deploy",
		Headers:       map[string]string{"Authorization": "Bearer secret"},
		Body:          `{"branch": "main"}`,
		SuccessStatus: []int{200, 201},
	}

	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Verify request was made correctly
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("auth header = %q, want %q", gotAuth, "Bearer secret")
	}
	if gotBody != `{"branch": "main"}` {
		t.Errorf("body = %q, want %q", gotBody, `{"branch": "main"}`)
	}

	// Response body captured in Content
	if !containsSubstring(result.Content, "deploy-123") {
		t.Errorf("content = %q, expected response body captured", result.Content)
	}
}

func TestAPI_SuccessStatusMultiple(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
		fmt.Fprint(w, `{"created": true}`)
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "create"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		Method:        "POST",
		URL:           server.URL,
		SuccessStatus: []int{200, 201},
	}

	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED (201 is in success_status)", result.Status)
	}
}

// =============================================================================
// SC-6: Non-success HTTP status returns error
// =============================================================================

func TestAPI_StatusMismatchError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		URL:           server.URL,
		SuccessStatus: []int{200},
	}

	_, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for HTTP 500 when success_status=[200]")
	}

	// Error should contain the actual HTTP status code
	if !containsSubstring(err.Error(), "500") {
		t.Errorf("error should contain status code 500, got: %q", err.Error())
	}
}

// =============================================================================
// SC-7: Network failure returns error
// =============================================================================

func TestAPI_NetworkError(t *testing.T) {
	t.Parallel()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Point to a port that nothing is listening on
	cfg := APIPhaseConfig{
		URL:           "http://127.0.0.1:1",
		SuccessStatus: []int{200},
	}

	_, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
}

// =============================================================================
// SC-8: Defaults — GET method, success_status [200]
// =============================================================================

func TestAPI_Defaults(t *testing.T) {
	t.Parallel()

	var gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "check"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Only URL set — method should default to GET, success_status to [200]
	cfg := APIPhaseConfig{
		URL: server.URL,
	}

	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET (default)", gotMethod)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

func TestAPI_DefaultStatusRejects201(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated) // 201
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "check"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Default success_status is [200] — 201 should fail
	cfg := APIPhaseConfig{
		URL: server.URL,
	}

	_, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error: 201 should not match default success_status [200]")
	}
}

// =============================================================================
// SC-9: Variable interpolation in URL, headers, body
// =============================================================================

func TestAPI_VariableInterpolation(t *testing.T) {
	t.Parallel()

	var gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	vars := variable.VariableSet{
		"DEPLOY_TOKEN": "secret-token",
		"TASK_BRANCH":  "orc/TASK-007",
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          vars,
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		Method:        "POST",
		URL:           server.URL + "/deploy",
		Headers:       map[string]string{"Authorization": "Bearer {{DEPLOY_TOKEN}}"},
		Body:          `{"branch": "{{TASK_BRANCH}}"}`,
		SuccessStatus: []int{200},
	}

	_, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer secret-token" {
		t.Errorf("auth = %q, want %q", gotAuth, "Bearer secret-token")
	}
	if gotBody != `{"branch": "orc/TASK-007"}` {
		t.Errorf("body = %q, want %q", gotBody, `{"branch": "orc/TASK-007"}`)
	}
}

// =============================================================================
// SC-10: Output variable storage
// =============================================================================

func TestAPI_OutputVar(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id": "deploy-123"}`)
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          vars,
		RCtx:          rctx,
	}

	cfg := APIPhaseConfig{
		URL:           server.URL,
		SuccessStatus: []int{200},
		OutputVar:     "DEPLOY_RESPONSE",
	}

	_, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Stored in params.Vars
	if vars["DEPLOY_RESPONSE"] == "" {
		t.Error("expected DEPLOY_RESPONSE in params.Vars")
	}
	if !containsSubstring(vars["DEPLOY_RESPONSE"], "deploy-123") {
		t.Errorf("Vars DEPLOY_RESPONSE = %q, expected response body", vars["DEPLOY_RESPONSE"])
	}

	// Stored in rctx.PhaseOutputVars for persistence
	if rctx.PhaseOutputVars["DEPLOY_RESPONSE"] == "" {
		t.Error("expected DEPLOY_RESPONSE in rctx.PhaseOutputVars")
	}
}

// =============================================================================
// SC-12: Zero LLM cost and positive duration
// =============================================================================

func TestAPI_ZeroCostAndDuration(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "check"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		URL:           server.URL,
		SuccessStatus: []int{200},
	}

	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", result.InputTokens)
	}
	if result.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", result.OutputTokens)
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", result.CostUSD)
	}
	if result.DurationMS <= 0 {
		t.Errorf("DurationMS = %d, want > 0", result.DurationMS)
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestAPI_EmptyBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent) // 204
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "notify"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		URL:           server.URL,
		SuccessStatus: []int{204},
	}

	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED (204 in success_status)", result.Status)
	}
}

func TestAPI_DefaultSuccessStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "check"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Empty success_status list → should default to [200]
	cfg := APIPhaseConfig{
		URL:           server.URL,
		SuccessStatus: nil,
	}

	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED (default [200] matches 200)", result.Status)
	}
}

func TestAPI_LargeResponse(t *testing.T) {
	t.Parallel()

	// Generate a large response body (1MB+)
	largeBody := strings.Repeat("x", 1024*1024+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, largeBody)
	}))
	defer server.Close()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "large"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		URL:           server.URL,
		SuccessStatus: []int{200},
	}

	// Should complete (potentially with truncated content) but not error
	result, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Content should be non-empty (may be truncated)
	if result.Content == "" {
		t.Error("expected non-empty content even for large response")
	}
}

// =============================================================================
// Failure modes
// =============================================================================

func TestAPI_EmptyURL(t *testing.T) {
	t.Parallel()

	executor := NewAPIPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "bad"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := APIPhaseConfig{
		URL: "",
	}

	_, err := executor.ExecuteAPI(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

// =============================================================================
// Name() returns executor type name
// =============================================================================

func TestAPI_Name(t *testing.T) {
	t.Parallel()

	executor := NewAPIPhaseExecutor()
	if executor.Name() != "api" {
		t.Errorf("Name() = %q, want %q", executor.Name(), "api")
	}
}

// =============================================================================
// SC-11: Registry registration (both script and api in default registry)
// =============================================================================

func TestDefaultRegistry_ScriptRegistered(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	got, err := registry.Get("script")
	if err != nil {
		t.Fatalf("'script' type should be registered by default: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil executor for 'script' type")
	}
	if got.Name() != "script" {
		t.Errorf("Name() = %q, want %q", got.Name(), "script")
	}
}

func TestDefaultRegistry_APIRegistered(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	got, err := registry.Get("api")
	if err != nil {
		t.Fatalf("'api' type should be registered by default: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil executor for 'api' type")
	}
	if got.Name() != "api" {
		t.Errorf("Name() = %q, want %q", got.Name(), "api")
	}
}

// =============================================================================
// Existing executors still registered after adding new ones
// =============================================================================

func TestDefaultRegistry_ExistingTypesPreserved(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	// LLM and knowledge should still be registered
	for _, typeName := range []string{"llm", "knowledge"} {
		got, err := registry.Get(typeName)
		if err != nil {
			t.Errorf("'%s' type should still be registered: %v", typeName, err)
		}
		if got == nil {
			t.Errorf("expected non-nil executor for '%s' type", typeName)
		}
	}
}
