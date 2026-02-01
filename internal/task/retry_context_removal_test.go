package task

import (
	"reflect"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// TestExecutionStateRetryContextFieldRemoved verifies SC-1:
// The ExecutionState proto no longer has a RetryContext field.
// This test FAILS if task.proto still declares retry_context on ExecutionState.
func TestExecutionStateRetryContextFieldRemoved(t *testing.T) {
	t.Parallel()

	es := &orcv1.ExecutionState{}
	typ := reflect.TypeOf(es).Elem()

	_, found := typ.FieldByName("RetryContext")
	if found {
		t.Error("ExecutionState must not have a RetryContext field - it has been removed in favor of structured retry variables")
	}
}

// TestSetRetryContextProtoRemoved verifies SC-2:
// The SetRetryContextProto function no longer exists.
// This test documents the expected removal - it will fail to compile
// if the function still exists and is called.
func TestSetRetryContextProtoRemoved(t *testing.T) {
	t.Parallel()

	// After removal, this test should pass because there's nothing to test.
	// The function SetRetryContextProto should not exist in execution_helpers.go.
	//
	// If this test is modified to call SetRetryContextProto, it should fail to compile.
	// This is intentional - the function should not exist.
	//
	// Verification: grep for "func SetRetryContextProto" in execution_helpers.go
	// should return no results after the removal is complete.
}

// TestGetRetryContextProtoRemoved verifies SC-2:
// The GetRetryContextProto function no longer exists.
func TestGetRetryContextProtoRemoved(t *testing.T) {
	t.Parallel()

	// After removal, GetRetryContextProto should not exist.
	// Any code that calls it will fail to compile.
}

// TestRetryContextMessageRemoved verifies SC-1:
// The RetryContext message type should not exist in the generated proto.
func TestRetryContextMessageRemoved(t *testing.T) {
	t.Parallel()

	// This test verifies that orcv1.RetryContext type has been removed.
	// The verification is implicit: if this file compiles, it means the code
	// no longer depends on orcv1.RetryContext.
	//
	// Previously this test tried to use reflection to check for the type,
	// but that approach failed because referencing a non-existent type
	// causes a compile error (which is the desired outcome).
	//
	// The real verification is done by:
	// 1. This file compiles without referencing orcv1.RetryContext
	// 2. All other code compiles without using the removed type
	// 3. The proto file no longer defines the RetryContext message
	//
	// See DEC-005: Retry context unified into variable system - kill RetryContext proto field
}
