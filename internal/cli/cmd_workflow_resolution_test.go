// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-801: Fix workflow resolution for file-based custom workflows.
//
// The bug: `orc new --workflow` and `orc workflows show` only check the GlobalDB
// (seeded built-ins) and don't fall back to the file-based Resolver. Custom workflows
// created via `orc workflows clone` (YAML files in .orc/workflows/) are invisible.
//
// NOTE: Tests use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel().
//
// Success Criteria Coverage:
// - SC-1: `orc new --workflow <id>` succeeds for file-based custom workflows
// - SC-2: `orc new --workflow <id>` still succeeds for built-in (GlobalDB) workflows
// - SC-3: `orc new --workflow <id>` returns clear error for nonexistent workflows
// - SC-4: `orc workflows show <id>` displays file-based custom workflow details
// - SC-5: `orc workflows show <id>` still works for built-in workflows
// - SC-6: `orc workflows show <id>` returns clear error for nonexistent workflows
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// customWorkflowYAML is a minimal valid workflow YAML for testing file-based resolution.
const customWorkflowYAML = `id: test-custom-file
name: "Test Custom File Workflow"
description: "A custom workflow stored as a YAML file"

phases:
  - template: implement
    sequence: 0

  - template: review
    sequence: 1
    depends_on: ["implement"]

  - template: docs
    sequence: 2
    depends_on: ["review"]
`

// setupFileBasedWorkflow creates a custom workflow YAML file in .orc/workflows/
// within the given tmpDir. Returns the workflow ID.
func setupFileBasedWorkflow(t *testing.T, tmpDir string) string {
	t.Helper()
	workflowsDir := filepath.Join(tmpDir, ".orc", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("create workflows dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workflowsDir, "test-custom-file.yaml"),
		[]byte(customWorkflowYAML),
		0644,
	); err != nil {
		t.Fatalf("write custom workflow YAML: %v", err)
	}
	return "test-custom-file"
}

// --- SC-1: orc new --workflow succeeds for file-based custom workflows ---

func TestNewCmd_FileBasedWorkflow_Accepted(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Create a custom workflow YAML file that does NOT exist in GlobalDB
	workflowID := setupFileBasedWorkflow(t, tmpDir)

	// Set a config with a default workflow so we don't hit "no default" error
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test file-based workflow task", "--workflow", workflowID})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("orc new --workflow %s should succeed for file-based workflow, got error: %v\nOutput: %s",
			workflowID, err, buf.String())
	}

	// Verify task was created with the correct workflow_id
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, err := reopened.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no tasks created")
	}

	createdTask := tasks[0]
	if createdTask.WorkflowId == nil || *createdTask.WorkflowId != workflowID {
		actual := ""
		if createdTask.WorkflowId != nil {
			actual = *createdTask.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q", actual, workflowID)
	}
}

// --- SC-2: orc new --workflow still works for built-in workflows ---

func TestNewCmd_BuiltinWorkflow_StillWorks(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test builtin workflow task", "--workflow", "implement-small"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("orc new --workflow implement-small should succeed for built-in workflow, got error: %v\nOutput: %s",
			err, buf.String())
	}

	// Verify task was created with the correct workflow_id
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, err := reopened.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no tasks created")
	}

	createdTask := tasks[0]
	if createdTask.WorkflowId == nil || *createdTask.WorkflowId != "implement-small" {
		actual := ""
		if createdTask.WorkflowId != nil {
			actual = *createdTask.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q", actual, "implement-small")
	}
}

// --- SC-3: orc new --workflow returns clear error for nonexistent workflows ---

func TestNewCmd_NonexistentWorkflow_ReturnsError(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test nonexistent workflow", "--workflow", "totally-nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent workflow, got nil")
	}

	errMsg := err.Error()

	// Error should mention workflow not found
	if !strings.Contains(strings.ToLower(errMsg), "workflow not found") &&
		!strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("error should mention 'not found', got: %s", errMsg)
	}

	// Error should contain the workflow ID for debugging
	if !strings.Contains(errMsg, "totally-nonexistent-workflow") {
		t.Errorf("error should contain the workflow ID 'totally-nonexistent-workflow', got: %s", errMsg)
	}

	// Verify no task was created
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, err := reopened.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks when workflow not found, got %d", len(tasks))
	}
}

// --- SC-4: orc workflows show displays file-based workflow details ---

func TestWorkflowShow_FileBasedWorkflow_DisplaysDetails(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	_ = createTestBackendInDir(t, tmpDir)

	// Create a custom workflow YAML file
	workflowID := setupFileBasedWorkflow(t, tmpDir)

	// The show command uses fmt.Printf which goes to os.Stdout.
	// We need to redirect to capture output. Use cmd.SetOut and verify the
	// command at least succeeds (the show command needs to be updated to
	// use cmd.OutOrStdout() for full output capture, but we can at least
	// verify it doesn't error).
	cmd := rootCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"workflows", "show", workflowID})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("orc workflows show %s should succeed for file-based workflow, got error: %v\nOutput: %s",
			workflowID, err, buf.String())
	}

	// The show command currently writes to os.Stdout via fmt.Printf.
	// After the fix, it should display the workflow details from the YAML.
	// We verify the command succeeds without error — the actual output
	// verification would require the show command to use cmd.OutOrStdout().
}

// --- SC-5: orc workflows show still works for built-in workflows ---

func TestWorkflowShow_BuiltinWorkflow_StillWorks(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	_ = createTestBackendInDir(t, tmpDir)

	cmd := rootCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"workflows", "show", "implement-small"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("orc workflows show implement-small should succeed, got error: %v\nOutput: %s",
			err, buf.String())
	}
}

// --- SC-6: orc workflows show returns clear error for nonexistent workflows ---

func TestWorkflowShow_NonexistentWorkflow_ReturnsError(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	_ = createTestBackendInDir(t, tmpDir)

	cmd := rootCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"workflows", "show", "totally-nonexistent-workflow"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent workflow, got nil")
	}

	errMsg := err.Error()

	// Error should mention workflow not found
	if !strings.Contains(strings.ToLower(errMsg), "workflow not found") &&
		!strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("error should mention 'not found', got: %s", errMsg)
	}
}

// --- Failure mode: malformed YAML in custom workflow file ---

func TestNewCmd_MalformedWorkflowYAML_ReturnsError(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Create a malformed YAML file
	workflowsDir := filepath.Join(tmpDir, ".orc", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("create workflows dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workflowsDir, "malformed-wf.yaml"),
		[]byte("this is: [not: valid: yaml: {{{}}}"),
		0644,
	); err != nil {
		t.Fatalf("write malformed YAML: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test malformed workflow", "--workflow", "malformed-wf"})

	err := cmd.Execute()
	// Should get "not found" error because the Resolver skips malformed files
	if err == nil {
		t.Fatal("expected error for malformed workflow YAML, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "not found") {
		t.Errorf("error should indicate workflow not found (malformed skipped), got: %s", errMsg)
	}
}

// --- Failure mode: .orc/workflows/ directory doesn't exist ---

func TestNewCmd_NoWorkflowsDir_FallsBackToGlobalDB(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// No .orc/workflows/ directory exists - this is the default state.
	// A built-in workflow should still resolve from GlobalDB.
	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test no workflows dir", "--workflow", "implement-medium"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("built-in workflow should resolve even without .orc/workflows/ dir, got: %v\nOutput: %s",
			err, buf.String())
	}
}

// --- Edge case: workflow exists in both GlobalDB and file system ---

func TestNewCmd_WorkflowInBothSources_Succeeds(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Create a file-based workflow with the same ID as a built-in.
	// The embedded resolver has implement-small; create a file override too.
	workflowsDir := filepath.Join(tmpDir, ".orc", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("create workflows dir: %v", err)
	}
	overrideYAML := `id: implement-small
name: "Implement (Small) - Overridden"
description: "Project-level override of implement-small"
phases:
  - template: implement
    sequence: 0
`
	if err := os.WriteFile(
		filepath.Join(workflowsDir, "implement-small.yaml"),
		[]byte(overrideYAML),
		0644,
	); err != nil {
		t.Fatalf("write override YAML: %v", err)
	}

	// GlobalDB is checked first; either source should work
	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test dual-source workflow", "--workflow", "implement-small"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("workflow in both sources should succeed, got: %v\nOutput: %s",
			err, buf.String())
	}
}
