// Package executor provides task phase execution for orc.
package executor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ChecklistItem represents a single item in the quality checklist.
type ChecklistItem struct {
	ID     string `json:"id"`
	Check  string `json:"check"`
	Passed bool   `json:"passed"`
}

// SpecCompletionWithChecklist represents the spec phase output with quality checklist.
type SpecCompletionWithChecklist struct {
	Status           string          `json:"status"`
	Summary          string          `json:"summary"`
	Content          string          `json:"content"`
	QualityChecklist []ChecklistItem `json:"quality_checklist"`
	Assumptions      []Assumption    `json:"assumptions"`
}

// Assumption represents an assumption made during spec creation.
type Assumption struct {
	Area       string `json:"area"`
	Assumption string `json:"assumption"`
	Rationale  string `json:"rationale"`
}

// requiredChecks defines which checklist items must pass for the spec to be valid.
// If any of these fail, the spec phase will retry with feedback.
var requiredChecks = map[string]bool{
	"all_criteria_verifiable": true,
	"no_existence_only_criteria": true,
	"p1_stories_independent":  true,
	"scope_explicit":          true,
	"max_3_clarifications":    true,
}

// ValidateSpecChecklist validates the quality checklist from spec output.
// Returns (passed, failures) where passed is true if all required checks pass,
// and failures contains the list of failed required checks.
func ValidateSpecChecklist(checklist []ChecklistItem) (bool, []ChecklistItem) {
	var failures []ChecklistItem
	for _, item := range checklist {
		if requiredChecks[item.ID] && !item.Passed {
			failures = append(failures, item)
		}
	}
	return len(failures) == 0, failures
}

// ExtractChecklistFromOutput parses the spec JSON output and extracts the quality checklist.
// Returns nil if the output doesn't contain a valid checklist.
func ExtractChecklistFromOutput(output string) []ChecklistItem {
	// Try to parse as JSON
	var completion SpecCompletionWithChecklist
	if err := json.Unmarshal([]byte(output), &completion); err != nil {
		return nil
	}
	return completion.QualityChecklist
}

// FormatChecklistFeedback creates a feedback message for failed checklist items.
// This message is injected into the retry context for the spec phase.
func FormatChecklistFeedback(failures []ChecklistItem) string {
	if len(failures) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Quality Checklist Failures\n\n")
	sb.WriteString("The following required quality checks did not pass:\n\n")

	for _, item := range failures {
		fmt.Fprintf(&sb, "- **%s**: %s\n", item.ID, item.Check)
	}

	sb.WriteString("\n### How to Fix\n\n")
	for _, item := range failures {
		switch item.ID {
		case "all_criteria_verifiable":
			sb.WriteString("- **all_criteria_verifiable**: Every success criterion must have an executable verification method (command, test, file check). No vague criteria like \"code is clean\" - use `npm run lint` exit 0.\n")
		case "no_existence_only_criteria":
			sb.WriteString("- **no_existence_only_criteria**: Success criteria must verify behavior, not just existence. Instead of \"file exists on disk\" or \"record created in DB\", require behavioral verification like \"script blocks first stop attempt (exit 2)\" or \"function returns error on invalid input\".\n")
		case "p1_stories_independent":
			sb.WriteString("- **p1_stories_independent**: Each P1 (MVP) user story must be completable and testable in isolation. If a story can't ship alone, break it down further.\n")
		case "scope_explicit":
			sb.WriteString("- **scope_explicit**: Both \"In Scope\" and \"Out of Scope\" sections must be present with concrete items. This prevents scope creep.\n")
		case "max_3_clarifications":
			sb.WriteString("- **max_3_clarifications**: Maximum 3 [NEEDS CLARIFICATION] items allowed. For everything else, make an informed assumption and document it in the Assumptions section.\n")
		}
	}

	sb.WriteString("\nPlease revise the spec to address these issues.")
	return sb.String()
}

// ParseSpecWithChecklist parses spec output and returns the structured data.
// Returns an error if parsing fails.
func ParseSpecWithChecklist(output string) (*SpecCompletionWithChecklist, error) {
	var completion SpecCompletionWithChecklist
	if err := json.Unmarshal([]byte(output), &completion); err != nil {
		return nil, fmt.Errorf("parse spec output: %w", err)
	}
	return &completion, nil
}
