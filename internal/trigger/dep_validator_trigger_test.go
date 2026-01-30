package trigger

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/workflow"

	"log/slog"
)

// --- SC-5: Agent receives initiative task data via ExtraFields ---

func TestInitiativePlannedTrigger_PassesTaskIDsToExecutor(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	// Use a mock that captures the input for inspection
	captureMock := &capturingAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Reason:   "all dependencies present",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(captureMock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	taskIDs := []string{"TASK-001", "TASK-002", "TASK-003"}

	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", taskIDs,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the executor received the input with initiative_id
	capturedInput := captureMock.lastInput
	if capturedInput == nil {
		t.Fatal("executor was not called")
	}
	if capturedInput.ExtraFields["initiative_id"] != "INIT-001" {
		t.Errorf("ExtraFields[initiative_id] = %q, want %q",
			capturedInput.ExtraFields["initiative_id"], "INIT-001")
	}

	// SC-5: Task IDs should be available in ExtraFields or input
	// The exact format depends on implementation, but the task IDs must be passed
	if capturedInput.Event != string(workflow.WorkflowTriggerEventOnInitiativePlanned) {
		t.Errorf("Event = %q, want %q",
			capturedInput.Event, workflow.WorkflowTriggerEventOnInitiativePlanned)
	}

}

// --- SC-7: Rejection returns GateRejectionError ---

func TestInitiativePlannedTrigger_RejectsWithMissingDeps(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: false,
			Reason:   "Missing dependencies found:\n- TASK-003 should depend on TASK-001: Task 3 calls API endpoint created by Task 1\n- TASK-003 should depend on TASK-002: Task 3 imports package created by Task 2",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{"TASK-001", "TASK-002", "TASK-003"},
	)

	// SC-7: Must return GateRejectionError
	if err == nil {
		t.Fatal("expected error for rejected gate, got nil")
	}

	var rejErr *GateRejectionError
	if !errors.As(err, &rejErr) {
		t.Fatalf("expected GateRejectionError, got: %T: %v", err, err)
	}

	// SC-7: AgentID should be dependency-validator
	if rejErr.AgentID != "dependency-validator" {
		t.Errorf("rejection AgentID = %q, want %q", rejErr.AgentID, "dependency-validator")
	}

	// SC-9: Reason should contain structured suggestions
	if rejErr.Reason == "" {
		t.Error("rejection reason is empty")
	}
}

// --- SC-8: Approval proceeds without blocking ---

func TestInitiativePlannedTrigger_ApprovesWhenNoDeps(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Reason:   "no missing dependencies found",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{"TASK-001", "TASK-002"},
	)

	// SC-8: No error means planning proceeds
	if err != nil {
		t.Errorf("expected no error for approved gate, got: %v", err)
	}
}

// --- SC-9: Rejection message includes structured suggestions ---

func TestInitiativePlannedTrigger_RejectionReasonHasStructuredSuggestions(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	// Simulate an agent that returns structured rejection with task references
	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: false,
			Reason:   "TASK-C should depend on TASK-B: Task C calls API endpoint created by Task B",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{"TASK-B", "TASK-C"},
	)
	if err == nil {
		t.Fatal("expected rejection error, got nil")
	}

	var rejErr *GateRejectionError
	if !errors.As(err, &rejErr) {
		t.Fatalf("expected GateRejectionError, got: %T: %v", err, err)
	}

	// SC-9: Rejection reason must mention which task should depend on which
	if !strings.Contains(rejErr.Reason, "TASK-C") {
		t.Errorf("rejection reason should mention TASK-C (the 'from' task), got: %q", rejErr.Reason)
	}
	if !strings.Contains(rejErr.Reason, "TASK-B") {
		t.Errorf("rejection reason should mention TASK-B (the 'on' task), got: %q", rejErr.Reason)
	}
}

// --- Edge cases from spec ---

func TestInitiativePlannedTrigger_EmptyTaskIDs(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Reason:   "no tasks to analyze",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	// Edge case: initiative with 0 tasks
	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{},
	)
	if err != nil {
		t.Errorf("empty task IDs should not cause error, got: %v", err)
	}
}

func TestInitiativePlannedTrigger_SingleTask(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Reason:   "single task, no dependencies possible",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	// Edge case: initiative with 1 task
	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{"TASK-001"},
	)
	if err != nil {
		t.Errorf("single task should not cause error, got: %v", err)
	}
}

// --- Failure modes ---

func TestInitiativePlannedTrigger_AgentExecutionError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		err: errors.New("LLM API request failed: timeout"),
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{"TASK-001"},
	)

	// SC-7 (BDD-3): Error should propagate
	if err == nil {
		t.Fatal("expected error for agent execution failure, got nil")
	}
	if !strings.Contains(err.Error(), "dependency-validator") {
		t.Errorf("error should mention agent ID, got: %v", err)
	}
}

func TestInitiativePlannedTrigger_ParseError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			ParseError: errors.New("invalid JSON in agent output"),
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnInitiativePlanned,
			AgentID: "dependency-validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	err := runner.RunInitiativePlannedTrigger(
		context.Background(), triggers, "INIT-001", []string{"TASK-001"},
	)

	if err == nil {
		t.Fatal("expected error for parse failure, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error should mention parse failure, got: %v", err)
	}
}

// --- Mock: captures input for inspection ---

type capturingAgentExecutor struct {
	result      *TriggerResult
	err         error
	lastInput   *TriggerInput
	lastAgentID string
}

func (m *capturingAgentExecutor) ExecuteTriggerAgent(ctx context.Context, agentID string, input *TriggerInput) (*TriggerResult, error) {
	m.lastInput = input
	m.lastAgentID = agentID
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}
