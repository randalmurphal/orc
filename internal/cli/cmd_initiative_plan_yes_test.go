// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-656: orc initiative plan --yes flag to skip interactive prompt.
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
//
// Success Criteria Coverage:
// - SC-1: `--yes` flag exists with `-y` shorthand and default value false
// - SC-2: `--yes` flag skips the interactive confirmation prompt
// - SC-3: `-y` short form works identically to `--yes`
package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
)

// --- SC-1: --yes flag exists with proper configuration ---

func TestInitiativePlanCommand_YesFlagExists(t *testing.T) {
	cmd := newInitiativePlanCmd()

	yesFlag := cmd.Flag("yes")
	if yesFlag == nil {
		t.Fatal("missing --yes flag")
	}

	// Verify -y shorthand
	if yesFlag.Shorthand != "y" {
		t.Errorf("yes flag shorthand = %q, want 'y'", yesFlag.Shorthand)
	}

	// Verify default is false (prompts by default)
	if yesFlag.DefValue != "false" {
		t.Errorf("yes flag default = %q, want 'false'", yesFlag.DefValue)
	}

	// Verify usage description mentions skipping prompt
	if yesFlag.Usage == "" {
		t.Error("yes flag should have a usage description")
	}
}

// --- SC-2: --yes skips the interactive confirmation prompt ---

func TestInitiativePlan_YesSkipsPrompt(t *testing.T) {
	tmpDir := withInitiativePlanYesTestDir(t)

	// Create an initiative
	backend := createTestBackendInDir(t, tmpDir)
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}
	_ = backend.Close()

	// Create manifest file
	manifestContent := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Task created with --yes"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Run plan command with --yes - should complete without stdin input
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})

	// Execute - if this blocks waiting for stdin, the test will timeout
	if err := cmd.Execute(); err != nil {
		t.Fatalf("initiative plan --yes failed: %v", err)
	}

	// Verify task was created
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}

	if len(allTasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(allTasks))
	}

	if len(allTasks) > 0 && allTasks[0].Title != "Task created with --yes" {
		t.Errorf("task title = %q, want %q", allTasks[0].Title, "Task created with --yes")
	}
}

// --- SC-3: -y short form works identically to --yes ---

func TestInitiativePlan_YShortFlag(t *testing.T) {
	tmpDir := withInitiativePlanYesTestDir(t)

	// Create an initiative
	backend := createTestBackendInDir(t, tmpDir)
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}
	_ = backend.Close()

	// Create manifest file
	manifestContent := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Task created with -y"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Run plan command with -y (short form) - should complete without stdin input
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "-y"})

	// Execute - if this blocks waiting for stdin, the test will timeout
	if err := cmd.Execute(); err != nil {
		t.Fatalf("initiative plan -y failed: %v", err)
	}

	// Verify task was created
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}

	if len(allTasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(allTasks))
	}

	if len(allTasks) > 0 && allTasks[0].Title != "Task created with -y" {
		t.Errorf("task title = %q, want %q", allTasks[0].Title, "Task created with -y")
	}
}

// --- Edge case: --yes combined with --dry-run ---

func TestInitiativePlan_YesWithDryRun(t *testing.T) {
	tmpDir := withInitiativePlanYesTestDir(t)

	// Create an initiative
	backend := createTestBackendInDir(t, tmpDir)
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}
	_ = backend.Close()

	// Create manifest file
	manifestContent := `version: 1
initiative: INIT-001
tasks:
  - id: 1
    title: "Task that should not be created"
    weight: small
`
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Run plan command with --yes --dry-run - should complete without creating tasks
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("initiative plan --yes --dry-run failed: %v", err)
	}

	// Verify NO tasks were created (dry-run takes precedence)
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}

	if len(allTasks) != 0 {
		t.Errorf("expected 0 tasks with --dry-run, got %d", len(allTasks))
	}
}

// --- Helpers ---

func withInitiativePlanYesTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Create .git directory for git detection
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("create .git directory: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}
