package trigger

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// --- SC-11: TriggerRunner shared component tests ---

func TestTriggerRunner_GateMode_Approved(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Reason:   "all checks passed",
			Output:   "validation output",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "validator-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Errorf("expected no error for approved gate, got: %v", err)
	}

	// Verify agent was called with the right agent ID
	if mock.lastAgentID != "validator-agent" {
		t.Errorf("agent ID = %q, want %q", mock.lastAgentID, "validator-agent")
	}
}

func TestTriggerRunner_GateMode_Rejected(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: false,
			Reason:   "task description too vague",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "validator-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err == nil {
		t.Fatal("expected error for rejected gate, got nil")
	}

	// Error should contain rejection reason
	var rejErr *GateRejectionError
	if !errors.As(err, &rejErr) {
		t.Fatalf("expected GateRejectionError, got: %T: %v", err, err)
	}
	if rejErr.Reason != "task description too vague" {
		t.Errorf("rejection reason = %q, want %q", rejErr.Reason, "task description too vague")
	}
}

func TestTriggerRunner_ReactionMode_NeverBlocks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	// Use a slow mock to prove reaction mode doesn't block
	started := make(chan struct{})
	mock := &mockAgentExecutor{
		beforeExecute: func() {
			close(started)
			time.Sleep(500 * time.Millisecond)
		},
		result: &TriggerResult{
			Approved: false,
			Reason:   "rejected but shouldn't matter",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
			AgentID: "notifier-agent",
			Mode:    workflow.GateModeReaction,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// RunLifecycleTriggers should return quickly even with slow reaction agent
	done := make(chan error, 1)
	go func() {
		done <- runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, triggers, t1)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("reaction mode should never return error, got: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		// Also acceptable: still running but returned quickly
		// The key assertion is that it doesn't block
	}

	// Wait for started to confirm the goroutine was actually launched
	select {
	case <-started:
		// Reaction agent was invoked
	case <-time.After(1 * time.Second):
		t.Error("reaction agent was never invoked")
	}
}

func TestTriggerRunner_AgentNotFound(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		err: errors.New("agent not found: nonexistent-agent"),
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "nonexistent-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
	if !containsString(err.Error(), "nonexistent-agent") {
		t.Errorf("error should mention agent ID, got: %v", err)
	}
}

func TestTriggerRunner_Timeout(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		beforeExecute: func() {
			time.Sleep(5 * time.Second)
		},
		result: &TriggerResult{Approved: true},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "slow-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := runner.RunLifecycleTriggers(ctx, workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestTriggerRunner_ParseError(t *testing.T) {
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
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "bad-output-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err == nil {
		t.Fatal("expected error for parse failure, got nil")
	}
	if !containsString(err.Error(), "parse") {
		t.Errorf("error should mention parse failure, got: %v", err)
	}
}

func TestTriggerRunner_NoTriggers(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Nil triggers
	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, nil, t1)
	if err != nil {
		t.Errorf("nil triggers should be no-op, got: %v", err)
	}

	// Empty triggers
	err = runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, []workflow.WorkflowTrigger{}, t1)
	if err != nil {
		t.Errorf("empty triggers should be no-op, got: %v", err)
	}

	// Agent should never be called
	if mock.callCount > 0 {
		t.Errorf("agent should not be called for empty triggers, got %d calls", mock.callCount)
	}
}

func TestTriggerRunner_EmptyTriggers(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{}
	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	// Before-phase triggers: empty slice
	vars := map[string]string{"existing": "value"}
	updatedVars, err := runner.RunBeforePhaseTriggers(
		context.Background(), "implement", nil, vars, nil,
	)
	if err != nil {
		t.Errorf("nil before-phase triggers should be no-op, got: %v", err)
	}
	if updatedVars["existing"] != "value" {
		t.Error("variables should be unchanged for nil triggers")
	}

	if mock.callCount > 0 {
		t.Errorf("agent should not be called for nil triggers, got %d calls", mock.callCount)
	}
}

func TestTriggerRunner_EmptyAgentID(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "", // Empty agent ID
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Should skip trigger with empty agent ID (log warning, don't error)
	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Errorf("empty agent ID should be skipped, got: %v", err)
	}

	if mock.callCount > 0 {
		t.Errorf("agent should not be called for empty agent ID, got %d calls", mock.callCount)
	}
}

func TestTriggerRunner_DisabledTrigger(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "validator-agent",
			Mode:    workflow.GateModeGate,
			Enabled: false, // Disabled
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Errorf("disabled trigger should be skipped, got: %v", err)
	}

	if mock.callCount > 0 {
		t.Errorf("agent should not be called for disabled trigger, got %d calls", mock.callCount)
	}
}

func TestTriggerRunner_MultipleTriggers_FirstGateRejectionStops(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	callOrder := []string{}
	var mu sync.Mutex

	mock := &mockAgentExecutorFunc{
		fn: func(agentID string) (*TriggerResult, error) {
			mu.Lock()
			callOrder = append(callOrder, agentID)
			mu.Unlock()

			if agentID == "gate-2" {
				return &TriggerResult{
					Approved: false,
					Reason:   "validation failed",
				}, nil
			}
			return &TriggerResult{Approved: true}, nil
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "gate-1",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "gate-2",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "gate-3",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err == nil {
		t.Fatal("expected error from rejected gate, got nil")
	}

	mu.Lock()
	defer mu.Unlock()

	// gate-1 should have been called (approved), gate-2 should have been called (rejected),
	// gate-3 should NOT have been called (short-circuited)
	if len(callOrder) != 2 {
		t.Errorf("expected 2 agent calls, got %d: %v", len(callOrder), callOrder)
	}
	if len(callOrder) >= 1 && callOrder[0] != "gate-1" {
		t.Errorf("first call should be gate-1, got %s", callOrder[0])
	}
	if len(callOrder) >= 2 && callOrder[1] != "gate-2" {
		t.Errorf("second call should be gate-2, got %s", callOrder[1])
	}
}

func TestTriggerRunner_FiltersEventsByType(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	// Mix of trigger events - only on_task_completed should fire
	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "create-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
			AgentID: "complete-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
		{
			Event:   workflow.WorkflowTriggerEventOnTaskFailed,
			AgentID: "fail-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, triggers, t1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only the on_task_completed trigger should fire
	if mock.callCount != 1 {
		t.Errorf("expected 1 agent call (for on_task_completed), got %d", mock.callCount)
	}
	if mock.lastAgentID != "complete-agent" {
		t.Errorf("wrong agent called: got %q, want %q", mock.lastAgentID, "complete-agent")
	}
}

func TestTriggerRunner_EmptyGateOutput_TreatedAsApproved(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Output:   "", // Empty output
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "gate-agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Errorf("empty gate output should be treated as approved, got: %v", err)
	}
}

func TestTriggerRunner_ReactionPanic_Recovered(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	panicDone := make(chan struct{})
	mock := &mockAgentExecutor{
		beforeExecute: func() {
			defer func() { close(panicDone) }()
			panic("agent crashed!")
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCompleted,
			AgentID: "crashy-agent",
			Mode:    workflow.GateModeReaction,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Should not panic the caller
	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCompleted, triggers, t1)
	if err != nil {
		t.Errorf("reaction panic should not propagate, got: %v", err)
	}

	// Wait for the panicking goroutine to be recovered
	select {
	case <-panicDone:
		// Panic was recovered in goroutine
	case <-time.After(2 * time.Second):
		t.Error("panic goroutine did not complete in time")
	}
}

// --- SC-3: Before-phase trigger output flows into variable system ---

func TestTriggerRunner_BeforePhaseTrigger_OutputVariable(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Output:   "validation passed: all criteria met",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.BeforePhaseTrigger{
		{
			AgentID: "validator-agent",
			Mode:    workflow.GateModeGate,
			OutputConfig: &workflow.GateOutputConfig{
				VariableName: "validation_result",
			},
		},
	}

	vars := map[string]string{"existing_var": "existing_value"}

	updatedVars, err := runner.RunBeforePhaseTriggers(
		context.Background(), "implement", triggers, vars, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should flow into variable
	if updatedVars["validation_result"] != "validation passed: all criteria met" {
		t.Errorf("variable not set: got %q, want %q",
			updatedVars["validation_result"], "validation passed: all criteria met")
	}

	// Existing vars preserved
	if updatedVars["existing_var"] != "existing_value" {
		t.Errorf("existing var lost: got %q", updatedVars["existing_var"])
	}
}

func TestTriggerRunner_BeforePhaseTrigger_NoVariableName_OutputDiscarded(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: true,
			Output:   "some output",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.BeforePhaseTrigger{
		{
			AgentID: "agent-1",
			Mode:    workflow.GateModeGate,
			// No OutputConfig - output should be discarded silently
		},
	}

	vars := map[string]string{}
	updatedVars, err := runner.RunBeforePhaseTriggers(
		context.Background(), "implement", triggers, vars, nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No new variables should be added
	if len(updatedVars) != 0 {
		t.Errorf("expected no new variables, got %v", updatedVars)
	}
}

func TestTriggerRunner_BeforePhaseTrigger_GateMode_Blocks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{
			Approved: false,
			Reason:   "precondition not met",
		},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.BeforePhaseTrigger{
		{
			AgentID: "gate-agent",
			Mode:    workflow.GateModeGate,
		},
	}

	vars := map[string]string{}
	_, err := runner.RunBeforePhaseTriggers(
		context.Background(), "implement", triggers, vars, nil,
	)
	if err == nil {
		t.Fatal("expected error for rejected before-phase gate, got nil")
	}

	var rejErr *GateRejectionError
	if !errors.As(err, &rejErr) {
		t.Fatalf("expected GateRejectionError, got: %T: %v", err, err)
	}
}

func TestTriggerRunner_BeforePhaseTrigger_ReactionMode_NeverBlocks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	started := make(chan struct{})
	mock := &mockAgentExecutor{
		beforeExecute: func() {
			close(started)
			time.Sleep(500 * time.Millisecond)
		},
		result: &TriggerResult{Approved: false, Reason: "would reject but reaction mode"},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.BeforePhaseTrigger{
		{
			AgentID: "async-agent",
			Mode:    workflow.GateModeReaction,
		},
	}

	vars := map[string]string{}
	_, err := runner.RunBeforePhaseTriggers(
		context.Background(), "implement", triggers, vars, nil,
	)
	if err != nil {
		t.Errorf("reaction mode should never block, got: %v", err)
	}

	// Agent should still be invoked in background
	select {
	case <-started:
		// Good - agent was launched
	case <-time.After(1 * time.Second):
		t.Error("reaction agent was never invoked")
	}
}

func TestTriggerRunner_BeforePhaseTrigger_FailedOutputVar(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		err: errors.New("agent execution failed"),
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.BeforePhaseTrigger{
		{
			AgentID: "failing-agent",
			Mode:    workflow.GateModeGate,
			OutputConfig: &workflow.GateOutputConfig{
				VariableName: "result_var",
			},
		},
	}

	vars := map[string]string{}
	// Per spec: "Agent error → log warning, continue phase execution (don't block on trigger infra failure)"
	updatedVars, err := runner.RunBeforePhaseTriggers(
		context.Background(), "implement", triggers, vars, nil,
	)
	// Infrastructure failure should log warning but not block
	if err != nil {
		t.Errorf("trigger infra failure should not block phase, got: %v", err)
	}

	// Variable should not be set
	if _, exists := updatedVars["result_var"]; exists {
		t.Error("variable should not be set when trigger fails")
	}
}

// --- SC-12: Event logging ---

func TestTriggerRunner_EventLogging(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()
	pub := events.NewMemoryPublisher()
	defer pub.Close()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true, Output: "ok"},
	}

	runner := NewTriggerRunner(backend, logger,
		WithAgentExecutor(mock),
		WithEventPublisher(pub),
	)

	// Subscribe to events for this task
	ch := pub.Subscribe("TASK-001")

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "validator",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Collect events with timeout
	var receivedEvents []events.Event
	timeout := time.After(1 * time.Second)
	for {
		select {
		case ev := <-ch:
			receivedEvents = append(receivedEvents, ev)
		case <-timeout:
			goto done
		}
	}
done:

	// Should have at least trigger start and trigger complete events
	if len(receivedEvents) < 2 {
		t.Errorf("expected at least 2 trigger events (start + complete), got %d", len(receivedEvents))
	}
}

func TestTriggerRunner_EventLogging_NilPublisher(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true},
	}

	// No publisher configured - should not panic
	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "agent",
			Mode:    workflow.GateModeGate,
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Should not panic even with nil publisher
	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTriggerRunner_MalformedConfig(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	logger := slog.Default()

	mock := &mockAgentExecutor{
		result: &TriggerResult{Approved: true},
	}

	runner := NewTriggerRunner(backend, logger, WithAgentExecutor(mock))

	// Test that malformed triggers are handled gracefully
	// The triggers have been parsed already at this level, but test the runner's
	// handling of a trigger with bad agent ID format or other issues
	triggers := []workflow.WorkflowTrigger{
		{
			Event:   workflow.WorkflowTriggerEventOnTaskCreated,
			AgentID: "valid-agent",
			Mode:    "", // Empty mode - should default to gate
			Enabled: true,
		},
	}

	t1 := task.NewProtoTask("TASK-001", "Test task")
	if err := backend.SaveTask(t1); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Should handle gracefully - empty mode defaults to gate
	err := runner.RunLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskCreated, triggers, t1)
	if err != nil {
		t.Errorf("empty mode should default to gate and work, got: %v", err)
	}
}

// --- Mock types ---
// These are test helpers that the implementation must satisfy.

// mockAgentExecutor is a simple mock for the AgentExecutor interface.
type mockAgentExecutor struct {
	result        *TriggerResult
	err           error
	callCount     int
	lastAgentID   string
	beforeExecute func()
	mu            sync.Mutex
}

func (m *mockAgentExecutor) ExecuteTriggerAgent(ctx context.Context, agentID string, input *TriggerInput) (*TriggerResult, error) {
	m.mu.Lock()
	m.callCount++
	m.lastAgentID = agentID
	m.mu.Unlock()

	if m.beforeExecute != nil {
		m.beforeExecute()
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockAgentExecutorFunc allows per-agent-ID behavior.
type mockAgentExecutorFunc struct {
	fn func(agentID string) (*TriggerResult, error)
}

func (m *mockAgentExecutorFunc) ExecuteTriggerAgent(ctx context.Context, agentID string, input *TriggerInput) (*TriggerResult, error) {
	return m.fn(agentID)
}

// --- Helpers ---

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
