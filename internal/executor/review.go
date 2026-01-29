package executor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
)

// ReviewDecisionStatus represents the status from a review round.
type ReviewDecisionStatus string

const (
	ReviewStatusPass           ReviewDecisionStatus = "pass"
	ReviewStatusFail           ReviewDecisionStatus = "fail"
	ReviewStatusNeedsUserInput ReviewDecisionStatus = "needs_user_input"
)

// ReviewFinding represents a single issue found during review.
type ReviewFinding struct {
	Severity              string `json:"severity"` // high, medium, low
	File                  string `json:"file,omitempty"`
	Line                  int    `json:"line,omitempty"`
	Description           string `json:"description"`
	Suggestion            string `json:"suggestion,omitempty"`
	AgentID               string `json:"agent_id,omitempty"`               // Which agent found this (e.g., "code-reviewer", "silent-failure-hunter")
	ConstitutionViolation string `json:"constitution_violation,omitempty"` // "invariant" (blocker) or "default" (warning)
}

// ReviewFindings represents the output of a review round.
type ReviewFindings struct {
	Status    string          `json:"status,omitempty"` // "complete" or "blocked" (empty treated as complete for backward compat)
	Round     int             `json:"round"`
	Summary   string          `json:"summary"`
	Issues    []ReviewFinding `json:"issues"`
	Questions []string        `json:"questions,omitempty"`
	Positives []string        `json:"positives,omitempty"`
	AgentID   string          `json:"agent_id,omitempty"` // Which agent produced these findings
}

// ReviewDecision represents the final decision from a review.
type ReviewDecision struct {
	Status          ReviewDecisionStatus `json:"status"`
	GapsAddressed   bool                 `json:"gaps_addressed"`
	Summary         string               `json:"summary"`
	IssuesResolved  []string             `json:"issues_resolved,omitempty"`
	RemainingIssues []ReviewFinding      `json:"remaining_issues,omitempty"`
	UserQuestions   []string             `json:"user_questions,omitempty"`
	Recommendation  string               `json:"recommendation"`
}

// ReviewResult represents the complete result of a multi-round review.
type ReviewResult struct {
	Round1Findings *ReviewFindings `json:"round1_findings,omitempty"`
	Round2Decision *ReviewDecision `json:"round2_decision,omitempty"`
	TotalRounds    int             `json:"total_rounds"`
	Passed         bool            `json:"passed"`
	NeedsUserInput bool            `json:"needs_user_input"`
}

// ShouldRunReview checks if review should run based on config and task weight.
func ShouldRunReview(cfg *config.Config, weight string) bool {
	if cfg == nil {
		return true // Default to run review
	}
	return cfg.Review.Enabled
}

// GetReviewRounds returns the number of review rounds from config.
func GetReviewRounds(cfg *config.Config) int {
	if cfg == nil || cfg.Review.Rounds < 1 {
		return 2 // Default to 2 rounds
	}
	return cfg.Review.Rounds
}

// JSON schemas for structured review output.
const (
	// ReviewFindingsSchema forces structured output for review findings.
	ReviewFindingsSchema = `{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"enum": ["complete", "blocked"],
				"description": "Whether review completed successfully or is blocked (e.g., no implementation exists to review)"
			},
			"round": {
				"type": "integer",
				"description": "The review round number"
			},
			"summary": {
				"type": "string",
				"description": "Brief overview of review findings"
			},
			"issues": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"severity": {
							"type": "string",
							"enum": ["high", "medium", "low"],
							"description": "Issue severity"
						},
						"file": {
							"type": "string",
							"description": "File path where issue was found"
						},
						"line": {
							"type": "integer",
							"description": "Line number of the issue"
						},
						"description": {
							"type": "string",
							"description": "Description of the issue"
						},
						"suggestion": {
							"type": "string",
							"description": "Suggested fix"
						}
					},
					"required": ["severity", "description"]
				},
				"description": "List of issues found"
			},
			"questions": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Questions requiring clarification"
			},
			"positives": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Positive aspects noted"
			}
		},
		"required": ["status", "round", "summary", "issues"]
	}`

	// ReviewDecisionSchema forces structured output for review decisions.
	ReviewDecisionSchema = `{
		"type": "object",
		"properties": {
			"status": {
				"type": "string",
				"enum": ["pass", "fail", "needs_user_input"],
				"description": "The review decision status"
			},
			"gaps_addressed": {
				"type": "boolean",
				"description": "Whether all gaps from previous round were addressed"
			},
			"summary": {
				"type": "string",
				"description": "Overall assessment of the implementation"
			},
			"issues_resolved": {
				"type": "array",
				"items": {"type": "string"},
				"description": "List of resolved issues from previous round"
			},
			"remaining_issues": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"severity": {
							"type": "string",
							"enum": ["high", "medium", "low"]
						},
						"description": {
							"type": "string"
						}
					},
					"required": ["severity", "description"]
				},
				"description": "Issues that still need attention"
			},
			"user_questions": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Questions requiring user decision"
			},
			"recommendation": {
				"type": "string",
				"description": "What should happen next"
			}
		},
		"required": ["status", "summary", "recommendation"]
	}`
)

// ParseReviewFindings parses JSON review findings from Claude's response.
func ParseReviewFindings(response string) (*ReviewFindings, error) {
	var findings ReviewFindings
	if err := json.Unmarshal([]byte(response), &findings); err != nil {
		return nil, fmt.Errorf("parse review findings JSON: %w", err)
	}

	// Initialize nil slices to empty
	if findings.Issues == nil {
		findings.Issues = []ReviewFinding{}
	}
	if findings.Questions == nil {
		findings.Questions = []string{}
	}
	if findings.Positives == nil {
		findings.Positives = []string{}
	}

	return &findings, nil
}

// ParseReviewDecision parses JSON review decision from Claude's response.
func ParseReviewDecision(response string) (*ReviewDecision, error) {
	var decision ReviewDecision
	if err := json.Unmarshal([]byte(response), &decision); err != nil {
		return nil, fmt.Errorf("parse review decision JSON: %w", err)
	}

	// Normalize status to lowercase
	decision.Status = ReviewDecisionStatus(strings.ToLower(string(decision.Status)))

	// Initialize nil slices to empty
	if decision.RemainingIssues == nil {
		decision.RemainingIssues = []ReviewFinding{}
	}
	if decision.IssuesResolved == nil {
		decision.IssuesResolved = []string{}
	}
	if decision.UserQuestions == nil {
		decision.UserQuestions = []string{}
	}

	return &decision, nil
}

// HasHighSeverityIssues checks if there are any high-severity issues.
func (f *ReviewFindings) HasHighSeverityIssues() bool {
	for _, issue := range f.Issues {
		if issue.Severity == "high" {
			return true
		}
	}
	return false
}

// CountBySeverity counts issues by severity level.
func (f *ReviewFindings) CountBySeverity() map[string]int {
	counts := map[string]int{
		"high":   0,
		"medium": 0,
		"low":    0,
	}
	for _, issue := range f.Issues {
		counts[issue.Severity]++
	}
	return counts
}

// FormatFindingsForRound2 formats Round 1 findings for injection into Round 2 prompt.
func FormatFindingsForRound2(findings *ReviewFindings) string {
	if findings == nil {
		return "No findings from Round 1."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Round 1 Summary: %s\n\n", findings.Summary))

	counts := findings.CountBySeverity()
	sb.WriteString(fmt.Sprintf("Issues Found: %d high, %d medium, %d low\n\n",
		counts["high"], counts["medium"], counts["low"]))

	if len(findings.Issues) > 0 {
		sb.WriteString("### Issues to Verify\n\n")
		for i, issue := range findings.Issues {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, strings.ToUpper(issue.Severity), issue.Description))
			if issue.File != "" {
				sb.WriteString(fmt.Sprintf(" (in %s", issue.File))
				if issue.Line > 0 {
					sb.WriteString(fmt.Sprintf(":%d", issue.Line))
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("   Suggested fix: %s\n", issue.Suggestion))
			}
		}
	}

	if len(findings.Positives) > 0 {
		sb.WriteString("\n### Positive Notes\n\n")
		for _, p := range findings.Positives {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	return sb.String()
}
