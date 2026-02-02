// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-733: CLI workflow-first task creation.
//
// Success Criteria Coverage:
// - SC-1: `-w small` flag creates task with workflow_id="implement-small"
// - SC-2: Error with clear message when no default workflow and no --workflow specified
// - SC-3: `orc workflows` shows DEFAULT indicator for configured default
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- SC-2: Error when no default workflow configured ---

// TestNewCmd_ErrorNoDefaultWorkflow verifies SC-2:
// When no --workflow and no --weight flags are provided, and no default
// workflow is configured, should return an error with a clear message.
func TestNewCmd_ErrorNoDefaultWorkflow(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	_ = backend.Close()

	// Create config WITHOUT a default workflow (empty workflow field)
	configContent := "version: 1\n# no workflow field\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// No --workflow, no --weight, no -w
	cmd.SetArgs([]string{"Test without workflow"})

	err := cmd.Execute()

	// Should fail with a clear error
	if err == nil {
		t.Fatal("expected error when no default workflow configured, got nil")
	}

	errMsg := err.Error()
	// Error should mention:
	// 1. That no default workflow is configured
	// 2. How to fix it (--workflow flag or config)
	if !strings.Contains(strings.ToLower(errMsg), "default workflow") && 
	   !strings.Contains(strings.ToLower(errMsg), "no workflow") {
		t.Errorf("error should mention 'default workflow' or 'no workflow', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "--workflow") {
		t.Errorf("error should mention --workflow flag, got: %s", errMsg)
	}
}

// TestNewCmd_DefaultWorkflowFromConfig verifies that when config has default workflow,
// task creation succeeds without explicit --workflow flag
func TestNewCmd_DefaultWorkflowFromConfig(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	_ = backend.Close()

	// Create config WITH a default workflow
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// No explicit flags - should use config default
	cmd.SetArgs([]string{"Test default from config"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected success with default workflow from config, got: %v\nOutput: %s", err, buf.String())
	}

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
	// Should use the default workflow from config, NOT the weight-derived workflow
	if task.WorkflowId == nil || *task.WorkflowId != "implement-small" {
		wfID := ""
		if task.WorkflowId != nil {
			wfID = *task.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q from config default", wfID, "implement-small")
	}
}

// --- SC-3: orc workflows shows DEFAULT indicator ---

// TestWorkflowsCmd_OutputFormat verifies the workflows list command output
// has a DEFAULT column showing ★ for the configured default workflow.
func TestWorkflowsCmd_OutputFormat(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	_ = createTestBackendInDir(t, tmpDir)

	// Create config WITH a default workflow
	configContent := "workflow: implement-small\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Use the root command with workflows subcommand
	cmd := rootCmd
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"workflows"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("workflows command failed: %v\nOutput: %s", err, buf.String())
	}

	output := buf.String()

	// Verify DEFAULT column header exists
	if !strings.Contains(output, "DEFAULT") {
		t.Errorf("output should contain 'DEFAULT' column header, got:\n%s", output)
	}

	// Verify ★ indicator appears (should be on implement-small line)
	if !strings.Contains(output, "★") {
		t.Errorf("output should contain ★ indicator for default workflow, got:\n%s", output)
	}

	// Verify the line with implement-small has the ★
	lines := strings.Split(output, "\n")
	foundSmallWithStar := false
	for _, line := range lines {
		if strings.Contains(line, "implement-small") && strings.Contains(line, "★") {
			foundSmallWithStar = true
			break
		}
	}
	if !foundSmallWithStar {
		t.Errorf("implement-small should have ★ indicator as default workflow, got:\n%s", output)
	}
}

// --- Additional edge case tests ---

// TestNewCmd_WorkflowFlagPrecedence verifies --workflow flag takes precedence
func TestNewCmd_WorkflowFlagPrecedence(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	_ = backend.Close()

	// Config has implement-medium as default
	configContent := "workflow: implement-medium\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Explicit --workflow should override config default
	cmd.SetArgs([]string{"Test workflow flag", "--workflow", "implement-large"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("new command failed: %v\nOutput: %s", err, buf.String())
	}

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
	// --workflow should take precedence over config default
	if task.WorkflowId == nil || *task.WorkflowId != "implement-large" {
		wfID := ""
		if task.WorkflowId != nil {
			wfID = *task.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q (--workflow should override config default)", wfID, "implement-large")
	}
}

// TestNewCmd_WeightFlagStillWorks verifies backward compatibility with --weight
func TestNewCmd_WeightFlagStillWorks(t *testing.T) {
	tmpDir := withNewCmdTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	_ = backend.Close()

	// Config has no default workflow
	configContent := "version: 1\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Using --weight (full flag, not shorthand) for backward compatibility
	cmd.SetArgs([]string{"Test weight flag", "--weight", "small"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("new command failed: %v\nOutput: %s", err, buf.String())
	}

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
	// --weight should map to workflow via weight-to-workflow mapping
	if task.WorkflowId == nil || *task.WorkflowId != "implement-small" {
		wfID := ""
		if task.WorkflowId != nil {
			wfID = *task.WorkflowId
		}
		t.Errorf("workflow_id = %q, want %q (--weight should still map to workflow)", wfID, "implement-small")
	}
}
