// Package executor provides the execution engine for orc.
// This file defines the JSON schema and types for phase completion responses.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/randalmurphal/llmkit/claude"
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
// NOTE: This function only does direct JSON parsing and requires pure JSON input.
// For session-based output that may contain mixed text+JSON (which is common when
// Claude provides explanatory text alongside completion JSON), use CheckPhaseCompletionMixed
// instead. CheckPhaseCompletionMixed handles markdown code blocks, embedded JSON,
// and other common patterns without requiring an LLM fallback.
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

// ExtractPhaseResponse extracts a phase response from session output using a two-phase approach:
// 1. Try direct JSON parsing (fast path for compliant output)
// 2. Fall back to LLM extraction with JSON schema (handles mixed text+JSON)
//
// This is necessary because Claude CLI's --json-schema only works with --print mode,
// not the stream-json mode that sessions use. Sessions can output mixed text and JSON,
// so we need robust extraction.
//
// The client parameter is used for fallback LLM extraction (typically Haiku for speed/cost).
// If client is nil, only direct parsing is attempted.
func ExtractPhaseResponse(ctx context.Context, client claude.Client, output string) (*PhaseResponse, error) {
	// Fast path: try direct JSON parsing
	if resp, err := ParsePhaseResponse(output); err == nil {
		return resp, nil
	}

	// Check if content might contain a phase response (avoid unnecessary LLM calls)
	if !mightContainPhaseResponse(output) {
		return nil, fmt.Errorf("output does not appear to contain a phase response")
	}

	// Fallback: use LLM extraction with JSON schema
	if client == nil {
		return nil, fmt.Errorf("cannot extract phase response: no client provided and direct parsing failed")
	}

	var resp PhaseResponse
	err := claude.ExtractStructured(ctx, client, output, PhaseCompletionSchema, &resp, &claude.ExtractStructuredOptions{
		Model:   "haiku",
		Context: "Phase completion status from an AI agent's work output. Extract the status (complete/blocked/continue), reason (if blocked or continuing), and summary (if complete).",
	})
	if err != nil {
		return nil, fmt.Errorf("extract phase response: %w", err)
	}

	// Normalize status to lowercase
	resp.Status = strings.ToLower(resp.Status)

	// Validate status
	switch resp.Status {
	case "complete", "blocked", "continue":
		// Valid
	default:
		return nil, fmt.Errorf("invalid extracted phase status: %q", resp.Status)
	}

	return &resp, nil
}

// mightContainPhaseResponse does a quick check to see if output might contain
// a phase response JSON. This avoids unnecessary LLM calls for content that
// clearly doesn't have completion status.
func mightContainPhaseResponse(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, `"status"`) ||
		strings.Contains(lower, "complete") ||
		strings.Contains(lower, "blocked")
}

// ExtractPhaseResponseFromMixed extracts a phase response from mixed text+JSON output
// without requiring an LLM client. It uses heuristics to find JSON in common patterns:
// 1. Direct JSON parsing (pure JSON output)
// 2. JSON in markdown code blocks (```json ... ```)
// 3. JSON object by brace matching (finds {"status": ...} pattern)
//
// This is the recommended function for session-based completion detection where
// output may contain explanatory text alongside the JSON response.
func ExtractPhaseResponseFromMixed(content string) (*PhaseResponse, error) {
	// Fast path: try direct JSON parsing
	if resp, err := ParsePhaseResponse(content); err == nil {
		return resp, nil
	}

	// Try to extract JSON from markdown code blocks
	if extracted := extractJSONFromCodeBlock(content); extracted != "" {
		if resp, err := ParsePhaseResponse(extracted); err == nil {
			return resp, nil
		}
	}

	// Try to find JSON object by brace matching
	if extracted := extractJSONByBraceMatching(content); extracted != "" {
		if resp, err := ParsePhaseResponse(extracted); err == nil {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("no valid phase response JSON found in content")
}

// extractJSONFromCodeBlock looks for JSON in markdown code blocks.
// Handles both ```json and ``` (untyped) code blocks.
func extractJSONFromCodeBlock(content string) string {
	// Look for ```json ... ``` pattern
	jsonBlockStart := strings.Index(content, "```json")
	if jsonBlockStart != -1 {
		start := jsonBlockStart + 7 // len("```json")
		// Skip newline after opening fence
		if start < len(content) && content[start] == '\n' {
			start++
		}
		end := strings.Index(content[start:], "```")
		if end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Look for untyped ``` ... ``` that contains JSON-like content
	blockStart := 0
	for {
		idx := strings.Index(content[blockStart:], "```")
		if idx == -1 {
			break
		}
		start := blockStart + idx + 3

		// Skip if this is a typed block (like ```go, ```python)
		// by checking if next non-whitespace is a letter
		checkPos := start
		for checkPos < len(content) && content[checkPos] == ' ' {
			checkPos++
		}
		if checkPos < len(content) && content[checkPos] != '\n' && content[checkPos] != '{' {
			// Typed block, skip to end
			endIdx := strings.Index(content[start:], "```")
			if endIdx == -1 {
				break
			}
			blockStart = start + endIdx + 3
			continue
		}

		// Skip newline
		if start < len(content) && content[start] == '\n' {
			start++
		}

		end := strings.Index(content[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(content[start : start+end])
			// Check if it looks like our JSON
			if strings.Contains(candidate, `"status"`) {
				return candidate
			}
		}
		blockStart = start
	}

	return ""
}

// extractJSONByBraceMatching finds a JSON object containing "status" by matching braces.
func extractJSONByBraceMatching(content string) string {
	// Look for {"status" pattern which is our completion JSON
	statusPattern := `{"status"`
	idx := strings.Index(content, statusPattern)
	if idx == -1 {
		// Also try with whitespace: { "status"
		idx = strings.Index(content, `{ "status"`)
		if idx == -1 {
			return ""
		}
	}

	// Find the opening brace
	braceStart := strings.LastIndex(content[:idx+1], "{")
	if braceStart == -1 {
		return ""
	}

	// Match braces to find the closing brace
	depth := 0
	inString := false
	escaped := false

	for i := braceStart; i < len(content); i++ {
		c := content[i]

		if escaped {
			escaped = false
			continue
		}

		if c == '\\' && inString {
			escaped = true
			continue
		}

		if c == '"' {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		if c == '{' {
			depth++
		} else if c == '}' {
			depth--
			if depth == 0 {
				return content[braceStart : i+1]
			}
		}
	}

	return ""
}

// CheckPhaseCompletionMixed extracts and checks phase completion from mixed text+JSON output.
// This is the recommended function for session-based completion detection.
// Returns (status, reason) where reason is summary for complete or reason for blocked/continue.
func CheckPhaseCompletionMixed(content string) (PhaseCompletionStatus, string) {
	resp, err := ExtractPhaseResponseFromMixed(content)
	if err != nil {
		// Can't extract - treat as continue (need more work)
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
