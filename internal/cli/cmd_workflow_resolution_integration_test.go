// Package cli implements the orc command-line interface.
//
// Integration Tests for TASK-801: Fix workflow resolution for file-based custom workflows.
//
// These tests verify that cmd_new.go and cmd_workflows.go show are wired to
// the file-based Resolver, not just the GlobalDB. They complement the unit
// tests in cmd_workflow_resolution_test.go by testing cross-command flows.
//
// What these tests catch that unit tests don't:
//   - Clone → New pipeline: clone-produced YAML is findable by cmd_new's Resolver
//   - Clone → Show pipeline: clone-produced YAML is findable by cmd_workflows show
//   - List ↔ New ↔ Show consistency: all commands share the same resolution behavior
//
// The unit tests create YAML manually (synthetic data). These tests use the
// actual Cloner (same as `orc workflows clone` CLI) to produce the YAML,
// verifying format compatibility between the Writer and Resolver as used
// across different CLI commands.
//
// NOTE: Tests use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel().
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/workflow"
)

// ============================================================================
// Integration: Clone → New pipeline
// ============================================================================

// TestIntegration_CloneThenNew_FileBasedWorkflow verifies the end-to-end user
// workflow: clone a built-in workflow to a file, then create a task with it.
//
// This exercises the PRODUCTION clone path (workflow.Cloner.CloneWorkflow) to
// produce the YAML file, then the PRODUCTION cmd_new path to consume it.
// Fails if cmd_new doesn't fall back to the file-based Resolver.
func TestIntegration_CloneThenNew_FileBasedWorkflow(t *testing.T) {
	tmpDir := withIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	orcDir := filepath.Join(tmpDir, ".orc")

	// Use the production Cloner (same as `orc workflows clone` CLI command)
	// to create a file-based workflow from a built-in source.
	cloner := workflow.NewClonerFromOrcDir(orcDir)
	result, err := cloner.CloneWorkflow(
		"implement-small",        // source: built-in workflow
		"my-cloned-for-new-test", // dest: custom ID that doesn't exist in GlobalDB
		workflow.WriteLevelProject,
		false, // no overwrite
	)
	if err != nil {
		t.Fatalf("clone workflow: %v", err)
	}

	// Verify clone produced a file
	if _, err := os.Stat(result.DestPath); err != nil {
		t.Fatalf("cloned workflow file should exist at %s: %v", result.DestPath, err)
	}

	// Now run `orc new --workflow my-cloned-for-new-test` through the
	// PRODUCTION command handler. This fails without the Resolver fallback
	// because "my-cloned-for-new-test" only exists as a file, not in GlobalDB.
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"Test cloned workflow task", "--workflow", "my-cloned-for-new-test"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("orc new --workflow my-cloned-for-new-test should accept clone-produced workflow, got: %v\nOutput: %s",
			err, buf.String())
	}

	// Verify task was created with the cloned workflow ID
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
	if createdTask.WorkflowId == nil || *createdTask.WorkflowId != "my-cloned-for-new-test" {
		actual := ""
		if createdTask.WorkflowId != nil {
			actual = *createdTask.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q", actual, "my-cloned-for-new-test")
	}
}

// ============================================================================
// Integration: Clone → Show pipeline
// ============================================================================

// TestIntegration_CloneThenShow_FileBasedWorkflow verifies: clone a built-in
// workflow to a file, then display it with `orc workflows show`.
//
// Uses the PRODUCTION Cloner to produce the file, then the PRODUCTION show
// command handler to display it. Fails if show doesn't fall back to the
// file-based Resolver.
func TestIntegration_CloneThenShow_FileBasedWorkflow(t *testing.T) {
	tmpDir := withIntegrationTestDir(t)
	_ = createTestBackendInDir(t, tmpDir)

	orcDir := filepath.Join(tmpDir, ".orc")

	// Clone a built-in workflow to a file using the production Cloner
	cloner := workflow.NewClonerFromOrcDir(orcDir)
	_, err := cloner.CloneWorkflow(
		"implement-medium",         // source
		"my-cloned-for-show-test",  // dest
		workflow.WriteLevelProject, // write to .orc/workflows/
		false,
	)
	if err != nil {
		t.Fatalf("clone workflow: %v", err)
	}

	// Run `orc workflows show my-cloned-for-show-test` through the
	// PRODUCTION command handler. Fails without Resolver fallback.
	cmd := rootCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"workflows", "show", "my-cloned-for-show-test"})

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("orc workflows show my-cloned-for-show-test should display clone-produced workflow, got: %v\nOutput: %s",
			err, buf.String())
	}
}

// ============================================================================
// Integration: List ↔ New ↔ Show consistency
// ============================================================================

// TestIntegration_ListNewShowConsistency_FileBasedWorkflow verifies that all
// three commands (list, new, show) see the same file-based workflows.
//
// The `orc workflows` (list) command already uses the Resolver correctly.
// This test verifies that `orc new` and `orc workflows show` use the SAME
// resolution behavior, so any workflow visible in the list is also usable.
//
// Catches: cmd_new or show creating the Resolver with wrong orcDir, or not
// creating it at all (the current bug).
func TestIntegration_ListNewShowConsistency_FileBasedWorkflow(t *testing.T) {
	tmpDir := withIntegrationTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	orcDir := filepath.Join(tmpDir, ".orc")

	// Create a file-based workflow — same setup as the list command would find
	setupFileBasedWorkflow(t, tmpDir)

	// Step 1: Verify the Resolver (used by `orc workflows` list) finds it.
	// This is the same code path as cmd_workflows.go:101.
	resolver := workflow.NewResolverFromOrcDir(orcDir)
	allWorkflows, err := resolver.ListWorkflows()
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}

	found := false
	for _, rw := range allWorkflows {
		if rw.Workflow.ID == "test-custom-file" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Resolver.ListWorkflows() should find test-custom-file, but didn't")
	}

	// Step 2: Verify `orc new --workflow test-custom-file` accepts it.
	// If this fails but Step 1 passes, cmd_new doesn't use the Resolver.
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	newCmd := newNewCmd()
	var newBuf bytes.Buffer
	newCmd.SetOut(&newBuf)
	newCmd.SetErr(&newBuf)
	newCmd.SetArgs([]string{"Test consistency task", "--workflow", "test-custom-file"})

	err = newCmd.Execute()
	if err != nil {
		t.Fatalf("orc new --workflow test-custom-file: workflow is listed by Resolver but rejected by new command: %v\nOutput: %s",
			err, newBuf.String())
	}

	// Step 3: Verify `orc workflows show test-custom-file` displays it.
	// If this fails but Step 1 passes, show doesn't use the Resolver.
	showCmd := rootCmd
	var showBuf bytes.Buffer
	showCmd.SetOut(&showBuf)
	showCmd.SetErr(&showBuf)
	showCmd.SetArgs([]string{"workflows", "show", "test-custom-file"})

	err = showCmd.Execute()
	if err != nil {
		t.Fatalf("orc workflows show test-custom-file: workflow is listed by Resolver but not showable: %v\nOutput: %s",
			err, showBuf.String())
	}
}

// ============================================================================
// Helpers
// ============================================================================

// withIntegrationTestDir creates a temp directory for integration tests.
// Identical to withNewCmdTestDir but named to avoid collisions.
func withIntegrationTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Initialize git repo (required for branch validation in new command)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("create .git directory: %v", err)
	}

	// Change to temp directory and restore on cleanup
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
