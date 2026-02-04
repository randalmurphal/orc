// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-748: Kill weight from task model - CLI component.
//
// Success Criteria Coverage:
// - SC-6: Remove --weight / -w flag from orc new CLI command
// - SC-9: Remove weight-based logic from CLI (weight mapping, weight classification)
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

// --- SC-6: --weight flag should NOT exist ---

// TestNewCmd_WeightFlagRemoved verifies SC-6:
// The --weight / -w flag should be completely removed from the 'orc new' command.
// Using the flag should produce an "unknown flag" error.
func TestNewCmd_WeightFlagRemoved(t *testing.T) {
	tmpDir := withWeightTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Create config with default workflow so task creation would work without --weight
	configContent := "workflow: implement-medium\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Try using --weight flag - should fail with "unknown flag" error
	cmd.SetArgs([]string{"Test task", "--weight", "medium"})

	err := cmd.Execute()

	// SC-6: After implementation, --weight flag should not exist
	// Test expects an error about unknown flag
	if err == nil {
		t.Error("SC-6 FAILED: --weight flag should be removed; expected 'unknown flag' error, got nil")
		return
	}

	errMsg := strings.ToLower(err.Error())
	// The error should mention "unknown flag" or similar
	if !strings.Contains(errMsg, "unknown flag") && !strings.Contains(errMsg, "flag provided but not defined") {
		t.Errorf("SC-6 FAILED: expected 'unknown flag' error for --weight, got: %s", err.Error())
	}
}

// TestNewCmd_WeightShortFlagRemoved verifies SC-6:
// The -w shorthand flag should also be removed.
func TestNewCmd_WeightShortFlagRemoved(t *testing.T) {
	tmpDir := withWeightTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Create config with default workflow
	configContent := "workflow: implement-medium\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Try using -w shorthand flag - should fail with "unknown flag" error
	cmd.SetArgs([]string{"Test task", "-w", "small"})

	err := cmd.Execute()

	// SC-6: After implementation, -w flag should not exist
	if err == nil {
		t.Error("SC-6 FAILED: -w shorthand flag should be removed; expected 'unknown flag' error, got nil")
		return
	}

	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "unknown flag") && !strings.Contains(errMsg, "flag provided but not defined") && !strings.Contains(errMsg, "unknown shorthand") {
		t.Errorf("SC-6 FAILED: expected 'unknown flag' error for -w, got: %s", err.Error())
	}
}

// --- SC-9: Weight-based workflow resolution should NOT exist ---

// TestNewCmd_NoWeightInOutput verifies SC-9 (partial):
// Task creation output should NOT mention weight after implementation.
func TestNewCmd_NoWeightInOutput(t *testing.T) {
	tmpDir := withWeightTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	_ = backend.Close()

	// Create config with default workflow
	configContent := "workflow: implement-medium\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".orc", "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	// Create task without --weight (using workflow directly)
	cmd.SetArgs([]string{"Test task", "--workflow", "implement-small"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("task creation failed: %v", err)
	}

	output := buf.String()

	// SC-9: Output should NOT contain "Weight:" line after implementation
	if strings.Contains(output, "Weight:") {
		t.Error("SC-9 FAILED: Task creation output should not display 'Weight:' - weight field should be removed")
	}
}

// TestNewCmd_HelpNoWeightFlag verifies SC-6:
// The 'orc new --help' output should NOT mention --weight flag.
func TestNewCmd_HelpNoWeightFlag(t *testing.T) {
	cmd := newNewCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	// Execute with --help (this returns an error for help, which is expected)
	_ = cmd.Execute()

	helpOutput := buf.String()

	// SC-6: Help should NOT mention --weight or -w flag
	if strings.Contains(helpOutput, "--weight") || strings.Contains(helpOutput, "-w,") {
		t.Error("SC-6 FAILED: 'orc new --help' should not mention --weight flag")
	}

	// SC-6: Help should NOT mention weight-related text in description
	helpLower := strings.ToLower(helpOutput)
	if strings.Contains(helpLower, "weight selection") || strings.Contains(helpLower, "task weight") {
		t.Error("SC-6 FAILED: 'orc new --help' should not contain weight-related documentation")
	}
}

// --- Helpers ---

// withWeightTestDir creates a temp directory with .orc structure for CLI testing.
func withWeightTestDir(t *testing.T) string {
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
