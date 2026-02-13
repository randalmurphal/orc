package brief_test

import (
	"context"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/variable"
)

// ============================================================================
// Integration test: PROJECT_BRIEF variable wired into resolver
//
// This tests the WIRING — that addBuiltinVariables() in the variable resolver
// includes PROJECT_BRIEF when ResolutionContext has a ProjectBrief field.
// Without this wiring, the brief generator exists but is never injected
// into phase prompts.
// ============================================================================

func TestResolver_IncludesProjectBrief(t *testing.T) {
	t.Parallel()

	resolver := variable.NewResolver("/tmp/test-project")
	rctx := &variable.ResolutionContext{
		TaskID:       "TASK-001",
		TaskTitle:    "Test task",
		ProjectBrief: "## Project Brief (auto-generated)\n\n### Decisions\n- Use JWT tokens [INIT-001]\n",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll() error: %v", err)
	}

	brief, ok := vars["PROJECT_BRIEF"]
	if !ok {
		t.Fatal("PROJECT_BRIEF variable not found in resolved variables — wiring is MISSING in addBuiltinVariables()")
	}

	if brief == "" {
		t.Error("PROJECT_BRIEF should not be empty when ResolutionContext.ProjectBrief is set")
	}

	if !strings.Contains(brief, "Decisions") {
		t.Errorf("PROJECT_BRIEF should contain brief content, got %q", brief)
	}
}

func TestResolver_ProjectBriefEmptyWhenNotSet(t *testing.T) {
	t.Parallel()

	resolver := variable.NewResolver("/tmp/test-project")
	rctx := &variable.ResolutionContext{
		TaskID:    "TASK-001",
		TaskTitle: "Test task",
		// ProjectBrief not set — fresh project
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll() error: %v", err)
	}

	brief := vars["PROJECT_BRIEF"]
	if brief != "" {
		t.Errorf("PROJECT_BRIEF should be empty when not set, got %q", brief)
	}
}

func TestResolver_ProjectBriefUsableInTemplate(t *testing.T) {
	t.Parallel()

	resolver := variable.NewResolver("/tmp/test-project")
	rctx := &variable.ResolutionContext{
		TaskID:       "TASK-001",
		TaskTitle:    "Test task",
		ProjectBrief: "### Decisions\n- Use bcrypt [INIT-001]\n",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll() error: %v", err)
	}

	// Verify it can be used in template rendering
	template := "Context:\n{{PROJECT_BRIEF}}\nTask: {{TASK_ID}}"
	result := variable.RenderTemplate(template, vars)

	if !strings.Contains(result, "Use bcrypt") {
		t.Error("rendered template should contain brief content")
	}
	if !strings.Contains(result, "TASK-001") {
		t.Error("rendered template should contain task ID")
	}

	// Verify the {{#if PROJECT_BRIEF}} conditional works
	conditionalTemplate := "{{#if PROJECT_BRIEF}}Brief:\n{{PROJECT_BRIEF}}{{/if}}"
	conditionalResult := variable.RenderTemplate(conditionalTemplate, vars)
	if !strings.Contains(conditionalResult, "Brief:") {
		t.Error("conditional should include brief when PROJECT_BRIEF is set")
	}
}

func TestResolver_ProjectBriefConditionalHiddenWhenEmpty(t *testing.T) {
	t.Parallel()

	resolver := variable.NewResolver("/tmp/test-project")
	rctx := &variable.ResolutionContext{
		TaskID:    "TASK-001",
		TaskTitle: "Test task",
		// ProjectBrief empty
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll() error: %v", err)
	}

	conditionalTemplate := "Start\n{{#if PROJECT_BRIEF}}Brief:\n{{PROJECT_BRIEF}}{{/if}}\nEnd"
	result := variable.RenderTemplate(conditionalTemplate, vars)

	if strings.Contains(result, "Brief:") {
		t.Error("conditional should hide brief section when PROJECT_BRIEF is empty")
	}
}
