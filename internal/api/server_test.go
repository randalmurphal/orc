package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claudeconfig"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/task"
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

// === Hooks API Tests (settings.json format) ===

func TestListHooksEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .claude directory
	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

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
	srv := New(nil)

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
	tmpDir := t.TempDir()

	// Create empty settings
	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{}`), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/hooks/PreToolUse", nil)
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

func TestCreateHookEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .claude directory
	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

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
	tmpDir := t.TempDir()

	// Create empty settings
	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(`{}`), 0644)

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
	tmpDir := t.TempDir()

	// Create .claude/skills directory
	os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

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
	tmpDir := t.TempDir()

	// Create .claude/skills directory
	os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude", "skills"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteSkillEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(tmpDir, ".claude", "skills", "delete-skill")
	os.MkdirAll(skillDir, 0755)
	skillMD := `---
name: delete-skill
description: To be deleted
---
Some content
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUpdateSettingsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

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
	srv := New(nil)

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
	srv := New(nil)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tools/permissions", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUpdateToolPermissionsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCreateAgentEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/scripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDiscoverScriptsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	// Create scripts directory with a test script
	scriptsDir := filepath.Join(tmpDir, ".claude", "scripts")
	os.MkdirAll(scriptsDir, 0755)
	os.WriteFile(filepath.Join(scriptsDir, "test.sh"), []byte("#!/bin/bash\n# Test script\necho hello"), 0755)

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
	tmpDir := t.TempDir()

	// Create CLAUDE.md
	os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Project CLAUDE.md\n\nTest content"), 0644)

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
	tmpDir := t.TempDir()

	// Create project CLAUDE.md
	os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("# Project"), 0644)

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
	// Create temp dir for .orc
	tmpDir := t.TempDir()

	// Create minimal .orc structure
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tasks []task.Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// TestListTasksEndpoint_NoOrcDir tests that the API returns an empty list
// when started from a directory that is not an orc project (no .orc directory).
// This is the fix for the issue where the server breaks when started from a
// different directory than the project.
func TestListTasksEndpoint_NoOrcDir(t *testing.T) {
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

	var tasks []task.Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestListTasksEndpoint_WithTasks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task directory and file
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-001")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-001
title: Test Task
description: A test task
status: pending
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tasks []task.Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].ID != "TASK-001" {
		t.Errorf("expected task ID 'TASK-001', got %q", tasks[0].ID)
	}
}

func TestListTasksEndpoint_Pagination(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple tasks for pagination testing
	for i := 1; i <= 25; i++ {
		taskDir := filepath.Join(tmpDir, ".orc", "tasks", fmt.Sprintf("TASK-%03d", i))
		os.MkdirAll(taskDir, 0755)

		taskYAML := fmt.Sprintf(`id: TASK-%03d
title: Test Task %d
description: Test task number %d
status: pending
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`, i, i, i)
		os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)
	}

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
	tmpDir := t.TempDir()

	// Create task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-002")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-002
title: Get Test Task
description: For testing GET endpoint
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-002", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.ID != "TASK-002" {
		t.Errorf("expected task ID 'TASK-002', got %q", tsk.ID)
	}

	if tsk.Title != "Get Test Task" {
		t.Errorf("expected title 'Get Test Task', got %q", tsk.Title)
	}
}

func TestGetTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateTaskEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	srv := New(nil)

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
	tmpDir := t.TempDir()

	// Create a task to delete
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-DEL-001")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-DEL-001
title: Delete Test
status: completed
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-DEL-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify task was deleted
	if task.ExistsIn(tmpDir, "TASK-DEL-001") {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteTaskEndpoint_RunningTask(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a running task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-RUN-001")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-RUN-001
title: Running Task
status: running
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-RUN-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	// Verify task still exists
	if !task.ExistsIn(tmpDir, "TASK-RUN-001") {
		t.Error("running task should not have been deleted")
	}
}

func TestServerConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Addr != ":8080" {
		t.Errorf("expected addr ':8080', got %q", cfg.Addr)
	}
}

func TestNewServer_WithConfig(t *testing.T) {
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
	tmpDir := t.TempDir()

	// Create config directory and file
	os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	configYAML := `version: 1
model: claude-sonnet-4-20250514
max_iterations: 30
timeout: 10m
`
	os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfigEndpoint_NoConfig(t *testing.T) {
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
	tmpDir := t.TempDir()

	// Create config directory
	os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

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
	srv := New(nil)

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
	tmpDir := t.TempDir()

	// Create .orc directory and config
	os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)
	configContent := `profile: safe
model: claude-sonnet
`
	os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644)

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

func TestGetSettingsHierarchyEndpoint(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .claude directory
	os.MkdirAll(filepath.Join(tmpDir, ".claude"), 0755)

	// Create project settings
	projectSettings := `{"env": {"PROJECT_VAR": "project_value"}}`
	os.WriteFile(filepath.Join(tmpDir, ".claude", "settings.json"), []byte(projectSettings), 0644)

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
	srv := New(nil)

	// Publisher method should return the internal publisher
	pub := srv.Publisher()
	if pub == nil {
		t.Error("expected non-nil publisher")
	}
}

// === Publish Tests ===

func TestPublishWithSubscribers(t *testing.T) {
	srv := New(nil)

	// Manually add a subscriber
	ch := make(chan Event, 10)
	srv.subscribersMu.Lock()
	srv.subscribers["TASK-001"] = append(srv.subscribers["TASK-001"], ch)
	srv.subscribersMu.Unlock()

	// Publish an event
	srv.Publish("TASK-001", Event{Type: "test", Data: "hello"})

	// Check if event was received
	select {
	case evt := <-ch:
		if evt.Type != "test" {
			t.Errorf("expected event type 'test', got %q", evt.Type)
		}
	default:
		t.Error("expected to receive event")
	}
}

func TestPublishWithFullChannel(t *testing.T) {
	srv := New(nil)

	// Create a full channel (capacity 0)
	ch := make(chan Event)
	srv.subscribersMu.Lock()
	srv.subscribers["TASK-001"] = append(srv.subscribers["TASK-001"], ch)
	srv.subscribersMu.Unlock()

	// Publish should not block even with full channel
	done := make(chan struct{})
	go func() {
		srv.Publish("TASK-001", Event{Type: "test", Data: "hello"})
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// Success - did not block
	case <-time.After(100 * time.Millisecond):
		t.Error("Publish blocked on full channel")
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	srv := New(nil)

	// Publish should not panic with no subscribers
	srv.Publish("NONEXISTENT", Event{Type: "test", Data: "hello"})
}

// === Save Prompt Success Test ===

func TestSavePromptEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .orc/prompts directory
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "prompts"), 0755)

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
	tmpDir := t.TempDir()

	// Create .orc/prompts directory with a prompt
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "prompts"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".orc", "prompts", "test.md"), []byte("test content"), 0644)

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
	tmpDir := t.TempDir()

	// Create task with plan file
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-010")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-010
title: Plan Test
status: pending
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	planYAML := `version: 1
weight: medium
description: Test plan
phases:
  - id: implement
    name: Implementation
    prompt: Do the work
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create task with planned status (can be run)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-RUN")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-RUN
title: Test Task
status: planned
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create plan file (required for run)
	planYAML := `phases:
  - id: implement
    status: pending
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

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
	if response.Task.Status != "running" {
		t.Errorf("expected task status 'running', got '%s'", response.Task.Status)
	}

	// Verify task file was updated on disk
	updatedTask, err := task.LoadFrom(tmpDir, "TASK-RUN")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	if updatedTask.Status != task.StatusRunning {
		t.Errorf("expected disk task status 'running', got '%s'", updatedTask.Status)
	}
}

// TestRunTaskEndpoint_SetsCurrentPhase verifies that when a task is run,
// its current_phase is set to the first phase in the plan. This ensures
// the UI shows the task in the correct column (e.g., "Spec" or "Implement")
// instead of "Queued".
func TestRunTaskEndpoint_SetsCurrentPhase(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task with planned status (can be run)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-PHASE")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-PHASE
title: Test Task
status: planned
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create plan with spec as first phase
	planYAML := `phases:
  - id: spec
    status: pending
  - id: implement
    status: pending
  - id: test
    status: pending
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

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
			CurrentPhase string `json:"current_phase"`
		} `json:"task"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response includes task with current_phase set to first phase
	if response.Task.CurrentPhase != "spec" {
		t.Errorf("expected response task current_phase 'spec', got '%s'", response.Task.CurrentPhase)
	}

	// Verify task file has current_phase set
	updatedTask, err := task.LoadFrom(tmpDir, "TASK-PHASE")
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}
	if updatedTask.CurrentPhase != "spec" {
		t.Errorf("expected disk task current_phase 'spec', got '%s'", updatedTask.CurrentPhase)
	}
}

func TestRunTaskEndpoint_TaskCannotRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task with running status
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-011")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-011
title: Running Task
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-011/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunTaskEndpoint_NoPlan(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without plan file (status must allow running)
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-012")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-012
title: No Plan Task
status: planned
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-012/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Pause/Resume Tests ===

func TestPauseTaskEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create running task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-013")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-013
title: Running Task
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-013/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "paused" {
		t.Errorf("expected status 'paused', got %q", resp["status"])
	}
}

func TestPauseTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestResumeTaskEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create paused task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-014")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-014
title: Paused Task
status: paused
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", "/api/tasks/TASK-014/resume", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "resumed" {
		t.Errorf("expected status 'resumed', got %q", resp["status"])
	}
}

func TestResumeTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	// Create task with empty transcripts directory
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TRANS-001")
	os.MkdirAll(filepath.Join(taskDir, "transcripts"), 0755)

	taskYAML := `id: TASK-TRANS-001
title: Transcript Test
status: pending
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TRANS-001/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify we get an empty array
	var transcripts []interface{}
	json.NewDecoder(w.Body).Decode(&transcripts)
	if len(transcripts) != 0 {
		t.Errorf("expected empty transcripts, got %d", len(transcripts))
	}
}

func TestGetTranscriptsEndpoint_WithTranscripts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task with transcripts
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TRANS-002")
	transcriptsDir := filepath.Join(taskDir, "transcripts")
	os.MkdirAll(transcriptsDir, 0755)

	taskYAML := `id: TASK-TRANS-002
title: Transcript Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create a transcript file
	transcriptContent := `# Phase: implement
## Iteration 1
Implementation done!
`
	os.WriteFile(filepath.Join(transcriptsDir, "implement-001.md"), []byte(transcriptContent), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TRANS-002/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var transcripts []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&transcripts)
	if len(transcripts) == 0 {
		t.Error("expected at least one transcript")
	}
}

// === Additional Create Task Tests ===

func TestCreateTaskEndpoint_WithWeight(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["weight"] != "large" {
		t.Errorf("weight = %v, want large", resp["weight"])
	}
}

func TestCreateTaskEndpoint_WithDescription(t *testing.T) {
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["description"] != "Detailed description here" {
		t.Errorf("description = %v, want 'Detailed description here'", resp["description"])
	}
}

// === Prompt Default Tests ===

func TestGetPromptDefaultEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent/default", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Project-scoped Task API Tests ===

// setupProjectTestEnv creates a temporary project with task for testing
func setupProjectTestEnv(t *testing.T) (srv *Server, projectID, taskID, cleanup string) {
	t.Helper()

	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(filepath.Join(projectDir, ".orc", "tasks"), 0755)

	// Point orc to the temp directory first so registry path resolves correctly
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	// Create global .orc directory where project registry lives
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	os.MkdirAll(globalOrcDir, 0755)
	projectID = "test-proj-123"

	// Create projects.yaml in the correct location ($HOME/.orc/projects.yaml)
	projectsYAML := fmt.Sprintf(`projects:
  - id: %s
    name: test-project
    path: %s
    created_at: 2025-01-01T00:00:00Z
`, projectID, projectDir)
	os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	// Create task directory
	taskID = "TASK-001"
	taskDir := filepath.Join(projectDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	// Create task.yaml
	taskYAML := fmt.Sprintf(`id: %s
title: Test Task
weight: medium
status: running
branch: orc/%s
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
started_at: 2025-01-01T00:00:00Z
`, taskID, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create plan.yaml
	planYAML := `phases:
  - id: implement
    status: running
  - id: test
    status: pending
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

	// Create state.yaml
	stateYAML := fmt.Sprintf(`task_id: %s
current_phase: implement
current_iteration: 1
status: running
started_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
phases:
  implement:
    status: running
    started_at: 2025-01-01T00:00:00Z
    iterations: 0
tokens:
  input_tokens: 0
  output_tokens: 0
  total_tokens: 0
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "state.yaml"), []byte(stateYAML), 0644)

	srv = New(nil)

	cleanup = origHome
	return srv, projectID, taskID, cleanup
}

func cleanupProjectTestEnv(origHome string) {
	os.Setenv("HOME", origHome)
}

func TestProjectTaskRun_ReturnsTask(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projectDir, 0755)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create global .orc directory where project registry lives
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	os.MkdirAll(globalOrcDir, 0755)
	projectID := "test-proj-run"

	// Create projects.yaml in the correct location ($HOME/.orc/projects.yaml)
	projectsYAML := fmt.Sprintf(`projects:
  - id: %s
    name: test-project
    path: %s
    created_at: 2025-01-01T00:00:00Z
`, projectID, projectDir)
	os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	// Create task directory
	taskID := "TASK-PROJRUN"
	taskDir := filepath.Join(projectDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	// Create task.yaml with planned status (can be run)
	taskYAML := fmt.Sprintf(`id: %s
title: Test Project Task
weight: medium
status: planned
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create plan.yaml
	planYAML := `phases:
  - id: implement
    status: pending
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

	srv := New(nil)

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
	if response.Task.Status != "running" {
		t.Errorf("expected task status 'running', got '%s'", response.Task.Status)
	}
}

func TestProjectTaskPause_Success(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

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
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Modify task to be completed
	home := os.Getenv("HOME")
	taskPath := filepath.Join(home, "test-project", ".orc", "tasks", taskID, "task.yaml")
	taskYAML := fmt.Sprintf(`id: %s
title: Test Task
weight: medium
status: completed
branch: orc/%s
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID, taskID)
	os.WriteFile(taskPath, []byte(taskYAML), 0644)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/pause", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskResume_Success(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Modify task to be paused
	home := os.Getenv("HOME")
	taskPath := filepath.Join(home, "test-project", ".orc", "tasks", taskID, "task.yaml")
	taskYAML := fmt.Sprintf(`id: %s
title: Test Task
weight: medium
status: paused
branch: orc/%s
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
started_at: 2025-01-01T00:00:00Z
`, taskID, taskID)
	os.WriteFile(taskPath, []byte(taskYAML), 0644)

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
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Task is running, not paused
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/resume", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskRewind_Success(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

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
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

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
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

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
	srv, projectID, _, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/projects/%s/tasks/NONEXISTENT", projectID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/projects/invalid-project/tasks/TASK-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Cost Summary API Tests ===

func TestGetCostSummaryEndpoint_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	// Create minimal .orc structure with no tasks
	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	// Create task with cost data
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-COST-001")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-COST-001
title: Cost Test Task
status: completed
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	stateYAML := `task_id: TASK-COST-001
current_phase: implement
current_iteration: 1
status: completed
started_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
phases:
  implement:
    status: completed
    tokens:
      input_tokens: 1000
      output_tokens: 500
      total_tokens: 1500
tokens:
  input_tokens: 1000
  output_tokens: 500
  total_tokens: 1500
cost:
  total_cost_usd: 0.025
  phase_costs:
    implement: 0.025
`
	os.WriteFile(filepath.Join(taskDir, "state.yaml"), []byte(stateYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create old task (more than a week old)
	oldTaskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-OLD")
	os.MkdirAll(oldTaskDir, 0755)

	oldTaskYAML := `id: TASK-OLD
title: Old Task
status: completed
weight: small
created_at: 2020-01-01T00:00:00Z
updated_at: 2020-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(oldTaskDir, "task.yaml"), []byte(oldTaskYAML), 0644)

	oldStateYAML := `task_id: TASK-OLD
status: completed
started_at: 2020-01-01T00:00:00Z
updated_at: 2020-01-01T00:00:00Z
tokens:
  input_tokens: 100
  output_tokens: 50
  total_tokens: 150
cost:
  total_cost_usd: 0.001
`
	os.WriteFile(filepath.Join(oldTaskDir, "state.yaml"), []byte(oldStateYAML), 0644)

	// Create recent task
	recentTaskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-NEW")
	os.MkdirAll(recentTaskDir, 0755)

	now := time.Now().Format(time.RFC3339)
	recentTaskYAML := fmt.Sprintf(`id: TASK-NEW
title: New Task
status: completed
weight: small
created_at: %s
updated_at: %s
`, now, now)
	os.WriteFile(filepath.Join(recentTaskDir, "task.yaml"), []byte(recentTaskYAML), 0644)

	recentStateYAML := fmt.Sprintf(`task_id: TASK-NEW
status: completed
started_at: %s
updated_at: %s
tokens:
  input_tokens: 200
  output_tokens: 100
  total_tokens: 300
cost:
  total_cost_usd: 0.002
`, now, now)
	os.WriteFile(filepath.Join(recentTaskDir, "state.yaml"), []byte(recentStateYAML), 0644)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	// Create task to update
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-001")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-001
title: Original Title
description: Original description
status: planned
weight: small
branch: orc/TASK-UPD-001
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %q", tsk.Title)
	}

	if tsk.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got %q", tsk.Description)
	}
}

func TestUpdateTaskEndpoint_UpdateWeight(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task to update
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-002")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-002
title: Weight Test
status: planned
weight: small
branch: orc/TASK-UPD-002
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.Weight != task.WeightLarge {
		t.Errorf("expected weight 'large', got %q", tsk.Weight)
	}
}

func TestUpdateTaskEndpoint_InvalidWeight(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-003")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-003
title: Invalid Weight Test
status: planned
weight: small
branch: orc/TASK-UPD-003
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-004")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-004
title: Empty Title Test
status: planned
weight: small
branch: orc/TASK-UPD-004
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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
	tmpDir := t.TempDir()

	os.MkdirAll(filepath.Join(tmpDir, ".orc", "tasks"), 0755)

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
	tmpDir := t.TempDir()

	// Create running task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-RUN")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-RUN
title: Running Task
status: running
weight: small
branch: orc/TASK-UPD-RUN
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-JSON")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-JSON
title: JSON Test
status: planned
weight: small
branch: orc/TASK-UPD-JSON
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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
	tmpDir := t.TempDir()

	// Create task
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-META")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-META
title: Metadata Test
status: planned
weight: small
branch: orc/TASK-UPD-META
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.Metadata["priority"] != "high" {
		t.Errorf("expected metadata['priority']='high', got %q", tsk.Metadata["priority"])
	}
	if tsk.Metadata["owner"] != "user1" {
		t.Errorf("expected metadata['owner']='user1', got %q", tsk.Metadata["owner"])
	}
}

func TestUpdateTaskEndpoint_PartialUpdate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task with all fields populated
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-UPD-PARTIAL")
	os.MkdirAll(taskDir, 0755)
	taskYAML := `id: TASK-UPD-PARTIAL
title: Original Title
description: Original description
status: planned
weight: medium
branch: orc/TASK-UPD-PARTIAL
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

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

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.Title != "Updated Title Only" {
		t.Errorf("expected title 'Updated Title Only', got %q", tsk.Title)
	}
	// Other fields should remain unchanged
	if tsk.Description != "Original description" {
		t.Errorf("expected description 'Original description', got %q", tsk.Description)
	}
	if tsk.Weight != task.WeightMedium {
		t.Errorf("expected weight 'medium', got %q", tsk.Weight)
	}
}
// === Default Project API Tests ===

func TestGetDefaultProjectEndpoint_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	srv := New(nil)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create a project
	projectDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(projectDir, 0755)

	// Register the project
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	os.MkdirAll(globalOrcDir, 0755)

	projectsYAML := `projects:
  - id: test-proj-123
    name: test-project
    path: ` + projectDir + `
    created_at: 2025-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	srv := New(nil)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	srv := New(nil)

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
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create global orc dir
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	os.MkdirAll(globalOrcDir, 0755)

	srv := New(nil)

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
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("PUT", "/api/projects/default", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
