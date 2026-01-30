// Package cli implements the orc command-line interface.
//
// Tests for gate info display in `orc show` and `orc show --gates`.
// These tests use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel().
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// =============================================================================
// SC-4: `orc show TASK-ID` displays gate decisions in phases section
// =============================================================================

func TestShowGateInfo_PhasesWithGateDecisions(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-100", "Gate test task")
	reason := "auto-approved on success"
	tk.Execution = &orcv1.ExecutionState{
		Gates: []*orcv1.GateDecision{
			{
				Phase:     "spec",
				GateType:  "auto",
				Approved:  true,
				Reason:    &reason,
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-100"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute show: %v", err)
		}
	})

	// Should show gate decision annotation on phase line
	// Expected format: "✓ spec (auto: approved)" or similar
	if !bytes.Contains([]byte(output), []byte("spec")) {
		t.Errorf("output should contain phase 'spec', got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("auto")) {
		t.Errorf("output should show gate type 'auto', got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("approved")) {
		t.Errorf("output should show 'approved' status, got:\n%s", output)
	}
}

func TestShowGateInfo_RejectedGate(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-101", "Rejected gate task")
	reason := "Needs tests"
	tk.Execution = &orcv1.ExecutionState{
		Gates: []*orcv1.GateDecision{
			{
				Phase:     "review",
				GateType:  "human",
				Approved:  false,
				Reason:    &reason,
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-101"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute show: %v", err)
		}
	})

	// Should show rejected gate decision
	if !bytes.Contains([]byte(output), []byte("human")) {
		t.Errorf("output should show gate type 'human', got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("rejected")) || !bytes.Contains([]byte(output), []byte("Needs tests")) {
		t.Errorf("output should show rejection reason, got:\n%s", output)
	}
}

// SC-4 edge case: no gate decisions → phases display as before
func TestShowGateInfo_NoGateDecisions(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-102", "No gates task")
	// No execution state or empty gates
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-102"})

	// Should not error
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute show: %v", err)
	}
}

// SC-4 edge case: gate decision for phase not in plan (orphaned data)
func TestShowGateInfo_OrphanGateDecision(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-103", "Orphan gate task")
	reason := "auto"
	tk.Execution = &orcv1.ExecutionState{
		Gates: []*orcv1.GateDecision{
			{
				Phase:     "nonexistent_phase",
				GateType:  "auto",
				Approved:  true,
				Reason:    &reason,
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-103"})

	// Should not error - orphaned gate decisions are silently ignored
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute show: %v", err)
	}
}

// =============================================================================
// SC-5: `orc show TASK-ID --json` includes gate decisions in execution.gates
// =============================================================================

func TestShowGateInfo_JSON_IncludesGates(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-104", "JSON gate task")
	reason := "auto-approved on success"
	tk.Execution = &orcv1.ExecutionState{
		Gates: []*orcv1.GateDecision{
			{
				Phase:     "implement",
				GateType:  "auto",
				Approved:  true,
				Reason:    &reason,
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	oldJSON := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJSON }()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-104"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute show --json: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, output)
	}

	// execution.gates should be an array
	execution, ok := result["execution"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'execution' object in JSON output, got: %T", result["execution"])
	}

	gates, ok := execution["gates"].([]any)
	if !ok {
		t.Fatalf("expected 'execution.gates' array, got: %T", execution["gates"])
	}

	if len(gates) != 1 {
		t.Errorf("expected 1 gate decision, got %d", len(gates))
	}

	gd, ok := gates[0].(map[string]any)
	if !ok {
		t.Fatalf("gate decision should be object, got: %T", gates[0])
	}

	if gd["phase"] != "implement" {
		t.Errorf("gate phase = %v, want 'implement'", gd["phase"])
	}
	if gd["approved"] != true {
		t.Errorf("gate approved = %v, want true", gd["approved"])
	}
}

func TestShowGateInfo_JSON_EmptyGates(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-105", "No gates JSON task")
	tk.Execution = &orcv1.ExecutionState{}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	oldJSON := jsonOut
	jsonOut = true
	defer func() { jsonOut = oldJSON }()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-105"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("JSON parse failed: %v\nOutput: %s", err, output)
	}

	execution, ok := result["execution"].(map[string]any)
	if !ok {
		// Execution may be present but gates empty
		return
	}

	// Gates should be empty array or nil
	if gates, ok := execution["gates"].([]any); ok && len(gates) != 0 {
		t.Errorf("expected empty gates array, got %d entries", len(gates))
	}
}

// =============================================================================
// SC-10: `orc show TASK-ID --gates` dedicated gate history section
// =============================================================================

func TestShowGates_DedicatedHistory(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-106", "Gate history task")
	reason1 := "auto-approved"
	reason2 := "Needs more tests"
	tk.Execution = &orcv1.ExecutionState{
		Gates: []*orcv1.GateDecision{
			{
				Phase:     "spec",
				GateType:  "auto",
				Approved:  true,
				Reason:    &reason1,
				Timestamp: timestamppb.New(time.Now().Add(-2 * time.Hour)),
			},
			{
				Phase:     "implement",
				GateType:  "human",
				Approved:  false,
				Reason:    &reason2,
				Timestamp: timestamppb.New(time.Now().Add(-1 * time.Hour)),
			},
		},
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-106", "--gates"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute show --gates: %v", err)
		}
	})

	// Should have a gate history section header
	if !bytes.Contains([]byte(output), []byte("Gate")) {
		t.Errorf("output should contain 'Gate' section header, got:\n%s", output)
	}

	// Should show both decisions chronologically
	if !bytes.Contains([]byte(output), []byte("spec")) {
		t.Errorf("output should show spec gate decision, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("implement")) {
		t.Errorf("output should show implement gate decision, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Needs more tests")) {
		t.Errorf("output should show rejection reason, got:\n%s", output)
	}
}

func TestShowGates_Empty(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-107", "Empty gates task")
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-107", "--gates"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute show --gates: %v", err)
		}
	})

	// Should show "No gate decisions" message
	if !bytes.Contains([]byte(output), []byte("No gate decisions")) {
		t.Errorf("output should say 'No gate decisions', got:\n%s", output)
	}
}

// Edge case: multiple gate decisions for same phase (retry)
func TestShowGates_MultiplePerPhase(t *testing.T) {
	dir := withShowTestDir(t)
	backend := createShowTestBackend(t, dir)

	tk := task.NewProtoTask("TASK-108", "Retry gate task")
	reason1 := "rejected - needs refactor"
	reason2 := "approved after retry"
	tk.Execution = &orcv1.ExecutionState{
		Gates: []*orcv1.GateDecision{
			{
				Phase:     "review",
				GateType:  "human",
				Approved:  false,
				Reason:    &reason1,
				Timestamp: timestamppb.New(time.Now().Add(-1 * time.Hour)),
			},
			{
				Phase:     "review",
				GateType:  "human",
				Approved:  true,
				Reason:    &reason2,
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	_ = backend.Close()

	cmd := newShowCmd()
	cmd.SetArgs([]string{"TASK-108", "--gates"})

	output := captureShowStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})

	// Both decisions should appear
	if !bytes.Contains([]byte(output), []byte("rejected")) || !bytes.Contains([]byte(output), []byte("needs refactor")) {
		t.Errorf("output should show first (rejected) decision, got:\n%s", output)
	}
	if !bytes.Contains([]byte(output), []byte("approved")) || !bytes.Contains([]byte(output), []byte("after retry")) {
		t.Errorf("output should show second (approved) decision, got:\n%s", output)
	}
}

// =============================================================================
// SC-10: --gates flag exists on show command
// =============================================================================

func TestShowCmd_GatesFlag(t *testing.T) {
	cmd := newShowCmd()

	if cmd.Flag("gates") == nil {
		t.Error("missing --gates flag on show command")
	}
}

// Helper to capture stdout for show tests
func captureShowStdout(t *testing.T, fn func()) string {
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
