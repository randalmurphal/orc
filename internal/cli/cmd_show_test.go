package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// Tests use task.NewProtoTask() for proto-based task creation

// withShowTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withShowTestDir(t *testing.T) string {
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

// createShowTestBackend creates a backend in the given directory.
func createShowTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestShowCommand_Flags(t *testing.T) {
	cmd := newShowCmd()

	// Verify command structure
	if cmd.Use != "show <task-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "show <task-id>")
	}

	// Verify flags exist
	if cmd.Flag("session") == nil {
		t.Error("missing --session flag")
	}
	if cmd.Flag("cost") == nil {
		t.Error("missing --cost flag")
	}
	if cmd.Flag("spec") == nil {
		t.Error("missing --spec flag")
	}
	if cmd.Flag("full") == nil {
		t.Error("missing --full flag")
	}
	if cmd.Flag("period") == nil {
		t.Error("missing --period flag")
	}

	// Verify period shorthand
	if cmd.Flag("period").Shorthand != "p" {
		t.Errorf("period shorthand = %q, want 'p'", cmd.Flag("period").Shorthand)
	}
}

func TestShowCommand_SpecFlag_Execution(t *testing.T) {
	tmpDir := withShowTestDir(t)

	// Create backend and save a task without a spec
	backend := createShowTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-001", "Test task without spec")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run show command with --spec - should not error
	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-001", "--spec"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}
}

func TestShowCommand_SpecFlag_WithSpec_Execution(t *testing.T) {
	tmpDir := withShowTestDir(t)

	// Create backend, task, and spec
	backend := createShowTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-002", "Test task with spec")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	specContent := `# Test Specification

## Intent
This is a test specification for the show command.

## Success Criteria
- The spec should be displayed when --spec flag is used
- Source and length should be shown`

	if err := backend.SaveSpecForTask("TASK-002", specContent, "generated"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run show command with --spec - should not error
	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-002", "--spec"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}
}

func TestShowCommand_SpecFlag_JSON(t *testing.T) {
	tmpDir := withShowTestDir(t)

	// Create backend, task, and spec
	backend := createShowTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-003", "Test task for JSON output")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	specContent := "## Intent\nTest spec content for JSON"
	if err := backend.SaveSpecForTask("TASK-003", specContent, "db"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout for JSON output verification
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run show command with --spec --json
	jsonOut = true
	defer func() { jsonOut = false }()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-003", "--spec"})
	if err := cmd.Execute(); err != nil {
		_ = w.Close()
		os.Stdout = oldStdout
		t.Fatalf("execute command: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nOutput was: %s", err, output)
	}

	// Verify spec in JSON
	spec, ok := result["spec"].(map[string]any)
	if !ok {
		t.Fatalf("JSON output should contain 'spec' object, got: %v", result["spec"])
	}

	if spec["source"] != "db" {
		t.Errorf("spec source = %v, want 'db'", spec["source"])
	}
	if spec["content"] != specContent {
		t.Errorf("spec content = %v, want %q", spec["content"], specContent)
	}
}

func TestShowCommand_SpecFlag_JSON_NoSpec(t *testing.T) {
	tmpDir := withShowTestDir(t)

	// Create backend and task without spec
	backend := createShowTestBackend(t, tmpDir)

	tk := task.NewProtoTask("TASK-004", "Test task without spec for JSON")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Capture stdout for JSON output verification
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run show command with --spec --json
	jsonOut = true
	defer func() { jsonOut = false }()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-004", "--spec"})
	if err := cmd.Execute(); err != nil {
		_ = w.Close()
		os.Stdout = oldStdout
		t.Fatalf("execute command: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Parse JSON output
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("parse JSON output: %v\nOutput was: %s", err, output)
	}

	// Verify spec is null in JSON
	if result["spec"] != nil {
		t.Errorf("spec should be null, got %v", result["spec"])
	}
}

func TestLoadFullSpec(t *testing.T) {
	tmpDir := t.TempDir()

	// Create backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	// Test GetFullSpecForTask for non-existent spec
	spec, err := backend.GetFullSpecForTask("NONEXISTENT")
	if err != nil {
		t.Fatalf("GetFullSpecForTask error: %v", err)
	}
	if spec != nil {
		t.Error("GetFullSpecForTask should return nil for non-existent spec")
	}

	// Create a task first (specs have foreign key to tasks)
	tk := task.NewProtoTask("TASK-001", "Test task for spec")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Save a spec
	content := "# Test Spec\n\nSome content here"
	if err := backend.SaveSpecForTask("TASK-001", content, "generated"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Test GetFullSpecForTask for existing spec
	spec, err = backend.GetFullSpecForTask("TASK-001")
	if err != nil {
		t.Fatalf("GetFullSpecForTask error: %v", err)
	}
	if spec == nil {
		t.Fatal("GetFullSpecForTask should return non-nil for existing spec")
	}

	// Verify fields
	if spec.TaskID == nil || *spec.TaskID != "TASK-001" {
		t.Errorf("TaskID = %v, want 'TASK-001'", spec.TaskID)
	}
	if spec.Content != content {
		t.Errorf("Content = %q, want %q", spec.Content, content)
	}
	if spec.Source != "generated" {
		t.Errorf("Source = %q, want 'generated'", spec.Source)
	}
	if spec.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestPrintSpecInfo_NilSpec(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSpecInfo(nil)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	if !strings.Contains(output, "No specification found") {
		t.Error("output should contain 'No specification found'")
	}
	if !strings.Contains(output, "Specification") {
		t.Error("output should contain 'Specification' header")
	}
}

func TestPrintSpecInfo_WithSpec(t *testing.T) {
	taskID := "TASK-001"
	spec := &storage.PhaseOutputInfo{
		TaskID:  &taskID,
		Content: "Line 1\nLine 2\nLine 3",
		Source:  "generated",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printSpecInfo(spec)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	os.Stdout = oldStdout

	output := buf.String()

	// Should show metadata
	if !strings.Contains(output, "Source:") {
		t.Error("output should contain 'Source:'")
	}
	if !strings.Contains(output, "generated") {
		t.Error("output should contain source value")
	}
	if !strings.Contains(output, "Length:") {
		t.Error("output should contain 'Length:'")
	}
	if !strings.Contains(output, "Lines:") {
		t.Error("output should contain 'Lines:'")
	}

	// Should show content
	if !strings.Contains(output, "Line 1") {
		t.Error("output should contain spec content")
	}
}

func TestShowWithPager_NotFound(t *testing.T) {
	// Test that showWithPager returns false when pager not available
	// We can't easily test success case without a real pager, but we can test
	// that it doesn't panic on empty/simple content

	// This test mostly verifies the function exists and handles edge cases
	// In a real environment with less/more, it would open a pager

	// showWithPager returns false if pager not found or fails
	// Most CI environments won't have an interactive pager available
	// so this just verifies the function handles that gracefully
	result := showWithPager("")
	// Result could be true or false depending on environment
	// Just verify no panic
	_ = result
}
