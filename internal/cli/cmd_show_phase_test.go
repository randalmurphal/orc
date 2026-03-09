package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

func TestShowCommand_DisplaysCurrentRunningPhase(t *testing.T) {
	tmpDir := withShowTestDir(t)

	backend := createShowTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-RUNNING-SHOW", "Running phase display")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement")
	tk.Execution = task.InitProtoExecutionState()
	task.EnsurePhaseProto(tk.Execution, "spec")
	task.CompletePhaseProto(tk.Execution, "spec", "")
	task.EnsurePhaseProto(tk.Execution, "implement")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-RUNNING-SHOW"})
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
	if !strings.Contains(output, "◐ implement (running)") {
		t.Fatalf("show output should mark current running phase, got:\n%s", output)
	}
}
