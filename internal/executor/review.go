package executor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// ReviewDecisionStatus represents the status from a review round.
type ReviewDecisionStatus string

const (
	ReviewStatusPass           ReviewDecisionStatus = "pass"
	ReviewStatusFail           ReviewDecisionStatus = "fail"
	ReviewStatusNeedsUserInput ReviewDecisionStatus = "needs_user_input"
)

// ReviewerPerspective defines the focus area for a reviewer agent.
type ReviewerPerspective string

const (
	// PerspectiveCorrectness focuses on whether the code works correctly.
	// Checks: logic errors, edge cases, error handling, input validation.
	PerspectiveCorrectness ReviewerPerspective = "correctness"

	// PerspectiveArchitecture focuses on design and maintainability.
	// Checks: design patterns, separation of concerns, DRY, testability.
	PerspectiveArchitecture ReviewerPerspective = "architecture"

	// PerspectiveSecurity focuses on security vulnerabilities.
	// Checks: OWASP top 10, input sanitization, auth/authz, secrets.
	PerspectiveSecurity ReviewerPerspective = "security"

	// PerspectivePerformance focuses on efficiency and resource usage.
	// Checks: O(n) complexity, memory allocation, database queries.
	PerspectivePerformance ReviewerPerspective = "performance"
)

// AllReviewPerspectives returns all available reviewer perspectives.
func AllReviewPerspectives() []ReviewerPerspective {
	return []ReviewerPerspective{
		PerspectiveCorrectness,
		PerspectiveArchitecture,
		PerspectiveSecurity,
		PerspectivePerformance,
	}
}

// DefaultReviewPerspectives returns the default perspectives for parallel review.
// For most tasks, correctness and architecture are the most valuable.
func DefaultReviewPerspectives() []ReviewerPerspective {
	return []ReviewerPerspective{
		PerspectiveCorrectness,
		PerspectiveArchitecture,
	}
}

// ReviewFinding represents a single issue found during review.
type ReviewFinding struct {
	Severity    string              `json:"severity"` // high, medium, low
	File        string              `json:"file,omitempty"`
	Line        int                 `json:"line,omitempty"`
	Description string              `json:"description"`
	Suggestion  string              `json:"suggestion,omitempty"`
	Perspective ReviewerPerspective `json:"perspective,omitempty"` // Which reviewer found this
}

// ReviewFindings represents the output of a review round.
type ReviewFindings struct {
	Round       int                 `json:"round"`
	Summary     string              `json:"summary"`
	Issues      []ReviewFinding     `json:"issues"`
	Questions   []string            `json:"questions,omitempty"`
	Positives   []string            `json:"positives,omitempty"`
	Perspective ReviewerPerspective `json:"perspective,omitempty"` // Which perspective produced these findings
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
		"required": ["round", "summary", "issues"]
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

// -----------------------------------------------------------------------------
// Parallel Review Support
// -----------------------------------------------------------------------------

// ParallelReviewResult aggregates findings from multiple parallel reviewers.
type ParallelReviewResult struct {
	Perspectives []ReviewerPerspective       `json:"perspectives"` // Which perspectives were used
	Findings     map[ReviewerPerspective]*ReviewFindings `json:"findings"` // Findings per perspective
	Merged       *ReviewFindings             `json:"merged"`       // Deduplicated combined findings
	Duration     int64                       `json:"duration_ms"`  // Total review duration
}

// NewParallelReviewResult creates a new parallel review result.
func NewParallelReviewResult(perspectives []ReviewerPerspective) *ParallelReviewResult {
	return &ParallelReviewResult{
		Perspectives: perspectives,
		Findings:     make(map[ReviewerPerspective]*ReviewFindings),
	}
}

// AddFindings adds findings from a specific perspective.
func (pr *ParallelReviewResult) AddFindings(perspective ReviewerPerspective, findings *ReviewFindings) {
	if findings != nil {
		findings.Perspective = perspective
		// Tag each issue with its source perspective
		for i := range findings.Issues {
			findings.Issues[i].Perspective = perspective
		}
		pr.Findings[perspective] = findings
	}
}

// Merge combines all findings into a single deduplicated result.
func (pr *ParallelReviewResult) Merge() *ReviewFindings {
	if pr.Merged != nil {
		return pr.Merged
	}

	merged := &ReviewFindings{
		Round:     1,
		Issues:    []ReviewFinding{},
		Questions: []string{},
		Positives: []string{},
	}

	// Collect all summaries
	var summaries []string

	// Collect and deduplicate issues
	seenIssues := make(map[string]bool)

	for _, perspective := range pr.Perspectives {
		findings, ok := pr.Findings[perspective]
		if !ok || findings == nil {
			continue
		}

		if findings.Summary != "" {
			summaries = append(summaries, fmt.Sprintf("[%s] %s", perspective, findings.Summary))
		}

		for _, issue := range findings.Issues {
			// Create a key for deduplication based on file+line+description
			key := issueKey(issue)
			if !seenIssues[key] {
				seenIssues[key] = true
				merged.Issues = append(merged.Issues, issue)
			}
		}

		// Questions are typically unique, collect all
		merged.Questions = append(merged.Questions, findings.Questions...)

		// Deduplicate positives
		for _, p := range findings.Positives {
			if !stringSliceContains(merged.Positives, p) {
				merged.Positives = append(merged.Positives, p)
			}
		}
	}

	// Combine summaries
	if len(summaries) > 0 {
		merged.Summary = strings.Join(summaries, "\n")
	}

	// Sort issues by severity (high > medium > low) then by file
	sort.Slice(merged.Issues, func(i, j int) bool {
		si := severityRank(merged.Issues[i].Severity)
		sj := severityRank(merged.Issues[j].Severity)
		if si != sj {
			return si > sj
		}
		return merged.Issues[i].File < merged.Issues[j].File
	})

	pr.Merged = merged
	return merged
}

// TotalIssues returns the count of all unique issues across perspectives.
func (pr *ParallelReviewResult) TotalIssues() int {
	if pr.Merged == nil {
		pr.Merge()
	}
	return len(pr.Merged.Issues)
}

// HasHighSeverityIssues returns true if any perspective found high-severity issues.
func (pr *ParallelReviewResult) HasHighSeverityIssues() bool {
	if pr.Merged == nil {
		pr.Merge()
	}
	return pr.Merged.HasHighSeverityIssues()
}

// issueKey generates a deduplication key for a finding.
func issueKey(f ReviewFinding) string {
	// Normalize and hash the key components
	desc := strings.ToLower(strings.TrimSpace(f.Description))
	if f.File != "" {
		return fmt.Sprintf("%s:%d:%s", f.File, f.Line, desc)
	}
	return desc
}

// severityRank returns a numeric rank for sorting (higher = more severe).
func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// stringSliceContains checks if a slice contains a string.
func stringSliceContains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ShouldRunParallelReview determines if parallel reviewers should be used.
// Parallel review is recommended for medium+ weight tasks where the cost
// of multiple reviewers is justified by catching diverse issues.
func ShouldRunParallelReview(cfg *config.Config, weight task.Weight) bool {
	if cfg == nil {
		return false
	}

	// Check if parallel review is explicitly enabled
	if !cfg.Review.Parallel.Enabled {
		return false
	}

	// Only use parallel review for medium/large/greenfield tasks
	switch weight {
	case task.WeightMedium, task.WeightLarge, task.WeightGreenfield:
		return true
	default:
		return false
	}
}

// GetReviewPerspectives returns the perspectives to use for parallel review.
func GetReviewPerspectives(cfg *config.Config) []ReviewerPerspective {
	if cfg == nil || len(cfg.Review.Parallel.Perspectives) == 0 {
		return DefaultReviewPerspectives()
	}

	perspectives := make([]ReviewerPerspective, 0, len(cfg.Review.Parallel.Perspectives))
	for _, p := range cfg.Review.Parallel.Perspectives {
		perspectives = append(perspectives, ReviewerPerspective(p))
	}
	return perspectives
}

// FormatParallelReviewSummary formats parallel review results for display.
func FormatParallelReviewSummary(pr *ParallelReviewResult) string {
	if pr == nil {
		return "No parallel review results."
	}

	var sb strings.Builder
	sb.WriteString("## Parallel Review Summary\n\n")
	sb.WriteString(fmt.Sprintf("**Perspectives Used:** %d\n", len(pr.Perspectives)))

	for _, perspective := range pr.Perspectives {
		findings, ok := pr.Findings[perspective]
		if !ok || findings == nil {
			sb.WriteString(fmt.Sprintf("- %s: No findings\n", perspective))
			continue
		}
		counts := findings.CountBySeverity()
		sb.WriteString(fmt.Sprintf("- %s: %d high, %d medium, %d low\n",
			perspective, counts["high"], counts["medium"], counts["low"]))
	}

	// Merged totals
	merged := pr.Merge()
	counts := merged.CountBySeverity()
	sb.WriteString(fmt.Sprintf("\n**Merged Totals:** %d issues (%d high, %d medium, %d low)\n",
		len(merged.Issues), counts["high"], counts["medium"], counts["low"]))

	return sb.String()
}

// GetPerspectivePromptContext returns additional context for a specific reviewer perspective.
func GetPerspectivePromptContext(perspective ReviewerPerspective) string {
	switch perspective {
	case PerspectiveCorrectness:
		return `Focus your review on **correctness**:
- Does the code work correctly for all inputs?
- Are edge cases handled?
- Is error handling comprehensive?
- Are there logic errors or off-by-one bugs?
- Is input validation sufficient?`

	case PerspectiveArchitecture:
		return `Focus your review on **architecture and maintainability**:
- Does the design follow established patterns?
- Is there proper separation of concerns?
- Is code DRY (Don't Repeat Yourself)?
- Is the code testable?
- Are dependencies managed correctly?`

	case PerspectiveSecurity:
		return `Focus your review on **security**:
- OWASP Top 10 vulnerabilities
- Input sanitization and validation
- Authentication and authorization
- Secrets handling and exposure
- SQL injection, XSS, CSRF risks`

	case PerspectivePerformance:
		return `Focus your review on **performance**:
- Algorithm complexity (Big O)
- Memory allocation patterns
- Database query efficiency
- Resource cleanup
- Caching opportunities`

	default:
		return ""
	}
}
