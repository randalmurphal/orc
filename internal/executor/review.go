package executor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
)

// ReviewDecisionStatus represents the status from a review round.
type ReviewDecisionStatus string

const (
	ReviewStatusPass          ReviewDecisionStatus = "pass"
	ReviewStatusFail          ReviewDecisionStatus = "fail"
	ReviewStatusNeedsUserInput ReviewDecisionStatus = "needs_user_input"
)

// ReviewFinding represents a single issue found during review.
type ReviewFinding struct {
	Severity    string `json:"severity"` // high, medium, low
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ReviewFindings represents the output of a review round.
type ReviewFindings struct {
	Round     int             `json:"round"`
	Summary   string          `json:"summary"`
	Issues    []ReviewFinding `json:"issues"`
	Questions []string        `json:"questions,omitempty"`
	Positives []string        `json:"positives,omitempty"`
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

// ParseReviewFindings extracts review findings from Claude's response.
func ParseReviewFindings(response string) (*ReviewFindings, error) {
	findings := &ReviewFindings{
		Issues: []ReviewFinding{},
	}

	// Extract <review_findings> block
	findingsRe := regexp.MustCompile(`(?s)<review_findings>(.*?)</review_findings>`)
	findingsMatch := findingsRe.FindStringSubmatch(response)
	if findingsMatch == nil {
		return nil, fmt.Errorf("no <review_findings> block found in response")
	}
	content := findingsMatch[1]

	// Parse round
	roundRe := regexp.MustCompile(`<round>(\d+)</round>`)
	if m := roundRe.FindStringSubmatch(content); m != nil {
		fmt.Sscanf(m[1], "%d", &findings.Round)
	}

	// Parse summary
	summaryRe := regexp.MustCompile(`<summary>(.*?)</summary>`)
	if m := summaryRe.FindStringSubmatch(content); m != nil {
		findings.Summary = strings.TrimSpace(m[1])
	}

	// Parse issues
	issueRe := regexp.MustCompile(`(?s)<issue severity="(high|medium|low)">(.*?)</issue>`)
	issueMatches := issueRe.FindAllStringSubmatch(content, -1)
	for _, m := range issueMatches {
		issue := ReviewFinding{
			Severity: m[1],
		}
		issueContent := m[2]

		// Parse file
		if fm := regexp.MustCompile(`<file>(.*?)</file>`).FindStringSubmatch(issueContent); fm != nil {
			issue.File = strings.TrimSpace(fm[1])
		}
		// Parse line
		if lm := regexp.MustCompile(`<line>(\d+)</line>`).FindStringSubmatch(issueContent); lm != nil {
			fmt.Sscanf(lm[1], "%d", &issue.Line)
		}
		// Parse description
		if dm := regexp.MustCompile(`<description>(.*?)</description>`).FindStringSubmatch(issueContent); dm != nil {
			issue.Description = strings.TrimSpace(dm[1])
		}
		// Parse suggestion
		if sm := regexp.MustCompile(`<suggestion>(.*?)</suggestion>`).FindStringSubmatch(issueContent); sm != nil {
			issue.Suggestion = strings.TrimSpace(sm[1])
		}

		findings.Issues = append(findings.Issues, issue)
	}

	// Parse questions
	// Note: use (?:\s+[^>]*)? to avoid matching <questions> wrapper tag
	questionRe := regexp.MustCompile(`(?s)<question(?:\s+[^>]*)?>(.+?)</question>`)
	questionMatches := questionRe.FindAllStringSubmatch(content, -1)
	for _, m := range questionMatches {
		findings.Questions = append(findings.Questions, strings.TrimSpace(m[1]))
	}

	// Parse positives
	positiveRe := regexp.MustCompile(`<positive>(.*?)</positive>`)
	positiveMatches := positiveRe.FindAllStringSubmatch(content, -1)
	for _, m := range positiveMatches {
		findings.Positives = append(findings.Positives, strings.TrimSpace(m[1]))
	}

	return findings, nil
}

// ParseReviewDecision extracts the review decision from Claude's response.
func ParseReviewDecision(response string) (*ReviewDecision, error) {
	decision := &ReviewDecision{
		RemainingIssues: []ReviewFinding{},
	}

	// Extract <review_decision> block
	decisionRe := regexp.MustCompile(`(?s)<review_decision>(.*?)</review_decision>`)
	decisionMatch := decisionRe.FindStringSubmatch(response)
	if decisionMatch == nil {
		return nil, fmt.Errorf("no <review_decision> block found in response")
	}
	content := decisionMatch[1]

	// Parse status
	statusRe := regexp.MustCompile(`<status>(pass|fail|needs_user_input)</status>`)
	if m := statusRe.FindStringSubmatch(content); m != nil {
		decision.Status = ReviewDecisionStatus(m[1])
	}

	// Parse gaps_addressed
	gapsRe := regexp.MustCompile(`<gaps_addressed>(true|false)</gaps_addressed>`)
	if m := gapsRe.FindStringSubmatch(content); m != nil {
		decision.GapsAddressed = m[1] == "true"
	}

	// Parse summary
	summaryRe := regexp.MustCompile(`<summary>(.*?)</summary>`)
	if m := summaryRe.FindStringSubmatch(content); m != nil {
		decision.Summary = strings.TrimSpace(m[1])
	}

	// Parse issues_resolved
	resolvedRe := regexp.MustCompile(`(?s)<issues_resolved>(.*?)</issues_resolved>`)
	if m := resolvedRe.FindStringSubmatch(content); m != nil {
		issueRe := regexp.MustCompile(`<issue>(.*?)</issue>`)
		issues := issueRe.FindAllStringSubmatch(m[1], -1)
		for _, im := range issues {
			decision.IssuesResolved = append(decision.IssuesResolved, strings.TrimSpace(im[1]))
		}
	}

	// Parse remaining_issues
	remainingRe := regexp.MustCompile(`(?s)<remaining_issues>(.*?)</remaining_issues>`)
	if m := remainingRe.FindStringSubmatch(content); m != nil {
		issueRe := regexp.MustCompile(`<issue severity="(high|medium|low)">(.*?)</issue>`)
		issues := issueRe.FindAllStringSubmatch(m[1], -1)
		for _, im := range issues {
			decision.RemainingIssues = append(decision.RemainingIssues, ReviewFinding{
				Severity:    im[1],
				Description: strings.TrimSpace(im[2]),
			})
		}
	}

	// Parse user_questions
	questionsRe := regexp.MustCompile(`(?s)<user_questions>(.*?)</user_questions>`)
	if m := questionsRe.FindStringSubmatch(content); m != nil {
		questionRe := regexp.MustCompile(`<question>(.*?)</question>`)
		questions := questionRe.FindAllStringSubmatch(m[1], -1)
		for _, qm := range questions {
			decision.UserQuestions = append(decision.UserQuestions, strings.TrimSpace(qm[1]))
		}
	}

	// Parse recommendation
	recRe := regexp.MustCompile(`<recommendation>(.*?)</recommendation>`)
	if m := recRe.FindStringSubmatch(content); m != nil {
		decision.Recommendation = strings.TrimSpace(m[1])
	}

	return decision, nil
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

// CountBySerity counts issues by severity level.
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
