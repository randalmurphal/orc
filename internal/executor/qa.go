package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
)

// QAStatus represents the status from a QA session.
type QAStatus string

const (
	QAStatusPass           QAStatus = "pass"
	QAStatusFail           QAStatus = "fail"
	QAStatusNeedsAttention QAStatus = "needs_attention"
)

// QATest represents a test written during QA.
type QATest struct {
	File        string `json:"file"`
	Description string `json:"description"`
	Type        string `json:"type"` // e2e, integration, unit
}

// QATestRun represents test execution results.
type QATestRun struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// QACoverage represents code coverage information.
type QACoverage struct {
	Percentage     float64 `json:"percentage"`
	UncoveredAreas string  `json:"uncovered_areas,omitempty"`
}

// QADoc represents documentation created during QA.
type QADoc struct {
	File string `json:"file"`
	Type string `json:"type"` // feature, api, testing
}

// QAIssue represents an issue found during QA.
type QAIssue struct {
	Severity     string `json:"severity"` // high, medium, low
	Description  string `json:"description"`
	Reproduction string `json:"reproduction,omitempty"`
}

// QAResult represents the complete result of a QA session.
type QAResult struct {
	Status         QAStatus    `json:"status"`
	Summary        string      `json:"summary"`
	TestsWritten   []QATest    `json:"tests_written,omitempty"`
	TestsRun       *QATestRun  `json:"tests_run,omitempty"`
	Coverage       *QACoverage `json:"coverage,omitempty"`
	Documentation  []QADoc     `json:"documentation,omitempty"`
	Issues         []QAIssue   `json:"issues,omitempty"`
	Recommendation string      `json:"recommendation"`
}

// ShouldRunQA checks if QA should run based on config and task weight.
func ShouldRunQA(cfg *config.Config, weight string) bool {
	if cfg == nil {
		// Default: run QA except for trivial tasks
		return weight != "trivial"
	}
	if !cfg.QA.Enabled {
		return false
	}
	// Check if weight is in skip list
	for _, skip := range cfg.QA.SkipForWeights {
		if skip == weight {
			return false
		}
	}
	return true
}

// QAResultSchema forces structured output for QA results.
const QAResultSchema = `{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["pass", "fail", "needs_attention"],
			"description": "The QA session status"
		},
		"summary": {
			"type": "string",
			"description": "Summary of the QA session"
		},
		"tests_written": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"file": {"type": "string"},
					"description": {"type": "string"},
					"type": {"type": "string", "enum": ["e2e", "integration", "unit"]}
				},
				"required": ["file", "description", "type"]
			},
			"description": "Tests written during QA"
		},
		"tests_run": {
			"type": "object",
			"properties": {
				"total": {"type": "integer"},
				"passed": {"type": "integer"},
				"failed": {"type": "integer"},
				"skipped": {"type": "integer"}
			},
			"required": ["total", "passed", "failed", "skipped"],
			"description": "Test execution results"
		},
		"coverage": {
			"type": "object",
			"properties": {
				"percentage": {"type": "number"},
				"uncovered_areas": {"type": "string"}
			},
			"description": "Code coverage information"
		},
		"documentation": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"file": {"type": "string"},
					"type": {"type": "string", "enum": ["feature", "api", "testing"]}
				},
				"required": ["file", "type"]
			},
			"description": "Documentation created during QA"
		},
		"issues": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"severity": {"type": "string", "enum": ["high", "medium", "low"]},
					"description": {"type": "string"},
					"reproduction": {"type": "string"}
				},
				"required": ["severity", "description"]
			},
			"description": "Issues found during QA"
		},
		"recommendation": {
			"type": "string",
			"description": "Recommendation for next steps"
		}
	},
	"required": ["status", "summary", "recommendation"]
}`

// ParseQAResult parses JSON QA result from Claude's response.
func ParseQAResult(response string) (*QAResult, error) {
	var result QAResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse QA result JSON: %w", err)
	}

	// Normalize status to lowercase
	result.Status = QAStatus(strings.ToLower(string(result.Status)))

	// Initialize nil slices to empty
	if result.TestsWritten == nil {
		result.TestsWritten = []QATest{}
	}
	if result.Documentation == nil {
		result.Documentation = []QADoc{}
	}
	if result.Issues == nil {
		result.Issues = []QAIssue{}
	}

	return &result, nil
}

// ExtractQAResult extracts QA result from session output using a two-phase
// approach: direct JSON parsing, then LLM extraction with schema.
// This handles cases where sessions emit mixed text + JSON output.
func ExtractQAResult(ctx context.Context, client claude.Client, output string) (*QAResult, error) {
	var result QAResult
	err := claude.ExtractStructured(ctx, client, output, QAResultSchema, &result, &claude.ExtractStructuredOptions{
		Model:   "haiku",
		Context: "QA session result. Extract the status (pass/fail/needs_attention), summary, tests written, test run results, coverage info, documentation created, issues found, and recommendation.",
	})
	if err != nil {
		return nil, fmt.Errorf("extract QA result: %w", err)
	}

	// Normalize status to lowercase
	result.Status = QAStatus(strings.ToLower(string(result.Status)))

	// Initialize nil slices to empty
	if result.TestsWritten == nil {
		result.TestsWritten = []QATest{}
	}
	if result.Documentation == nil {
		result.Documentation = []QADoc{}
	}
	if result.Issues == nil {
		result.Issues = []QAIssue{}
	}

	return &result, nil
}

// HasHighSeverityIssues checks if there are any high-severity issues.
func (r *QAResult) HasHighSeverityIssues() bool {
	for _, issue := range r.Issues {
		if issue.Severity == "high" {
			return true
		}
	}
	return false
}

// AllTestsPassed checks if all tests passed.
func (r *QAResult) AllTestsPassed() bool {
	if r.TestsRun == nil {
		return true // No tests run, assume pass
	}
	return r.TestsRun.Failed == 0
}

// FormatQAResultSummary formats QA result for display.
func FormatQAResultSummary(result *QAResult) string {
	if result == nil {
		return "No QA result available."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("QA Status: %s\n\n", strings.ToUpper(string(result.Status))))
	sb.WriteString(fmt.Sprintf("Summary: %s\n\n", result.Summary))

	if result.TestsRun != nil {
		sb.WriteString(fmt.Sprintf("Tests: %d total, %d passed, %d failed, %d skipped\n",
			result.TestsRun.Total, result.TestsRun.Passed, result.TestsRun.Failed, result.TestsRun.Skipped))
	}

	if result.Coverage != nil {
		sb.WriteString(fmt.Sprintf("Coverage: %.1f%%\n", result.Coverage.Percentage))
	}

	if len(result.TestsWritten) > 0 {
		sb.WriteString(fmt.Sprintf("\nTests Written: %d\n", len(result.TestsWritten)))
		for _, t := range result.TestsWritten {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", t.File, t.Type, t.Description))
		}
	}

	if len(result.Documentation) > 0 {
		sb.WriteString(fmt.Sprintf("\nDocumentation: %d files\n", len(result.Documentation)))
		for _, d := range result.Documentation {
			sb.WriteString(fmt.Sprintf("  - %s (%s)\n", d.File, d.Type))
		}
	}

	if len(result.Issues) > 0 {
		sb.WriteString(fmt.Sprintf("\nIssues: %d\n", len(result.Issues)))
		for _, i := range result.Issues {
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", strings.ToUpper(i.Severity), i.Description))
		}
	}

	if result.Recommendation != "" {
		sb.WriteString(fmt.Sprintf("\nRecommendation: %s\n", result.Recommendation))
	}

	return sb.String()
}
