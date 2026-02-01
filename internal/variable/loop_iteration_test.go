// Tests for TASK-710: LoopIteration in ResolutionContext.
//
// These tests define the contract for the new LoopIteration field that
// replaces ReviewRound for generic loop iteration tracking.
//
// Coverage mapping:
//
//	SC-7: TestResolutionContext_LoopIterationField
//	SC-7: TestResolver_LoopIterationVariable
package variable

import (
	"context"
	"testing"
)

// =============================================================================
// SC-7: ResolutionContext has LoopIteration field
// =============================================================================

func TestResolutionContext_LoopIterationField(t *testing.T) {
	t.Parallel()

	rctx := &ResolutionContext{
		LoopIteration: 2,
	}

	if rctx.LoopIteration != 2 {
		t.Errorf("LoopIteration = %d, want 2", rctx.LoopIteration)
	}
}

func TestResolutionContext_LoopIterationDefault(t *testing.T) {
	t.Parallel()

	rctx := &ResolutionContext{}

	// Default value should be 0 (not in a loop)
	if rctx.LoopIteration != 0 {
		t.Errorf("LoopIteration default = %d, want 0", rctx.LoopIteration)
	}
}

// =============================================================================
// SC-7: Resolver exposes LOOP_ITERATION variable
// =============================================================================

func TestResolver_LoopIterationVariable(t *testing.T) {
	t.Parallel()

	rctx := &ResolutionContext{
		LoopIteration: 3,
	}

	resolver := NewResolver(t.TempDir())
	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	// LOOP_ITERATION should be set to "3"
	if vars["LOOP_ITERATION"] != "3" {
		t.Errorf("LOOP_ITERATION = %q, want %q", vars["LOOP_ITERATION"], "3")
	}
}

func TestResolver_LoopIterationVariableZero(t *testing.T) {
	t.Parallel()

	rctx := &ResolutionContext{
		LoopIteration: 0,
	}

	resolver := NewResolver(t.TempDir())
	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	// When LoopIteration is 0, LOOP_ITERATION should be empty or "0"
	// This indicates "not in a loop" state
	if vars["LOOP_ITERATION"] != "" && vars["LOOP_ITERATION"] != "0" {
		t.Errorf("LOOP_ITERATION = %q, want empty or %q", vars["LOOP_ITERATION"], "0")
	}
}

// =============================================================================
// SC-7: LoopIteration is used for review round detection (not RetryContext)
// =============================================================================

func TestResolutionContext_ReviewRoundFromLoopIteration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		loopIteration int
		wantRound     int
	}{
		{"iteration 0 = round 1", 0, 1},
		{"iteration 1 = round 1", 1, 1},
		{"iteration 2 = round 2", 2, 2},
		{"iteration 3 = round 3", 3, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rctx := &ResolutionContext{
				LoopIteration: tt.loopIteration,
			}

			// The review round should be derived from LoopIteration
			// GetEffectiveReviewRound is the new method that prefers LoopIteration
			round := rctx.GetEffectiveReviewRound()
			if round != tt.wantRound {
				t.Errorf("GetEffectiveReviewRound() = %d, want %d", round, tt.wantRound)
			}
		})
	}
}

// =============================================================================
// SC-7: LoopIteration coexists with ReviewRound for backward compat
// =============================================================================

func TestResolutionContext_LoopIterationOverridesReviewRound(t *testing.T) {
	t.Parallel()

	// When both are set, LoopIteration should take precedence
	rctx := &ResolutionContext{
		ReviewRound:   1, // Legacy field
		LoopIteration: 2, // New field
	}

	// GetEffectiveReviewRound should prefer LoopIteration
	round := rctx.GetEffectiveReviewRound()
	if round != 2 {
		t.Errorf("GetEffectiveReviewRound() = %d, want 2 (from LoopIteration)", round)
	}
}

func TestResolutionContext_FallbackToReviewRound(t *testing.T) {
	t.Parallel()

	// When LoopIteration is 0, fall back to ReviewRound
	rctx := &ResolutionContext{
		ReviewRound:   2, // Legacy field set
		LoopIteration: 0, // Not in a loop
	}

	// GetEffectiveReviewRound should use ReviewRound when LoopIteration is 0
	round := rctx.GetEffectiveReviewRound()
	if round != 2 {
		t.Errorf("GetEffectiveReviewRound() = %d, want 2 (fallback to ReviewRound)", round)
	}
}
