package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestHandleGetConfigStats_BasicResponse(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetConfigStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ConfigStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify response structure
	if response.SlashCommandsCount < 0 {
		t.Errorf("expected slashCommandsCount>=0, got %d", response.SlashCommandsCount)
	}
	if response.ClaudeMdSize < 0 {
		t.Errorf("expected claudeMdSize>=0, got %d", response.ClaudeMdSize)
	}
	if response.McpServersCount < 0 {
		t.Errorf("expected mcpServersCount>=0, got %d", response.McpServersCount)
	}
	if response.PermissionsProfile == "" {
		t.Error("expected permissionsProfile to be set")
	}
}

func TestHandleGetConfigStats_WithClaudeMd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create a CLAUDE.md file in the project
	claudeMdContent := "# Project Instructions\n\nSome documentation here."
	claudeMdPath := filepath.Join(tmpDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMdPath, []byte(claudeMdContent), 0644); err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetConfigStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ConfigStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify CLAUDE.md size is detected
	expectedSize := len(claudeMdContent)
	if response.ClaudeMdSize != expectedSize {
		t.Errorf("expected claudeMdSize=%d, got %d", expectedSize, response.ClaudeMdSize)
	}
}

func TestHandleGetConfigStats_DefaultProfile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/config/stats", nil)
	rr := httptest.NewRecorder()

	server.handleGetConfigStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var response ConfigStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Without custom config, should use default profile
	if response.PermissionsProfile != string(config.ProfileAuto) {
		t.Errorf("expected permissionsProfile=%q, got %q", config.ProfileAuto, response.PermissionsProfile)
	}
}
