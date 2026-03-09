// Package executor provides the execution engine for orc.
// This file defines the JSON schema and types for phase completion responses.
package executor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
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

// GetSchemaForPhase returns the appropriate JSON schema for a phase.
// For review phases, use GetSchemaForPhaseWithRound to get round-specific schemas.
// This convenience wrapper assumes producesArtifact=false — use GetSchemaForPhaseWithRound
// with the template's ProducesArtifact field for proper schema selection.
func GetSchemaForPhase(phaseID string) string {
	return GetSchemaForPhaseWithRound(phaseID, 0, false)
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
				},
				"wiring": {
					"type": "object",
					"description": "Verification that newly-created files are imported by production code.",
					"properties": {
						"status": {"type": "string", "enum": ["PASS", "FAIL", "SKIPPED"]},
						"evidence": {"type": "string", "description": "Proof that production code imports the new files"},
						"new_files": {
							"type": "array",
							"items": {
								"type": "object",
								"properties": {
									"file": {"type": "string"},
									"imported_by": {"type": "string"}
								},
								"required": ["file", "imported_by"]
							}
						}
					}
				},
				"browser_validation": {
					"type": "object",
					"description": "Browser-validation decision and evidence for user-visible browser behavior.",
					"properties": {
						"browser_surface_change": {
							"type": "boolean",
							"description": "True when the implemented change affects browser-visible behavior, even through backend/API changes."
						},
						"required": {
							"type": "boolean",
							"description": "True when browser validation was required for this implementation."
						},
						"performed": {
							"type": "boolean",
							"description": "True when browser validation was actually performed."
						},
						"reason": {
							"type": "string",
							"description": "Why browser validation was or was not required."
						},
						"evidence": {
							"type": "string",
							"description": "What browser validation proved, including commands or observed behavior."
						},
						"artifacts": {
							"type": "array",
							"description": "Optional screenshots, logs, or trace artifact paths.",
							"items": {"type": "string"}
						}
					},
					"required": ["browser_surface_change", "required", "performed", "reason", "evidence", "artifacts"]
				}
			}
		},
		"pre_existing_issues": {
			"type": "array",
			"description": "Out-of-scope issues discovered during implementation.",
			"items": {"type": "string"}
		}
	},
	"required": ["status"]
}`

// DocsCompletionSchema is the JSON schema for docs phase.
// Extends ContentProducingPhaseSchema with initiative_notes for knowledge extraction.
const DocsCompletionSchema = `{
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
			"description": "The documentation summary content. REQUIRED when status is complete."
		},
		"initiative_notes": {
			"type": "array",
			"description": "Knowledge notes extracted for the initiative. Optional - include when task is part of an initiative.",
			"items": {
				"type": "object",
				"properties": {
					"type": {
						"type": "string",
						"enum": ["pattern", "warning", "learning", "handoff"],
						"description": "Note type: pattern (reusable approach), warning (gotcha to avoid), learning (non-obvious discovery), handoff (incomplete work)"
					},
					"content": {
						"type": "string",
						"description": "The note content - concise but self-contained"
					},
					"relevant_files": {
						"type": "array",
						"items": {"type": "string"},
						"description": "Optional file paths related to this note"
					}
				},
				"required": ["type", "content"]
			}
		},
		"notes_rationale": {
			"type": "string",
			"description": "Brief explanation of why notes were/weren't extracted"
		}
	},
	"required": ["status"]
}`

// GetSchemaForPhaseWithRound returns the appropriate JSON schema for a phase,
// with support for round-specific schemas (e.g., review round 1 vs round 2).
// producesArtifact comes from the phase template's ProducesArtifact field.
func GetSchemaForPhaseWithRound(phaseID string, round int, producesArtifact bool) string {
	// Docs phase uses specialized schema with initiative_notes support
	if phaseID == "docs" {
		return DocsCompletionSchema
	}

	// Plan phase uses specialized schema with policy signals.
	if phaseID == "plan" {
		return PlanCompletionSchema
	}

	// Content-producing phases get schema with content field
	if producesArtifact {
		return ContentProducingPhaseSchema
	}

	// Implementation phases use verification schema
	if isImplementationPhase(phaseID) {
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
	if phaseID == "review_cross" {
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

// GetSchemaForIteration returns the appropriate JSON schema for a phase iteration.
// If loopCfg has LoopSchemas configured, uses the schema identifier for the iteration.
// Otherwise falls back to round-based logic via GetSchemaForPhaseWithRound.
func GetSchemaForIteration(loopCfg *db.LoopConfig, iteration int, phaseID string, producesArtifact bool) string {
	// If no loop config or no LoopSchemas, fall back to round-based schema
	if loopCfg == nil || len(loopCfg.LoopSchemas) == 0 {
		// For review phase, iteration maps to round
		round := iteration
		return GetSchemaForPhaseWithRound(phaseID, round, producesArtifact)
	}

	// Get schema identifier from LoopConfig
	identifier := loopCfg.GetSchemaForIteration(iteration)
	return MapSchemaIdentifierToSchema(identifier, phaseID, producesArtifact)
}

// MapSchemaIdentifierToSchema maps a schema identifier string to the actual JSON schema.
// Identifiers are phase-specific:
//   - review: "findings" -> ReviewFindingsSchema, "decision" -> ReviewDecisionSchema
//   - qa: "qa_result" -> QAResultSchema
//   - Empty identifier uses phase default
//   - Unknown identifier falls back to PhaseCompletionSchema
func MapSchemaIdentifierToSchema(identifier string, phaseID string, producesArtifact bool) string {
	// Docs phase uses specialized schema (overrides producesArtifact check)
	if phaseID == "docs" {
		return DocsCompletionSchema
	}
	if phaseID == "plan" {
		return PlanCompletionSchema
	}

	// Content-producing phases always use ContentProducingPhaseSchema
	if producesArtifact {
		return ContentProducingPhaseSchema
	}

	switch phaseID {
	case "review", "review_cross":
		switch identifier {
		case "findings", "":
			return ReviewFindingsSchema
		case "decision":
			return ReviewDecisionSchema
		default:
			return PhaseCompletionSchema
		}

	case "qa":
		switch identifier {
		case "qa_result", "":
			return QAResultSchema
		default:
			return PhaseCompletionSchema
		}

	case "implement", "implement_codex":
		return ImplementCompletionSchema

	case "qa_e2e_test":
		return QAE2ETestResultSchema

	case "qa_e2e_fix":
		return QAE2EFixResultSchema

	default:
		// For unknown phases, use generic schema
		return PhaseCompletionSchema
	}
}

func isImplementationPhase(phaseID string) bool {
	switch phaseID {
	case "implement", "implement_codex":
		return true
	default:
		return false
	}
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
	Tests           *VerificationStatus      `json:"tests,omitempty"`
	SuccessCriteria []SuccessCriterionResult `json:"success_criteria,omitempty"`
	Build           *VerificationStatus      `json:"build,omitempty"`
	Linting         *VerificationStatus      `json:"linting,omitempty"`
	Wiring          *WiringVerification      `json:"wiring,omitempty"`
	BrowserValidation *BrowserValidation     `json:"browser_validation,omitempty"`
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

// WiringVerification records proof that new files are reachable from production code.
type WiringVerification struct {
	Status   string          `json:"status"`
	Evidence string          `json:"evidence,omitempty"`
	NewFiles []WiringNewFile `json:"new_files,omitempty"`
}

// WiringNewFile records the production importer for a newly-created file.
type WiringNewFile struct {
	File       string `json:"file"`
	ImportedBy string `json:"imported_by"`
}

// BrowserValidation records whether browser validation was required and what evidence was gathered.
type BrowserValidation struct {
	BrowserSurfaceChange bool     `json:"browser_surface_change"`
	Required             bool     `json:"required"`
	Performed            bool     `json:"performed"`
	Reason               string   `json:"reason"`
	Evidence             string   `json:"evidence"`
	Artifacts            []string `json:"artifacts,omitempty"`
}

// ImplementResponse extends PhaseResponse with verification evidence.
type ImplementResponse struct {
	Status            string                 `json:"status"`
	Reason            string                 `json:"reason,omitempty"`
	Summary           string                 `json:"summary,omitempty"`
	Verification      *ImplementVerification `json:"verification,omitempty"`
	PreExistingIssues []string               `json:"pre_existing_issues,omitempty"`
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

	if resp.Verification.Tests == nil {
		failures = append(failures, "tests verification missing")
	}
	if len(resp.Verification.SuccessCriteria) == 0 {
		failures = append(failures, "success criteria verification missing")
	}
	if resp.Verification.Build == nil {
		failures = append(failures, "build verification missing")
	}
	if resp.Verification.Linting == nil {
		failures = append(failures, "linting verification missing")
	}
	if resp.Verification.Wiring == nil {
		failures = append(failures, "wiring verification missing")
	}
	if resp.Verification.BrowserValidation == nil {
		failures = append(failures, "browser validation verdict missing")
	}

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

	// Check wiring passed (dead code must block completion)
	if resp.Verification.Wiring != nil && resp.Verification.Wiring.Status == "FAIL" {
		failures = append(failures, "wiring failed")
	}

	if resp.Verification.BrowserValidation != nil {
		browserValidation := resp.Verification.BrowserValidation
		if browserValidation.BrowserSurfaceChange && !browserValidation.Required {
			failures = append(failures, "browser validation required for browser-surface change")
		}
		if browserValidation.Required && !browserValidation.Performed {
			failures = append(failures, "browser validation required but not performed")
		}
		if browserValidation.Required && strings.TrimSpace(browserValidation.Evidence) == "" {
			failures = append(failures, "browser validation evidence missing")
		}
		if browserValidation.Performed && strings.TrimSpace(browserValidation.Evidence) == "" {
			failures = append(failures, "browser validation evidence missing")
		}
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

// unmarshalWithFallback attempts json.Unmarshal, and on failure tries stripping
// any non-JSON prefix (e.g., "(no content)" from Claude session resume). This is
// a defensive measure — Claude sometimes prepends text before JSON output.
func unmarshalWithFallback(s string, v any) error {
	if err := json.Unmarshal([]byte(s), v); err != nil {
		if idx := strings.Index(s, "{"); idx > 0 {
			if err2 := json.Unmarshal([]byte(s[idx:]), v); err2 == nil {
				return nil
			}
		}
		return err
	}
	return nil
}

// ParsePhaseResponse parses a JSON response into a PhaseResponse struct.
// Returns an error if the content is not valid JSON or doesn't match the schema.
// Only accepts standard statuses: "complete", "blocked", "continue".
// For contexts where non-standard statuses are valid (loop conditions),
// use CheckPhaseCompletionJSON which handles them via fallback parsing.
func ParsePhaseResponse(content string) (*PhaseResponse, error) {
	var resp PhaseResponse
	if err := unmarshalWithFallback(content, &resp); err != nil {
		return nil, fmt.Errorf("invalid phase response JSON: %w", err)
	}

	switch resp.Status {
	case "complete", "blocked", "continue":
		// Valid standard statuses
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
//
// Non-standard statuses (e.g., "needs_changes", "needs_review") are treated as
// PhaseStatusComplete. The loop system evaluates phase_output.<phase>.status to
// decide whether to loop back, so the phase itself must complete normally.
func CheckPhaseCompletionJSON(content string) (PhaseCompletionStatus, string, error) {
	trimmed := strings.TrimSpace(content)
	resp, err := ParsePhaseResponse(trimmed)
	if err != nil {
		// ParsePhaseResponse rejects non-standard statuses, but the generic loop
		// system requires phases to return arbitrary status values. Fall back to
		// raw JSON parsing: if the JSON is valid with a non-empty status, treat it
		// as PhaseStatusComplete so loop conditions can evaluate the status.
		var raw PhaseResponse
		if unmarshalErr := unmarshalWithFallback(trimmed, &raw); unmarshalErr == nil && raw.Status != "" {
			return PhaseStatusComplete, raw.Summary, nil
		}
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
		// Unreachable: ParsePhaseResponse validates to complete/blocked/continue
		return PhaseStatusComplete, resp.Summary, nil
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
	if phaseID == "review" || phaseID == "review_cross" {
		if phaseID == "review" && reviewRound == 2 {
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
		if findings.NeedsChanges {
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
