package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claudeconfig"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// newTestServer creates a server with an isolated temp database.
// Use this for parallel tests that don't need a specific project setup.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	return New(&Config{WorkDir: tmpDir})
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

	// CORS headers are set on actual requests, not just OPTIONS
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header to be set")
	}
}

func TestListPromptsEndpoint(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetPromptDefaultEndpoint(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

	// Try to delete a prompt that has no override
	req := httptest.NewRequest("DELETE", "/api/prompts/implement", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Hooks API Tests (settings.json format) ===

func TestListHooksEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/hooks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Response is now a map of event -> hooks
	var hookMap map[string][]claudeconfig.Hook
	if err := json.NewDecoder(w.Body).Decode(&hookMap); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty map is OK if no hooks exist
	if hookMap == nil {
		t.Error("expected non-nil map")
	}
}

func TestGetHookTypesEndpoint(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/hooks/types", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var types []claudeconfig.HookEvent
	if err := json.NewDecoder(w.Body).Decode(&types); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(types) == 0 {
		t.Error("expected at least one hook type")
	}
}

func TestGetHookEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create empty settings
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{}`), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/hooks/PreToolUse", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateHookEndpoint_InvalidBody(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/hooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateHookEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// New format: event + hook
	body := `{"event": "PreToolUse", "hook": {"matcher": "Read", "hooks": [{"type": "command", "command": "echo test"}]}}`
	req := httptest.NewRequest("POST", "/api/hooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteHookEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create empty settings
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{}`), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/hooks/PreToolUse", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Skills API Tests (SKILL.md format) ===

func TestListSkillsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude/skills directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/skills", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var skillList []claudeconfig.SkillInfo
	if err := json.NewDecoder(w.Body).Decode(&skillList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty list is OK if no skills exist
	if skillList == nil {
		t.Error("expected non-nil list")
	}
}

func TestGetSkillEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateSkillEndpoint_InvalidBody(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

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
	t.Parallel()
	srv := newTestServer(t)

	body := bytes.NewBufferString(`{"description":"Something"}`)
	req := httptest.NewRequest("POST", "/api/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateSkillEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude/skills directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"name": "test-skill", "description": "A test skill", "content": "Do something useful"}`
	req := httptest.NewRequest("POST", "/api/skills", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the SKILL.md was created
	skillPath := filepath.Join(tmpDir, ".claude", "skills", "test-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("expected SKILL.md file to be created")
	}
}

func TestDeleteSkillEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteSkillEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(tmpDir, ".claude", "skills", "delete-skill")
	_ = os.MkdirAll(skillDir, 0755)
	skillMD := `---
name: delete-skill
description: To be deleted
---
Some content
`
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/skills/delete-skill", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify directory was deleted
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("expected skill directory to be deleted")
	}
}

// === Settings API Tests ===

func TestGetSettingsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUpdateSettingsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"env": {"TEST_VAR": "test_value"}}`
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// === Tools API Tests ===

func TestListToolsEndpoint(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tools", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tools []claudeconfig.ToolInfo
	if err := json.NewDecoder(w.Body).Decode(&tools); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tools) == 0 {
		t.Error("expected at least one tool")
	}
}

func TestListToolsByCategory(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tools?by_category=true", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var byCategory map[string][]claudeconfig.ToolInfo
	if err := json.NewDecoder(w.Body).Decode(&byCategory); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(byCategory) == 0 {
		t.Error("expected at least one category")
	}
}

func TestGetToolPermissionsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tools/permissions", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUpdateToolPermissionsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"allow": ["Read", "Write"], "deny": ["Bash"]}`
	req := httptest.NewRequest("PUT", "/api/tools/permissions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// === Agents API Tests ===

func TestListAgentsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCreateAgentEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"name": "test-agent", "description": "A test agent"}`
	req := httptest.NewRequest("POST", "/api/agents", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetAgentEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/agents/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Scripts API Tests ===

func TestListScriptsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/scripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDiscoverScriptsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create scripts directory with a test script
	scriptsDir := filepath.Join(tmpDir, ".claude", "scripts")
	_ = os.MkdirAll(scriptsDir, 0755)
	_ = os.WriteFile(filepath.Join(scriptsDir, "test.sh"), []byte("#!/bin/bash\n# Test script\necho hello"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/scripts/discover", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var scripts []claudeconfig.ProjectScript
	if err := json.NewDecoder(w.Body).Decode(&scripts); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(scripts) == 0 {
		t.Error("expected at least one discovered script")
	}
}

// === CLAUDE.md API Tests ===

func TestGetClaudeMDEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create CLAUDE.md
	_ = os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Project CLAUDE.md\n\nTest content"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/claudemd", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var claudeMD claudeconfig.ClaudeMD
	if err := json.NewDecoder(w.Body).Decode(&claudeMD); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if claudeMD.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestGetClaudeMDEndpoint_EmptyProject(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/claudemd", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Returns 200 with empty content for editing purposes (not 404)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Content string `json:"content"`
		Path    string `json:"path"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if resp.Path == "" {
		t.Error("expected path to be set")
	}
}

func TestUpdateClaudeMDEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"content": "# Updated CLAUDE.md\n\nNew content"}`
	req := httptest.NewRequest("PUT", "/api/claudemd", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was written
	content, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	if string(content) != "# Updated CLAUDE.md\n\nNew content" {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestGetClaudeMDHierarchyEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create project CLAUDE.md
	_ = os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Project"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/claudemd/hierarchy", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var hierarchy claudeconfig.ClaudeMDHierarchy
	if err := json.NewDecoder(w.Body).Decode(&hierarchy); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if hierarchy.Project == nil {
		t.Error("expected project CLAUDE.md in hierarchy")
	}
}

// === Task API Tests ===

func TestListTasksEndpoint_EmptyDir(t *testing.T) {
	t.Parallel()
	// Create temp dir for .orc
	tmpDir := t.TempDir()

	// Create minimal .orc structure
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Tasks      []*task.Task `json:"tasks"`
		Total      int          `json:"total"`
		Page       int          `json:"page"`
		Limit      int          `json:"limit"`
		TotalPages int          `json:"total_pages"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(resp.Tasks))
	}
}

// TestListTasksEndpoint_NoOrcDir tests that the API returns an empty list
// when started from a directory that is not an orc project (no .orc directory).
// This is the fix for the issue where the server breaks when started from a
// different directory than the project.
func TestListTasksEndpoint_NoOrcDir(t *testing.T) {
	t.Parallel()
	// Create a temp dir that is NOT an orc project (no .orc)
	tmpDir := t.TempDir()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 OK with empty list, not error
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Tasks      []*task.Task `json:"tasks"`
		Total      int          `json:"total"`
		Page       int          `json:"page"`
		Limit      int          `json:"limit"`
		TotalPages int          `json:"total_pages"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(resp.Tasks))
	}
}

func TestListTasksEndpoint_WithTasks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-001", "Test Task")
	tsk.Description = "A test task"
	tsk.Status = task.StatusCreated
	tsk.Weight = "small"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Tasks      []*task.Task `json:"tasks"`
		Total      int          `json:"total"`
		Page       int          `json:"page"`
		Limit      int          `json:"limit"`
		TotalPages int          `json:"total_pages"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(resp.Tasks))
	}

	if resp.Tasks[0].ID != "TASK-001" {
		t.Errorf("expected task ID 'TASK-001', got %q", resp.Tasks[0].ID)
	}
}

func TestListTasksEndpoint_Pagination(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and multiple tasks for pagination testing
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	for i := 1; i <= 25; i++ {
		tsk := task.New(fmt.Sprintf("TASK-%03d", i), fmt.Sprintf("Test Task %d", i))
		tsk.Description = fmt.Sprintf("Test task number %d", i)
		tsk.Status = task.StatusCreated
		tsk.Weight = "small"
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task %d: %v", i, err)
		}
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Test pagination with page=1, limit=10
	req := httptest.NewRequest("GET", "/api/tasks?page=1&limit=10", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Tasks      []task.Task `json:"tasks"`
		Total      int         `json:"total"`
		Page       int         `json:"page"`
		Limit      int         `json:"limit"`
		TotalPages int         `json:"total_pages"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 10 {
		t.Errorf("expected 10 tasks, got %d", len(resp.Tasks))
	}
	if resp.Total != 25 {
		t.Errorf("expected total 25, got %d", resp.Total)
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", resp.TotalPages)
	}
}

func TestGetTaskEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-002", "Get Test Task")
	tsk.Description = "For testing GET endpoint"
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-002", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var loaded task.Task
	if err := json.NewDecoder(w.Body).Decode(&loaded); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if loaded.ID != "TASK-002" {
		t.Errorf("expected task ID 'TASK-002', got %q", loaded.ID)
	}

	if loaded.Title != "Get Test Task" {
		t.Errorf("expected title 'Get Test Task', got %q", loaded.Title)
	}
}

func TestGetTaskEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateTaskEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{"title":"New Task","description":"Create test","weight":"small"}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("expected status 200 or 201, got %d: %s", w.Code, w.Body.String())
	}

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.Title != "New Task" {
		t.Errorf("expected title 'New Task', got %q", tsk.Title)
	}

	if tsk.ID == "" {
		t.Error("expected non-empty task ID")
	}
}

func TestCreateTaskEndpoint_MissingTitle(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{"description":"No title"}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateTaskEndpoint_InvalidJSON(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteTaskEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	// Create a task to delete via backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-DEL-001", "Delete Test")
	tsk.Status = task.StatusCompleted
	tsk.Weight = task.WeightSmall
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-DEL-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify task was deleted
	_, err = srv.Backend().LoadTask("TASK-DEL-001")
	if err == nil {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTaskEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteTaskEndpoint_RunningTask(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	// Create a running task via backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-RUN-001", "Running Task")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightSmall
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-RUN-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	// Verify task still exists
	_, err = srv.Backend().LoadTask("TASK-RUN-001")
	if err != nil {
		t.Error("running task should not have been deleted")
	}
}

func TestServerConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()

	if cfg.Addr != ":8080" {
		t.Errorf("expected addr ':8080', got %q", cfg.Addr)
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Addr: ":9090",
	}

	srv := New(cfg)

	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

// TranscriptFile is needed for decoding transcripts response
type TranscriptFile struct {
	Filename  string `json:"filename"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// === Config API Tests ===

func TestGetConfigEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create config directory and file
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	configYAML := `version: 1
model: sonnet
max_iterations: 30
timeout: 10m
`
	_ = os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfigEndpoint_NoConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// No config file exists
	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should still return OK with default config
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfigEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create config directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Update config with new values
	body := bytes.NewBufferString(`{
		"profile": "safe",
		"automation": {
			"gates_default": "human",
			"retry_enabled": true,
			"retry_max": 5
		},
		"execution": {
			"model": "claude-opus-4-20250514",
			"max_iterations": 50,
			"timeout": "1h"
		},
		"git": {
			"branch_prefix": "test/",
			"commit_prefix": "[test]"
		}
	}`)
	req := httptest.NewRequest("PUT", "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response contains updated values
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["profile"] != "safe" {
		t.Errorf("expected profile 'safe', got %v", resp["profile"])
	}
}

func TestUpdateConfigEndpoint_InvalidBody(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("PUT", "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetConfigWithSourcesEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory and config
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	configContent := `profile: safe
model: claude-sonnet
`
	_ = os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/config?with_sources=true", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should have sources field
	sources, ok := resp["sources"].(map[string]any)
	if !ok {
		t.Fatal("expected sources field in response")
	}

	// Check that profile source is tracked
	profileSource, ok := sources["profile"].(map[string]any)
	if !ok {
		t.Fatal("expected profile in sources")
	}

	if profileSource["source"] == "" {
		t.Error("expected non-empty source for profile")
	}
}

func TestGetConfigStatsEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	// Create config file
	configYAML := `
profile: auto
model: claude-sonnet-4
`
	_ = os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configYAML), 0644)

	// Create CLAUDE.md
	_ = os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Test\n\nTest content"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/config/stats", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should have all required fields
	if _, ok := resp["slashCommandsCount"]; !ok {
		t.Error("expected slashCommandsCount in response")
	}
	if _, ok := resp["claudeMdSize"]; !ok {
		t.Error("expected claudeMdSize in response")
	}
	if _, ok := resp["mcpServersCount"]; !ok {
		t.Error("expected mcpServersCount in response")
	}
	if profile, ok := resp["permissionsProfile"].(string); !ok || profile == "" {
		t.Error("expected non-empty permissionsProfile in response")
	}
}

func TestGetSettingsHierarchyEndpoint(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	// Create project settings
	projectSettings := `{"env": {"PROJECT_VAR": "project_value"}}`
	_ = os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(projectSettings), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/settings/hierarchy", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Should have merged, global, project, and sources fields
	if _, ok := resp["merged"]; !ok {
		t.Error("expected merged field in response")
	}
	if _, ok := resp["global"]; !ok {
		t.Error("expected global field in response")
	}
	if _, ok := resp["project"]; !ok {
		t.Error("expected project field in response")
	}
	if _, ok := resp["sources"]; !ok {
		t.Error("expected sources field in response")
	}

	// Check project path is set
	project, ok := resp["project"].(map[string]any)
	if !ok {
		t.Fatal("expected project to be an object")
	}
	if project["path"] == "" {
		t.Error("expected non-empty project path")
	}
}

// === Publisher Test ===

func TestServerPublisher(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	// Publisher method should return the internal publisher
	pub := srv.Publisher()
	if pub == nil {
		t.Error("expected non-nil publisher")
	}
}

// === Publish Tests ===

func TestPublishNoSubscribers(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	// Publish should not panic with no subscribers
	srv.Publish("NONEXISTENT", Event{Type: "test", Data: "hello"})
}

// === Save Prompt Success Test ===

func TestSavePromptEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc/prompts directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "prompts"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{"content":"Custom prompt content for testing"}`)
	req := httptest.NewRequest("PUT", "/api/prompts/implement", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify prompt was saved
	content, err := os.ReadFile(filepath.Join(tmpDir, ".orc", "prompts", "implement.md"))
	if err != nil {
		t.Errorf("failed to read saved prompt: %v", err)
	}
	if string(content) != "Custom prompt content for testing" {
		t.Errorf("prompt content mismatch: got %q", string(content))
	}
}

func TestDeletePromptEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc/prompts directory with a prompt
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "prompts"), 0755)
	_ = os.WriteFile(filepath.Join(tmpDir, ".orc", "prompts", "test.md"), []byte("test content"), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/prompts/test", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify file was deleted
	if _, err := os.Stat(filepath.Join(tmpDir, ".orc", "prompts", "test.md")); !os.IsNotExist(err) {
		t.Error("expected prompt file to be deleted")
	}
}

// === Get Plan Success Test ===

func TestGetPlanEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend with task and plan
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-010", "Plan Test")
	tsk.Status = task.StatusPlanned
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-010/plan", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// === Run Task Additional Tests ===

func TestRunTaskEndpoint_Success_UpdatesStatusAndReturnsTask(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	// Create task with planned status (can be run) via backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-RUN", "Test Task")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightMedium
	tsk.WorkflowID = "implement-medium" // Required: workflow_id must be set
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-RUN/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Status string `json:"status"`
		TaskID string `json:"task_id"`
		Task   struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"task"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response includes task with updated status
	if response.Status != "started" {
		t.Errorf("expected status 'started', got '%s'", response.Status)
	}
	if response.Task.ID != "TASK-RUN" {
		t.Errorf("expected task id 'TASK-RUN', got '%s'", response.Task.ID)
	}
	if response.Task.Status != "TASK_STATUS_RUNNING" {
		t.Errorf("expected task status 'TASK_STATUS_RUNNING', got '%s'", response.Task.Status)
	}

	// Verify task was updated in database
	updatedTask, err := srv.Backend().LoadTask("TASK-RUN")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	if updatedTask.Status != task.StatusRunning {
		t.Errorf("expected task status 'running', got '%s'", updatedTask.Status)
	}
}

// TestRunTaskEndpoint_SetsCurrentPhase verifies that when a task is run,
// its current_phase is set to the first phase in the plan. This ensures
// the UI shows the task in the correct column (e.g., "Spec" or "Implement")
// instead of "Queued".
func TestRunTaskEndpoint_SetsCurrentPhase(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	// Create task with planned status (can be run) via backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-PHASE", "Test Task")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightMedium
	tsk.WorkflowID = "implement-medium" // Required: workflow_id must be set
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-PHASE/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Task struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			CurrentPhase string `json:"currentPhase"` // protojson uses camelCase
		} `json:"task"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response includes task with current_phase set to first phase
	if response.Task.CurrentPhase != "spec" {
		t.Errorf("expected response task currentPhase 'spec', got '%s'", response.Task.CurrentPhase)
	}

	// Verify task has current_phase set
	updatedTask, err := srv.Backend().LoadTask("TASK-PHASE")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	if updatedTask.CurrentPhase != "spec" {
		t.Errorf("expected task current_phase 'spec', got '%s'", updatedTask.CurrentPhase)
	}
}

func TestRunTaskEndpoint_TaskCannotRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task with running status
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-011", "Running Task")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-011/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// === Blocking Enforcement Tests ===

func TestRunTaskEndpoint_BlockedByIncompleteTasks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and tasks
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create blocking task (not completed)
	blocker := task.New("TASK-BLOCKER", "Blocking Task")
	blocker.Status = task.StatusPlanned
	blocker.Weight = "medium"
	if err := backend.SaveTask(blocker); err != nil {
		t.Fatalf("failed to save blocker task: %v", err)
	}

	// Create task that is blocked by the incomplete task
	blocked := task.New("TASK-BLOCKED", "Blocked Task")
	blocked.Status = task.StatusPlanned
	blocked.Weight = "medium"
	blocked.BlockedBy = []string{"TASK-BLOCKER"}
	if err := backend.SaveTask(blocked); err != nil {
		t.Fatalf("failed to save blocked task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-BLOCKED/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 409 Conflict
	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Error     string `json:"error"`
		Message   string `json:"message"`
		BlockedBy []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		} `json:"blocked_by"`
		ForceAvailable bool `json:"force_available"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Error != "task_blocked" {
		t.Errorf("expected error 'task_blocked', got '%s'", response.Error)
	}
	if len(response.BlockedBy) != 1 {
		t.Errorf("expected 1 blocker, got %d", len(response.BlockedBy))
	}
	if response.BlockedBy[0].ID != "TASK-BLOCKER" {
		t.Errorf("expected blocker ID 'TASK-BLOCKER', got '%s'", response.BlockedBy[0].ID)
	}
	if response.BlockedBy[0].Status != "TASK_STATUS_PLANNED" {
		t.Errorf("expected blocker status 'planned', got '%s'", response.BlockedBy[0].Status)
	}
	if !response.ForceAvailable {
		t.Error("expected force_available to be true")
	}
}

func TestRunTaskEndpoint_BlockedByCompletedTask_CanRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and tasks
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create blocking task that is completed
	blocker := task.New("TASK-DONE", "Completed Blocking Task")
	blocker.Status = task.StatusCompleted
	blocker.Weight = "medium"
	if err := backend.SaveTask(blocker); err != nil {
		t.Fatalf("failed to save blocker task: %v", err)
	}

	// Create task that is blocked by the completed task
	ready := task.New("TASK-READY", "Ready Task")
	ready.Status = task.StatusPlanned
	ready.Weight = "medium"
	ready.WorkflowID = "implement-medium" // Required: workflow_id must be set
	ready.BlockedBy = []string{"TASK-DONE"}
	if err := backend.SaveTask(ready); err != nil {
		t.Fatalf("failed to save ready task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-READY/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 OK (all blockers are completed)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunTaskEndpoint_BlockedWithForce(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and tasks
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create blocking task (not completed)
	blocker := task.New("TASK-BLOCK2", "Blocking Task")
	blocker.Status = task.StatusRunning
	blocker.Weight = "medium"
	if err := backend.SaveTask(blocker); err != nil {
		t.Fatalf("failed to save blocker task: %v", err)
	}

	// Create task that is blocked
	forced := task.New("TASK-FORCE", "Force Run Task")
	forced.Status = task.StatusPlanned
	forced.Weight = "medium"
	forced.WorkflowID = "implement-medium" // Required: workflow_id must be set
	forced.BlockedBy = []string{"TASK-BLOCK2"}
	if err := backend.SaveTask(forced); err != nil {
		t.Fatalf("failed to save forced task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Use force=true query param
	req := httptest.NewRequest("POST", "/api/tasks/TASK-FORCE/run?force=true", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 OK when force=true
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 with force=true, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunTaskEndpoint_NoBlockers_CanRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task with no blockers
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-FREE", "Free Task")
	tsk.Status = task.StatusPlanned
	tsk.Weight = "medium"
	tsk.WorkflowID = "implement-medium" // Required: workflow_id must be set
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-FREE/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 OK
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// === Pause/Resume Tests ===

func TestPauseTaskEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and running task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-013", "Running Task")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-013/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]string
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "paused" {
		t.Errorf("expected status 'paused', got %q", resp["status"])
	}
}

func TestPauseTaskEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestResumeTaskEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend with task, plan, and state
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-014", "Paused Task")
	tsk.Status = task.StatusPaused
	tsk.Weight = "medium"
	// Set execution state with a paused implement phase
	tsk.CurrentPhase = "implement"
	tsk.Execution = task.InitExecutionState()
	tsk.Execution.Phases["implement"] = &task.PhaseState{Status: task.PhaseStatusInterrupted}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-014/resume", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "resumed" {
		t.Errorf("expected status 'resumed', got %q", resp["status"])
	}
}

func TestResumeTaskEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/resume", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === SSE Stream Tests ===

func TestStreamEndpoint_TaskNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/stream", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Additional Transcript Tests ===

func TestGetTranscriptsEndpoint_Empty(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-TRANS-001", "Transcript Test")
	tsk.Status = task.StatusCreated
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Create empty transcripts directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TRANS-001")
	_ = os.MkdirAll(filepath.Join(taskDir, "transcripts"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TRANS-001/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify we get an empty array
	var transcripts []interface{}
	_ = json.NewDecoder(w.Body).Decode(&transcripts)
	if len(transcripts) != 0 {
		t.Errorf("expected empty transcripts, got %d", len(transcripts))
	}
}

func TestGetTranscriptsEndpoint_WithTranscripts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-TRANS-002", "Transcript Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Add transcripts to database (JSONL-based schema)
	transcripts := []storage.Transcript{
		{
			TaskID:      "TASK-TRANS-002",
			Phase:       "implement",
			SessionID:   "sess-001",
			MessageUUID: "msg-001",
			Type:        "user",
			Role:        "user",
			Content:     "Implement the feature",
			Timestamp:   1700000000000,
		},
		{
			TaskID:      "TASK-TRANS-002",
			Phase:       "implement",
			SessionID:   "sess-001",
			MessageUUID: "msg-002",
			Type:        "assistant",
			Role:        "assistant",
			Content:     "Implementation done!",
			Timestamp:   1700000001000,
		},
	}
	if err := backend.AddTranscriptBatch(context.Background(), transcripts); err != nil {
		t.Fatalf("failed to add transcripts: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TRANS-002/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result struct {
		Transcripts []map[string]interface{} `json:"transcripts"`
		Pagination  struct {
			TotalCount int  `json:"total_count"`
			HasMore    bool `json:"has_more"`
		} `json:"pagination"`
	}
	_ = json.NewDecoder(w.Body).Decode(&result)
	if len(result.Transcripts) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(result.Transcripts))
	}

	// Verify transcript content from database (new JSONL-based schema)
	if len(result.Transcripts) > 0 {
		if result.Transcripts[0]["phase"] != "implement" {
			t.Errorf("expected phase 'implement', got %v", result.Transcripts[0]["phase"])
		}
		if result.Transcripts[0]["role"] != "user" {
			t.Errorf("expected role 'user', got %v", result.Transcripts[0]["role"])
		}
		if result.Transcripts[0]["type"] != "user" {
			t.Errorf("expected type 'user', got %v", result.Transcripts[0]["type"])
		}
	}
}

// === Additional Create Task Tests ===

func TestCreateTaskEndpoint_WithWeight(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"title": "Test Task", "weight": "large"}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["weight"] != "TASK_WEIGHT_LARGE" {
		t.Errorf("weight = %v, want TASK_WEIGHT_LARGE", resp["weight"])
	}
}

func TestCreateTaskEndpoint_WithDescription(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := `{"title": "Test Task", "description": "Detailed description here"}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["description"] != "Detailed description here" {
		t.Errorf("description = %v, want 'Detailed description here'", resp["description"])
	}
}

// === Prompt Default Tests ===

func TestGetPromptDefaultEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent/default", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Project-scoped Task API Tests ===

// setupProjectTestEnv creates a temporary project with task for testing
func setupProjectTestEnv(t *testing.T) (srv *Server, projectID, taskID, projectDir string) {
	t.Helper()

	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir = filepath.Join(tmpDir, "test-project")
	_ = os.MkdirAll(filepath.Join(projectDir, ".orc"), 0755)

	// Point orc to the temp directory so registry path resolves correctly
	// t.Setenv automatically restores the original value AND marks this test
	// as not parallel-safe, preventing race conditions with other tests.
	t.Setenv("HOME", tmpDir)

	// Create global .orc directory where project registry lives
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(globalOrcDir, 0755)
	projectID = "test-proj-123"

	// Create projects.yaml in the correct location ($HOME/.orc/projects.yaml)
	projectsYAML := fmt.Sprintf(`projects:
  - id: %s
    name: test-project
    path: %s
    created_at: 2025-01-01T00:00:00Z
`, projectID, projectDir)
	_ = os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	// Create backend and task in project directory
	taskID = "TASK-001"
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(projectDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Seed built-in workflows so phase lookups work
	if _, err := workflow.SeedBuiltins(backend.DB()); err != nil {
		t.Fatalf("failed to seed workflows: %v", err)
	}

	tsk := task.New(taskID, "Test Task")
	tsk.Weight = task.WeightMedium
	tsk.WorkflowID = "implement-medium" // Required: workflow_id must be set
	tsk.Status = task.StatusRunning
	tsk.Branch = "orc/" + taskID
	// Set execution state
	tsk.CurrentPhase = "implement"
	tsk.Execution = task.InitExecutionState()
	tsk.Execution.CurrentIteration = 1
	tsk.Execution.Phases["implement"] = &task.PhaseState{
		Status: task.PhaseStatusRunning,
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	_ = backend.Close()

	// Use the test project directory, not the real one
	srv = New(&Config{WorkDir: projectDir})

	return srv, projectID, taskID, projectDir
}

func TestProjectTaskRun_ReturnsTask(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	_ = os.MkdirAll(projectDir, 0755)

	t.Setenv("HOME", tmpDir)

	// Track server for cleanup
	var srv *Server
	t.Cleanup(func() {
		if srv != nil {
			srv.CancelAllRunningTasks()
		}
	})

	// Create global .orc directory where project registry lives
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(globalOrcDir, 0755)
	projectID := "test-proj-run"

	// Create projects.yaml in the correct location ($HOME/.orc/projects.yaml)
	projectsYAML := fmt.Sprintf(`projects:
  - id: %s
    name: test-project
    path: %s
    created_at: 2025-01-01T00:00:00Z
`, projectID, projectDir)
	_ = os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	// Create backend and task with planned status (can be run)
	taskID := "TASK-PROJRUN"
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(projectDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New(taskID, "Test Project Task")
	tsk.Weight = task.WeightMedium
	tsk.WorkflowID = "implement-medium" // Required: workflow_id must be set
	tsk.Status = task.StatusPlanned
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Use the test project directory, not the real one
	srv = New(&Config{WorkDir: projectDir})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/run", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Status string `json:"status"`
		TaskID string `json:"task_id"`
		Task   struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"task"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response includes task with updated status
	if response.Status != "started" {
		t.Errorf("expected status 'started', got '%s'", response.Status)
	}
	if response.Task.ID != taskID {
		t.Errorf("expected task id '%s', got '%s'", taskID, response.Task.ID)
	}
	if response.Task.Status != "TASK_STATUS_RUNNING" {
		t.Errorf("expected task status 'TASK_STATUS_RUNNING', got '%s'", response.Task.Status)
	}
}

func TestProjectTaskPause_Success(t *testing.T) {
	srv, projectID, taskID, _ := setupProjectTestEnv(t)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/pause", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "paused" {
		t.Errorf("expected status 'paused', got %q", resp["status"])
	}
}

func TestProjectTaskPause_NotRunning(t *testing.T) {
	srv, projectID, taskID, projectDir := setupProjectTestEnv(t)

	// Modify task to be completed via backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(projectDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	tsk, err := backend.LoadTask(taskID)
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	tsk.Status = task.StatusCompleted
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/pause", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskResume_Success(t *testing.T) {
	srv, projectID, taskID, projectDir := setupProjectTestEnv(t)

	// Cancel any background tasks before test cleanup to prevent file handle leaks
	t.Cleanup(func() {
		srv.CancelAllRunningTasks()
	})

	// Modify task to be paused via backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(projectDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	tsk, err := backend.LoadTask(taskID)
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	tsk.Status = task.StatusPaused
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/resume", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "resumed" {
		t.Errorf("expected status 'resumed', got %q", resp["status"])
	}
}

func TestProjectTaskResume_NotPaused(t *testing.T) {
	srv, projectID, taskID, _ := setupProjectTestEnv(t)

	// Task is running, not paused
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/resume", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskRewind_Success(t *testing.T) {
	srv, projectID, taskID, _ := setupProjectTestEnv(t)

	// Request body
	body := bytes.NewBufferString(`{"phase": "implement"}`)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/rewind", projectID, taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "rewound" {
		t.Errorf("expected status 'rewound', got %q", resp["status"])
	}
	if resp["phase"] != "implement" {
		t.Errorf("expected phase 'implement', got %q", resp["phase"])
	}
}

func TestProjectTaskRewind_InvalidPhase(t *testing.T) {
	srv, projectID, taskID, _ := setupProjectTestEnv(t)

	body := bytes.NewBufferString(`{"phase": "nonexistent"}`)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/rewind", projectID, taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskRewind_MissingPhase(t *testing.T) {
	srv, projectID, taskID, _ := setupProjectTestEnv(t)

	body := bytes.NewBufferString(`{}`)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/rewind", projectID, taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskNotFound(t *testing.T) {
	srv, projectID, _, _ := setupProjectTestEnv(t)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/projects/%s/tasks/NONEXISTENT", projectID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/projects/invalid-project/tasks/TASK-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Cost Summary API Tests ===

func TestGetCostSummaryEndpoint_Empty(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create minimal .orc structure with no tasks
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/cost/summary", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["task_count"].(float64) != 0 {
		t.Errorf("expected 0 tasks, got %v", resp["task_count"])
	}
}

func TestGetCostSummaryEndpoint_WithTasks(t *testing.T) {
	t.Parallel()
	// Skip: DatabaseBackend doesn't persist cost data (state.Cost is not saved/loaded)
	// This test requires cost tracking to be properly implemented in DatabaseBackend
	t.Skip("DatabaseBackend does not persist cost tracking data - requires implementation")

	tmpDir := t.TempDir()

	// Create backend and task with cost data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-COST-001", "Cost Test Task")
	tsk.Status = task.StatusCompleted
	tsk.Weight = task.WeightMedium
	// Set execution state with cost data
	tsk.CurrentPhase = "implement"
	tsk.Execution = task.InitExecutionState()
	tsk.Execution.CurrentIteration = 1
	tsk.Execution.Phases["implement"] = &task.PhaseState{
		Status: task.PhaseStatusCompleted,
		Tokens: task.TokenUsage{
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
		},
	}
	tsk.Execution.Cost = task.CostTracking{
		TotalCostUSD: 0.025,
		PhaseCosts:   map[string]float64{"implement": 0.025},
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/cost/summary", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["task_count"].(float64) != 1 {
		t.Errorf("expected 1 task, got %v", resp["task_count"])
	}

	total := resp["total"].(map[string]interface{})
	if total["cost_usd"].(float64) != 0.025 {
		t.Errorf("expected cost 0.025, got %v", total["cost_usd"])
	}

	byPhase := resp["by_phase"].(map[string]interface{})
	if byPhase["implement"].(float64) != 0.025 {
		t.Errorf("expected implement phase cost 0.025, got %v", byPhase["implement"])
	}
}

func TestGetCostSummaryEndpoint_PeriodFiltering(t *testing.T) {
	t.Parallel()
	// Skip: DatabaseBackend doesn't persist cost data (state.Cost is not saved/loaded)
	// This test requires cost tracking to be properly implemented in DatabaseBackend
	t.Skip("DatabaseBackend does not persist cost tracking data - requires implementation")

	tmpDir := t.TempDir()

	// Create backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create old task (more than a week old)
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	oldTask := task.New("TASK-OLD", "Old Task")
	oldTask.Status = task.StatusCompleted
	oldTask.Weight = task.WeightSmall
	oldTask.CreatedAt = oldTime
	oldTask.UpdatedAt = oldTime
	// Set execution state with token/cost data for old task
	oldTask.Execution = task.InitExecutionState()
	oldTask.Execution.Phases["implement"] = &task.PhaseState{
		Status:    task.PhaseStatusCompleted,
		StartedAt: oldTime,
		Tokens: task.TokenUsage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}
	oldTask.Execution.Cost = task.CostTracking{
		TotalCostUSD: 0.001,
	}
	if err := backend.SaveTask(oldTask); err != nil {
		t.Fatalf("failed to save old task: %v", err)
	}

	// Create recent task
	now := time.Now()
	recentTask := task.New("TASK-NEW", "New Task")
	recentTask.Status = task.StatusCompleted
	recentTask.Weight = task.WeightSmall
	recentTask.CreatedAt = now
	recentTask.UpdatedAt = now
	// Set execution state with token/cost data for recent task
	recentTask.Execution = task.InitExecutionState()
	recentTask.Execution.Phases["implement"] = &task.PhaseState{
		Status:    task.PhaseStatusCompleted,
		StartedAt: now,
		Tokens: task.TokenUsage{
			InputTokens:  200,
			OutputTokens: 100,
			TotalTokens:  300,
		},
	}
	recentTask.Execution.Cost = task.CostTracking{
		TotalCostUSD: 0.002,
	}
	if err := backend.SaveTask(recentTask); err != nil {
		t.Fatalf("failed to save recent task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Test with period=week (should only include recent task)
	req := httptest.NewRequest("GET", "/api/cost/summary?period=week", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should only have the recent task
	if resp["task_count"].(float64) != 1 {
		t.Errorf("expected 1 task for week period, got %v", resp["task_count"])
	}

	// Test with period=all (should include both tasks)
	req = httptest.NewRequest("GET", "/api/cost/summary?period=all", nil)
	w = httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["task_count"].(float64) != 2 {
		t.Errorf("expected 2 tasks for all period, got %v", resp["task_count"])
	}
}

func TestGetCostSummaryEndpoint_InvalidPeriod(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Invalid period with no 'since' should still work (falls through to no filter)
	req := httptest.NewRequest("GET", "/api/cost/summary?period=invalid", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return OK (with no filtering if period is invalid and no since provided)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetCostSummaryEndpoint_InvalidSinceParameter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Invalid 'since' parameter should return error
	req := httptest.NewRequest("GET", "/api/cost/summary?period=custom&since=not-a-date", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// === Update Task API Tests ===

func TestUpdateTaskEndpoint_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task to update
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-001", "Original Title")
	tsk.Description = "Original description"
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-001"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Update title and description
	body := bytes.NewBufferString(`{"title":"Updated Title","description":"Updated description"}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-001", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var respTask task.Task
	if err := json.NewDecoder(w.Body).Decode(&respTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respTask.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", respTask.Title)
	}

	if respTask.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", respTask.Description)
	}
}

func TestUpdateTaskEndpoint_UpdateWeight(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task to update
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-002", "Weight Test")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-002"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Update weight
	body := bytes.NewBufferString(`{"weight":"large"}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-002", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var respTask map[string]any
	if err := json.NewDecoder(w.Body).Decode(&respTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respTask["weight"] != "TASK_WEIGHT_LARGE" {
		t.Errorf("expected weight 'TASK_WEIGHT_LARGE', got %q", respTask["weight"])
	}
}

func TestUpdateTaskEndpoint_InvalidWeight(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-003", "Invalid Weight Test")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-003"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Try to set invalid weight
	body := bytes.NewBufferString(`{"weight":"invalid"}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-003", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTaskEndpoint_EmptyTitle(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-004", "Empty Title Test")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-004"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Try to set empty title
	body := bytes.NewBufferString(`{"title":""}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-004", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTaskEndpoint_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{"title":"Updated Title"}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-NONEXISTENT", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTaskEndpoint_RunningTask(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and running task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-RUN", "Running Task")
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-RUN"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Try to update running task
	body := bytes.NewBufferString(`{"title":"New Title"}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-RUN", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTaskEndpoint_InvalidJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-JSON", "JSON Test")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-JSON"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-JSON", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateTaskEndpoint_Metadata(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-META", "Metadata Test")
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightSmall
	tsk.Branch = "orc/TASK-UPD-META"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Add metadata
	body := bytes.NewBufferString(`{"metadata":{"priority":"high","owner":"user1"}}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-META", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var respTask task.Task
	if err := json.NewDecoder(w.Body).Decode(&respTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respTask.Metadata["priority"] != "high" {
		t.Errorf("expected metadata['priority']='high', got %q", respTask.Metadata["priority"])
	}
	if respTask.Metadata["owner"] != "user1" {
		t.Errorf("expected metadata['owner']='user1', got %q", respTask.Metadata["owner"])
	}
}

func TestUpdateTaskEndpoint_PartialUpdate(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend and task with all fields populated
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New("TASK-UPD-PARTIAL", "Original Title")
	tsk.Description = "Original description"
	tsk.Status = task.StatusPlanned
	tsk.Weight = task.WeightMedium
	tsk.Branch = "orc/TASK-UPD-PARTIAL"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	// Only update title, keep other fields
	body := bytes.NewBufferString(`{"title":"Updated Title Only"}`)
	req := httptest.NewRequest("PATCH", "/api/tasks/TASK-UPD-PARTIAL", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var respTask map[string]any
	if err := json.NewDecoder(w.Body).Decode(&respTask); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respTask["title"] != "Updated Title Only" {
		t.Errorf("expected title 'Updated Title Only', got %q", respTask["title"])
	}
	// Other fields should remain unchanged
	if respTask["description"] != "Original description" {
		t.Errorf("expected description 'Original description', got %q", respTask["description"])
	}
	if respTask["weight"] != "TASK_WEIGHT_MEDIUM" {
		t.Errorf("expected weight 'TASK_WEIGHT_MEDIUM', got %q", respTask["weight"])
	}
}

// === Default Project API Tests ===

func TestGetDefaultProjectEndpoint_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create .orc dir so server can initialize its backend
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/projects/default", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty default is OK
	if resp["default_project"] != "" {
		t.Errorf("expected empty default_project, got %q", resp["default_project"])
	}
}

func TestSetDefaultProjectEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create a project
	projectDir := filepath.Join(tmpDir, "test-project")
	_ = os.MkdirAll(projectDir, 0755)

	// Register the project
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(globalOrcDir, 0755)

	projectsYAML := `projects:
  - id: test-proj-123
    name: test-project
    path: ` + projectDir + `
    created_at: 2025-01-01T00:00:00Z
`
	_ = os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	// Set the default project
	body := bytes.NewBufferString(`{"project_id": "test-proj-123"}`)
	req := httptest.NewRequest("PUT", "/api/projects/default", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["default_project"] != "test-proj-123" {
		t.Errorf("expected default_project 'test-proj-123', got %q", resp["default_project"])
	}

	// Verify by getting it
	req = httptest.NewRequest("GET", "/api/projects/default", nil)
	w = httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["default_project"] != "test-proj-123" {
		t.Errorf("expected default_project 'test-proj-123' after set, got %q", resp["default_project"])
	}
}

func TestSetDefaultProjectEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create .orc dir so server can initialize its backend
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{"project_id": "nonexistent-id"}`)
	req := httptest.NewRequest("PUT", "/api/projects/default", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetDefaultProjectEndpoint_ClearDefault(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create global orc dir
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(globalOrcDir, 0755)

	srv := New(&Config{WorkDir: tmpDir})

	// Setting empty project_id clears the default
	body := bytes.NewBufferString(`{"project_id": ""}`)
	req := httptest.NewRequest("PUT", "/api/projects/default", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetDefaultProjectEndpoint_InvalidBody(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("PUT", "/api/projects/default", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestSessionEndpoint(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/session", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp SessionMetricsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify all required fields are present (non-zero values not required for empty state)
	if resp.SessionID == "" {
		t.Error("expected session_id to be non-empty")
	}
	if resp.StartedAt.IsZero() {
		t.Error("expected started_at to be non-zero")
	}
	// duration_seconds, tokens, cost can be zero for empty state
	// just verify they exist in the response (already validated by successful decode)
}

func TestSessionEndpointCORS(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/session", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header to be set on /api/session endpoint")
	}
}
