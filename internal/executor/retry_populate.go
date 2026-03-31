package executor

import (
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// PopulateRetryFields extracts structured retry fields from the task's metadata
// into the ResolutionContext. This enables templates to use individual retry variables
// (RETRY_ATTEMPT, RETRY_FROM_PHASE, RETRY_REASON, RETRY_FEEDBACK) instead of a single
// formatted string.
//
// Retry state is now stored in task metadata (via task.GetRetryState) rather than
// the removed ExecutionState.RetryContext proto field (DEC-005).
func PopulateRetryFields(rctx *variable.ResolutionContext, t *orcv1.Task) {
	if t == nil {
		return
	}

	rs := task.GetRetryState(t)
	if rs == nil {
		return
	}

	rctx.RetryAttempt = int(rs.Attempt)
	rctx.RetryFromPhase = rs.FromPhase
	rctx.RetryReason = rs.Reason
	rctx.RetryFeedback = rs.FailureOutput
}
