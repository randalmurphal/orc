// Package variable tests for scratchpad template variable injection.
//
// TDD Tests for TASK-020: Scratchpad template variables
//
// Success Criteria Coverage:
//   - SC-4: PREV_SCRATCHPAD renders categorized markdown from prior phases
//   - SC-5: RETRY_SCRATCHPAD renders entries from the prior attempt
//
// Edge Cases:
//   - No previous entries → empty string
//   - First attempt → RETRY_SCRATCHPAD is empty
//   - First phase → PREV_SCRATCHPAD is empty
package variable

import (
	"context"
	"strings"
	"testing"
)

// ============================================================================
// SC-4: PREV_SCRATCHPAD renders categorized markdown
// ============================================================================

// TestPrevScratchpad_RendersGroupedByCategory verifies that PREV_SCRATCHPAD
// contains entries from prior phases grouped by category with markdown headings.
func TestPrevScratchpad_RendersGroupedByCategory(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:         "TASK-001",
		Phase:          "implement",
		PrevScratchpad: "## Decisions\n\n- Chose token bucket for rate limiting\n\n## Observations\n\n- Existing middleware uses chi router\n",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	prevScratchpad := vars["PREV_SCRATCHPAD"]

	// Verify it contains category headings
	if !strings.Contains(prevScratchpad, "## Decisions") {
		t.Errorf("PREV_SCRATCHPAD should contain '## Decisions' heading\ngot: %s", prevScratchpad)
	}
	if !strings.Contains(prevScratchpad, "## Observations") {
		t.Errorf("PREV_SCRATCHPAD should contain '## Observations' heading\ngot: %s", prevScratchpad)
	}

	// Verify it contains the actual entry content
	if !strings.Contains(prevScratchpad, "Chose token bucket for rate limiting") {
		t.Errorf("PREV_SCRATCHPAD should contain decision content\ngot: %s", prevScratchpad)
	}
	if !strings.Contains(prevScratchpad, "Existing middleware uses chi router") {
		t.Errorf("PREV_SCRATCHPAD should contain observation content\ngot: %s", prevScratchpad)
	}
}

// TestPrevScratchpad_EmptyWhenNoPriorEntries verifies that PREV_SCRATCHPAD
// is empty when no prior phases have scratchpad entries.
func TestPrevScratchpad_EmptyWhenNoPriorEntries(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:         "TASK-001",
		Phase:          "implement",
		PrevScratchpad: "",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	prevScratchpad := vars["PREV_SCRATCHPAD"]
	if prevScratchpad != "" {
		t.Errorf("PREV_SCRATCHPAD should be empty when no prior entries, got: %q", prevScratchpad)
	}
}

// TestPrevScratchpad_ConditionalBlockCollapse verifies that the {{#if PREV_SCRATCHPAD}}
// conditional block collapses when PREV_SCRATCHPAD is empty.
func TestPrevScratchpad_ConditionalBlockCollapse(t *testing.T) {
	t.Parallel()

	template := `Before
{{#if PREV_SCRATCHPAD}}
## Previous Phase Notes
{{PREV_SCRATCHPAD}}
{{/if}}
After`

	// With empty PREV_SCRATCHPAD
	emptyVars := VariableSet{
		"PREV_SCRATCHPAD": "",
	}
	result := RenderTemplate(template, emptyVars)
	if strings.Contains(result, "Previous Phase Notes") {
		t.Error("conditional block should collapse when PREV_SCRATCHPAD is empty")
	}
	if !strings.Contains(result, "Before") || !strings.Contains(result, "After") {
		t.Error("non-conditional content should be preserved")
	}

	// With non-empty PREV_SCRATCHPAD
	nonEmptyVars := VariableSet{
		"PREV_SCRATCHPAD": "## Decisions\n\n- Used Redis for caching\n",
	}
	result = RenderTemplate(template, nonEmptyVars)
	if !strings.Contains(result, "Previous Phase Notes") {
		t.Error("conditional block should be shown when PREV_SCRATCHPAD is non-empty")
	}
	if !strings.Contains(result, "Used Redis for caching") {
		t.Error("PREV_SCRATCHPAD content should be rendered inside the block")
	}
}

// TestFirstPhaseNoPrevScratchpad verifies that the first phase in a workflow
// has an empty PREV_SCRATCHPAD.
func TestFirstPhaseNoPrevScratchpad(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// spec is the first phase - no previous phases exist
	rctx := &ResolutionContext{
		TaskID:         "TASK-001",
		Phase:          "spec",
		PrevScratchpad: "", // No prior phases
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	if vars["PREV_SCRATCHPAD"] != "" {
		t.Errorf("PREV_SCRATCHPAD should be empty for first phase, got: %q", vars["PREV_SCRATCHPAD"])
	}
}

// ============================================================================
// SC-5: RETRY_SCRATCHPAD renders entries from prior attempt
// ============================================================================

// TestRetryScratchpad_RendersAttempt1EntriesOnAttempt2 verifies that
// RETRY_SCRATCHPAD contains the prior attempt's entries during retry.
func TestRetryScratchpad_RendersAttempt1EntriesOnAttempt2(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:          "TASK-001",
		Phase:           "implement",
		RetryAttempt:    2,
		RetryScratchpad: "## Blockers\n\n- Test framework requires Node 18+\n",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	retryScratchpad := vars["RETRY_SCRATCHPAD"]

	if !strings.Contains(retryScratchpad, "Test framework requires Node 18+") {
		t.Errorf("RETRY_SCRATCHPAD should contain blocker from attempt 1\ngot: %s", retryScratchpad)
	}
}

// TestRetryScratchpad_EmptyOnFirstAttempt verifies that RETRY_SCRATCHPAD
// is empty on the first attempt (no prior attempts to pull from).
func TestRetryScratchpad_EmptyOnFirstAttempt(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:          "TASK-001",
		Phase:           "implement",
		RetryAttempt:    0, // First attempt (not a retry)
		RetryScratchpad: "",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	retryScratchpad := vars["RETRY_SCRATCHPAD"]
	if retryScratchpad != "" {
		t.Errorf("RETRY_SCRATCHPAD should be empty on first attempt, got: %q", retryScratchpad)
	}
}

// TestRetryScratchpad_ConditionalBlockCollapse verifies that the
// {{#if RETRY_SCRATCHPAD}} conditional block collapses on first attempt.
func TestRetryScratchpad_ConditionalBlockCollapse(t *testing.T) {
	t.Parallel()

	template := `Start
{{#if RETRY_SCRATCHPAD}}
## Notes From Previous Attempt
{{RETRY_SCRATCHPAD}}
{{/if}}
End`

	// On first attempt — should collapse
	vars := VariableSet{
		"RETRY_SCRATCHPAD": "",
	}
	result := RenderTemplate(template, vars)
	if strings.Contains(result, "Notes From Previous Attempt") {
		t.Error("conditional block should collapse when RETRY_SCRATCHPAD is empty")
	}

	// On retry — should render
	vars["RETRY_SCRATCHPAD"] = "## Blockers\n\n- Could not connect to DB\n"
	result = RenderTemplate(template, vars)
	if !strings.Contains(result, "Notes From Previous Attempt") {
		t.Error("conditional block should be shown when RETRY_SCRATCHPAD is non-empty")
	}
	if !strings.Contains(result, "Could not connect to DB") {
		t.Error("RETRY_SCRATCHPAD content should be rendered inside the block")
	}
}

// TestRetryScratchpad_CoexistsWithExistingRetryVars verifies that RETRY_SCRATCHPAD
// does not interfere with existing retry variables like RETRY_ATTEMPT.
func TestRetryScratchpad_CoexistsWithExistingRetryVars(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:          "TASK-001",
		Phase:           "implement",
		RetryAttempt:    2,
		RetryFromPhase:  "implement",
		RetryReason:     "tests failed",
		RetryScratchpad: "## Blockers\n\n- Missing dependency\n",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll error: %v", err)
	}

	// Existing retry variables should still work
	if vars["RETRY_ATTEMPT"] != "2" {
		t.Errorf("RETRY_ATTEMPT = %q, want %q", vars["RETRY_ATTEMPT"], "2")
	}
	if vars["RETRY_FROM_PHASE"] != "implement" {
		t.Errorf("RETRY_FROM_PHASE = %q, want %q", vars["RETRY_FROM_PHASE"], "implement")
	}
	if vars["RETRY_REASON"] != "tests failed" {
		t.Errorf("RETRY_REASON = %q, want %q", vars["RETRY_REASON"], "tests failed")
	}

	// New scratchpad variable should also be present
	if !strings.Contains(vars["RETRY_SCRATCHPAD"], "Missing dependency") {
		t.Errorf("RETRY_SCRATCHPAD should contain blocker content\ngot: %s", vars["RETRY_SCRATCHPAD"])
	}
}
