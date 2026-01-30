package templates_test

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/templates"
)

// readReviewTemplate reads a review template from the embedded FS.
// Fails the test if the template cannot be read.
func readReviewTemplate(t *testing.T, name string) string {
	t.Helper()
	content, err := templates.Prompts.ReadFile("prompts/" + name)
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	return string(content)
}

// --- SC-1: All three review templates contain zero JSON code blocks ---

func TestReviewTemplate_NoJSONCodeBlocks(t *testing.T) {
	t.Parallel()
	templateFiles := []string{"review.md", "review_round1.md", "review_round2.md"}
	for _, file := range templateFiles {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			content := readReviewTemplate(t, file)
			count := strings.Count(content, "```json")
			if count != 0 {
				t.Errorf("%s contains %d JSON code blocks (```json), want 0", file, count)
			}
		})
	}
}

// --- SC-2: No "Bias toward Outcome 1" or bias-favoring instruction ---

func TestReviewTemplate_NoBiasInstruction(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	if strings.Contains(strings.ToLower(content), "bias toward outcome") {
		t.Error("review.md contains 'Bias toward Outcome' instruction, which should be removed")
	}
	if strings.Contains(strings.ToLower(content), "bias toward outcome 1") {
		t.Error("review.md contains 'Bias toward Outcome 1' instruction")
	}
}

func TestReviewTemplate_HasNeutralSeverityGuidance(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	// Must contain severity-based decision guidance instead of bias
	if !strings.Contains(content, "severity of findings") {
		t.Error("review.md should contain neutral severity-based guidance (expected 'severity of findings')")
	}
}

// --- SC-3: review.md contains Integration Completeness section with 4 checklist items ---

func TestReviewTemplate_IntegrationCompletenessSection(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	if !strings.Contains(content, "Integration Completeness") {
		t.Fatal("review.md missing 'Integration Completeness' section")
	}

	// 4 required checklist items
	checklistItems := []struct {
		keyword     string
		description string
	}{
		{"called from", "functions called from production code path"},
		{"never-called", "no defined-but-never-called functions (dead code)"},
		{"interfaces", "new interfaces wired into the system"},
		{"registered", "hooks/callbacks/triggers registered"},
	}

	for _, item := range checklistItems {
		t.Run(item.description, func(t *testing.T) {
			if !strings.Contains(strings.ToLower(content), item.keyword) {
				t.Errorf("review.md Integration Completeness missing checklist item: %s (looking for keyword %q)",
					item.description, item.keyword)
			}
		})
	}
}

// --- SC-4: review_round1.md contains Integration Completeness checks ---

func TestReviewRound1_IntegrationCompletenessChecks(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round1.md")

	if !strings.Contains(content, "Integration Completeness") {
		t.Fatal("review_round1.md missing 'Integration Completeness' checks")
	}

	// Should have checklist items for integration checks
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "dead code") {
		t.Error("review_round1.md Integration Completeness should reference dead code")
	}
	if !strings.Contains(lower, "called from") || !strings.Contains(lower, "production") {
		t.Error("review_round1.md Integration Completeness should check functions called from production code")
	}
}

// --- SC-5: Decision Guide maps high-severity to blocked outcome ---

func TestReviewTemplate_DecisionGuideMapping(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	if !strings.Contains(content, "Decision Guide") {
		t.Fatal("review.md missing 'Decision Guide' section")
	}

	// Extract Decision Guide section (everything after "Decision Guide" heading)
	idx := strings.Index(content, "Decision Guide")
	if idx == -1 {
		t.Fatal("could not find Decision Guide section")
	}
	guideSection := content[idx:]

	// High-severity should map to blocked (Outcome 2 or 3)
	if !strings.Contains(guideSection, "high") {
		t.Error("Decision Guide should reference high-severity findings")
	}

	// Must mention dead code and missing integration as examples of high-severity
	lower := strings.ToLower(guideSection)
	if !strings.Contains(lower, "dead code") {
		t.Error("Decision Guide should mention dead code as high-severity")
	}
	if !strings.Contains(lower, "missing integration") {
		t.Error("Decision Guide should mention missing integration as high-severity")
	}

	// High-severity should lead to block outcome (2 or 3)
	if !strings.Contains(lower, "block") {
		t.Error("Decision Guide should map high-severity to blocked outcome")
	}

	// No issues should map to pass (Outcome 1)
	if !strings.Contains(lower, "outcome 1") {
		t.Error("Decision Guide should have Outcome 1 (pass) path")
	}
}

// --- SC-6: review_round1.md classifies dead code and missing integration as high-severity ---

func TestReviewRound1_HighSeverityClassification(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round1.md")
	lower := strings.ToLower(content)

	// Both "dead code" and "missing integration" should appear near "high" severity
	if !strings.Contains(lower, "dead code") {
		t.Error("review_round1.md should classify 'dead code' in severity definitions")
	}
	if !strings.Contains(lower, "missing integration") {
		t.Error("review_round1.md should classify 'missing integration' in severity definitions")
	}

	// Verify they're associated with high severity.
	// Find the section containing severity definitions and check both terms
	// appear in context with "high"
	severityTerms := []string{"dead code", "missing integration"}
	for _, term := range severityTerms {
		termIdx := strings.Index(lower, term)
		if termIdx == -1 {
			continue // already reported above
		}
		// Check within a reasonable window (500 chars before/after) for "high"
		start := max(termIdx-500, 0)
		end := min(termIdx+500, len(lower))
		window := lower[start:end]
		if !strings.Contains(window, "high") {
			t.Errorf("%q should be associated with high severity in review_round1.md", term)
		}
	}
}

// --- SC-7: Outcomes use natural language, not JSON syntax ---

func TestReviewTemplate_OutcomesUseNaturalLanguage(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	// Find each Outcome section and verify no JSON syntax
	outcomes := []string{"Outcome 1", "Outcome 2", "Outcome 3"}
	for _, outcome := range outcomes {
		t.Run(outcome, func(t *testing.T) {
			idx := strings.Index(content, outcome)
			if idx == -1 {
				t.Fatalf("review.md missing %s", outcome)
			}

			// Get text from this outcome until next outcome or end
			start := idx
			end := len(content)
			for _, nextOutcome := range outcomes {
				if nextOutcome == outcome {
					continue
				}
				nextIdx := strings.Index(content[start+len(outcome):], nextOutcome)
				if nextIdx != -1 {
					nextAbsIdx := start + len(outcome) + nextIdx
					if nextAbsIdx < end {
						end = nextAbsIdx
					}
				}
			}
			section := content[start:end]

			// No JSON code blocks in outcome section
			if strings.Contains(section, "```json") {
				t.Errorf("%s section contains JSON code block", outcome)
			}

			// No raw JSON object patterns like {"status": or "status":
			if strings.Contains(section, `"status":`) {
				t.Errorf("%s section contains JSON-style field syntax (\"status\":)", outcome)
			}
		})
	}
}

// --- Preservation: 3-outcome structure ---

func TestReviewTemplate_ThreeOutcomesPreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	for _, outcome := range []string{"Outcome 1", "Outcome 2", "Outcome 3"} {
		if !strings.Contains(content, outcome) {
			t.Errorf("review.md must preserve %s (executor depends on 3-outcome structure)", outcome)
		}
	}
}

// --- Preservation: Template variable placeholders ---

func TestReviewRound1_TemplatePlaceholdersPreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round1.md")

	requiredVars := []string{"SPEC_CONTENT"}
	for _, v := range requiredVars {
		if !strings.Contains(content, "{{"+v+"}}") {
			t.Errorf("review_round1.md must preserve {{%s}} placeholder", v)
		}
	}
}

func TestReviewRound2_TemplatePlaceholdersPreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round2.md")

	requiredVars := []string{"REVIEW_FINDINGS"}
	for _, v := range requiredVars {
		if !strings.Contains(content, "{{"+v+"}}") {
			t.Errorf("review_round2.md must preserve {{%s}} placeholder", v)
		}
	}
}

// --- Preservation: Constitution violation handling ---

func TestReviewRound1_ConstitutionHandlingPreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round1.md")

	if !strings.Contains(content, "constitution_violation") {
		t.Error("review_round1.md must preserve constitution_violation handling")
	}
}

func TestReviewRound2_ConstitutionHandlingPreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round2.md")

	if !strings.Contains(content, "constitution_violation") {
		t.Error("review_round2.md must preserve constitution_violation handling")
	}
}

// --- Preservation: Severity levels in round1 ---

func TestReviewRound1_SeverityLevelsPreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review_round1.md")
	lower := strings.ToLower(content)

	for _, severity := range []string{"critical", "high", "medium", "low"} {
		if !strings.Contains(lower, severity) {
			t.Errorf("review_round1.md must preserve %s severity level", severity)
		}
	}
}

// --- Preservation: Reviewer can fix small bugs in Outcome 1 ---

func TestReviewTemplate_FixInPlacePreserved(t *testing.T) {
	t.Parallel()
	content := readReviewTemplate(t, "review.md")

	// Outcome 1 should mention ability to fix small issues
	idx := strings.Index(content, "Outcome 1")
	if idx == -1 {
		t.Fatal("review.md missing Outcome 1")
	}
	// Check from Outcome 1 to Outcome 2
	end := strings.Index(content[idx:], "Outcome 2")
	if end == -1 {
		end = len(content) - idx
	}
	outcome1Section := strings.ToLower(content[idx : idx+end])

	if !strings.Contains(outcome1Section, "fix") {
		t.Error("Outcome 1 should preserve reviewer's ability to fix small issues")
	}
}
