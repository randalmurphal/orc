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

// JSON schemas for structured validation output.
// Using schemas ensures consistent, parseable output.
const (
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
	sb.WriteString("**IMPORTANT:** External validation determined that NOT ALL success criteria from the spec are satisfied.\n\n")
	sb.WriteString("You MUST:\n")
	sb.WriteString("1. Re-read the full specification carefully\n")
	sb.WriteString("2. Study each finding below to understand why it failed\n")
	sb.WriteString("3. Verify that your implementation actually meets 100% of the criteria\n")
	sb.WriteString("4. Fix any gaps before claiming completion again\n\n")

	sb.WriteString("### Failed Criteria\n\n")
	for _, c := range result.Criteria {
		if c.Status != "MET" {
			sb.WriteString(fmt.Sprintf("**%s: %s**\n", c.ID, c.Status))
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("- Criterion: %s\n", c.Description))
			}
			sb.WriteString(fmt.Sprintf("- Issue: %s\n\n", c.Reason))
		}
	}

	if result.MissingSummary != "" {
		sb.WriteString("### Summary\n")
		sb.WriteString(result.MissingSummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Do not claim completion until ALL criteria are fully satisfied. If you believe a criterion is already met but was marked NOT_MET, verify your implementation by re-reading the relevant code and tests.\n")
	return sb.String()
}
