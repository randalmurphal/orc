// Package cli implements the orc command-line interface.
//
// TDD Integration Tests for TASK-748: Kill weight from task model.
//
// Success Criteria Coverage:
// - SC-6: Remove --weight / -w flag from orc new CLI command (integration)
// - SC-9: Remove weight-based logic from CLI (integration)
// - SC-10: Remove weight from CLI output (integration)
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Integration: Workflow-first task creation works without weight
// ============================================================================

// TestIntegration_CreateTaskWithWorkflowOnly verifies workflow-first model:
// Task creation using --workflow flag should work without any weight field.
func TestIntegration_CreateTaskWithWorkflowOnly(t *testing.T) {
	tmpDir := withWeightIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Config with no default workflow - must specify explicitly
	configContent := "version: 1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Create task using workflow directly (the new workflow-first way)
	cmd.SetArgs([]string{"Test workflow task", "--workflow", "implement-medium"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("task creation failed: %v\nOutput: %s", err, buf.String())
	}

	output := buf.String()

	// Verify task was created
	if !strings.Contains(output, "Task created:") {
		t.Error("output should contain 'Task created:'")
	}

	// Verify workflow is shown in output
	if !strings.Contains(output, "Workflow:") && !strings.Contains(output, "implement-medium") {
		t.Error("output should mention the workflow")
	}

	// SC-10: Output should NOT contain "Weight:" after implementation
	if strings.Contains(output, "Weight:") {
		t.Error("SC-10 FAILED: Output should not contain 'Weight:' - weight should be removed")
	}
}

// TestIntegration_CreateTaskWithConfigDefault verifies config-based workflow:
// Task creation should use config default workflow when not specified.
func TestIntegration_CreateTaskWithConfigDefault(t *testing.T) {
	tmpDir := withWeightIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Config with default workflow
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Create task without specifying workflow - should use config default
	cmd.SetArgs([]string{"Test default workflow task"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("task creation failed: %v\nOutput: %s", err, buf.String())
	}

	// Reload and verify workflow was set from config
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, err := reopened.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no tasks created")
	}

	task := tasks[0]
	if task.WorkflowId == nil || *task.WorkflowId != "implement-small" {
		wfID := ""
		if task.WorkflowId != nil {
			wfID = *task.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q from config default", wfID, "implement-small")
	}
}

// TestIntegration_ErrorWhenNoWorkflowAndNoDefault verifies error handling:
// Task creation should fail with clear error when no workflow and no default.
func TestIntegration_ErrorWhenNoWorkflowAndNoDefault(t *testing.T) {
	tmpDir := withWeightIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Config without default workflow
	configContent := "version: 1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// No --workflow specified, no config default
	cmd.SetArgs([]string{"Test task without workflow"})

	err := cmd.Execute()

	// Should fail with clear error
	if err == nil {
		t.Fatal("expected error when no workflow specified and no default configured")
	}

	errMsg := strings.ToLower(err.Error())
	// Error should mention workflow (not weight)
	if !strings.Contains(errMsg, "workflow") {
		t.Errorf("error should mention 'workflow', got: %s", err.Error())
	}
}

// TestIntegration_WorkflowFlagOverridesDefault verifies precedence:
// Explicit --workflow flag should override config default.
func TestIntegration_WorkflowFlagOverridesDefault(t *testing.T) {
	tmpDir := withWeightIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Config with default workflow
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Specify different workflow than config default
	cmd.SetArgs([]string{"Test override task", "--workflow", "implement-large"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("task creation failed: %v\nOutput: %s", err, buf.String())
	}

	// Reload and verify explicit workflow was used
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, err := reopened.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no tasks created")
	}

	task := tasks[0]
	// Explicit --workflow should override config default
	if task.WorkflowId == nil || *task.WorkflowId != "implement-large" {
		wfID := ""
		if task.WorkflowId != nil {
			wfID = *task.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q (explicit flag should override config)", wfID, "implement-large")
	}
}

// TestIntegration_TaskSavedWithWorkflowNotWeight verifies persistence:
// Created tasks should have workflow_id set, not weight.
func TestIntegration_TaskSavedWithWorkflowNotWeight(t *testing.T) {
	tmpDir := withWeightIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	configContent := "workflow: implement-medium\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test persistence task"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("task creation failed: %v", err)
	}

	// Reload and verify task fields
	reopened := createTestBackendInDir(t, tmpDir)
	defer func() { _ = reopened.Close() }()

	tasks, err := reopened.LoadAllTasks()
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("no tasks created")
	}

	task := tasks[0]

	// Workflow should be set
	if task.WorkflowId == nil {
		t.Error("workflow_id should be set")
	}

	// After implementation, Task should not have Weight field at all
	// This is checked via reflection in the weight_removal_test.go
}

// ============================================================================
// Helpers
// ============================================================================

// withWeightIntegrationTestDir creates a temp directory for integration tests.
func withWeightIntegrationTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Initialize git repo (required for branch validation)
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
