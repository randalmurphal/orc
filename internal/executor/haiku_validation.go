// Package executor provides the flowgraph-based execution engine for orc.
// This file contains Haiku-based validation functions for objective quality assessment.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"text/template"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/llmutil"
	"github.com/randalmurphal/orc/templates"
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

	// criteriaCompletionSchema forces structured output for success criteria validation.
	criteriaCompletionSchema = `{
		"type": "object",
		"properties": {
			"all_met": {
				"type": "boolean",
				"description": "true if ALL success criteria are satisfied, false if any are missing"
			},
			"criteria": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {
							"type": "string",
							"description": "Criterion ID (e.g., SC-1)"
						},
						"description": {
							"type": "string",
							"description": "Brief description of the criterion"
						},
						"status": {
							"type": "string",
							"enum": ["MET", "NOT_MET", "PARTIAL"],
							"description": "Whether this criterion is satisfied"
						},
						"reason": {
							"type": "string",
							"description": "Why it is or isn't met"
						}
					},
					"required": ["id", "status", "reason"]
				},
				"description": "Status of each success criterion"
			},
			"missing_summary": {
				"type": "string",
				"description": "Brief summary of what's still needed (empty if all_met)"
			}
		},
		"required": ["all_met", "criteria", "missing_summary"]
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

// CriterionStatus represents the status of a single success criterion.
type CriterionStatus struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"` // MET, NOT_MET, PARTIAL
	Reason      string `json:"reason"`
}

// criteriaCompletionResponse is the JSON structure for success criteria validation.
type criteriaCompletionResponse struct {
	AllMet         bool              `json:"all_met"`
	Criteria       []CriterionStatus `json:"criteria"`
	MissingSummary string            `json:"missing_summary"`
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

	// Load and execute template
	tmplContent, err := templates.Prompts.ReadFile("prompts/haiku_iteration_progress.md")
	if err != nil {
		return ValidationContinue, "", fmt.Errorf("read iteration progress template: %w", err)
	}

	tmpl, err := template.New("iteration_progress").Parse(string(tmplContent))
	if err != nil {
		return ValidationContinue, "", fmt.Errorf("parse iteration progress template: %w", err)
	}

	data := map[string]any{
		"SpecContent":     specContent,
		"IterationOutput": truncatedOutput,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return ValidationContinue, "", fmt.Errorf("execute iteration progress template: %w", err)
	}
	prompt := buf.String()

	// Use consolidated schema executor - no fallbacks, explicit errors
	schemaResult, err := llmutil.ExecuteWithSchema[progressResponse](ctx, client, prompt, iterationProgressSchema)
	if err != nil {
		slog.Warn("haiku validation failed", "error", err)
		return ValidationContinue, "", fmt.Errorf("validation failed: %w", err)
	}

	decision := strings.ToUpper(schemaResult.Data.Decision)
	switch decision {
	case "CONTINUE":
		return ValidationContinue, schemaResult.Data.Reason, nil
	case "RETRY":
		return ValidationRetry, schemaResult.Data.Reason, nil
	case "STOP":
		return ValidationStop, schemaResult.Data.Reason, nil
	default:
		return ValidationContinue, "", fmt.Errorf("unexpected decision: %s", schemaResult.Data.Decision)
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

	// Load and execute template
	tmplContent, err := templates.Prompts.ReadFile("prompts/haiku_task_readiness.md")
	if err != nil {
		return true, nil, fmt.Errorf("read task readiness template: %w", err)
	}

	tmpl, err := template.New("task_readiness").Parse(string(tmplContent))
	if err != nil {
		return true, nil, fmt.Errorf("parse task readiness template: %w", err)
	}

	data := map[string]any{
		"TaskDescription": taskDescription,
		"Weight":          weight,
		"SpecContent":     specContent,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return true, nil, fmt.Errorf("execute task readiness template: %w", err)
	}
	prompt := buf.String()

	// Use consolidated schema executor - no fallbacks, explicit errors
	schemaResult, err := llmutil.ExecuteWithSchema[readinessResponse](ctx, client, prompt, taskReadinessSchema)
	if err != nil {
		slog.Warn("haiku spec validation failed", "error", err)
		return true, nil, fmt.Errorf("spec validation failed: %w", err)
	}

	return schemaResult.Data.Ready, schemaResult.Data.Suggestions, nil
}

// CriteriaValidationResult holds the result of success criteria validation.
type CriteriaValidationResult struct {
	AllMet         bool
	Criteria       []CriterionStatus
	MissingSummary string
}

// ValidateSuccessCriteria checks if all success criteria from the spec are satisfied.
// This is the key gate for implement phase completion - it ensures the agent has
// actually done what the spec requires, not just claimed completion.
//
// Returns:
//   - result: detailed status of each criterion
//   - error: on API/parse failures
//
// The caller should check result.AllMet to decide whether to accept phase completion.
func ValidateSuccessCriteria(
	ctx context.Context,
	client claude.Client,
	specContent string,
	implementationSummary string,
) (*CriteriaValidationResult, error) {
	if client == nil {
		// No client = skip validation (optimistic)
		return &CriteriaValidationResult{AllMet: true}, nil
	}

	if specContent == "" {
		// No spec = can't validate criteria
		return &CriteriaValidationResult{AllMet: true}, nil
	}

	// Truncate implementation summary to keep costs reasonable
	maxSummaryLen := 6000
	truncatedSummary := implementationSummary
	if len(implementationSummary) > maxSummaryLen {
		truncatedSummary = implementationSummary[:maxSummaryLen] + "\n...[truncated]"
	}

	// Load and execute template
	tmplContent, err := templates.Prompts.ReadFile("prompts/haiku_success_criteria.md")
	if err != nil {
		return nil, fmt.Errorf("read success criteria template: %w", err)
	}

	tmpl, err := template.New("success_criteria").Parse(string(tmplContent))
	if err != nil {
		return nil, fmt.Errorf("parse success criteria template: %w", err)
	}

	data := map[string]any{
		"SpecContent":           specContent,
		"ImplementationSummary": truncatedSummary,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute success criteria template: %w", err)
	}
	prompt := buf.String()

	// Use consolidated schema executor - no fallbacks, explicit errors
	schemaResult, err := llmutil.ExecuteWithSchema[criteriaCompletionResponse](ctx, client, prompt, criteriaCompletionSchema)
	if err != nil {
		slog.Warn("criteria validation failed", "error", err)
		return nil, fmt.Errorf("criteria validation failed: %w", err)
	}

	return &CriteriaValidationResult{
		AllMet:         schemaResult.Data.AllMet,
		Criteria:       schemaResult.Data.Criteria,
		MissingSummary: schemaResult.Data.MissingSummary,
	}, nil
}

// FormatCriteriaFeedback formats missing criteria as actionable feedback for the agent.
func FormatCriteriaFeedback(result *CriteriaValidationResult) string {
	if result == nil || result.AllMet {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Criteria Validation Failed\n\n")
	sb.WriteString("Not all success criteria from the spec are satisfied. You must address the following:\n\n")

	for _, c := range result.Criteria {
		if c.Status != "MET" {
			sb.WriteString(fmt.Sprintf("### %s: %s\n", c.ID, c.Status))
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("**Criterion:** %s\n", c.Description))
			}
			sb.WriteString(fmt.Sprintf("**Issue:** %s\n\n", c.Reason))
		}
	}

	if result.MissingSummary != "" {
		sb.WriteString("### Summary\n")
		sb.WriteString(result.MissingSummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Please address all NOT_MET and PARTIAL criteria before claiming completion.\n")
	return sb.String()
}
