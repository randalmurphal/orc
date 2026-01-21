// Package executor provides the execution engine for orc.
// This file defines the JSON schema and types for phase completion responses.
package executor

import (
	"encoding/json"
	"fmt"
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

// PhaseCompletionSchema is the JSON schema that forces structured output for phase completion.
// This replaces the legacy XML marker parsing (<phase_complete>, <phase_blocked>).
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

// PhaseResponse represents the structured response from a phase execution.
type PhaseResponse struct {
	Status  string `json:"status"`            // "complete", "blocked", or "continue"
	Reason  string `json:"reason,omitempty"`  // Required for blocked, optional for others
	Summary string `json:"summary,omitempty"` // Work summary for complete status
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

// HasJSONCompletion checks if accumulated content contains a JSON object
// indicating phase completion or blocking. Used during streaming to detect
// early completion (workaround for Claude CLI bug #1920).
func HasJSONCompletion(content string) bool {
	resp, err := ParsePhaseResponse(content)
	if err != nil {
		return false
	}
	// Only complete/blocked indicate we're done; continue means more work needed
	return resp.IsComplete() || resp.IsBlocked()
}

// CheckPhaseCompletionJSON parses the content as JSON and returns the phase status.
// This is the JSON equivalent of the legacy CheckPhaseCompletion function.
// Returns (status, reason) where status is PhaseCompletionStatus and reason is the
// summary (for complete) or reason (for blocked/continue).
//
// This function requires pure JSON input, which is guaranteed when using
// ClaudeExecutor with --json-schema in headless mode.
func CheckPhaseCompletionJSON(content string) (PhaseCompletionStatus, string) {
	resp, err := ParsePhaseResponse(content)
	if err != nil {
		// Can't parse as JSON - treat as continue (need more work)
		return PhaseStatusContinue, ""
	}

	switch resp.Status {
	case "complete":
		return PhaseStatusComplete, resp.Summary
	case "blocked":
		return PhaseStatusBlocked, resp.Reason
	case "continue":
		return PhaseStatusContinue, resp.Reason
	default:
		return PhaseStatusContinue, ""
	}
}
