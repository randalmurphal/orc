package templates_test

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/templates"
)

// readPromptTemplate reads a prompt template from the embedded FS.
func readPromptTemplate(t *testing.T, name string) string {
	t.Helper()
	content, err := templates.Prompts.ReadFile("prompts/" + name)
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	return string(content)
}

// =============================================================================
// SC-1: implement.md uses {{#if RETRY_ATTEMPT}} instead of bare {{RETRY_CONTEXT}}
// =============================================================================

func TestImplementTemplate_UsesStructuredRetryConditional(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "implement.md")

	if !strings.Contains(content, "{{#if RETRY_ATTEMPT}}") {
		t.Error("implement.md must contain {{#if RETRY_ATTEMPT}} conditional block")
	}
}

func TestImplementTemplate_NoBareRetryContext(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "implement.md")

	if strings.Contains(content, "{{RETRY_CONTEXT}}") {
		t.Error("implement.md must not contain bare {{RETRY_CONTEXT}} — use structured retry variables instead")
	}
}

func TestImplementTemplate_ContainsAllStructuredRetryVars(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "implement.md")

	retryVars := []string{"{{RETRY_ATTEMPT}}", "{{RETRY_FROM_PHASE}}", "{{RETRY_REASON}}"}
	for _, v := range retryVars {
		if !strings.Contains(content, v) {
			t.Errorf("implement.md must contain %s in retry block", v)
		}
	}
}

// =============================================================================
// SC-2: review.md uses {{#if RETRY_ATTEMPT}} instead of bare {{RETRY_CONTEXT}}
// =============================================================================

func TestReviewTemplate_UsesStructuredRetryConditional_SC2(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "review.md")

	if !strings.Contains(content, "{{#if RETRY_ATTEMPT}}") {
		t.Error("review.md must contain {{#if RETRY_ATTEMPT}} conditional block")
	}
}

func TestReviewTemplate_NoBareRetryContext_SC2(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "review.md")

	if strings.Contains(content, "{{RETRY_CONTEXT}}") {
		t.Error("review.md must not contain bare {{RETRY_CONTEXT}} — use structured retry variables instead")
	}
}

func TestReviewTemplate_ContainsAllStructuredRetryVars_SC2(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "review.md")

	retryVars := []string{"{{RETRY_ATTEMPT}}", "{{RETRY_FROM_PHASE}}", "{{RETRY_REASON}}"}
	for _, v := range retryVars {
		if !strings.Contains(content, v) {
			t.Errorf("review.md must contain %s in retry block", v)
		}
	}
}

// =============================================================================
// SC-3: Implement template retry block renders correctly with retry variables set
// =============================================================================

func TestImplementTemplate_RetryBlockRendersWithVars(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "implement.md")

	vars := variable.VariableSet{
		"RETRY_ATTEMPT":    "2",
		"RETRY_FROM_PHASE": "review",
		"RETRY_REASON":     "Missing error handling in API layer",
		"OUTPUT_REVIEW":    "Found 3 issues in error handling",
		"TASK_ID":          "TASK-099",
		"TASK_TITLE":       "Test task",
		"WEIGHT":           "medium",
		"TASK_CATEGORY":    "feature",
		"WORKTREE_PATH":    "/tmp/test",
		"TASK_BRANCH":      "orc/TASK-099",
		"TARGET_BRANCH":    "main",
	}

	rendered := variable.RenderTemplate(content, vars)

	// Retry attempt number should appear in rendered output
	if !strings.Contains(rendered, "2") {
		t.Error("rendered implement template should contain retry attempt number '2'")
	}

	// The from-phase should be rendered
	if !strings.Contains(rendered, "review") {
		t.Error("rendered implement template should contain retry from phase 'review'")
	}

	// The retry reason should be rendered
	if !strings.Contains(rendered, "Missing error handling in API layer") {
		t.Error("rendered implement template should contain retry reason")
	}

	// The retry block XML tag should be present (content inside conditional was kept)
	if !strings.Contains(rendered, "<retry_context>") {
		t.Error("rendered implement template should contain <retry_context> tag when retry is active")
	}

	// Review output should be included in the retry block
	if !strings.Contains(rendered, "Found 3 issues in error handling") {
		t.Error("rendered implement template should include OUTPUT_REVIEW content in retry block")
	}
}

// =============================================================================
// SC-4: Implement template retry block is completely stripped when no retry
// =============================================================================

func TestImplementTemplate_RetryBlockStrippedWhenNoRetry(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "implement.md")

	// No retry variables set — simulates first run
	vars := variable.VariableSet{
		"TASK_ID":       "TASK-099",
		"TASK_TITLE":    "Test task",
		"WEIGHT":        "medium",
		"TASK_CATEGORY": "feature",
		"WORKTREE_PATH": "/tmp/test",
		"TASK_BRANCH":   "orc/TASK-099",
		"TARGET_BRANCH": "main",
	}

	rendered := variable.RenderTemplate(content, vars)

	// The retry_context XML tag should NOT appear
	if strings.Contains(rendered, "<retry_context>") {
		t.Error("rendered implement template should not contain <retry_context> tag when no retry is active")
	}

	// No retry-specific headings
	if strings.Contains(rendered, "Retry Context") {
		t.Error("rendered implement template should not contain 'Retry Context' heading when no retry is active")
	}

	// No unresolved retry variable references
	for _, v := range []string{"RETRY_ATTEMPT", "RETRY_FROM_PHASE", "RETRY_REASON"} {
		if strings.Contains(rendered, "{{"+v+"}}") {
			t.Errorf("rendered implement template should not contain unresolved {{%s}}", v)
		}
	}
}

// =============================================================================
// SC-5: Review template retry block renders on re-review, stripped on first review
// =============================================================================

func TestReviewTemplate_RetryBlockRendersOnReReview(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "review.md")

	vars := variable.VariableSet{
		"RETRY_ATTEMPT":    "3",
		"RETRY_FROM_PHASE": "review",
		"RETRY_REASON":     "Dead code detected in handler.go",
		"TASK_ID":          "TASK-099",
		"TASK_TITLE":       "Test task",
		"WEIGHT":           "medium",
		"TASK_CATEGORY":    "feature",
		"WORKTREE_PATH":    "/tmp/test",
		"TASK_BRANCH":      "orc/TASK-099",
		"TARGET_BRANCH":    "main",
	}

	rendered := variable.RenderTemplate(content, vars)

	// Retry attempt number should appear
	if !strings.Contains(rendered, "3") {
		t.Error("rendered review template should contain retry attempt number '3' on re-review")
	}

	// Retry reason should be rendered
	if !strings.Contains(rendered, "Dead code detected in handler.go") {
		t.Error("rendered review template should contain retry reason on re-review")
	}

	// The retry block XML tag should be present
	if !strings.Contains(rendered, "<retry_context>") {
		t.Error("rendered review template should contain <retry_context> tag on re-review")
	}

	// Should reference re-review context
	if !strings.Contains(rendered, "Re-Review") {
		t.Error("rendered review template should contain 'Re-Review' heading on re-review")
	}
}

func TestReviewTemplate_RetryBlockStrippedOnFirstReview(t *testing.T) {
	t.Parallel()
	content := readPromptTemplate(t, "review.md")

	// No retry variables set — simulates first review
	vars := variable.VariableSet{
		"TASK_ID":       "TASK-099",
		"TASK_TITLE":    "Test task",
		"WEIGHT":        "medium",
		"TASK_CATEGORY": "feature",
		"WORKTREE_PATH": "/tmp/test",
		"TASK_BRANCH":   "orc/TASK-099",
		"TARGET_BRANCH": "main",
	}

	rendered := variable.RenderTemplate(content, vars)

	// The retry_context XML tag should NOT appear
	if strings.Contains(rendered, "<retry_context>") {
		t.Error("rendered review template should not contain <retry_context> tag on first review")
	}

	// No re-review headings
	if strings.Contains(rendered, "Re-Review") {
		t.Error("rendered review template should not contain 'Re-Review' heading on first review")
	}

	// No unresolved retry variable references
	for _, v := range []string{"RETRY_ATTEMPT", "RETRY_FROM_PHASE", "RETRY_REASON"} {
		if strings.Contains(rendered, "{{"+v+"}}") {
			t.Errorf("rendered review template should not contain unresolved {{%s}}", v)
		}
	}
}
