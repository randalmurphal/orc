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

func TestImplementTemplate_SelfReviewDeadCodeChecklist(t *testing.T) {
	content, err := Prompts.ReadFile("prompts/implement.md")
	if err != nil {
		t.Fatal("failed to read implement.md:", err)
	}

	text := string(content)

	// Find the Step 7 Self-Review section to scope our checks
	step7Idx := strings.Index(text, "## Step 7: Self-Review")
	if step7Idx == -1 {
		t.Fatal("implement template missing 'Step 7: Self-Review' section")
	}

	// Look at content from Step 7 onward until the next major section
	step7Section := text[step7Idx:]
	nextSection := strings.Index(step7Section[1:], "\n## ")
	if nextSection > 0 {
		step7Section = step7Section[:nextSection+1]
	}

	if !strings.Contains(step7Section, "new functions") || !strings.Contains(step7Section, "production code") {
		t.Error("Step 7 Self-Review missing checklist item about new functions being called from production code (no dead code)")
	}

	if !strings.Contains(step7Section, "new interfaces") || !strings.Contains(step7Section, "registered") || !strings.Contains(step7Section, "wired") {
		t.Error("Step 7 Self-Review missing checklist item about new interfaces being registered/wired into the system")
	}
}
