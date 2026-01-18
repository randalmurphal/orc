// Package executor provides the flowgraph-based execution engine for orc.
// This file contains Haiku-based validation functions for objective quality assessment.
package executor

import (
	"context"
	"encoding/json"
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

// JSON schemas for structured validation output.
// Using schemas ensures consistent, parseable output.
const (
	// iterationProgressSchema forces structured output for progress validation.
	iterationProgressSchema = `{
		"type": "object",
		"properties": {
			"decision": {
				"type": "string",
				"enum": ["CONTINUE", "RETRY", "STOP"],
				"description": "CONTINUE if on track, RETRY if off track, STOP if blocked"
			},
			"reason": {
				"type": "string",
				"description": "Brief explanation of the decision"
			}
		},
		"required": ["decision", "reason"]
	}`

	// taskReadinessSchema forces structured output for spec validation.
	taskReadinessSchema = `{
		"type": "object",
		"properties": {
			"ready": {
				"type": "boolean",
				"description": "true if spec is ready for implementation, false otherwise"
			},
			"suggestions": {
				"type": "array",
				"items": {"type": "string"},
				"description": "List of specific improvements needed (empty if ready)"
			}
		},
		"required": ["ready", "suggestions"]
	}`
)

// progressResponse is the JSON structure for iteration progress validation.
type progressResponse struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// readinessResponse is the JSON structure for spec readiness validation.
type readinessResponse struct {
	Ready       bool     `json:"ready"`
	Suggestions []string `json:"suggestions"`
}

// ValidateIterationProgress uses Haiku to assess whether an iteration is on track.
// It evaluates the iteration output against the spec's success criteria.
//
// Returns:
//   - ValidationContinue: The work is progressing toward the success criteria
//   - ValidationRetry: The approach has diverged, needs redirection
//   - ValidationStop: Fundamentally blocked, cannot proceed
//
// On error (API failure, timeout, parse failure), returns the error to let caller decide:
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

	prompt := fmt.Sprintf(`Evaluate whether an AI agent's work is progressing toward the success criteria.

## Specification
%s

## Agent's Latest Output
%s

## Task
Assess if the work is:
- ON TRACK: Making progress toward success criteria → decision: "CONTINUE"
- OFF TRACK: Wrong approach, scope creep, misunderstanding → decision: "RETRY"
- BLOCKED: Missing dependencies, impossible requirements → decision: "STOP"`, specContent, truncatedOutput)

	resp, err := client.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: prompt},
		},
		Model:       HaikuValidationModel,
		MaxTokens:   200,
		Temperature: 0,
		JSONSchema:  iterationProgressSchema,
	})

	if err != nil {
		slog.Warn("haiku validation API error", "error", err)
		return ValidationContinue, "", fmt.Errorf("validation API error: %w", err)
	}

	if resp == nil {
		return ValidationContinue, "", fmt.Errorf("validation API returned nil response")
	}

	// Parse JSON response
	var result progressResponse
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		slog.Warn("haiku validation parse error",
			"error", err,
			"content", resp.Content,
		)
		return ValidationContinue, "", fmt.Errorf("validation parse error: %w", err)
	}

	decision := strings.ToUpper(result.Decision)
	switch decision {
	case "CONTINUE":
		return ValidationContinue, result.Reason, nil
	case "RETRY":
		return ValidationRetry, result.Reason, nil
	case "STOP":
		return ValidationStop, result.Reason, nil
	default:
		return ValidationContinue, "", fmt.Errorf("unexpected decision: %s", result.Decision)
	}
}

// ValidateTaskReadiness checks if a task has a quality spec before execution.
// This is a pre-execution gate to catch poorly-specified tasks before they waste
// compute on doomed implementations.
//
// Returns:
//   - ready: true if the spec is sufficient for execution
//   - suggestions: list of improvements if not ready
//   - error: on API/parse failures, returned to let caller decide based on config.Validation.FailOnAPIError
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

	prompt := fmt.Sprintf(`Evaluate whether this task specification is complete enough for implementation.

## Task Description
%s

## Task Weight
%s (higher weights require more thorough specs)

## Specification
%s

## Criteria
For a %s task, the spec should have:
1. INTENT - Clear statement of why this work matters
2. SUCCESS CRITERIA - Specific, testable conditions for "done"
3. TESTING - How to verify the implementation works

Set ready=true only if all criteria are met. Otherwise, list specific improvements needed.`, taskDescription, weight, specContent, weight)

	resp, err := client.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: prompt},
		},
		Model:       HaikuValidationModel,
		MaxTokens:   300,
		Temperature: 0,
		JSONSchema:  taskReadinessSchema,
	})

	if err != nil {
		slog.Warn("haiku spec validation API error", "error", err)
		return true, nil, fmt.Errorf("spec validation API error: %w", err)
	}

	if resp == nil {
		return true, nil, fmt.Errorf("spec validation API returned nil response")
	}

	// Parse JSON response
	var result readinessResponse
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		slog.Warn("haiku spec validation parse error",
			"error", err,
			"content", resp.Content,
		)
		return true, nil, fmt.Errorf("spec validation parse error: %w", err)
	}

	return result.Ready, result.Suggestions, nil
}
