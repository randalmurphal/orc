package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// Tests for SC-1: orc status --json outputs valid JSON with structured data
// =============================================================================

func TestStatusCommand_JSONOutput_ValidJSON(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)

	// Create a task
	t1 := task.NewProtoTask("TASK-001", "Test task")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Set jsonOut global for the test
	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	// Execute command
	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	// Parse output as JSON
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out.String())
	}

	// Should have tasks field
	if _, ok := result["tasks"]; !ok {
		t.Error("JSON output missing 'tasks' field")
	}

	// Should have summary field
	if _, ok := result["summary"]; !ok {
		t.Error("JSON output missing 'summary' field")
	}
}

func TestStatusCommand_JSONOutput_TaskFields(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)

	// Create initiative
	init := initiative.New("INIT-001", "Test Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task with various fields
	t1 := task.NewProtoTask("TASK-001", "Test task")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	t1.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	task.SetInitiativeProto(t1, "INIT-001")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify tasks array contains task data with expected fields
	tasks, ok := result["tasks"].([]interface{})
	if !ok || len(tasks) == 0 {
		t.Fatal("tasks should be a non-empty array")
	}

	task0, ok := tasks[0].(map[string]interface{})
	if !ok {
		t.Fatal("task should be an object")
	}

	// Verify essential fields are present
	if _, ok := task0["id"]; !ok {
		t.Error("task missing 'id' field")
	}
	if _, ok := task0["title"]; !ok {
		t.Error("task missing 'title' field")
	}
	if _, ok := task0["status"]; !ok {
		t.Error("task missing 'status' field")
	}
}

func TestStatusCommand_JSONOutput_Categorization(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)

	// Create tasks in different states
	t1 := task.NewProtoTask("TASK-001", "Running task")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	t2 := task.NewProtoTask("TASK-002", "Ready task")
	t2.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(t2); err != nil {
		t.Fatalf("save task: %v", err)
	}

	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Verify categorization is present
	summary, ok := result["summary"].(map[string]interface{})
	if !ok {
		t.Fatal("summary should be an object")
	}

	// Should have count fields
	if _, ok := summary["running"]; !ok {
		t.Error("summary missing 'running' count")
	}
	if _, ok := summary["ready"]; !ok {
		t.Error("summary missing 'ready' count")
	}
}

// =============================================================================
// Tests for SC-2: orc workflows --json outputs valid JSON with workflow list
// Note: Uses helper that extracts just the list logic from the global command.
// The implementation should ensure this works properly.
// =============================================================================

func TestWorkflowsListCommand_JSONOutput_ValidJSON(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	_ = tmpDir

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	// Create a fresh command instance
	cmd := newWorkflowsListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out.String())
	}

	// Should have workflows field
	if _, ok := result["workflows"]; !ok {
		t.Error("JSON output missing 'workflows' field")
	}
}

func TestWorkflowsListCommand_JSONOutput_WorkflowFields(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	_ = tmpDir

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newWorkflowsListCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	workflows, ok := result["workflows"].([]interface{})
	if !ok {
		t.Fatal("workflows should be an array")
	}

	// If there are workflows, verify they have expected fields
	if len(workflows) > 0 {
		wf0, ok := workflows[0].(map[string]interface{})
		if !ok {
			t.Fatal("workflow should be an object")
		}

		// Verify essential fields
		if _, ok := wf0["id"]; !ok {
			t.Error("workflow missing 'id' field")
		}
		if _, ok := wf0["name"]; !ok {
			t.Error("workflow missing 'name' field")
		}
		if _, ok := wf0["phase_count"]; !ok {
			t.Error("workflow missing 'phase_count' field")
		}
	}
}

// =============================================================================
// Tests for SC-3: orc new --json outputs {"task_id": "TASK-XXX"}
// =============================================================================

func TestNewCommand_JSONOutput_ReturnsTaskID(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)
	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newNewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--workflow", "implement-trivial", "Test task"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, out.String())
	}

	// Should have task_id field
	taskID, ok := result["task_id"].(string)
	if !ok {
		t.Error("JSON output missing 'task_id' field")
	}

	// Task ID should follow format
	if taskID == "" {
		t.Error("task_id should not be empty")
	}
}

func TestNewCommand_JSONOutput_IncludesTaskDetails(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)
	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newNewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--workflow", "implement-trivial", "Test task with details"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	// Should include basic task info in response
	if _, ok := result["task_id"]; !ok {
		t.Error("JSON output missing 'task_id' field")
	}
	if _, ok := result["title"]; !ok {
		t.Error("JSON output missing 'title' field")
	}
	if _, ok := result["workflow_id"]; !ok {
		t.Error("JSON output missing 'workflow_id' field")
	}
}

// =============================================================================
// Tests for SC-4: Errors output as JSON when --json specified
// =============================================================================

func TestStatusCommand_JSONOutput_ErrorFormat(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)
	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// Try to filter by non-existent initiative
	cmd.SetArgs([]string{"--initiative", "NONEXISTENT"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent initiative")
	}

	// When jsonOut is true, error should be output as JSON to stdout
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("error output should be valid JSON: %v\nOutput: %s", err, out.String())
	}

	if _, ok := result["error"]; !ok {
		t.Error("JSON error output missing 'error' field")
	}
}

func TestNewCommand_JSONOutput_ErrorFormat(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)
	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJsonOut }()

	cmd := newNewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// Try to create task with non-existent workflow
	cmd.SetArgs([]string{"--workflow", "nonexistent-workflow", "Test"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent workflow")
	}

	// Error should be formatted as JSON
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("error output should be valid JSON: %v\nOutput: %s", err, out.String())
	}

	if _, ok := result["error"]; !ok {
		t.Error("JSON error output missing 'error' field")
	}
}

// =============================================================================
// Tests for SC-5: Regular output unchanged when --json not specified
// =============================================================================

func TestStatusCommand_RegularOutput_Unchanged(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)

	t1 := task.NewProtoTask("TASK-001", "Test task")
	t1.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	// Ensure jsonOut is false
	oldJsonOut := jsonOut
	jsonOut = false
	defer func() { jsonOut = oldJsonOut }()

	cmd := newStatusCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	output := out.String()

	// Should NOT be JSON with the new structured format
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err == nil {
		if _, ok := result["tasks"]; ok {
			t.Error("regular output should NOT be JSON with 'tasks' field")
		}
	}

	// Should contain human-readable text (task ID or READY section)
	if !strings.Contains(output, "TASK-001") && !strings.Contains(output, "READY") {
		t.Errorf("regular output should contain task info, got: %s", output)
	}
}

func TestNewCommand_RegularOutput_Unchanged(t *testing.T) {
	tmpDir := withStatusTestDir(t)
	backend := createStatusTestBackend(t, tmpDir)
	_ = backend.Close()

	oldJsonOut := jsonOut
	jsonOut = false
	defer func() { jsonOut = oldJsonOut }()

	cmd := newNewCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--workflow", "implement-trivial", "Test task"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}

	// The current cmd_new.go uses fmt.Printf which goes to os.Stdout, 
	// not cmd.OutOrStdout(). Test that it doesn't output JSON to the
	// cmd output writer.

	// Should NOT be pure JSON with task_id field
	var result map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &result); err == nil {
		if _, ok := result["task_id"]; ok {
			t.Error("regular output should NOT be JSON with 'task_id' field")
		}
	}

	// Note: cmd_new.go currently uses fmt.Printf so cmd.OutOrStdout() 
	// won't capture the output. This test verifies it doesn't accidentally
	// output JSON when jsonOut is false.
}

// =============================================================================
// Helper: newWorkflowsListCmd returns a fresh workflows list command.
// Creating a new instance avoids issues with the global command being
// attached to the root command hierarchy.
// =============================================================================
func newWorkflowsListCmd() *cobra.Command {
	// Create a new command instance that mirrors the global workflowsCmd
	// but is not attached to the root command hierarchy.
	// This is necessary because calling Execute() on a subcommand that's
	// attached to a parent doesn't work correctly in tests.
	cmd := &cobra.Command{
		Use:   "workflows",
		Short: "List available workflows",
		RunE:  workflowsCmd.RunE,
	}
	// Copy the flags from the global command
	cmd.Flags().AddFlagSet(workflowsCmd.Flags())
	return cmd
}

// =============================================================================
// Note: orc show --json already works (cmd_show.go:119). Tests exist in
// cmd_show_test.go. We don't need additional tests for show --json.
// =============================================================================
