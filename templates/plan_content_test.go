package templates

import (
	"strings"
	"testing"
)

func TestPlanPrompt_RequiresEventDrivenAndProjectScopedChecks(t *testing.T) {
	t.Parallel()

	content, err := Prompts.ReadFile("prompts/plan.md")
	if err != nil {
		t.Fatalf("failed to read plan.md: %v", err)
	}

	text := string(content)
	for _, required := range []string{
		"external-update behavior",
		"project or tenant isolation",
		"event-driven",
		"multi-project",
		"`verification_plan.e2e`",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("plan.md missing planning guidance %q", required)
		}
	}
}
