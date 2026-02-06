// Package cli tests for the scratchpad CLI command.
//
// TDD Tests for TASK-020: CLI scratchpad command
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
//
// Success Criteria Coverage:
//   - SC-7: `orc scratchpad TASK-001` outputs all entries in human-readable format
//   - SC-8: `orc scratchpad TASK-001 --phase implement` filters to that phase
//
// Failure Mode Coverage:
//   - Non-existent task ID prints "no scratchpad entries found"
//   - Non-existent phase prints "no scratchpad entries found for phase: <phase>"
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
)

// withScratchpadTestDir creates a temp directory with .orc/config.yaml,
// changes to it, and restores the original working directory on cleanup.
func withScratchpadTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
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

// createScratchpadTestBackend creates a backend in the given directory.
func createScratchpadTestBackend(t *testing.T, dir string) *storage.DatabaseBackend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

// ============================================================================
// SC-7: orc scratchpad TASK-001 outputs all entries grouped by phase
// ============================================================================

// TestScratchpadCommand_ShowsAllEntries verifies the command outputs entries
// for a task grouped by phase with category labels.
func TestScratchpadCommand_ShowsAllEntries(t *testing.T) {
	tmpDir := withScratchpadTestDir(t)
	backend := createScratchpadTestBackend(t, tmpDir)

	// Save scratchpad entries for different phases
	entries := []storage.ScratchpadEntry{
		{TaskID: "TASK-001", PhaseID: "spec", Category: "decision", Content: "Chose REST over GraphQL", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Existing API uses JSON:API format", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "blocker", Content: "Need to add OpenAPI dependency", Attempt: 1},
	}

	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry[%d]: %v", i, err)
		}
	}

	// Close backend before running command (command creates its own)
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newScratchpadCmd()
	cmd.SetArgs([]string{"TASK-001"})
	err := cmd.Execute()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command execution error: %v", err)
	}

	output := buf.String()

	// Should contain phase headers
	if !strings.Contains(output, "spec") {
		t.Errorf("output should contain 'spec' phase header\ngot: %s", output)
	}
	if !strings.Contains(output, "implement") {
		t.Errorf("output should contain 'implement' phase header\ngot: %s", output)
	}

	// Should contain entry content
	if !strings.Contains(output, "Chose REST over GraphQL") {
		t.Errorf("output should contain decision content\ngot: %s", output)
	}
	if !strings.Contains(output, "Existing API uses JSON:API format") {
		t.Errorf("output should contain observation content\ngot: %s", output)
	}
	if !strings.Contains(output, "Need to add OpenAPI dependency") {
		t.Errorf("output should contain blocker content\ngot: %s", output)
	}

	// Should contain category labels
	if !strings.Contains(output, "decision") {
		t.Errorf("output should contain 'decision' category label\ngot: %s", output)
	}
}

// TestScratchpadCommand_NonExistentTask prints "no scratchpad entries found".
func TestScratchpadCommand_NonExistentTask(t *testing.T) {
	tmpDir := withScratchpadTestDir(t)
	backend := createScratchpadTestBackend(t, tmpDir)
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newScratchpadCmd()
	cmd.SetArgs([]string{"NONEXISTENT"})
	err := cmd.Execute()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command should not error for non-existent task, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "no scratchpad entries found") {
		t.Errorf("output should contain 'no scratchpad entries found'\ngot: %s", output)
	}
}

// ============================================================================
// SC-8: orc scratchpad TASK-001 --phase implement filters by phase
// ============================================================================

// TestScratchpadCommand_FilterByPhase verifies --phase flag filters output.
func TestScratchpadCommand_FilterByPhase(t *testing.T) {
	tmpDir := withScratchpadTestDir(t)
	backend := createScratchpadTestBackend(t, tmpDir)

	// Save entries for two phases
	entries := []storage.ScratchpadEntry{
		{TaskID: "TASK-001", PhaseID: "spec", Category: "decision", Content: "Spec-only entry", Attempt: 1},
		{TaskID: "TASK-001", PhaseID: "implement", Category: "observation", Content: "Implement-only entry", Attempt: 1},
	}

	for i := range entries {
		if err := backend.SaveScratchpadEntry(&entries[i]); err != nil {
			t.Fatalf("save entry[%d]: %v", i, err)
		}
	}
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newScratchpadCmd()
	cmd.SetArgs([]string{"TASK-001", "--phase", "implement"})
	err := cmd.Execute()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command execution error: %v", err)
	}

	output := buf.String()

	// Should contain implement entries
	if !strings.Contains(output, "Implement-only entry") {
		t.Errorf("output should contain implement entry\ngot: %s", output)
	}

	// Should NOT contain spec entries
	if strings.Contains(output, "Spec-only entry") {
		t.Errorf("output should NOT contain spec entry when filtered to implement\ngot: %s", output)
	}
}

// TestScratchpadCommand_FilterByNonExistentPhase prints phase-specific message.
func TestScratchpadCommand_FilterByNonExistentPhase(t *testing.T) {
	tmpDir := withScratchpadTestDir(t)
	backend := createScratchpadTestBackend(t, tmpDir)

	// Save an entry for spec phase
	entry := &storage.ScratchpadEntry{TaskID: "TASK-001", PhaseID: "spec", Category: "observation", Content: "Entry", Attempt: 1}
	if err := backend.SaveScratchpadEntry(entry); err != nil {
		t.Fatalf("save entry: %v", err)
	}
	_ = backend.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newScratchpadCmd()
	cmd.SetArgs([]string{"TASK-001", "--phase", "nonexistent"})
	err := cmd.Execute()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("command should not error, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "no scratchpad entries found for phase") {
		t.Errorf("output should contain 'no scratchpad entries found for phase'\ngot: %s", output)
	}
}

// TestScratchpadCommand_Flags verifies command structure and flag registration.
func TestScratchpadCommand_Flags(t *testing.T) {
	cmd := newScratchpadCmd()

	if cmd.Use != "scratchpad <task-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "scratchpad <task-id>")
	}

	if cmd.Flag("phase") == nil {
		t.Error("missing --phase flag")
	}
}
