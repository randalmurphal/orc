package prompts_test

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/templates"
)

// mustReadTemplate reads the embedded spec.md template and fails the test if it cannot be read.
func mustReadTemplate(t *testing.T) string {
	t.Helper()
	data, err := templates.Prompts.ReadFile("prompts/spec.md")
	if err != nil {
		t.Fatalf("failed to read embedded spec.md template: %v", err)
	}
	return string(data)
}

func TestSpecTemplate_IntegrationRequirementsMandatoryQuestions(t *testing.T) {
	content := mustReadTemplate(t)

	required := []string{
		"What existing code paths will call the new code",
		"Where will the new code be registered/wired",
		"What integration tests will verify the wiring",
	}

	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("Integration Requirements section missing mandatory question: %q", s)
		}
	}
}

func TestSpecTemplate_WiringChecklist(t *testing.T) {
	content := mustReadTemplate(t)

	required := []string{
		"All new functions are called from at least one production code path",
		"All new interfaces have registered implementations",
		"Integration tests verify the wiring exists",
	}

	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("Success criteria section missing wiring checklist item: %q", s)
		}
	}
}
