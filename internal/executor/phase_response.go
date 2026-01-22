// Package executor provides the execution engine for orc.
// This file defines the JSON schema and types for phase completion responses.
package executor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PhaseCompletionStatus represents the completion status of a phase.
type PhaseCompletionStatus int

const (
	// PhaseStatusContinue indicates the phase should continue iterating.
	PhaseStatusContinue PhaseCompletionStatus = iota

	// PhaseStatusComplete indicates the phase completed successfully.
	PhaseStatusComplete

	// PhaseStatusBlocked indicates the phase is blocked and needs intervention.
	PhaseStatusBlocked
)

// PhaseCompletionSchema is the JSON schema for phases that don't produce artifacts.
// Used by: implement, test, validate, finalize
const PhaseCompletionSchema = `{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete", "blocked", "continue"],
			"description": "Phase status: complete (work done), blocked (cannot proceed), continue (more work needed)"
		},
		"reason": {
			"type": "string",
			"description": "Explanation for blocked status, or progress summary for continue"
		},
		"summary": {
			"type": "string",
			"description": "Work summary for complete status"
		}
	},
	"required": ["status"]
}`

// PhaseCompletionWithArtifactSchema is the JSON schema for phases that produce artifacts.
// Used by: spec, tiny_spec, research, tdd_write, breakdown, docs
// The artifact field contains the full artifact content (spec, design doc, etc.)
// The quality_checklist field is required for spec/tiny_spec phases.
const PhaseCompletionWithArtifactSchema = `{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete", "blocked", "continue"],
			"description": "Phase status: complete (work done), blocked (cannot proceed), continue (more work needed)"
		},
		"reason": {
			"type": "string",
			"description": "Explanation for blocked status, or progress summary for continue"
		},
		"summary": {
			"type": "string",
			"description": "Work summary for complete status"
		},
		"artifact": {
			"type": "string",
			"description": "The full artifact content (spec, design doc, research notes, etc.). REQUIRED when status is complete."
		},
		"quality_checklist": {
			"type": "array",
			"description": "Quality self-assessment checklist. REQUIRED for spec/tiny_spec phases.",
			"items": {
				"type": "object",
				"properties": {
					"id": {"type": "string", "description": "Check ID (e.g., all_criteria_verifiable)"},
					"check": {"type": "string", "description": "What was checked"},
					"passed": {"type": "boolean", "description": "Whether check passed"}
				},
				"required": ["id", "check", "passed"]
			}
		}
	},
	"required": ["status"]
}`

// PhasesWithArtifacts lists phases that produce artifacts and should use PhaseCompletionWithArtifactSchema
var PhasesWithArtifacts = map[string]bool{
	"spec":      true,
	"tiny_spec": true, // Combined spec+TDD for trivial/small tasks
	"research":  true,
	"tdd_write": true, // TDD test-writing phase for medium+
	"breakdown": true, // Implementation breakdown for medium/large
	"docs":      true,
}

// GetSchemaForPhase returns the appropriate JSON schema for a phase.
// For review phases, use GetSchemaForPhaseWithRound to get round-specific schemas.
func GetSchemaForPhase(phaseID string) string {
	return GetSchemaForPhaseWithRound(phaseID, 0)
}

// GetSchemaForPhaseWithRound returns the appropriate JSON schema for a phase,
// with support for round-specific schemas (e.g., review round 1 vs round 2).
func GetSchemaForPhaseWithRound(phaseID string, round int) string {
	// Artifact-producing phases get schema with artifact field
	if PhasesWithArtifacts[phaseID] {
		return PhaseCompletionWithArtifactSchema
	}

	// Review phase has round-specific schemas
	if phaseID == "review" {
		if round == 2 {
			return ReviewDecisionSchema
		}
		// Round 1 (or unspecified) uses findings schema
		return ReviewFindingsSchema
	}

	// QA phase has its own schema
	if phaseID == "qa" {
		return QAResultSchema
	}

	return PhaseCompletionSchema
}

// PhaseResponse represents the structured response from a phase execution.
type PhaseResponse struct {
	Status   string `json:"status"`             // "complete", "blocked", or "continue"
	Reason   string `json:"reason,omitempty"`   // Required for blocked, optional for others
	Summary  string `json:"summary,omitempty"`  // Work summary for complete status
	Artifact string `json:"artifact,omitempty"` // Artifact content for phases that produce them (spec, design, research, docs)
}

// ParsePhaseResponse parses a JSON response into a PhaseResponse struct.
// Returns an error if the content is not valid JSON or doesn't match the schema.
func ParsePhaseResponse(content string) (*PhaseResponse, error) {
	var resp PhaseResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("invalid phase response JSON: %w", err)
	}

	// Validate status is one of the expected values
	switch resp.Status {
	case "complete", "blocked", "continue":
		// Valid
	default:
		return nil, fmt.Errorf("invalid phase status: %q (expected complete, blocked, or continue)", resp.Status)
	}

	return &resp, nil
}

// IsComplete returns true if the phase completed successfully.
func (r *PhaseResponse) IsComplete() bool {
	return r.Status == "complete"
}

// IsBlocked returns true if the phase is blocked and needs intervention.
func (r *PhaseResponse) IsBlocked() bool {
	return r.Status == "blocked"
}

// IsContinue returns true if the phase needs more iterations.
func (r *PhaseResponse) IsContinue() bool {
	return r.Status == "continue"
}

// BuildJSONRetryPrompt creates a prompt to send when Claude returns invalid JSON.
// This should be rare since --json-schema guarantees valid JSON, but provides a fallback.
func BuildJSONRetryPrompt(invalidContent string, parseErr error) string {
	return fmt.Sprintf(`Your previous response was not valid JSON. Please output ONLY valid JSON matching this schema:

%s

Error: %v

Your invalid response was:
%s

Please try again with valid JSON only.`, PhaseCompletionSchema, parseErr, truncateForPrompt(invalidContent, 500))
}

// truncateForPrompt truncates content for inclusion in a prompt.
func truncateForPrompt(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "...[truncated]"
}

// HasJSONCompletion checks if content is valid JSON indicating phase completion.
// Content MUST be pure JSON from --json-schema.
func HasJSONCompletion(content string) bool {
	resp, err := ParsePhaseResponse(strings.TrimSpace(content))
	if err != nil {
		return false
	}
	return resp.IsComplete() || resp.IsBlocked()
}

// CheckPhaseCompletionJSON parses JSON content and returns the phase status.
// Content MUST be pure JSON from --json-schema. No extraction, no mixed content.
// Returns error if JSON parsing fails - callers must handle this explicitly.
// NO silent continue on parse failure - that hides bugs.
func CheckPhaseCompletionJSON(content string) (PhaseCompletionStatus, string, error) {
	resp, err := ParsePhaseResponse(strings.TrimSpace(content))
	if err != nil {
		return PhaseStatusContinue, "", fmt.Errorf("invalid phase completion JSON: %w (content=%q)",
			err, truncateForPrompt(content, 200))
	}

	switch resp.Status {
	case "complete":
		return PhaseStatusComplete, resp.Summary, nil
	case "blocked":
		return PhaseStatusBlocked, resp.Reason, nil
	case "continue":
		return PhaseStatusContinue, resp.Reason, nil
	default:
		return PhaseStatusContinue, "", fmt.Errorf("unexpected status %q in phase response", resp.Status)
	}
}

// ExtractArtifactFromOutput parses JSON and returns the artifact field.
// Content MUST be pure JSON from --json-schema.
func ExtractArtifactFromOutput(content string) string {
	resp, err := ParsePhaseResponse(strings.TrimSpace(content))
	if err != nil {
		return ""
	}
	return resp.Artifact
}

// ParsePhaseSpecificResponse parses JSON response using the appropriate parser
// for the given phase. Different phases use different schemas:
//   - review round 1: ReviewFindingsSchema (no status field, valid JSON = complete)
//   - review round 2: ReviewDecisionSchema (status: pass/fail/needs_user_input)
//   - qa: QAResultSchema (status: pass/fail/needs_attention)
//   - other phases: PhaseCompletionSchema (status: complete/blocked/continue)
//
// Returns (status, reason, error) similar to CheckPhaseCompletionJSON.
func ParsePhaseSpecificResponse(phaseID string, reviewRound int, content string) (PhaseCompletionStatus, string, error) {
	content = strings.TrimSpace(content)

	// Review phase uses specialized schemas
	if phaseID == "review" {
		if reviewRound == 2 {
			// Round 2: ReviewDecisionSchema with pass/fail/needs_user_input
			decision, err := ParseReviewDecision(content)
			if err != nil {
				return PhaseStatusContinue, "", fmt.Errorf("invalid review decision JSON: %w (content=%q)",
					err, truncateForPrompt(content, 200))
			}
			// Map review decision status to phase completion status
			switch decision.Status {
			case ReviewStatusPass:
				return PhaseStatusComplete, decision.Summary, nil
			case ReviewStatusFail, ReviewStatusNeedsUserInput:
				reason := decision.Recommendation
				if reason == "" {
					reason = decision.Summary
				}
				return PhaseStatusBlocked, reason, nil
			default:
				return PhaseStatusBlocked, decision.Summary, nil
			}
		}
		// Round 1: ReviewFindingsSchema (no status field)
		// Valid JSON with findings = complete
		findings, err := ParseReviewFindings(content)
		if err != nil {
			return PhaseStatusContinue, "", fmt.Errorf("invalid review findings JSON: %w (content=%q)",
				err, truncateForPrompt(content, 200))
		}
		// Valid findings response means review round 1 is complete
		return PhaseStatusComplete, findings.Summary, nil
	}

	// QA phase uses QAResultSchema
	if phaseID == "qa" {
		result, err := ParseQAResult(content)
		if err != nil {
			return PhaseStatusContinue, "", fmt.Errorf("invalid QA result JSON: %w (content=%q)",
				err, truncateForPrompt(content, 200))
		}
		// Map QA status to phase completion status
		switch result.Status {
		case QAStatusPass:
			return PhaseStatusComplete, result.Summary, nil
		case QAStatusFail, QAStatusNeedsAttention:
			reason := result.Recommendation
			if reason == "" {
				reason = result.Summary
			}
			return PhaseStatusBlocked, reason, nil
		default:
			return PhaseStatusBlocked, result.Summary, nil
		}
	}

	// All other phases use standard PhaseCompletionSchema
	return CheckPhaseCompletionJSON(content)
}
