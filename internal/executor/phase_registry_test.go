// Tests for TASK-004: Phase type registry that maps type strings to executor implementations.
//
// Coverage mapping:
//   SC-1: TestPhaseTypeRegistry_RegisterAndLookup, TestPhaseTypeRegistry_UnknownTypeError
//   SC-1: TestPhaseTypeRegistry_LLMDefault, TestPhaseTypeRegistry_KnowledgeRegistered
//   SC-2: TestPhaseTypeRegistry_EmptyTypeDefaultsToLLM
//
// Failure modes:
//   TestPhaseTypeRegistry_UnknownTypeError - unknown type returns descriptive error
//   TestPhaseTypeRegistry_NilRegistration - registering nil executor is rejected
package executor

import (
	"context"
	"fmt"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// =============================================================================
// SC-1: Registry returns correct executor for "llm" and "knowledge" types
// =============================================================================

// TestPhaseTypeRegistry_RegisterAndLookup verifies that registered executors
// can be retrieved by their type string.
func TestPhaseTypeRegistry_RegisterAndLookup(t *testing.T) {
	t.Parallel()

	registry := NewPhaseTypeRegistry()

	mock := &mockPhaseTypeExecutor{name: "test-executor"}
	registry.Register("test", mock)

	got, err := registry.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil executor")
	}
	if got.Name() != "test-executor" {
		t.Errorf("Name() = %q, want %q", got.Name(), "test-executor")
	}
}

// TestPhaseTypeRegistry_LLMDefault verifies the registry has an "llm"
// executor registered (this should be set up during initialization).
func TestPhaseTypeRegistry_LLMDefault(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	got, err := registry.Get("llm")
	if err != nil {
		t.Fatalf("'llm' type should be registered by default: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil executor for 'llm' type")
	}
}

// TestPhaseTypeRegistry_KnowledgeRegistered verifies the registry has a
// "knowledge" executor registered.
func TestPhaseTypeRegistry_KnowledgeRegistered(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	got, err := registry.Get("knowledge")
	if err != nil {
		t.Fatalf("'knowledge' type should be registered by default: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil executor for 'knowledge' type")
	}
}

// =============================================================================
// SC-1 (error path): Unknown type returns error (not nil executor)
// =============================================================================

func TestPhaseTypeRegistry_UnknownTypeError(t *testing.T) {
	t.Parallel()

	registry := NewPhaseTypeRegistry()

	got, err := registry.Get("nonexistent_xyz")
	if err == nil {
		t.Fatal("expected error for unknown type, got nil")
	}
	if got != nil {
		t.Error("expected nil executor for unknown type")
	}
	// Error message should include the unknown type for debugging
	errStr := err.Error()
	if !containsSubstring(errStr, "nonexistent_xyz") {
		t.Errorf("error message should include unknown type, got: %q", errStr)
	}
}

// =============================================================================
// SC-2: Empty type field defaults to "llm"
// =============================================================================

func TestPhaseTypeRegistry_EmptyTypeDefaultsToLLM(t *testing.T) {
	t.Parallel()

	registry := NewDefaultPhaseTypeRegistry()

	// Empty string should resolve to "llm"
	got, err := registry.Get("")
	if err != nil {
		t.Fatalf("empty type should default to 'llm': %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil executor for empty type")
	}
}

// =============================================================================
// Edge case: Registering nil executor is rejected
// =============================================================================

func TestPhaseTypeRegistry_NilRegistration(t *testing.T) {
	t.Parallel()

	registry := NewPhaseTypeRegistry()

	// Should panic or error when registering nil
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when registering nil executor")
		}
	}()
	registry.Register("bad", nil)
}

// =============================================================================
// Helper: mock phase type executor
// =============================================================================

// mockPhaseTypeExecutor is a minimal PhaseTypeExecutor for registry tests.
type mockPhaseTypeExecutor struct {
	name string
}

func (m *mockPhaseTypeExecutor) ExecutePhase(
	ctx context.Context,
	params PhaseTypeParams,
) (PhaseResult, error) {
	return PhaseResult{
		PhaseID: params.PhaseTemplate.ID,
		Status:  orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String(),
	}, nil
}

func (m *mockPhaseTypeExecutor) Name() string {
	return m.name
}

// Suppress unused import warning
var _ = fmt.Sprintf
