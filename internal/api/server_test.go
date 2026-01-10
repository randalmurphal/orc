package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/randalmurphal/orc/internal/hooks"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/skills"
)

func TestHealthEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := New(nil)

	// CORS headers are set on actual requests, not just OPTIONS
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header to be set")
	}
}

func TestListPromptsEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var prompts []prompt.PromptInfo
	if err := json.NewDecoder(w.Body).Decode(&prompts); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have at least one prompt (embedded)
	if len(prompts) == 0 {
		t.Error("expected at least one prompt")
	}

	// Check for implement phase
	found := false
	for _, p := range prompts {
		if p.Phase == "implement" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'implement' phase")
	}
}

func TestGetPromptVariablesEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/variables", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var vars map[string]string
	if err := json.NewDecoder(w.Body).Decode(&vars); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check for required variables
	if _, ok := vars["{{TASK_ID}}"]; !ok {
		t.Error("expected TASK_ID variable")
	}
}

func TestGetPromptEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/implement", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var p prompt.Prompt
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if p.Phase != "implement" {
		t.Errorf("expected phase 'implement', got %q", p.Phase)
	}

	if p.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestGetPromptEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetPromptDefaultEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/implement/default", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var p prompt.Prompt
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if p.Source != prompt.SourceEmbedded {
		t.Errorf("expected source 'embedded', got %q", p.Source)
	}
}

func TestSavePromptEndpoint_EmptyContent(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"content":""}`)
	req := httptest.NewRequest("PUT", "/api/prompts/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeletePromptEndpoint_NoOverride(t *testing.T) {
	srv := New(nil)

	// Try to delete a prompt that has no override
	req := httptest.NewRequest("DELETE", "/api/prompts/implement", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Hooks API Tests ===

func TestListHooksEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/hooks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var hookList []hooks.HookInfo
	if err := json.NewDecoder(w.Body).Decode(&hookList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty list is OK if no hooks exist
	if hookList == nil {
		t.Error("expected non-nil list")
	}
}

func TestGetHookTypesEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/hooks/types", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var types []hooks.HookType
	if err := json.NewDecoder(w.Body).Decode(&types); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(types) == 0 {
		t.Error("expected at least one hook type")
	}
}

func TestGetHookEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/hooks/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateHookEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/hooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateHookEndpoint_MissingName(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"type":"pre:tool","command":"echo test"}`)
	req := httptest.NewRequest("POST", "/api/hooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteHookEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/hooks/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Skills API Tests ===

func TestListSkillsEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/skills", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var skillList []skills.SkillInfo
	if err := json.NewDecoder(w.Body).Decode(&skillList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty list is OK if no skills exist
	if skillList == nil {
		t.Error("expected non-nil list")
	}
}

func TestGetSkillEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateSkillEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateSkillEndpoint_MissingName(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"prompt":"Do something"}`)
	req := httptest.NewRequest("POST", "/api/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteSkillEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
