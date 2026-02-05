// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-749: Initiative plan command doesn't create decisions from manifest.
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
//
// Success Criteria Coverage:
// - SC-2: `orc initiative plan` creates decisions on the initiative when manifest contains decisions
package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// --- SC-2: initiative plan creates decisions from manifest ---

func TestInitiativePlan_CreatesDecisions(t *testing.T) {
	tmpDir := withInitiativePlanDecisionsTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)

	// Create manifest file with decisions
	manifestContent := `version: 1
create_initiative:
  title: "Test Initiative"
  vision: "Test creating decisions from manifest"
decisions:
  - text: "Kill Agents from nav"
    rationale: "Implementation detail"
  - text: "Use WebSocket for real-time"
    rationale: "Bidirectional communication needed"
tasks:
  - id: 1
    title: "First task"
`
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command (command opens its own)
	_ = backend.Close()

	// Run initiative plan command
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("initiative plan failed: %v", err)
	}

	// Reopen backend to check results
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Load the created initiative
	init, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}

	// SC-2: Decisions should be created on the initiative
	if len(init.Decisions) != 2 {
		t.Fatalf("Decisions count = %d, want 2", len(init.Decisions))
	}

	// Verify first decision
	if init.Decisions[0].Decision != "Kill Agents from nav" {
		t.Errorf("Decisions[0].Decision = %q, want %q", init.Decisions[0].Decision, "Kill Agents from nav")
	}
	if init.Decisions[0].Rationale != "Implementation detail" {
		t.Errorf("Decisions[0].Rationale = %q, want %q", init.Decisions[0].Rationale, "Implementation detail")
	}

	// Verify second decision
	if init.Decisions[1].Decision != "Use WebSocket for real-time" {
		t.Errorf("Decisions[1].Decision = %q, want %q", init.Decisions[1].Decision, "Use WebSocket for real-time")
	}
	if init.Decisions[1].Rationale != "Bidirectional communication needed" {
		t.Errorf("Decisions[1].Rationale = %q, want %q", init.Decisions[1].Rationale, "Bidirectional communication needed")
	}

	// Verify decision IDs are assigned
	if init.Decisions[0].ID != "DEC-001" {
		t.Errorf("Decisions[0].ID = %q, want %q", init.Decisions[0].ID, "DEC-001")
	}
	if init.Decisions[1].ID != "DEC-002" {
		t.Errorf("Decisions[1].ID = %q, want %q", init.Decisions[1].ID, "DEC-002")
	}
}

func TestInitiativePlan_NoDecisions_NoError(t *testing.T) {
	tmpDir := withInitiativePlanDecisionsTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)

	// Create manifest without decisions
	manifestContent := `version: 1
create_initiative:
  title: "Test Initiative"
  vision: "No decisions"
tasks:
  - id: 1
    title: "First task"
`
	manifestPath := filepath.Join(tmpDir, "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte(manifestContent), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run initiative plan command
	cmd := newInitiativePlanCmd()
	cmd.SetArgs([]string{manifestPath, "--yes"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("initiative plan failed: %v", err)
	}

	// Reopen backend to check results
	backend = createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Load the created initiative
	init, err := backend.LoadInitiative("INIT-001")
	if err != nil {
		t.Fatalf("load initiative: %v", err)
	}

	// Should have no decisions
	if len(init.Decisions) != 0 {
		t.Errorf("Decisions count = %d, want 0", len(init.Decisions))
	}
}

// --- Helpers ---

func withInitiativePlanDecisionsTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

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
