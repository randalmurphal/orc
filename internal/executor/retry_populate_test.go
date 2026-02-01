package executor

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestPopulateRetryFields_FromProto(t *testing.T) {
	t.Parallel()

	rctx := &variable.ResolutionContext{}

	e := &orcv1.ExecutionState{
		RetryContext: &orcv1.RetryContext{
			FromPhase: "review",
			Reason:    "Gate rejected: code quality issues",
			Attempt:   3,
		},
	}

	PopulateRetryFields(rctx, e)

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

func TestPopulateRetryFields_NilExecution(t *testing.T) {
	t.Parallel()

	rctx := &variable.ResolutionContext{}

	// Should not panic with nil execution state
	PopulateRetryFields(rctx, nil)

	if rctx.RetryAttempt != 0 {
		t.Errorf("RetryAttempt should be 0 with nil execution, got %d", rctx.RetryAttempt)
	}
	if rctx.RetryFromPhase != "" {
		t.Errorf("RetryFromPhase should be empty with nil execution, got %q", rctx.RetryFromPhase)
	}
	if rctx.RetryReason != "" {
		t.Errorf("RetryReason should be empty with nil execution, got %q", rctx.RetryReason)
	}
}

func TestPopulateRetryFields_NilRetryContext(t *testing.T) {
	t.Parallel()

	rctx := &variable.ResolutionContext{}

	e := &orcv1.ExecutionState{
		RetryContext: nil,
	}

	PopulateRetryFields(rctx, e)

	if rctx.RetryAttempt != 0 {
		t.Errorf("RetryAttempt should be 0 with nil retry context, got %d", rctx.RetryAttempt)
	}
}
