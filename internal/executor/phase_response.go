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

// ContentProducingPhaseSchema is the JSON schema for phases that produce document content.
// Used by: spec, tiny_spec, research, tdd_write, breakdown, docs
// The content field contains the full document content (spec, design doc, etc.)
// The quality_checklist field is required for spec/tiny_spec phases.
const ContentProducingPhaseSchema = `{
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
		"content": {
			"type": "string",
			"description": "The full phase output content (spec, design doc, research notes, etc.). REQUIRED when status is complete."
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

// contentProducingPhases lists phases that produce document content.
// NOTE: This is used for schema selection. The authoritative source is
// PhaseTemplate.ProducesArtifact in the database.
var contentProducingPhases = map[string]bool{
	"spec":      true,
	"tiny_spec": true,
	"research":  true,
	"tdd_write": true,
	"breakdown": true,
	"docs":      true,
}

// GetSchemaForPhase returns the appropriate JSON schema for a phase.
// For review phases, use GetSchemaForPhaseWithRound to get round-specific schemas.
func GetSchemaForPhase(phaseID string) string {
	return GetSchemaForPhaseWithRound(phaseID, 0)
}

// ImplementCompletionSchema is the JSON schema for implement phase.
// Requires verification evidence when claiming completion.
const ImplementCompletionSchema = `{
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
		"verification": {
			"type": "object",
			"description": "Verification evidence. REQUIRED when status is complete.",
			"properties": {
				"tests": {
					"type": "object",
					"properties": {
						"command": {"type": "string", "description": "Test command that was run"},
						"status": {"type": "string", "enum": ["PASS", "FAIL", "SKIPPED"], "description": "Test result"},
						"evidence": {"type": "string", "description": "Test output showing pass (e.g., 'ok  package/name  0.5s')"}
					},
					"required": ["status"]
				},
				"success_criteria": {
					"type": "array",
					"description": "Verification of each success criterion from the spec",
					"items": {
						"type": "object",
						"properties": {
							"id": {"type": "string", "description": "Criterion ID (e.g., SC-1)"},
							"status": {"type": "string", "enum": ["PASS", "FAIL"], "description": "Verification result"},
							"evidence": {"type": "string", "description": "How the criterion was verified"}
						},
						"required": ["id", "status"]
					}
				},
				"build": {
					"type": "object",
					"properties": {
						"status": {"type": "string", "enum": ["PASS", "FAIL", "SKIPPED"]}
					}
				},
				"linting": {
					"type": "object",
					"properties": {
						"status": {"type": "string", "enum": ["PASS", "FAIL", "SKIPPED"]}
					}
				}
			}
		}
	},
	"required": ["status"]
}`

// GetSchemaForPhaseWithRound returns the appropriate JSON schema for a phase,
// with support for round-specific schemas (e.g., review round 1 vs round 2).
func GetSchemaForPhaseWithRound(phaseID string, round int) string {
	// Content-producing phases get schema with content field
	if contentProducingPhases[phaseID] {
		return ContentProducingPhaseSchema
	}

	// Implement phase uses verification schema
	if phaseID == "implement" {
		return ImplementCompletionSchema
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

	// QA E2E phases have specialized schemas
	if phaseID == "qa_e2e_test" {
		return QAE2ETestResultSchema
	}
	if phaseID == "qa_e2e_fix" {
		return QAE2EFixResultSchema
	}

	return PhaseCompletionSchema
}

// PhaseResponse represents the structured response from a phase execution.
type PhaseResponse struct {
	Status  string `json:"status"`            // "complete", "blocked", or "continue"
	Reason  string `json:"reason,omitempty"`  // Required for blocked, optional for others
	Summary string `json:"summary,omitempty"` // Work summary for complete status
	Content string `json:"content,omitempty"` // Phase output content (spec, design doc, research notes, etc.)
}

// ImplementVerification represents the verification evidence for implement phase completion.
type ImplementVerification struct {
	Tests           *VerificationStatus          `json:"tests,omitempty"`
	SuccessCriteria []SuccessCriterionResult     `json:"success_criteria,omitempty"`
	Build           *VerificationStatus          `json:"build,omitempty"`
	Linting         *VerificationStatus          `json:"linting,omitempty"`
}

// VerificationStatus represents a single verification check result.
type VerificationStatus struct {
	Command  string `json:"command,omitempty"`
	Status   string `json:"status"` // "PASS", "FAIL", or "SKIPPED"
	Evidence string `json:"evidence,omitempty"`
}

// SuccessCriterionResult represents verification of a single success criterion.
type SuccessCriterionResult struct {
	ID       string `json:"id"`
	Status   string `json:"status"` // "PASS" or "FAIL"
	Evidence string `json:"evidence,omitempty"`
}

// ImplementResponse extends PhaseResponse with verification evidence.
type ImplementResponse struct {
	Status       string                 `json:"status"`
	Reason       string                 `json:"reason,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Verification *ImplementVerification `json:"verification,omitempty"`
}

// ParseImplementResponse parses implement phase JSON response with verification.
func ParseImplementResponse(content string) (*ImplementResponse, error) {
	var resp ImplementResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		return nil, fmt.Errorf("invalid implement response JSON: %w", err)
	}

	// Validate status is one of the expected values
	switch resp.Status {
	case "complete", "blocked", "continue":
		// Valid
	default:
		return nil, fmt.Errorf("invalid implement status: %q (expected complete, blocked, or continue)", resp.Status)
	}

	return &resp, nil
}

// ValidateImplementCompletion checks if an implement phase completion has valid verification.
// Returns an error describing what's missing if verification is incomplete.
func ValidateImplementCompletion(content string) error {
	resp, err := ParseImplementResponse(strings.TrimSpace(content))
	if err != nil {
		return err
	}

	// Only validate completions - blocked/continue don't need verification
	if resp.Status != "complete" {
		return nil
	}

	// Verification is required for completion
	if resp.Verification == nil {
		return fmt.Errorf("completion claimed without verification evidence - please run tests and verify success criteria")
	}

	var failures []string

	// Check tests passed
	if resp.Verification.Tests != nil && resp.Verification.Tests.Status == "FAIL" {
		failures = append(failures, "tests failed")
	}

	// Check all success criteria passed
	for _, sc := range resp.Verification.SuccessCriteria {
		if sc.Status == "FAIL" {
			failures = append(failures, fmt.Sprintf("success criterion %s failed", sc.ID))
		}
	}

	// Check build passed (if not skipped)
	if resp.Verification.Build != nil && resp.Verification.Build.Status == "FAIL" {
		failures = append(failures, "build failed")
	}

	// Check linting passed (if not skipped)
	if resp.Verification.Linting != nil && resp.Verification.Linting.Status == "FAIL" {
		failures = append(failures, "linting failed")
	}

	if len(failures) > 0 {
		return fmt.Errorf("verification failed: %s - fix issues and re-verify", strings.Join(failures, ", "))
	}

	return nil
}

// FormatVerificationFeedback creates a prompt for the agent to fix verification failures.
func FormatVerificationFeedback(err error) string {
	return fmt.Sprintf(`## Verification Gate Failed

%s

**IMPORTANT:** You cannot claim completion until ALL verifications pass.

Please:
1. Fix the failing issues
2. Re-run verifications
3. Output completion JSON with updated verification evidence showing all PASS

Do NOT output completion until all verifications pass.`, err.Error())
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

// ExtractContentFromOutput parses JSON and returns the content field.
// Content MUST be pure JSON from --json-schema.
func ExtractContentFromOutput(content string) string {
	resp, err := ParsePhaseResponse(strings.TrimSpace(content))
	if err != nil {
		return ""
	}
	return resp.Content
}

// ParsePhaseSpecificResponse parses JSON response using the appropriate parser
// for the given phase. Different phases use different schemas:
//   - review round 1: ReviewFindingsSchema (status: complete/blocked)
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
		// Round 1: ReviewFindingsSchema (status: complete/blocked)
		findings, err := ParseReviewFindings(content)
		if err != nil {
			return PhaseStatusContinue, "", fmt.Errorf("invalid review findings JSON: %w (content=%q)",
				err, truncateForPrompt(content, 200))
		}
		if strings.EqualFold(findings.Status, "blocked") {
			return PhaseStatusBlocked, findings.Summary, nil
		}
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

	// QA E2E test phase uses QAE2ETestResultSchema
	if phaseID == "qa_e2e_test" {
		result, err := ParseQAE2ETestResult(content)
		if err != nil {
			return PhaseStatusContinue, "", fmt.Errorf("invalid QA E2E test result JSON: %w (content=%q)",
				err, truncateForPrompt(content, 200))
		}
		// QA E2E test always completes (findings handled by loop mechanism)
		if result.Status == "blocked" {
			return PhaseStatusBlocked, result.Summary, nil
		}
		return PhaseStatusComplete, result.Summary, nil
	}

	// QA E2E fix phase uses QAE2EFixResultSchema
	if phaseID == "qa_e2e_fix" {
		result, err := ParseQAE2EFixResult(content)
		if err != nil {
			return PhaseStatusContinue, "", fmt.Errorf("invalid QA E2E fix result JSON: %w (content=%q)",
				err, truncateForPrompt(content, 200))
		}
		if result.Status == "blocked" {
			return PhaseStatusBlocked, result.Summary, nil
		}
		return PhaseStatusComplete, result.Summary, nil
	}

	// All other phases use standard PhaseCompletionSchema
	return CheckPhaseCompletionJSON(content)
}
