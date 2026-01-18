// Package executor provides the flowgraph-based execution engine for orc.
// This file contains Haiku-based validation functions for objective quality assessment.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/randalmurphal/llmkit/claude"
)

// ValidationDecision represents Haiku's judgment on iteration progress.
type ValidationDecision int

const (
	// ValidationContinue indicates the iteration is on track, keep going.
	ValidationContinue ValidationDecision = iota
	// ValidationRetry indicates the approach is going off track, redirect.
	ValidationRetry
	// ValidationStop indicates the task is fundamentally blocked.
	ValidationStop
)

// String returns a human-readable representation of the decision.
func (v ValidationDecision) String() string {
	switch v {
	case ValidationContinue:
		return "continue"
	case ValidationRetry:
		return "retry"
	case ValidationStop:
		return "stop"
	default:
		return "unknown"
	}
}

// HaikuValidationModel is the default model for validation calls.
// Use the alias "haiku" for resilience against model name changes.
const HaikuValidationModel = "haiku"

// ValidateIterationProgress uses Haiku to assess whether an iteration is on track.
// It evaluates the iteration output against the spec's success criteria.
//
// Returns:
//   - ValidationContinue: The work is progressing toward the success criteria
//   - ValidationRetry: The approach has diverged, needs redirection
//   - ValidationStop: Fundamentally blocked, cannot proceed
//
// On error (API failure, timeout), returns the error to let caller decide:
//   - If config.Validation.FailOnAPIError is true: Fail the task (resumable)
//   - If config.Validation.FailOnAPIError is false: Fail open, continue execution
func ValidateIterationProgress(
	ctx context.Context,
	client claude.Client,
	specContent string,
	iterationOutput string,
) (ValidationDecision, string, error) {
	if client == nil {
		return ValidationContinue, "", nil
	}

	// Skip validation if no spec to validate against
	if specContent == "" {
		return ValidationContinue, "", nil
	}

	// Truncate long outputs to keep token costs reasonable
	maxOutputLen := 8000
	truncatedOutput := iterationOutput
	if len(iterationOutput) > maxOutputLen {
		truncatedOutput = iterationOutput[:maxOutputLen] + "\n...[truncated]"
	}

	prompt := fmt.Sprintf(`You are evaluating whether an AI agent's work is progressing toward the success criteria in a specification.

## Specification (contains success criteria)
%s

## Agent's Latest Output
%s

## Evaluation Task

Assess whether the agent's work is:
1. ON TRACK - Making progress toward the success criteria
2. OFF TRACK - Diverging from the specification (wrong approach, scope creep, misunderstanding)
3. BLOCKED - Cannot proceed (missing dependencies, impossible requirements, fundamental issues)

Respond with EXACTLY one of these three words on the first line, followed by a brief reason:
- CONTINUE (if on track)
- RETRY (if off track - explain what went wrong)
- STOP (if blocked - explain what's blocking)

Example responses:
"CONTINUE
Making good progress on the authentication flow."

"RETRY
The agent is implementing a REST API but the spec calls for GraphQL."

"STOP
The spec requires a third-party service that doesn't exist."`, specContent, truncatedOutput)

	resp, err := client.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: prompt},
		},
		Model:       HaikuValidationModel,
		MaxTokens:   200,
		Temperature: 0,
	})

	if err != nil {
		// Return the error - let caller decide whether to fail open or closed
		slog.Warn("haiku validation API error",
			"error", err,
		)
		return ValidationContinue, "", fmt.Errorf("validation API error: %w", err)
	}

	// Return error if response is nil (shouldn't happen, but be defensive)
	if resp == nil {
		return ValidationContinue, "", fmt.Errorf("validation API returned nil response")
	}

	// Parse the response
	decision, reason := parseValidationResponse(resp.Content)
	return decision, reason, nil
}

// ValidateTaskReadiness checks if a task has a quality spec before execution.
// This is a pre-execution gate to catch poorly-specified tasks before they waste
// compute on doomed implementations.
//
// Returns:
//   - ready: true if the spec is sufficient for execution
//   - suggestions: list of improvements if not ready
//   - error: on API failures, returned to let caller decide based on config.Validation.FailOnAPIError
func ValidateTaskReadiness(
	ctx context.Context,
	client claude.Client,
	taskDescription string,
	specContent string,
	weight string,
) (bool, []string, error) {
	if client == nil {
		return true, nil, nil
	}

	// Trivial/small tasks don't need spec validation
	if weight == "trivial" || weight == "small" {
		return true, nil, nil
	}

	prompt := fmt.Sprintf(`You are evaluating whether a task specification is complete enough for implementation.

## Task Description
%s

## Task Weight
%s (higher weights require more thorough specs)

## Specification
%s

## Evaluation Criteria

For a %s task, the specification should have:
1. INTENT - Clear statement of why this work matters
2. SUCCESS CRITERIA - Specific, testable conditions for "done"
3. TESTING - How to verify the implementation works

Evaluate if this spec is ready for implementation.

Respond with EXACTLY "READY" or "NOT READY" on the first line.
If NOT READY, list specific improvements needed (one per line, starting with "- ").

Example response for good spec:
"READY"

Example response for bad spec:
"NOT READY
- Success criteria are vague - need specific measurable conditions
- No testing section - add how to verify the implementation
- Intent unclear - why does the user need this feature?"`, taskDescription, weight, specContent, weight)

	resp, err := client.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: prompt},
		},
		Model:       HaikuValidationModel,
		MaxTokens:   300,
		Temperature: 0,
	})

	if err != nil {
		// Return the error - let caller decide whether to fail open or closed
		slog.Warn("haiku spec validation API error",
			"error", err,
		)
		return true, nil, fmt.Errorf("spec validation API error: %w", err)
	}

	// Return error if response is nil (shouldn't happen, but be defensive)
	if resp == nil {
		return true, nil, fmt.Errorf("spec validation API returned nil response")
	}

	return parseReadinessResponse(resp.Content)
}

// parseValidationResponse extracts the decision and reason from Haiku's response.
func parseValidationResponse(content string) (ValidationDecision, string) {
	content = strings.TrimSpace(content)
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 0 {
		return ValidationContinue, ""
	}

	firstLine := strings.ToUpper(strings.TrimSpace(lines[0]))
	reason := ""
	if len(lines) > 1 {
		reason = strings.TrimSpace(lines[1])
	}

	switch {
	case strings.HasPrefix(firstLine, "CONTINUE"):
		return ValidationContinue, reason
	case strings.HasPrefix(firstLine, "RETRY"):
		return ValidationRetry, reason
	case strings.HasPrefix(firstLine, "STOP"):
		return ValidationStop, reason
	default:
		// Default to continue if response is malformed
		return ValidationContinue, ""
	}
}

// parseReadinessResponse extracts the readiness status and suggestions from Haiku's response.
func parseReadinessResponse(content string) (bool, []string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		// Fail open - empty response means ready
		return true, nil, nil
	}

	lines := strings.Split(content, "\n")
	firstLine := strings.ToUpper(strings.TrimSpace(lines[0]))
	ready := strings.HasPrefix(firstLine, "READY") && !strings.HasPrefix(firstLine, "NOT READY")

	var suggestions []string
	if !ready {
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
				// Remove the bullet point prefix
				suggestion := strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*")
				suggestion = strings.TrimSpace(suggestion)
				if suggestion != "" {
					suggestions = append(suggestions, suggestion)
				}
			}
		}
	}

	return ready, suggestions, nil
}
