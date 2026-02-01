package executor

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestPopulateRetryFields_FromTaskMetadata(t *testing.T) {
	t.Parallel()

	rctx := &variable.ResolutionContext{}

	// Create task with retry state in metadata
	tsk := &orcv1.Task{
		Id: "TASK-001",
	}
	task.SetRetryState(tsk, "review", "implement", "Gate rejected: code quality issues", "", 3)

	PopulateRetryFields(rctx, tsk)

	if rctx.RetryAttempt != 3 {
		t.Errorf("RetryAttempt: expected 3, got %d", rctx.RetryAttempt)
	}
	if rctx.RetryFromPhase != "review" {
		t.Errorf("RetryFromPhase: expected %q, got %q", "review", rctx.RetryFromPhase)
	}
	if rctx.RetryReason != "Gate rejected: code quality issues" {
		t.Errorf("RetryReason: expected %q, got %q", "Gate rejected: code quality issues", rctx.RetryReason)
	}
}

func TestPopulateRetryFields_NilTask(t *testing.T) {
	t.Parallel()

	rctx := &variable.ResolutionContext{}

	// Should not panic with nil task
	PopulateRetryFields(rctx, nil)

	if rctx.RetryAttempt != 0 {
		t.Errorf("RetryAttempt should be 0 with nil task, got %d", rctx.RetryAttempt)
	}
	if rctx.RetryFromPhase != "" {
		t.Errorf("RetryFromPhase should be empty with nil task, got %q", rctx.RetryFromPhase)
	}
	if rctx.RetryReason != "" {
		t.Errorf("RetryReason should be empty with nil task, got %q", rctx.RetryReason)
	}
}

func TestPopulateRetryFields_NoRetryState(t *testing.T) {
	t.Parallel()

	rctx := &variable.ResolutionContext{}

	// Task without any retry state set
	tsk := &orcv1.Task{
		Id: "TASK-002",
	}

	PopulateRetryFields(rctx, tsk)

	if rctx.RetryAttempt != 0 {
		t.Errorf("RetryAttempt should be 0 with no retry state, got %d", rctx.RetryAttempt)
	}
}
