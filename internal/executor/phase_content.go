// Package executor provides task phase execution for orc.
package executor

import (
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/storage"
)

// inferOutputVarName returns the standard output variable name for a phase ID.
func inferOutputVarName(phaseID string) string {
	switch phaseID {
	case "spec", "tiny_spec":
		return "SPEC_CONTENT"
	case "tdd_write":
		return "TDD_TESTS_CONTENT"
	case "breakdown":
		return "BREAKDOWN_CONTENT"
	case "research":
		return "RESEARCH_CONTENT"
	case "docs":
		return "DOCS_CONTENT"
	default:
		return "OUTPUT_" + strings.ToUpper(strings.ReplaceAll(phaseID, "-", "_"))
	}
}

// ExtractPhaseContent extracts content from JSON output.
// This is the only mechanism for capturing phase content - no file lookups or XML parsing.
func ExtractPhaseContent(output string) string {
	return ExtractContentFromOutput(output)
}


// SaveSpecToDatabase saves spec content to the database for the spec phase.
// This is the SOLE mechanism for saving spec content - no files are created
// for the spec phase to avoid merge conflicts in worktrees. The database is the source
// of truth for spec content, which is loaded via backend.LoadSpec() to populate
// {{SPEC_CONTENT}} in subsequent phase templates.
//
// Spec content is extracted from the JSON "content" field in the agent's output.
// The --json-schema constraint ensures reliable structured output.
//
// This should be called after a successful spec phase completion.
// Returns true if the spec was saved, false if the phase is not "spec" or no content found.

// SpecExtractionError provides details about why spec extraction failed
type SpecExtractionError struct {
	Reason            string
	OutputLen         int
	OutputPreview     string // First 200 chars of output for debugging
	ValidationFailure string // Specific reason why validateSpecContent failed
}

func (e *SpecExtractionError) Error() string {
	var b strings.Builder
	b.WriteString(e.Reason)

	// Add diagnostic details
	fmt.Fprintf(&b, "\n  output_length: %d bytes", e.OutputLen)
	if e.OutputPreview != "" {
		fmt.Fprintf(&b, "\n  output_preview: %q", e.OutputPreview)
	}
	if e.ValidationFailure != "" {
		fmt.Fprintf(&b, "\n  validation_failure: %s", e.ValidationFailure)
	}

	return b.String()
}

// SaveSpecToDatabase extracts spec from JSON output and saves to phase_outputs table.
// The worktreePath parameter is deprecated and ignored - specs come from JSON output only.
// NOTE: This function requires a workflow run ID. For direct execution from workflow_phase.go,
// use backend.SavePhaseOutput() directly which is the preferred approach.
func SaveSpecToDatabase(backend storage.Backend, runID, taskID, phaseID, output string, _ ...string) (bool, error) {
	// Only save for spec phases (spec and tiny_spec)
	if phaseID != "spec" && phaseID != "tiny_spec" {
		return false, nil
	}

	if backend == nil {
		return false, fmt.Errorf("backend is nil - cannot save spec")
	}

	// Helper to get first N chars of output for preview
	outputPreview := func(s string, maxLen int) string {
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen] + "..."
	}

	// Extract spec from JSON content field
	specContent := ExtractContentFromOutput(output)
	if specContent == "" {
		return false, &SpecExtractionError{
			Reason:        "no content field in JSON output - agent must output spec in content field",
			OutputLen:     len(output),
			OutputPreview: outputPreview(output, 200),
		}
	}

	specContent = strings.TrimSpace(specContent)

	// Validate that the spec content looks like a valid spec
	if validationFailure := validateSpecContent(specContent); validationFailure != "" {
		return false, &SpecExtractionError{
			Reason:            "spec content failed validation",
			OutputLen:         len(output),
			OutputPreview:     outputPreview(specContent, 200),
			ValidationFailure: validationFailure,
		}
	}

	// Save to phase_outputs table with source indicating it came from execution
	phaseOutput := &storage.PhaseOutputInfo{
		WorkflowRunID:   runID,
		PhaseTemplateID: phaseID,
		TaskID:          &taskID,
		Content:         specContent,
		OutputVarName:   "SPEC_CONTENT",
		ArtifactType:    "spec",
		Source:          "executor",
	}
	if err := backend.SavePhaseOutput(phaseOutput); err != nil {
		return false, fmt.Errorf("database save failed: %w", err)
	}

	return true, nil
}

// isValidSpecContent validates that spec content is meaningful and not just noise.
// A valid spec should:
// - Have a minimum length (50 chars)
// - Not consist primarily of completion markers
// - Ideally have at least one spec-like section (Intent, Success Criteria, etc.)
func isValidSpecContent(content string) bool {
	reason := validateSpecContent(content)
	return reason == ""
}

// validateSpecContent checks if spec content is valid and returns an empty string
// if valid, or a description of why validation failed.
func validateSpecContent(content string) string {
	trimmed := strings.TrimSpace(content)

	// Minimum length check - a real spec should have at least some content
	if len(trimmed) < 50 {
		return fmt.Sprintf("content too short (%d chars, need at least 50)", len(trimmed))
	}

	lowerContent := strings.ToLower(trimmed)

	// Reject content that is primarily completion markers or noise
	noisePatterns := []string{
		`"status": "complete"`,
		`"status": "blocked"`,
		"the working tree is clean",
		"the spec was created as output in this conversation",
		"spec is in conversation output",
		"n/a (spec is in conversation",
	}

	for _, noise := range noisePatterns {
		if strings.Contains(lowerContent, noise) {
			// If noise pattern is found, check if there's meaningful content before it
			noiseIdx := strings.Index(lowerContent, noise)
			beforeNoise := strings.TrimSpace(trimmed[:noiseIdx])
			// Need at least 50 meaningful chars before the noise
			if len(beforeNoise) < 50 {
				return fmt.Sprintf("noise pattern detected (%q) with only %d chars of content before it (need 50)", noise, len(beforeNoise))
			}
		}
	}

	// Check for at least one spec-like section header (case insensitive)
	specSections := []string{
		"intent",
		"success criteria",
		"testing",
		"scope",
		"requirements",
		"approach",
		"technical",
		"acceptance",
		"specification",
		"overview",
		"background",
	}

	for _, section := range specSections {
		if strings.Contains(lowerContent, section) {
			return "" // Valid: has spec section
		}
	}

	// If no recognized spec sections, require longer content (200 chars)
	// to avoid accepting random garbage
	if len(trimmed) >= 200 {
		return "" // Valid: long enough without sections
	}

	return fmt.Sprintf("no recognized spec sections (intent, success criteria, etc.) and content too short (%d chars, need 200 without sections)", len(trimmed))
}
