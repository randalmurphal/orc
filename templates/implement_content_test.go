package templates

import (
	"strings"
	"testing"
)

func TestImplementTemplate_ForwardLookingIntegrationCheck(t *testing.T) {
	content, err := Prompts.ReadFile("prompts/implement.md")
	if err != nil {
		t.Fatal("failed to read implement.md:", err)
	}

	text := string(content)

	if !strings.Contains(text, "2d") {
		t.Error("implement template missing subsection '2d' in Step 2 (Impact Analysis)")
	}

	if !strings.Contains(text, "Forward-Looking Integration Check") {
		t.Error("implement template missing 'Forward-Looking Integration Check' heading")
	}

	if !strings.Contains(text, "new functions") && !strings.Contains(text, "new interfaces") {
		t.Error("implement template missing text about verifying new functions/interfaces are called from production code")
	}

	if !strings.Contains(text, "wir") || !strings.Contains(text, "unused") {
		t.Error("implement template missing text about wiring in unused new code")
	}
}

func TestImplementTemplate_VerifyAndCompleteDeadCodeChecklist(t *testing.T) {
	content, err := Prompts.ReadFile("prompts/implement.md")
	if err != nil {
		t.Fatal("failed to read implement.md:", err)
	}

	text := string(content)

	// Find the Step 5: Verify and Complete section to scope our checks
	step5Idx := strings.Index(text, "## Step 5: Verify and Complete")
	if step5Idx == -1 {
		t.Fatal("implement template missing 'Step 5: Verify and Complete' section")
	}

	// Look at content from Step 5 onward until the next top-level step
	step5Section := text[step5Idx:]
	nextSection := strings.Index(step5Section[1:], "\n## Step ")
	if nextSection > 0 {
		step5Section = step5Section[:nextSection+1]
	}

	if !strings.Contains(step5Section, "new functions") || !strings.Contains(step5Section, "production code") {
		t.Error("Step 5 Verify and Complete missing checklist item about new functions being called from production code (no dead code)")
	}

	if !strings.Contains(step5Section, "new interfaces") || !strings.Contains(step5Section, "registered") || !strings.Contains(step5Section, "wired") {
		t.Error("Step 5 Verify and Complete missing checklist item about new interfaces being registered/wired into the system")
	}
}

func TestImplementPrompts_PreExistingVerificationFailuresUseSkipped(t *testing.T) {
	t.Parallel()

	files := []string{"prompts/implement.md", "prompts/implement_codex.md"}
	for _, file := range files {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			content, err := Prompts.ReadFile(file)
			if err != nil {
				t.Fatalf("failed to read %s: %v", file, err)
			}

			text := string(content)
			for _, required := range []string{
				"`SKIPPED`",
				"`pre_existing_issues`",
				"Do NOT start fixing unrelated files",
				"Mark that verification entry as `SKIPPED`, not `FAIL`",
			} {
				if !strings.Contains(text, required) {
					t.Errorf("%s missing verification guidance %q", file, required)
				}
			}
		})
	}
}

func TestImplementPrompts_RequireBrowserValidationContract(t *testing.T) {
	t.Parallel()

	files := []string{"prompts/implement.md", "prompts/implement_codex.md"}
	for _, file := range files {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			content, err := Prompts.ReadFile(file)
			if err != nil {
				t.Fatalf("failed to read %s: %v", file, err)
			}

			text := string(content)
			for _, required := range []string{
				"browser_validation",
				"browser-visible behavior",
				"backend, API",
				"advisory only",
				"live_update_surface",
				"external_mutation_validated",
				"project_scoped_surface",
				"project_isolation_validated",
				"external mutation",
				"project or tenant",
			} {
				if !strings.Contains(text, required) {
					t.Errorf("%s missing browser-validation guidance %q", file, required)
				}
			}

			for _, required := range []string{
				"repeated/shared path",
				"conditional or bounded",
				"failed to load",
				"no data",
				"computed/live",
				"persisted/materialized state",
				"rollout parity",
				"every production transition",
				"atomicity or explicit rollback",
			} {
				if !strings.Contains(text, required) {
					t.Errorf("%s missing shared-path/failure-semantics guidance %q", file, required)
				}
			}
		})
	}
}

func TestImplementPrompts_RequireAlternateWriterAndStateParityChecks(t *testing.T) {
	t.Parallel()

	files := []string{"prompts/implement.md", "prompts/implement_codex.md"}
	for _, file := range files {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			content, err := Prompts.ReadFile(file)
			if err != nil {
				t.Fatalf("failed to read %s: %v", file, err)
			}

			text := string(content)
			for _, required := range []string{
				"alternate write path",
				"mirrored linkage",
				"project-scoped cache",
				"local-ID-only",
				"source of truth",
				"distributed state parity",
				"provenance variant",
				"event-driven reload",
				"stale-response",
			} {
				if !strings.Contains(text, required) {
					t.Errorf("%s missing thoroughness guidance %q", file, required)
				}
			}
		})
	}
}

func TestImplementTemplate_BehavioralParityChecklistCoversMirrorsCachesAndDistributedState(t *testing.T) {
	content, err := Prompts.ReadFile("prompts/implement.md")
	if err != nil {
		t.Fatal("failed to read implement.md:", err)
	}

	text := string(content)
	required := []string{
		"Hidden alternate write paths",
		"Mirrored linkage or join-table updates",
		"Project-scoped cache key parity",
		"Distributed state parity",
		"provenance variant",
		"stale-response",
		"RPC-vs-event race check",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Errorf("implement.md missing behavioral parity checklist item %q", needle)
		}
	}
}
