// Package cli implements the orc command-line interface.
//
// Tests for `orc gates list` and `orc gates show` commands.
// These tests use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel().
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// =============================================================================
// Helpers
// =============================================================================

// withGatesTestDir creates a temp dir with .orc subdirectory, chdir to it, and returns cleanup.
func withGatesTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
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

// createGatesTestBackend creates a database backend at the given directory.
func createGatesTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

// seedWorkflow creates a workflow with phases in the project DB for testing.
func seedWorkflow(t *testing.T, dir string) {
	t.Helper()
	pdb, err := db.OpenProject(dir)
	if err != nil {
		t.Fatalf("open project DB: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create phase templates
	templates := []*db.PhaseTemplate{
		{
			ID:       "spec",
			Name:     "Specification",
			GateType: "auto",
		},
		{
			ID:             "implement",
			Name:           "Implementation",
			GateType:       "human",
			RetryFromPhase: "spec",
		},
		{
			ID:       "review",
			Name:     "Review",
			GateType: "auto",
		},
	}
	for _, tmpl := range templates {
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save phase template %s: %v", tmpl.ID, err)
		}
	}

	// Create workflow
	wf := &db.Workflow{
		ID:           "wf-default",
		Name:         "Default",
		WorkflowType: "task",
	}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Add phases to workflow
	phases := []*db.WorkflowPhase{
		{WorkflowID: "wf-default", PhaseTemplateID: "spec", Sequence: 1},
		{WorkflowID: "wf-default", PhaseTemplateID: "implement", Sequence: 2},
		{WorkflowID: "wf-default", PhaseTemplateID: "review", Sequence: 3},
	}
	for _, ph := range phases {
		if err := pdb.SaveWorkflowPhase(ph); err != nil {
			t.Fatalf("save workflow phase %s: %v", ph.PhaseTemplateID, err)
		}
	}
}

// captureStdout runs a function and captures stdout output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	return buf.String()
}

// =============================================================================
// SC-1: `orc gates list` displays configured gates for each phase
// =============================================================================

func TestGatesList_DisplaysPhaseGateTable(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	seedWorkflow(t, dir)
	_ = backend.Close()

	// Write a minimal config that references the workflow
	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workflow: wf-default\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"list"})

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute gates list: %v", err)
		}
	})

	// Should show all 3 phases
	for _, phase := range []string{"spec", "implement", "review"} {
		if !bytes.Contains([]byte(output), []byte(phase)) {
			t.Errorf("output should contain phase %q, got:\n%s", phase, output)
		}
	}

	// Should show gate types
	if !bytes.Contains([]byte(output), []byte("auto")) {
		t.Errorf("output should contain gate type 'auto', got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("human")) {
		t.Errorf("output should contain gate type 'human', got:\n%s", output)
	}
}

// SC-1 error path: no workflow → helpful error
func TestGatesList_NoWorkflow_Error(t *testing.T) {
	dir := withGatesTestDir(t)
	// Create backend but do NOT seed a workflow
	backend := createGatesTestBackend(t, dir)
	_ = backend.Close()

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"list"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no workflow exists")
	}

	errMsg := err.Error()
	if !bytes.Contains([]byte(errMsg), []byte("workflow")) && !bytes.Contains([]byte(errMsg), []byte("init")) {
		t.Errorf("error should mention workflow/init, got: %s", errMsg)
	}
}

// =============================================================================
// SC-2: `orc gates show PHASE` displays detailed gate config
// =============================================================================

func TestGatesShow_DetailedGateConfig(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	seedWorkflow(t, dir)
	_ = backend.Close()

	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workflow: wf-default\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"show", "implement"})

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute gates show: %v", err)
		}
	})

	// Should show the phase name
	if !bytes.Contains([]byte(output), []byte("implement")) {
		t.Errorf("output should contain phase 'implement', got:\n%s", output)
	}

	// Should show gate type
	if !bytes.Contains([]byte(output), []byte("human")) {
		t.Errorf("output should show gate type 'human', got:\n%s", output)
	}

	// Should show retry-from phase
	if !bytes.Contains([]byte(output), []byte("spec")) {
		t.Errorf("output should show retry-from phase 'spec', got:\n%s", output)
	}
}

// SC-2 error path: phase not found
func TestGatesShow_PhaseNotFound_Error(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	seedWorkflow(t, dir)
	_ = backend.Close()

	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workflow: wf-default\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"show", "nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent phase")
	}

	errMsg := err.Error()
	if !bytes.Contains([]byte(errMsg), []byte("nonexistent")) {
		t.Errorf("error should mention the missing phase name, got: %s", errMsg)
	}
}

// =============================================================================
// SC-3: `orc gates list --json` and `orc gates show PHASE --json`
// =============================================================================

func TestGatesList_JSONOutput(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	seedWorkflow(t, dir)
	_ = backend.Close()

	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workflow: wf-default\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Set the global jsonOut flag
	oldJSON := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJSON }()

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"list"})

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute gates list --json: %v", err)
		}
	})

	// Parse as JSON
	var result []map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, output)
	}

	// Should have 3 entries (one per phase)
	if len(result) != 3 {
		t.Errorf("expected 3 gate configs, got %d", len(result))
	}

	// Each entry should have phase, gate_type, source
	for _, entry := range result {
		if _, ok := entry["phase"]; !ok {
			t.Errorf("JSON entry missing 'phase' field: %v", entry)
		}
		if _, ok := entry["gate_type"]; !ok {
			t.Errorf("JSON entry missing 'gate_type' field: %v", entry)
		}
		if _, ok := entry["source"]; !ok {
			t.Errorf("JSON entry missing 'source' field: %v", entry)
		}
	}
}

func TestGatesShow_JSONOutput(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	seedWorkflow(t, dir)
	_ = backend.Close()

	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workflow: wf-default\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldJSON := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJSON }()

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"show", "implement"})

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute gates show --json: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, output)
	}

	// Should have detailed fields
	if result["phase"] != "implement" {
		t.Errorf("phase = %v, want 'implement'", result["phase"])
	}
	if result["gate_type"] != "human" {
		t.Errorf("gate_type = %v, want 'human'", result["gate_type"])
	}
}

// =============================================================================
// Edge case: Workflow with all phases set to GateSkip
// =============================================================================

func TestGatesList_AllSkip(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	_ = backend.Close()

	pdb, err := db.OpenProject(dir)
	if err != nil {
		t.Fatalf("open project DB: %v", err)
	}

	// Create templates with skip gate type
	for _, id := range []string{"phase1", "phase2"} {
		tmpl := &db.PhaseTemplate{ID: id, Name: id, GateType: "skip"}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template: %v", err)
		}
	}
	wf := &db.Workflow{ID: "wf-skip", Name: "Skip Workflow", WorkflowType: "task"}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	for i, id := range []string{"phase1", "phase2"} {
		ph := &db.WorkflowPhase{WorkflowID: "wf-skip", PhaseTemplateID: id, Sequence: i + 1}
		if err := pdb.SaveWorkflowPhase(ph); err != nil {
			t.Fatalf("save phase: %v", err)
		}
	}
	_ = pdb.Close()

	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("workflow: wf-skip\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"list"})

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	// Both phases should show "skip"
	if !bytes.Contains([]byte(output), []byte("skip")) {
		t.Errorf("output should show 'skip' gate type, got:\n%s", output)
	}
}

// =============================================================================
// SC-1: Command structure
// =============================================================================

func TestGatesCmd_Structure(t *testing.T) {
	cmd := newGatesCmd()

	if cmd.Use != "gates" {
		t.Errorf("command Use = %q, want 'gates'", cmd.Use)
	}

	// Should have list and show subcommands
	subcommands := cmd.Commands()
	subNames := make(map[string]bool)
	for _, sub := range subcommands {
		subNames[sub.Name()] = true
	}

	if !subNames["list"] {
		t.Error("missing 'list' subcommand")
	}
	if !subNames["show"] {
		t.Error("missing 'show' subcommand")
	}
}

// =============================================================================
// Edge case: `orc gates show` with task-specific override
// =============================================================================

func TestGatesShow_TaskOverride(t *testing.T) {
	dir := withGatesTestDir(t)
	backend := createGatesTestBackend(t, dir)
	seedWorkflow(t, dir)
	_ = backend.Close()

	cfgPath := filepath.Join(dir, ".orc", "config.yaml")
	cfgContent := `workflow: wf-default
gates:
  phase_overrides:
    implement: ai
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Use JSON output to verify source field
	oldJSON := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJSON }()

	cmd := newGatesCmd()
	cmd.SetArgs([]string{"show", "implement"})

	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, output)
	}

	// Phase override from config should take effect
	if result["gate_type"] != "ai" {
		t.Errorf("gate_type = %v, want 'ai' (from phase_overrides)", result["gate_type"])
	}
	if source, ok := result["source"].(string); ok {
		if source != "phase_override" {
			t.Errorf("source = %q, want 'phase_override'", source)
		}
	}
}

// Suppress unused import warnings - these will be used by the implementation
var _ = config.Config{}
