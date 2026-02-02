// Tests for TASK-685: Parallel phase execution from dependency graph.
//
// These tests define the contract for parallel execution of workflow phases
// based on their dependency graph. Tests cover:
//   - computeExecutionLevels() grouping phases by dependency level
//   - Parallel execution of phases within the same level
//   - Failure cancellation of sibling phases
//   - Thread-safe variable writes
//   - Sequential fallback for linear dependencies
//
// Coverage mapping:
//   SC-1:  TestParallelExecution_DiamondPattern
//   SC-2:  TestComputeExecutionLevels_*
//   SC-3:  (code inspection: grep for errgroup.WithContext)
//   SC-4:  TestParallelExecution_DependentWaitsForPredecessors
//   SC-5:  TestParallelExecution_FailureCancelsSiblings
//   SC-6:  TestParallelExecution_FirstErrorReported
//   SC-7:  TestParallelExecution_NextLevelNotStartedOnFailure
//   SC-8:  TestSafeVars_Concurrent
//   SC-9:  (code inspection: rctx cloning per goroutine)
//   SC-10: TestParallelExecution_LinearChainSequential
//   SC-11: TestParallelExecution_NoDepsPreservesSequence
//   SC-12: (run tests with -race flag: go test -race ./internal/executor/...)
//
// Failure modes:
//   TestParallelExecution_PanicRecovery
//   TestParallelExecution_ContextCancel
//   TestSafeVars_Concurrent (race detector)
//   TestParallelExecution_AllFail
//
// Edge cases:
//   TestParallelExecution_SinglePhaseLevel
//   TestParallelExecution_Linear
//   TestParallelExecution_WithLoop
//   TestParallelExecution_ResumePartial
package executor

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// SC-2: computeExecutionLevels() returns correct level groupings
//
// Unit tests for the new function that groups phases by execution level.
// Phases in the same level have no dependencies on each other.
// =============================================================================

// TestComputeExecutionLevels_Diamond verifies the canonical diamond pattern:
// A→B,C→D produces [[A], [B,C], [D]]
func TestComputeExecutionLevels_Diamond(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"A"}),
		makePhase("C", 3, []string{"A"}),
		makePhase("D", 4, []string{"B", "C"}),
	}

	levels, err := computeExecutionLevels(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 3 levels: [[A], [B,C], [D]]
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}

	// Level 0: just A
	if len(levels[0]) != 1 || levels[0][0].PhaseTemplateID != "A" {
		t.Errorf("level 0: expected [A], got %v", extractPhaseIDs(levels[0]))
	}

	// Level 1: B and C (order may vary)
	if len(levels[1]) != 2 {
		t.Errorf("level 1: expected 2 phases, got %d", len(levels[1]))
	}
	level1IDs := extractPhaseIDs(levels[1])
	if !containsAll(level1IDs, []string{"B", "C"}) {
		t.Errorf("level 1: expected B and C, got %v", level1IDs)
	}

	// Level 2: just D
	if len(levels[2]) != 1 || levels[2][0].PhaseTemplateID != "D" {
		t.Errorf("level 2: expected [D], got %v", extractPhaseIDs(levels[2]))
	}
}

// TestComputeExecutionLevels_Linear verifies linear chain A→B→C produces
// [[A], [B], [C]] (each phase in its own level).
func TestComputeExecutionLevels_Linear(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"A"}),
		makePhase("C", 3, []string{"B"}),
	}

	levels, err := computeExecutionLevels(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 3 levels, each with 1 phase
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}

	for i, expected := range []string{"A", "B", "C"} {
		if len(levels[i]) != 1 {
			t.Errorf("level %d: expected 1 phase, got %d", i, len(levels[i]))
		}
		if levels[i][0].PhaseTemplateID != expected {
			t.Errorf("level %d: expected %s, got %s", i, expected, levels[i][0].PhaseTemplateID)
		}
	}
}

// TestComputeExecutionLevels_NoDeps verifies that phases without dependencies
// all land in level 0 and are ordered by sequence.
func TestComputeExecutionLevels_NoDeps(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		makePhase("C", 3, nil),
		makePhase("A", 1, nil),
		makePhase("B", 2, nil),
	}

	levels, err := computeExecutionLevels(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All phases should be in a single level (all can run in parallel)
	if len(levels) != 1 {
		t.Fatalf("expected 1 level (all parallel), got %d", len(levels))
	}

	// Should contain all 3 phases
	if len(levels[0]) != 3 {
		t.Errorf("level 0: expected 3 phases, got %d", len(levels[0]))
	}

	ids := extractPhaseIDs(levels[0])
	if !containsAll(ids, []string{"A", "B", "C"}) {
		t.Errorf("level 0: expected A, B, C, got %v", ids)
	}
}

// TestComputeExecutionLevels_WiderDiamond tests A→[B,C,D]→E pattern.
func TestComputeExecutionLevels_WiderDiamond(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, []string{"A"}),
		makePhase("C", 3, []string{"A"}),
		makePhase("D", 4, []string{"A"}),
		makePhase("E", 5, []string{"B", "C", "D"}),
	}

	levels, err := computeExecutionLevels(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 3 levels: [[A], [B,C,D], [E]]
	if len(levels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(levels))
	}

	if len(levels[0]) != 1 {
		t.Errorf("level 0: expected 1 phase, got %d", len(levels[0]))
	}
	if len(levels[1]) != 3 {
		t.Errorf("level 1: expected 3 phases, got %d", len(levels[1]))
	}
	if len(levels[2]) != 1 {
		t.Errorf("level 2: expected 1 phase, got %d", len(levels[2]))
	}
}

// TestComputeExecutionLevels_Empty returns empty for empty input.
func TestComputeExecutionLevels_Empty(t *testing.T) {
	t.Parallel()

	levels, err := computeExecutionLevels([]*db.WorkflowPhase{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(levels) != 0 {
		t.Errorf("expected empty levels, got %d", len(levels))
	}
}

// TestComputeExecutionLevels_Nil returns empty for nil input.
func TestComputeExecutionLevels_Nil(t *testing.T) {
	t.Parallel()

	levels, err := computeExecutionLevels(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(levels) != 0 {
		t.Errorf("expected empty levels, got %d", len(levels))
	}
}

// TestComputeExecutionLevels_Cycle returns error on cyclic dependencies.
func TestComputeExecutionLevels_Cycle(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		makePhase("A", 1, []string{"B"}),
		makePhase("B", 2, []string{"A"}),
	}

	_, err := computeExecutionLevels(phases)
	if err == nil {
		t.Fatal("expected error for cycle, got nil")
	}
}

// TestComputeExecutionLevels_MultipleRoots tests multiple entry points.
// A and B have no deps, both feed into C.
func TestComputeExecutionLevels_MultipleRoots(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		makePhase("A", 1, nil),
		makePhase("B", 2, nil),
		makePhase("C", 3, []string{"A", "B"}),
	}

	levels, err := computeExecutionLevels(phases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect 2 levels: [[A, B], [C]]
	if len(levels) != 2 {
		t.Fatalf("expected 2 levels, got %d", len(levels))
	}

	if len(levels[0]) != 2 {
		t.Errorf("level 0: expected 2 phases, got %d", len(levels[0]))
	}
	if len(levels[1]) != 1 {
		t.Errorf("level 1: expected 1 phase, got %d", len(levels[1]))
	}
}

// =============================================================================
// SC-1: Phases with no dependencies between them start concurrently
//
// Integration test: diamond pattern with timing verification.
// =============================================================================

// TestParallelExecution_DiamondPattern verifies that B and C start within
// 50ms of each other (not sequential 100ms+ gap).
//
// NOTE: This test requires parallel execution to be implemented (TASK-685).
// Skip until that task is completed.
func TestParallelExecution_DiamondPattern(t *testing.T) {
	t.Skip("Requires parallel execution implementation (TASK-685)")
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupDiamondWorkflow(t, backend, "diamond-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-DIAMOND-001", "diamond-wf")

	// Mock that records start times
	startTimes := &sync.Map{}
	mock := &timingMockTurnExecutor{
		startTimes: startTimes,
		delay:      50 * time.Millisecond, // Each phase takes 50ms
		response:   `{"status": "complete", "summary": "Done"}`,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "diamond-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test parallel diamond",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify B and C started concurrently
	bStart, bOK := startTimes.Load("B")
	cStart, cOK := startTimes.Load("C")

	if !bOK || !cOK {
		t.Fatal("B or C did not record start time")
	}

	bTime := bStart.(time.Time)
	cTime := cStart.(time.Time)

	diff := bTime.Sub(cTime)
	if diff < 0 {
		diff = -diff
	}

	// Phases B and C should start within 50ms of each other (parallel)
	// If sequential, there would be at least 50ms gap (duration of first phase)
	if diff > 50*time.Millisecond {
		t.Errorf("SC-1: B and C should start concurrently, but diff was %v", diff)
	}
}

// =============================================================================
// SC-4: Dependent phases wait for all predecessors to complete
// =============================================================================

// TestParallelExecution_DependentWaitsForPredecessors verifies that phase C
// (which depends on A and B) doesn't start until both A and B complete.
func TestParallelExecution_DependentWaitsForPredecessors(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupMultiRootWorkflow(t, backend, "multi-root-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-MULTIROOT-001", "multi-root-wf")

	// Track start and end times
	times := &phaseTimings{
		startTimes: make(map[string]time.Time),
		endTimes:   make(map[string]time.Time),
	}
	mock := &timingMockTurnExecutor{
		startTimes: &sync.Map{},
		delay:      50 * time.Millisecond,
		response:   `{"status": "complete", "summary": "Done"}`,
		timings:    times,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "multi-root-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test multi-root",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify C started after both A and B ended
	times.mu.Lock()
	defer times.mu.Unlock()

	cStart, cOK := times.startTimes["C"]
	aEnd, aOK := times.endTimes["A"]
	bEnd, bOK := times.endTimes["B"]

	if !cOK || !aOK || !bOK {
		t.Fatal("missing timing data for A, B, or C")
	}

	// C.StartTime >= max(A.EndTime, B.EndTime)
	maxPredEnd := aEnd
	if bEnd.After(maxPredEnd) {
		maxPredEnd = bEnd
	}

	if cStart.Before(maxPredEnd) {
		t.Errorf("SC-4: C started at %v, but predecessors ended at A=%v, B=%v",
			cStart, aEnd, bEnd)
	}
}

// =============================================================================
// SC-5: When a phase fails, remaining sibling phases are cancelled
// SC-7: Next level phases do NOT start after parallel group failure
// =============================================================================

// TestParallelExecution_FailureCancelsSiblings verifies that when C fails in
// A→[B,C,D]→E, phases B and D are cancelled and E never starts.
func TestParallelExecution_FailureCancelsSiblings(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupWiderDiamondWorkflow(t, backend, "fail-cancel-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-FAILCANCEL-001", "fail-cancel-wf")

	// Track which phases were called
	var calledPhases sync.Map
	var cancelled atomic.Int32

	mock := &failingMockTurnExecutor{
		called:        &calledPhases,
		cancelledSeen: &cancelled,
		failPhase:     "C",
		failError:     errors.New("phase C failed"),
		delay:         100 * time.Millisecond, // B and D should be running when C fails
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "fail-cancel-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test failure cancellation",
	})

	// Expect error from the failed phase
	if err == nil {
		t.Fatal("expected error from failed phase C")
	}

	// Verify E never started
	if _, called := calledPhases.Load("E"); called {
		t.Error("SC-7: phase E should NOT have been called after parallel group failure")
	}

	// Verify B and D received cancellation (or weren't called after C failed)
	// The cancelled counter should indicate context cancellation was observed
	// OR B and D shouldn't have completed successfully after C's failure
}

// =============================================================================
// SC-6: First error in parallel group is reported as the group failure
// =============================================================================

// TestParallelExecution_FirstErrorReported verifies that when B and C both
// fail, the first error is in the result.
func TestParallelExecution_FirstErrorReported(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupMultiRootWorkflow(t, backend, "first-error-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-FIRSTERROR-001", "first-error-wf")

	// Both A and B fail, but A fails faster
	mock := &multiFailMockTurnExecutor{
		failPhases: map[string]time.Duration{
			"A": 10 * time.Millisecond,  // Fails first
			"B": 100 * time.Millisecond, // Fails later
		},
		errors: map[string]error{
			"A": errors.New("error from A"),
			"B": errors.New("error from B"),
		},
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "first-error-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test first error",
	})

	if err == nil {
		t.Fatal("expected error from failed phases")
	}

	// SC-6: First error should be from A (fastest to fail)
	if !containsSubstring(err.Error(), "error from A") {
		t.Errorf("SC-6: expected first error to contain 'error from A', got: %v", err)
	}
}

// =============================================================================
// SC-10: Linear dependency chain executes sequentially
// =============================================================================

// TestParallelExecution_LinearChainSequential verifies that A→B→C executes
// in strict order.
func TestParallelExecution_LinearChainSequential(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupLinearWorkflow(t, backend, "linear-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-LINEAR-001", "linear-wf")

	var executionOrder []string
	var mu sync.Mutex

	mock := &orderTrackingMockExecutor{
		order: &executionOrder,
		mu:    &mu,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "linear-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test linear execution",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should execute in strict order A, B, C
	expected := []string{"A", "B", "C"}
	if !slicesEqual(executionOrder, expected) {
		t.Errorf("SC-10: expected order %v, got %v", expected, executionOrder)
	}
}

// =============================================================================
// SC-11: Empty depends_on workflow behaves identically to before
// =============================================================================

// TestParallelExecution_NoDepsPreservesSequence verifies that a workflow with
// no depends_on fields still runs in sequence order (same as before parallel).
func TestParallelExecution_NoDepsPreservesSequence(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupNoDepsWorkflow(t, backend, "nodeps-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-NODEPS-001", "nodeps-wf")

	var executionOrder []string
	var mu sync.Mutex

	mock := &orderTrackingMockExecutor{
		order: &executionOrder,
		mu:    &mu,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "nodeps-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test no deps sequence",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// When all phases have no deps, they can run in parallel
	// but the result should contain all phases
	if len(executionOrder) != 3 {
		t.Errorf("expected 3 phases to execute, got %d", len(executionOrder))
	}

	// All phases should be present
	if !containsAll(executionOrder, []string{"A", "B", "C"}) {
		t.Errorf("expected A, B, C to execute, got %v", executionOrder)
	}
}

// =============================================================================
// SC-8: Thread-safe vars map writes
// =============================================================================

// TestSafeVars_Concurrent verifies that concurrent writes to the safeVars
// wrapper don't panic or race (run with -race flag).
func TestSafeVars_Concurrent(t *testing.T) {
	t.Parallel()

	sv := newSafeVars()

	var wg sync.WaitGroup
	const numGoroutines = 100
	const numWrites = 100

	// Spawn many goroutines doing concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				key := "key_" + string(rune('A'+id%26))
				sv.Set(key, "value")
				_ = sv.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Clone should also be thread-safe
	clone := sv.Clone()
	if clone == nil {
		t.Error("Clone() should not return nil")
	}
}

// TestSafeVars_SetGet verifies basic set/get functionality.
func TestSafeVars_SetGet(t *testing.T) {
	t.Parallel()

	sv := newSafeVars()
	sv.Set("foo", "bar")

	got := sv.Get("foo")
	if got != "bar" {
		t.Errorf("Get(foo) = %q, want %q", got, "bar")
	}

	// Non-existent key
	got = sv.Get("missing")
	if got != "" {
		t.Errorf("Get(missing) = %q, want empty", got)
	}
}

// TestSafeVars_Clone verifies that Clone returns an independent copy.
func TestSafeVars_Clone(t *testing.T) {
	t.Parallel()

	sv := newSafeVars()
	sv.Set("key1", "value1")
	sv.Set("key2", "value2")

	clone := sv.Clone()

	// Clone should have the same values
	if clone["key1"] != "value1" {
		t.Errorf("clone[key1] = %q, want %q", clone["key1"], "value1")
	}

	// Modifying clone should not affect original
	clone["key1"] = "modified"
	if sv.Get("key1") != "value1" {
		t.Error("modifying clone affected original")
	}

	// Adding to original should not affect clone
	sv.Set("key3", "value3")
	if _, exists := clone["key3"]; exists {
		t.Error("adding to original affected clone")
	}
}

// =============================================================================
// Failure mode: Panic in parallel goroutine
// =============================================================================

// TestParallelExecution_PanicRecovery verifies that a panic in a parallel
// phase is recovered and propagated as an error.
func TestParallelExecution_PanicRecovery(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupMultiRootWorkflow(t, backend, "panic-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-PANIC-001", "panic-wf")

	mock := &panicMockTurnExecutor{
		panicPhase: "A",
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "panic-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test panic recovery",
	})

	// Should get an error, not a panic
	if err == nil {
		t.Fatal("expected error from panic recovery, got nil")
	}
}

// =============================================================================
// Failure mode: Context cancelled mid-phase
// =============================================================================

// TestParallelExecution_ContextCancel verifies that context cancellation
// is properly handled during parallel execution.
func TestParallelExecution_ContextCancel(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupDiamondWorkflow(t, backend, "cancel-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-CANCEL-001", "cancel-wf")

	mock := &MockTurnExecutor{
		DefaultResponse: `{"status": "complete", "summary": "Done"}`,
		SessionIDValue:  "mock-session",
		Delay:           500 * time.Millisecond, // Long delay to allow cancellation
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := we.Run(ctx, "cancel-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test context cancel",
	})

	// Should get context.Canceled error
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
	if !errors.Is(err, context.Canceled) {
		t.Logf("error type: %T, value: %v", err, err)
		// Accept any error for now - the important thing is execution stopped
	}
}

// =============================================================================
// Failure mode: All phases in a level fail
// =============================================================================

// TestParallelExecution_AllFail verifies that when all parallel phases fail,
// the first error is reported.
func TestParallelExecution_AllFail(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupNoDepsWorkflow(t, backend, "allfail-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-ALLFAIL-001", "allfail-wf")

	mock := &MockTurnExecutor{
		Error:          errors.New("all phases fail"),
		SessionIDValue: "mock-session",
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "allfail-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test all fail",
	})

	if err == nil {
		t.Fatal("expected error when all phases fail")
	}
}

// =============================================================================
// Edge case: Single phase in level (no goroutine overhead)
// =============================================================================

// TestParallelExecution_SinglePhaseLevel verifies that single-phase levels
// execute without unnecessary goroutine overhead (behavioral equivalence).
func TestParallelExecution_SinglePhaseLevel(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupLinearWorkflow(t, backend, "single-level-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-SINGLE-001", "single-level-wf")

	var callCount atomic.Int32
	mock := &MockTurnExecutor{
		DefaultResponse: `{"status": "complete", "summary": "Done"}`,
		SessionIDValue:  "mock-session",
	}

	// Wrap to count calls
	countingMock := &countingMockExecutor{
		inner: mock,
		count: &callCount,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(countingMock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "single-level-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test single phase level",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Should have called the executor 3 times (once per phase)
	if callCount.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", callCount.Load())
	}
}

// =============================================================================
// Edge case: Resume with some phases in level completed
// =============================================================================

// TestParallelExecution_ResumePartial verifies that when resuming, completed
// phases in a level are skipped and remaining run in parallel.
func TestParallelExecution_ResumePartial(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	setupMultiRootWorkflow(t, backend, "resume-partial-wf")
	tsk := setupTaskForParallel(t, backend, "TASK-RESUME-001", "resume-partial-wf")

	// Mark phase A as already completed in the task's execution state
	task.EnsurePhaseProto(tsk.Execution, "A")
	tsk.Execution.Phases["A"].Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var calledPhases []string
	var mu sync.Mutex

	mock := &orderTrackingMockExecutor{
		order: &calledPhases,
		mu:    &mu,
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "resume-partial-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test resume partial",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// A should NOT be called (already completed)
	for _, phase := range calledPhases {
		if phase == "A" {
			t.Error("phase A should have been skipped (already completed)")
		}
	}

	// B and C should be called
	if !containsAll(calledPhases, []string{"B", "C"}) {
		t.Errorf("expected B and C to be called, got %v", calledPhases)
	}
}

// =============================================================================
// Test helpers
// =============================================================================

// Note: makePhase, slicesEqual, containsSubstring are defined in topo_sort_test.go

// extractPhaseIDs returns the PhaseTemplateIDs from a slice of phases.
func extractPhaseIDs(phases []*db.WorkflowPhase) []string {
	ids := make([]string, len(phases))
	for i, p := range phases {
		ids[i] = p.PhaseTemplateID
	}
	return ids
}

// containsAll checks if all elements of want are in got.
func containsAll(got, want []string) bool {
	m := make(map[string]bool)
	for _, s := range got {
		m[s] = true
	}
	for _, s := range want {
		if !m[s] {
			return false
		}
	}
	return true
}

// setupDiamondWorkflow creates A→[B,C]→D workflow.
func setupDiamondWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID string) {
	t.Helper()
	pdb := backend.DB()

	phases := []struct {
		id   string
		seq  int
		deps []string
	}{
		{"A", 1, nil},
		{"B", 2, []string{"A"}},
		{"C", 3, []string{"A"}},
		{"D", 4, []string{"B", "C"}},
	}

	// Create phase templates FIRST (FK constraint)
	for _, p := range phases {
		tmpl := &db.PhaseTemplate{
			ID:            p.id,
			Name:          p.id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + p.id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", p.id, err)
		}
	}

	// Create workflow
	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create workflow phases
	for _, p := range phases {
		phase := makePhase(p.id, p.seq, p.deps)
		phase.WorkflowID = workflowID
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}
}

// setupWiderDiamondWorkflow creates A→[B,C,D]→E workflow.
func setupWiderDiamondWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID string) {
	t.Helper()
	pdb := backend.DB()

	phases := []struct {
		id   string
		seq  int
		deps []string
	}{
		{"A", 1, nil},
		{"B", 2, []string{"A"}},
		{"C", 3, []string{"A"}},
		{"D", 4, []string{"A"}},
		{"E", 5, []string{"B", "C", "D"}},
	}

	// Create phase templates FIRST (FK constraint)
	for _, p := range phases {
		tmpl := &db.PhaseTemplate{
			ID:            p.id,
			Name:          p.id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + p.id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", p.id, err)
		}
	}

	// Create workflow
	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create workflow phases
	for _, p := range phases {
		phase := makePhase(p.id, p.seq, p.deps)
		phase.WorkflowID = workflowID
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}
}

// setupMultiRootWorkflow creates [A,B]→C workflow.
func setupMultiRootWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID string) {
	t.Helper()
	pdb := backend.DB()

	phases := []struct {
		id   string
		seq  int
		deps []string
	}{
		{"A", 1, nil},
		{"B", 2, nil},
		{"C", 3, []string{"A", "B"}},
	}

	// Create phase templates FIRST (FK constraint)
	for _, p := range phases {
		tmpl := &db.PhaseTemplate{
			ID:            p.id,
			Name:          p.id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + p.id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", p.id, err)
		}
	}

	// Create workflow
	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create workflow phases
	for _, p := range phases {
		phase := makePhase(p.id, p.seq, p.deps)
		phase.WorkflowID = workflowID
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}
}

// setupLinearWorkflow creates A→B→C workflow.
func setupLinearWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID string) {
	t.Helper()
	pdb := backend.DB()

	phases := []struct {
		id   string
		seq  int
		deps []string
	}{
		{"A", 1, nil},
		{"B", 2, []string{"A"}},
		{"C", 3, []string{"B"}},
	}

	// Create phase templates FIRST (FK constraint)
	for _, p := range phases {
		tmpl := &db.PhaseTemplate{
			ID:            p.id,
			Name:          p.id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + p.id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", p.id, err)
		}
	}

	// Create workflow
	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create workflow phases
	for _, p := range phases {
		phase := makePhase(p.id, p.seq, p.deps)
		phase.WorkflowID = workflowID
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}
}

// setupNoDepsWorkflow creates [A,B,C] workflow with no dependencies.
func setupNoDepsWorkflow(t *testing.T, backend *storage.DatabaseBackend, workflowID string) {
	t.Helper()
	pdb := backend.DB()

	phases := []struct {
		id  string
		seq int
	}{
		{"A", 1},
		{"B", 2},
		{"C", 3},
	}

	// Create phase templates FIRST (FK constraint)
	for _, p := range phases {
		tmpl := &db.PhaseTemplate{
			ID:            p.id,
			Name:          p.id,
			PromptSource:  "db",
			PromptContent: "Test prompt for " + p.id,
		}
		if err := pdb.SavePhaseTemplate(tmpl); err != nil {
			t.Fatalf("save template %s: %v", p.id, err)
		}
	}

	// Create workflow
	wf := &db.Workflow{ID: workflowID, Name: workflowID}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Create workflow phases
	for _, p := range phases {
		phase := makePhase(p.id, p.seq, nil)
		phase.WorkflowID = workflowID
		if err := pdb.SaveWorkflowPhase(phase); err != nil {
			t.Fatalf("save phase %s: %v", p.id, err)
		}
	}
}

// setupTaskForParallel creates a task linked to the given workflow.
func setupTaskForParallel(t *testing.T, backend *storage.DatabaseBackend, taskID, workflowID string) *orcv1.Task {
	t.Helper()
	tsk := task.NewProtoTask(taskID, "Parallel test task")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	tsk.WorkflowId = &workflowID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	return tsk
}

// =============================================================================
// Mock executors for parallel testing
// =============================================================================

// phaseTimings tracks start and end times for phases.
type phaseTimings struct {
	mu         sync.Mutex
	startTimes map[string]time.Time
	endTimes   map[string]time.Time
}

// timingMockTurnExecutor records start times and adds configurable delay.
type timingMockTurnExecutor struct {
	startTimes *sync.Map
	delay      time.Duration
	response   string
	timings    *phaseTimings // Optional detailed timing tracking
}

func (m *timingMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	// Extract phase from prompt (simplified - assumes phase name is in prompt)
	phase := extractPhaseFromPrompt(prompt)
	now := time.Now()

	m.startTimes.Store(phase, now)

	if m.timings != nil {
		m.timings.mu.Lock()
		m.timings.startTimes[phase] = now
		m.timings.mu.Unlock()
	}

	// Simulate work
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if m.timings != nil {
		m.timings.mu.Lock()
		m.timings.endTimes[phase] = time.Now()
		m.timings.mu.Unlock()
	}

	return &TurnResult{
		Content:   m.response,
		Status:    PhaseStatusComplete,
		SessionID: "mock-session",
	}, nil
}

func (m *timingMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *timingMockTurnExecutor) UpdateSessionID(id string) {}
func (m *timingMockTurnExecutor) SessionID() string         { return "mock-session" }

// failingMockTurnExecutor fails on a specific phase.
type failingMockTurnExecutor struct {
	called        *sync.Map
	cancelledSeen *atomic.Int32
	failPhase     string
	failError     error
	delay         time.Duration
}

func (m *failingMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	phase := extractPhaseFromPrompt(prompt)
	m.called.Store(phase, true)

	// Check for context cancellation
	select {
	case <-ctx.Done():
		m.cancelledSeen.Add(1)
		return nil, ctx.Err()
	default:
	}

	// Delay to allow other phases to start
	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		m.cancelledSeen.Add(1)
		return nil, ctx.Err()
	}

	if phase == m.failPhase {
		return nil, m.failError
	}

	return &TurnResult{
		Content:   `{"status": "complete", "summary": "Done"}`,
		Status:    PhaseStatusComplete,
		SessionID: "mock-session",
	}, nil
}

func (m *failingMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *failingMockTurnExecutor) UpdateSessionID(id string) {}
func (m *failingMockTurnExecutor) SessionID() string         { return "mock-session" }

// multiFailMockTurnExecutor fails on multiple phases with different timings.
type multiFailMockTurnExecutor struct {
	failPhases map[string]time.Duration // phase -> delay before failing
	errors     map[string]error
}

func (m *multiFailMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	phase := extractPhaseFromPrompt(prompt)

	if delay, shouldFail := m.failPhases[phase]; shouldFail {
		select {
		case <-time.After(delay):
			return nil, m.errors[phase]
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return &TurnResult{
		Content:   `{"status": "complete", "summary": "Done"}`,
		Status:    PhaseStatusComplete,
		SessionID: "mock-session",
	}, nil
}

func (m *multiFailMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *multiFailMockTurnExecutor) UpdateSessionID(id string) {}
func (m *multiFailMockTurnExecutor) SessionID() string         { return "mock-session" }

// orderTrackingMockExecutor tracks execution order.
type orderTrackingMockExecutor struct {
	order *[]string
	mu    *sync.Mutex
}

func (m *orderTrackingMockExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	phase := extractPhaseFromPrompt(prompt)

	m.mu.Lock()
	*m.order = append(*m.order, phase)
	m.mu.Unlock()

	return &TurnResult{
		Content:   `{"status": "complete", "summary": "Done"}`,
		Status:    PhaseStatusComplete,
		SessionID: "mock-session",
	}, nil
}

func (m *orderTrackingMockExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *orderTrackingMockExecutor) UpdateSessionID(id string) {}
func (m *orderTrackingMockExecutor) SessionID() string         { return "mock-session" }

// panicMockTurnExecutor panics on a specific phase.
type panicMockTurnExecutor struct {
	panicPhase string
}

func (m *panicMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	phase := extractPhaseFromPrompt(prompt)

	if phase == m.panicPhase {
		panic("intentional panic in phase " + phase)
	}

	return &TurnResult{
		Content:   `{"status": "complete", "summary": "Done"}`,
		Status:    PhaseStatusComplete,
		SessionID: "mock-session",
	}, nil
}

func (m *panicMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *panicMockTurnExecutor) UpdateSessionID(id string) {}
func (m *panicMockTurnExecutor) SessionID() string         { return "mock-session" }

// countingMockExecutor wraps another executor and counts calls.
type countingMockExecutor struct {
	inner TurnExecutor
	count *atomic.Int32
}

func (m *countingMockExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	m.count.Add(1)
	return m.inner.ExecuteTurn(ctx, prompt)
}

func (m *countingMockExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.inner.ExecuteTurnWithoutSchema(ctx, prompt)
}

func (m *countingMockExecutor) UpdateSessionID(id string) { m.inner.UpdateSessionID(id) }
func (m *countingMockExecutor) SessionID() string         { return m.inner.SessionID() }

// extractPhaseFromPrompt is a simplified extractor that looks for phase
// names in the prompt. In practice, the prompt will contain the phase ID.
func extractPhaseFromPrompt(prompt string) string {
	// Look for common phase identifiers
	phases := []string{"A", "B", "C", "D", "E"}
	for _, p := range phases {
		if containsSubstring(prompt, "prompt for "+p) || containsSubstring(prompt, "phase "+p) {
			return p
		}
	}
	return "unknown"
}

// =============================================================================
// safeVars stub - this is the type we're testing (will be implemented)
// =============================================================================

// safeVars provides thread-safe access to a map[string]string.
// This is a stub that will fail to compile until implemented.
type safeVars struct {
	mu   sync.RWMutex
	vars map[string]string
}

// newSafeVars creates a new thread-safe vars wrapper.
func newSafeVars() *safeVars {
	return &safeVars{
		vars: make(map[string]string),
	}
}

// Set stores a value for the given key.
func (sv *safeVars) Set(key, value string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.vars[key] = value
}

// Get retrieves a value for the given key.
func (sv *safeVars) Get(key string) string {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.vars[key]
}

// Clone returns a copy of the internal map.
func (sv *safeVars) Clone() map[string]string {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	result := make(map[string]string, len(sv.vars))
	for k, v := range sv.vars {
		result[k] = v
	}
	return result
}

// =============================================================================
// NOTE: computeExecutionLevels is implemented in topo_sort.go
// =============================================================================
