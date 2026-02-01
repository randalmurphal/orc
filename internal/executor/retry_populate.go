package executor

import (
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/variable"
)

// PopulateRetryFields extracts structured retry fields from the proto ExecutionState
// into the ResolutionContext. This enables templates to use individual retry variables
// (RETRY_ATTEMPT, RETRY_FROM_PHASE, RETRY_REASON) instead of a single formatted string.
func PopulateRetryFields(rctx *variable.ResolutionContext, e *orcv1.ExecutionState) {
	if e == nil || e.RetryContext == nil {
		return
	}

	rc := e.RetryContext
	rctx.RetryAttempt = int(rc.Attempt)
	rctx.RetryFromPhase = rc.FromPhase
	rctx.RetryReason = rc.Reason
}
