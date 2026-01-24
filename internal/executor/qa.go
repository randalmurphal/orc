package executor

import (
	"encoding/json"
	"fmt"
	"strings"

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

// =============================================================================
// QA E2E Types and Schema (Browser-based E2E testing with Playwright MCP)
// =============================================================================

// QAE2EFindingSeverity represents the severity of a QA E2E finding.
type QAE2EFindingSeverity string

const (
	QAE2ESeverityCritical QAE2EFindingSeverity = "critical"
	QAE2ESeverityHigh     QAE2EFindingSeverity = "high"
	QAE2ESeverityMedium   QAE2EFindingSeverity = "medium"
	QAE2ESeverityLow      QAE2EFindingSeverity = "low"
)

// QAE2EFindingCategory represents the category of a QA E2E finding.
type QAE2EFindingCategory string

const (
	QAE2ECategoryFunctional    QAE2EFindingCategory = "functional"
	QAE2ECategoryVisual        QAE2EFindingCategory = "visual"
	QAE2ECategoryAccessibility QAE2EFindingCategory = "accessibility"
	QAE2ECategoryPerformance   QAE2EFindingCategory = "performance"
)

// QAE2EFinding represents a single finding from browser-based E2E testing.
type QAE2EFinding struct {
	ID              string               `json:"id"`                        // e.g., "QA-001"
	Severity        QAE2EFindingSeverity `json:"severity"`                  // critical, high, medium, low
	Confidence      int                  `json:"confidence"`                // 0-100, only report >= 80
	Category        QAE2EFindingCategory `json:"category"`                  // functional, visual, accessibility, performance
	Title           string               `json:"title"`                     // Brief description
	StepsToReproduce []string            `json:"steps_to_reproduce"`        // Step-by-step reproduction
	Expected        string               `json:"expected"`                  // Expected behavior
	Actual          string               `json:"actual"`                    // Actual behavior
	ScreenshotPath  string               `json:"screenshot_path,omitempty"` // Path to screenshot evidence
	SuggestedFix    string               `json:"suggested_fix,omitempty"`   // Optional fix suggestion
}

// QAE2EVerification represents verification metadata.
type QAE2EVerification struct {
	ScenariosTested         int      `json:"scenarios_tested"`
	ViewportsTested         []string `json:"viewports_tested"`           // e.g., ["desktop", "mobile"]
	PreviousIssuesVerified  []string `json:"previous_issues_verified,omitempty"` // e.g., ["QA-001: FIXED", "QA-002: STILL_PRESENT"]
}

// QAE2ETestResult represents the complete result of a QA E2E testing session.
type QAE2ETestResult struct {
	Status       string            `json:"status"`  // "complete" or "blocked"
	Summary      string            `json:"summary"` // e.g., "Tested 15 scenarios, found 3 issues"
	Findings     []QAE2EFinding    `json:"findings"`
	Verification *QAE2EVerification `json:"verification,omitempty"`
}

// QAE2EFixResult represents the result of a QA E2E fix session.
type QAE2EFixResult struct {
	Status         string           `json:"status"`  // "complete" or "blocked"
	Summary        string           `json:"summary"` // e.g., "Fixed 2 of 3 issues"
	FixesApplied   []QAE2EFixApplied `json:"fixes_applied"`
	IssuesDeferred []QAE2EIssueDeferred `json:"issues_deferred,omitempty"`
}

// QAE2EFixApplied represents a single applied fix.
type QAE2EFixApplied struct {
	FindingID         string   `json:"finding_id"`
	Status            string   `json:"status"` // "fixed", "partial", "unable"
	FilesModified     []string `json:"files_modified"`
	ChangeDescription string   `json:"change_description"`
}

// QAE2EIssueDeferred represents a deferred issue.
type QAE2EIssueDeferred struct {
	FindingID string `json:"finding_id"`
	Reason    string `json:"reason"`
}

// QAE2ETestResultSchema is the JSON schema for qa_e2e_test phase output.
const QAE2ETestResultSchema = `{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete", "blocked"],
			"description": "Phase status: complete (testing done), blocked (cannot test)"
		},
		"summary": {
			"type": "string",
			"description": "Brief summary of testing session (e.g., 'Tested 15 scenarios, found 3 issues')"
		},
		"findings": {
			"type": "array",
			"description": "Issues found during testing. Only include findings with confidence >= 80.",
			"items": {
				"type": "object",
				"properties": {
					"id": {"type": "string", "description": "Unique finding ID (e.g., QA-001)"},
					"severity": {"type": "string", "enum": ["critical", "high", "medium", "low"]},
					"confidence": {"type": "integer", "minimum": 0, "maximum": 100, "description": "Confidence score (0-100). Only report >= 80."},
					"category": {"type": "string", "enum": ["functional", "visual", "accessibility", "performance"]},
					"title": {"type": "string", "description": "Brief description of the issue"},
					"steps_to_reproduce": {"type": "array", "items": {"type": "string"}, "description": "Step-by-step reproduction instructions"},
					"expected": {"type": "string", "description": "Expected behavior"},
					"actual": {"type": "string", "description": "Actual behavior observed"},
					"screenshot_path": {"type": "string", "description": "Path to screenshot evidence"},
					"suggested_fix": {"type": "string", "description": "Optional: where to look for the fix"}
				},
				"required": ["id", "severity", "confidence", "category", "title", "steps_to_reproduce", "expected", "actual"]
			}
		},
		"verification": {
			"type": "object",
			"description": "Testing session metadata",
			"properties": {
				"scenarios_tested": {"type": "integer", "description": "Number of test scenarios executed"},
				"viewports_tested": {"type": "array", "items": {"type": "string"}, "description": "Viewports tested (e.g., desktop, mobile)"},
				"previous_issues_verified": {"type": "array", "items": {"type": "string"}, "description": "Previous findings verified (e.g., 'QA-001: FIXED')"}
			}
		}
	},
	"required": ["status", "summary", "findings"]
}`

// QAE2EFixResultSchema is the JSON schema for qa_e2e_fix phase output.
const QAE2EFixResultSchema = `{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete", "blocked"],
			"description": "Phase status"
		},
		"summary": {
			"type": "string",
			"description": "Brief summary of fixes applied (e.g., 'Fixed 2 of 3 issues')"
		},
		"fixes_applied": {
			"type": "array",
			"description": "Fixes that were applied",
			"items": {
				"type": "object",
				"properties": {
					"finding_id": {"type": "string", "description": "ID of the finding that was fixed (e.g., QA-001)"},
					"status": {"type": "string", "enum": ["fixed", "partial", "unable"], "description": "Fix status"},
					"files_modified": {"type": "array", "items": {"type": "string"}, "description": "Files that were modified"},
					"change_description": {"type": "string", "description": "Description of the change made"}
				},
				"required": ["finding_id", "status", "files_modified", "change_description"]
			}
		},
		"issues_deferred": {
			"type": "array",
			"description": "Issues that were deferred and not fixed",
			"items": {
				"type": "object",
				"properties": {
					"finding_id": {"type": "string", "description": "ID of the deferred finding"},
					"reason": {"type": "string", "description": "Reason for deferring"}
				},
				"required": ["finding_id", "reason"]
			}
		}
	},
	"required": ["status", "summary", "fixes_applied"]
}`

// ParseQAE2ETestResult parses JSON QA E2E test result from Claude's response.
func ParseQAE2ETestResult(response string) (*QAE2ETestResult, error) {
	var result QAE2ETestResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse QA E2E test result JSON: %w", err)
	}

	// Initialize nil slices to empty
	if result.Findings == nil {
		result.Findings = []QAE2EFinding{}
	}

	return &result, nil
}

// ParseQAE2EFixResult parses JSON QA E2E fix result from Claude's response.
func ParseQAE2EFixResult(response string) (*QAE2EFixResult, error) {
	var result QAE2EFixResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("parse QA E2E fix result JSON: %w", err)
	}

	// Initialize nil slices to empty
	if result.FixesApplied == nil {
		result.FixesApplied = []QAE2EFixApplied{}
	}
	if result.IssuesDeferred == nil {
		result.IssuesDeferred = []QAE2EIssueDeferred{}
	}

	return &result, nil
}

// HasFindings returns true if the QA E2E test result has any findings.
func (r *QAE2ETestResult) HasFindings() bool {
	return len(r.Findings) > 0
}

// HighSeverityCount returns the number of critical or high severity findings.
func (r *QAE2ETestResult) HighSeverityCount() int {
	count := 0
	for _, f := range r.Findings {
		if f.Severity == QAE2ESeverityCritical || f.Severity == QAE2ESeverityHigh {
			count++
		}
	}
	return count
}

// FormatFindingsForFix formats findings for the fix phase prompt.
func (r *QAE2ETestResult) FormatFindingsForFix() string {
	if len(r.Findings) == 0 {
		return "No findings to fix."
	}

	var sb strings.Builder
	for _, f := range r.Findings {
		sb.WriteString(fmt.Sprintf("### %s [%s] - %s\n\n", f.ID, strings.ToUpper(string(f.Severity)), f.Title))
		sb.WriteString(fmt.Sprintf("**Category:** %s\n", f.Category))
		sb.WriteString(fmt.Sprintf("**Confidence:** %d\n\n", f.Confidence))
		sb.WriteString("**Steps to Reproduce:**\n")
		for i, step := range f.StepsToReproduce {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString(fmt.Sprintf("\n**Expected:** %s\n", f.Expected))
		sb.WriteString(fmt.Sprintf("**Actual:** %s\n", f.Actual))
		if f.ScreenshotPath != "" {
			sb.WriteString(fmt.Sprintf("**Screenshot:** %s\n", f.ScreenshotPath))
		}
		if f.SuggestedFix != "" {
			sb.WriteString(fmt.Sprintf("**Suggested Fix:** %s\n", f.SuggestedFix))
		}
		sb.WriteString("\n---\n\n")
	}

	return sb.String()
}

// FormatQAE2EResultSummary formats QA E2E test result for display.
func FormatQAE2EResultSummary(result *QAE2ETestResult) string {
	if result == nil {
		return "No QA E2E result available."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("QA E2E Status: %s\n\n", strings.ToUpper(result.Status)))
	sb.WriteString(fmt.Sprintf("Summary: %s\n\n", result.Summary))

	if result.Verification != nil {
		sb.WriteString(fmt.Sprintf("Scenarios Tested: %d\n", result.Verification.ScenariosTested))
		if len(result.Verification.ViewportsTested) > 0 {
			sb.WriteString(fmt.Sprintf("Viewports: %s\n", strings.Join(result.Verification.ViewportsTested, ", ")))
		}
	}

	if len(result.Findings) > 0 {
		sb.WriteString(fmt.Sprintf("\nFindings: %d total (%d critical/high)\n", len(result.Findings), result.HighSeverityCount()))
		for _, f := range result.Findings {
			sb.WriteString(fmt.Sprintf("  - [%s] %s: %s\n", strings.ToUpper(string(f.Severity)), f.ID, f.Title))
		}
	} else {
		sb.WriteString("\nNo issues found - PASS\n")
	}

	return sb.String()
}
