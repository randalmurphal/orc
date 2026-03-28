// Tests for TASK-758: Remove loadReviewContextProto hardcoding.
//
// This task removes the review-specific context loading from workflow_context.go.
// The loop system (LoopIteration) replaces the hardcoded RetryContext.FromPhase detection.
//
// SC-1: enrichContextForPhase does NOT set ReviewRound via hardcoded logic
// SC-2: Review round detection uses LoopIteration (verified via existing test SC-7)
// SC-3: Existing review tests pass after removal
package executor

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-1: enrichContextForPhase does NOT set ReviewRound via hardcoded logic
//
// After removing loadReviewContextProto, the enrichContextForPhase function
// should NOT modify rctx.ReviewRound. The loop system sets LoopIteration instead.
// =============================================================================

func TestEnrichContextForPhase_DoesNotSetReviewRoundDirectly(t *testing.T) {
	t.Parallel()

	// Setup
	backend := storage.NewTestBackend(t)
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), nil, t.TempDir())

	// Create a task with no retry context (fresh review)
	tsk := task.NewProtoTask("TASK-TEST-758", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create resolution context with ReviewRound = 0 (unset)
	rctx := &variable.ResolutionContext{
		TaskID:      tsk.Id,
		ReviewRound: 0, // Should remain 0 after enrichContextForPhase
	}

	// Call enrichContextForPhase for the review phase
	if err := we.enrichContextForPhase(rctx, "review", tsk, threadVariableUsage{}); err != nil {
		t.Fatalf("enrichContextForPhase() error = %v", err)
	}

	// SC-1: ReviewRound should NOT be set by enrichContextForPhase
	// The hardcoded loadReviewContextProto sets it to 1, so this fails until removal
	if rctx.ReviewRound != 0 {
		t.Errorf("enrichContextForPhase set ReviewRound = %d; want 0 (loop system should handle this)",
			rctx.ReviewRound)
	}
}

// =============================================================================
// SC-1b: enrichContextForPhase does NOT set ReviewFindings via hardcoded logic
//
// The loop system's output_transform handles REVIEW_FINDINGS instead.
// =============================================================================

func TestEnrichContextForPhase_DoesNotSetReviewFindingsDirectly(t *testing.T) {
	t.Parallel()

	// Setup
	backend := storage.NewTestBackend(t)
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), nil, t.TempDir())

	// Create a task with retry state that would trigger round 2 in old code
	tsk := task.NewProtoTask("TASK-TEST-758-2", "Test task with retry")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING

	// Set retry state that old loadReviewContextProto would parse
	// Signature: SetRetryState(t, fromPhase, toPhase, reason, failureOutput, attempt)
	failureOutput := `{"status":"needs_changes","round":1,"summary":"Issues found","issues":[{"severity":"high","description":"Bug"}]}`
	task.SetRetryState(tsk, "review", "implement", "needs changes", failureOutput, 1)

	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create resolution context with empty ReviewFindings
	rctx := &variable.ResolutionContext{
		TaskID:         tsk.Id,
		ReviewRound:    0,  // Should remain 0
		ReviewFindings: "", // Should remain empty
	}

	// Call enrichContextForPhase for the review phase
	if err := we.enrichContextForPhase(rctx, "review", tsk, threadVariableUsage{}); err != nil {
		t.Fatalf("enrichContextForPhase() error = %v", err)
	}

	// SC-1: ReviewRound should NOT be set by enrichContextForPhase
	if rctx.ReviewRound != 0 {
		t.Errorf("enrichContextForPhase set ReviewRound = %d; want 0", rctx.ReviewRound)
	}

	// SC-1b: ReviewFindings should NOT be set by enrichContextForPhase
	// The loop system's output_transform handles this instead
	if rctx.ReviewFindings != "" {
		t.Errorf("enrichContextForPhase set ReviewFindings = %q; want empty", rctx.ReviewFindings)
	}
}
