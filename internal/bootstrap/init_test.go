package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_CreatesStructure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake go.mod so detection works
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)

	result, err := Run(Options{WorkDir: tmpDir})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check directories created
	dirs := []string{
		".orc",
		".orc/tasks",
		".orc/prompts",
	}
	for _, dir := range dirs {
		path := filepath.Join(tmpDir, dir)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("directory %s not created", dir)
		}
	}

	// Check config created
	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Error("config.yaml not created")
	}

	// Check database created
	if _, err := os.Stat(result.DatabasePath); err != nil {
		t.Error("orc.db not created")
	}

	// Check detection worked
	if result.Detection == nil {
		t.Error("detection is nil")
	}
	if result.Detection.Language != "go" {
		t.Errorf("detection.Language = %q, want go", result.Detection.Language)
	}

	// Check project ID assigned
	if result.ProjectID == "" {
		t.Error("project ID is empty")
	}
}

func TestRun_Performance(t *testing.T) {
	tmpDir := t.TempDir()

	start := time.Now()
	_, err := Run(Options{WorkDir: tmpDir})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Target: < 500ms
	if duration > 500*time.Millisecond {
		t.Errorf("init took %v, want < 500ms", duration)
	}
}

func TestRun_AlreadyInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	// First init
	_, err := Run(Options{WorkDir: tmpDir})
	if err != nil {
		t.Fatalf("first Run failed: %v", err)
	}

	// Second init without force should fail
	_, err = Run(Options{WorkDir: tmpDir})
	if err == nil {
		t.Error("expected error for already initialized")
	}
	if !strings.Contains(err.Error(), "already initialized") {
		t.Errorf("unexpected error: %v", err)
	}

	// With force should succeed
	_, err = Run(Options{WorkDir: tmpDir, Force: true})
	if err != nil {
		t.Fatalf("Run with Force failed: %v", err)
	}
}

func TestRun_WithProfile(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := Run(Options{WorkDir: tmpDir, Profile: "strict"})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Read config and verify profile
	data, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if !strings.Contains(string(data), "profile: strict") {
		t.Error("profile not set to strict in config")
	}
}

func TestUpdateGitignore(t *testing.T) {
	tmpDir := t.TempDir()

	// Test creating new .gitignore
	err := updateGitignore(tmpDir)
	if err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	// Check all entries present
	for _, entry := range orcGitignoreEntries {
		if !strings.Contains(string(content), entry) {
			t.Errorf(".gitignore missing entry: %s", entry)
		}
	}
}

func TestUpdateGitignore_Existing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create existing .gitignore
	existing := "node_modules/\n.env\n"
	os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(existing), 0644)

	err := updateGitignore(tmpDir)
	if err != nil {
		t.Fatalf("updateGitignore failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	// Check existing content preserved
	if !strings.Contains(string(content), "node_modules/") {
		t.Error("existing entry node_modules/ not preserved")
	}
	if !strings.Contains(string(content), ".env") {
		t.Error("existing entry .env not preserved")
	}

	// Check new entries added
	if !strings.Contains(string(content), ".orc/worktrees/") {
		t.Error(".orc/worktrees/ not added")
	}
}

func TestUpdateGitignore_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	// Run twice
	updateGitignore(tmpDir)
	updateGitignore(tmpDir)

	content, err := os.ReadFile(filepath.Join(tmpDir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}

	// Count occurrences of orc comment - should be exactly 1
	count := strings.Count(string(content), "# orc - Claude Code Task Orchestrator")
	if count != 1 {
		t.Errorf("orc comment appears %d times, want 1", count)
	}
}
