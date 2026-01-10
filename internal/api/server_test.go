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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .claude directory
	os.MkdirAll(".claude", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create empty settings
	os.MkdirAll(".claude", 0755)
	os.WriteFile(".claude/settings.json", []byte(`{}`), 0644)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .claude directory
	os.MkdirAll(".claude", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create empty settings
	os.MkdirAll(".claude", 0755)
	os.WriteFile(".claude/settings.json", []byte(`{}`), 0644)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .claude/skills directory
	os.MkdirAll(".claude/skills", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude/skills", 0755)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .claude/skills directory
	os.MkdirAll(".claude/skills", 0755)

	srv := New(nil)

	body := `{"name": "test-skill", "description": "A test skill", "content": "Do something useful"}`
	req := httptest.NewRequest("POST", "/api/skills", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the SKILL.md was created
	skillPath := filepath.Join(".claude", "skills", "test-skill", "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Error("expected SKILL.md file to be created")
	}
}

func TestDeleteSkillEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude/skills", 0755)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteSkillEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create skill directory with SKILL.md
	skillDir := filepath.Join(".claude", "skills", "delete-skill")
	os.MkdirAll(skillDir, 0755)
	skillMD := `---
name: delete-skill
description: To be deleted
---
Some content
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/settings", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUpdateSettingsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tools/permissions", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestUpdateToolPermissionsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/agents", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestCreateAgentEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".claude", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/scripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestDiscoverScriptsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create scripts directory with a test script
	scriptsDir := filepath.Join(".claude", "scripts")
	os.MkdirAll(scriptsDir, 0755)
	os.WriteFile(filepath.Join(scriptsDir, "test.sh"), []byte("#!/bin/bash\n# Test script\necho hello"), 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create CLAUDE.md
	os.WriteFile("CLAUDE.md", []byte("# Project CLAUDE.md\n\nTest content"), 0644)

	srv := New(nil)

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

func TestGetClaudeMDEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/claudemd", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateClaudeMDEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	srv := New(nil)

	body := `{"content": "# Updated CLAUDE.md\n\nNew content"}`
	req := httptest.NewRequest("PUT", "/api/claudemd", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was written
	content, err := os.ReadFile("CLAUDE.md")
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	if string(content) != "# Updated CLAUDE.md\n\nNew content" {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestGetClaudeMDHierarchyEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create project CLAUDE.md
	os.WriteFile("CLAUDE.md", []byte("# Project"), 0644)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create minimal .orc structure
	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

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

func TestListTasksEndpoint_WithTasks(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task directory and file
	taskDir := filepath.Join(".orc", "tasks", "TASK-001")
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

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create multiple tasks for pagination testing
	for i := 1; i <= 25; i++ {
		taskDir := filepath.Join(".orc", "tasks", fmt.Sprintf("TASK-%03d", i))
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

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task
	taskDir := filepath.Join(".orc", "tasks", "TASK-002")
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

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateTaskEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a task to delete
	os.MkdirAll(".orc/tasks/TASK-DEL-001", 0755)
	testTask := task.New("TASK-DEL-001", "Delete Test")
	testTask.Status = task.StatusCompleted
	testTask.Save()

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-DEL-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify task was deleted
	if task.Exists("TASK-DEL-001") {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteTaskEndpoint_RunningTask(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a running task
	os.MkdirAll(".orc/tasks/TASK-RUN-001", 0755)
	testTask := task.New("TASK-RUN-001", "Running Task")
	testTask.Status = task.StatusRunning
	testTask.Save()

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-RUN-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	// Verify task still exists
	if !task.Exists("TASK-RUN-001") {
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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create config directory and file
	os.MkdirAll(".orc", 0755)
	configYAML := `version: 1
model: claude-sonnet-4-20250514
max_iterations: 30
timeout: 10m
`
	os.WriteFile(".orc/config.yaml", []byte(configYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfigEndpoint_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// No config file exists
	srv := New(nil)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create config directory
	os.MkdirAll(".orc", 0755)

	srv := New(nil)

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
	ch := make(chan Event, 0)
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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .orc/prompts directory
	os.MkdirAll(".orc/prompts", 0755)

	srv := New(nil)

	body := bytes.NewBufferString(`{"content":"Custom prompt content for testing"}`)
	req := httptest.NewRequest("PUT", "/api/prompts/implement", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify prompt was saved
	content, err := os.ReadFile(".orc/prompts/implement.md")
	if err != nil {
		t.Errorf("failed to read saved prompt: %v", err)
	}
	if string(content) != "Custom prompt content for testing" {
		t.Errorf("prompt content mismatch: got %q", string(content))
	}
}

func TestDeletePromptEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .orc/prompts directory with a prompt
	os.MkdirAll(".orc/prompts", 0755)
	os.WriteFile(".orc/prompts/test.md", []byte("test content"), 0644)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/prompts/test", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify file was deleted
	if _, err := os.Stat(".orc/prompts/test.md"); !os.IsNotExist(err) {
		t.Error("expected prompt file to be deleted")
	}
}
