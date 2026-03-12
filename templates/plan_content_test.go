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
		"always-on cost",
		"failed to load data",
		"no data",
		"computed/live reconstruction",
		"persisted/materialized state",
		"rollout parity",
		"production transition",
		"atomicity or rollback",
		"async race handling",
		"provenance variant",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("plan.md missing planning guidance %q", required)
		}
	}
}

func TestPlanPrompt_RequiresAlternateWritersScopedCachesAndStateParity(t *testing.T) {
	t.Parallel()

	content, err := Prompts.ReadFile("prompts/plan.md")
	if err != nil {
		t.Fatalf("failed to read plan.md: %v", err)
	}

	text := string(content)
	for _, required := range []string{
		"alternate write path",
		"mirrored linkage",
		"project-scoped cache",
		"local ID alone is not sufficient",
		"source of truth",
		"distributed state parity",
		"task-linked without a workflow run",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("plan.md missing planning guidance %q", required)
		}
	}
}

func TestPlanPrompt_RequiresConcreteInventories(t *testing.T) {
	t.Parallel()

	content, err := Prompts.ReadFile("prompts/plan.md")
	if err != nil {
		t.Fatalf("failed to read plan.md: %v", err)
	}

	text := string(content)
	for _, required := range []string{
		"canonical_associations",
		"provenance_variants",
		"ui_invalidation_paths",
		"actual writers",
		"supported task/run/thread/initiative combinations",
		"browser surfaces",
		"stale-response rule",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("plan.md missing concrete inventory guidance %q", required)
		}
	}
}
