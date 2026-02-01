package variable

import (
	"context"
	"reflect"
	"testing"
)

// TestResolveAll_RetryContextVariableRemoved verifies that RETRY_CONTEXT is NOT
// present in the resolved variable set. The pre-formatted RETRY_CONTEXT string
// has been replaced by structured variables (RETRY_ATTEMPT, RETRY_FROM_PHASE,
// RETRY_REASON) which templates consume directly.
//
// This test FAILS if resolver.go still sets vars["RETRY_CONTEXT"].
func TestResolveAll_RetryContextVariableRemoved(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:         "TASK-001",
		Phase:          "implement",
		RetryAttempt:   2,
		RetryFromPhase: "review",
		RetryReason:    "Gate rejected: 3 issues found",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// RETRY_CONTEXT must NOT be in the variable set.
	// Structured retry variables replace it entirely.
	if _, ok := vars["RETRY_CONTEXT"]; ok {
		t.Error("RETRY_CONTEXT variable must not be set — use structured retry variables instead")
	}

	// Structured retry variables must still resolve correctly.
	if vars["RETRY_ATTEMPT"] != "2" {
		t.Errorf("RETRY_ATTEMPT: want %q, got %q", "2", vars["RETRY_ATTEMPT"])
	}
	if vars["RETRY_FROM_PHASE"] != "review" {
		t.Errorf("RETRY_FROM_PHASE: want %q, got %q", "review", vars["RETRY_FROM_PHASE"])
	}
	if vars["RETRY_REASON"] != "Gate rejected: 3 issues found" {
		t.Errorf("RETRY_REASON: want %q, got %q", "Gate rejected: 3 issues found", vars["RETRY_REASON"])
	}
}

// TestResolutionContext_RetryContextFieldRemoved verifies that the ResolutionContext
// struct no longer has a RetryContext field. The field was the carrier for the
// pre-formatted retry string; with structured variables it serves no purpose.
//
// This test FAILS if types.go still declares RetryContext on ResolutionContext.
func TestResolutionContext_RetryContextFieldRemoved(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(ResolutionContext{})

	_, found := typ.FieldByName("RetryContext")
	if found {
		t.Error("ResolutionContext must not have a RetryContext field — use RetryAttempt, RetryFromPhase, RetryReason instead")
	}

	// The structured fields must still exist.
	for _, name := range []string{"RetryAttempt", "RetryFromPhase", "RetryReason"} {
		if _, ok := typ.FieldByName(name); !ok {
			t.Errorf("ResolutionContext must retain structured field %q", name)
		}
	}
}
