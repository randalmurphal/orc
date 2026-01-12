package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestShouldRunReview(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		weight   string
		expected bool
	}{
		{
			name:     "nil config defaults to true",
			cfg:      nil,
			weight:   "small",
			expected: true,
		},
		{
			name: "enabled config returns true",
			cfg: &config.Config{
				Review: config.ReviewConfig{Enabled: true},
			},
			weight:   "medium",
			expected: true,
		},
		{
			name: "disabled config returns false",
			cfg: &config.Config{
				Review: config.ReviewConfig{Enabled: false},
			},
			weight:   "large",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRunReview(tt.cfg, tt.weight)
			if result != tt.expected {
				t.Errorf("ShouldRunReview() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetReviewRounds(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		expected int
	}{
		{
			name:     "nil config defaults to 2",
			cfg:      nil,
			expected: 2,
		},
		{
			name: "zero rounds defaults to 2",
			cfg: &config.Config{
				Review: config.ReviewConfig{Rounds: 0},
			},
			expected: 2,
		},
		{
			name: "configured rounds returned",
			cfg: &config.Config{
				Review: config.ReviewConfig{Rounds: 3},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReviewRounds(tt.cfg)
			if result != tt.expected {
				t.Errorf("GetReviewRounds() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseReviewFindings(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		wantErr   bool
		wantRound int
		wantIssue int
	}{
		{
			name:     "no review_findings block",
			response: "Some random response without the expected block",
			wantErr:  true,
		},
		{
			name: "valid findings with issues",
			response: `
Here are my findings:

<review_findings>
  <round>1</round>
  <summary>Code looks good overall with minor issues</summary>
  <issues>
    <issue severity="high">
      <file>main.go</file>
      <line>42</line>
      <description>Missing error handling</description>
      <suggestion>Add error check after db.Query()</suggestion>
    </issue>
    <issue severity="medium">
      <file>utils.go</file>
      <line>15</line>
      <description>Unused variable</description>
      <suggestion>Remove unused variable x</suggestion>
    </issue>
    <issue severity="low">
      <description>Consider adding comments</description>
    </issue>
  </issues>
  <questions>
    <question context="architecture">Should we use a different database driver?</question>
  </questions>
  <positives>
    <positive>Good test coverage</positive>
    <positive>Clean code structure</positive>
  </positives>
</review_findings>

<phase_complete>true</phase_complete>
`,
			wantErr:   false,
			wantRound: 1,
			wantIssue: 3,
		},
		{
			name: "empty findings",
			response: `
<review_findings>
  <round>1</round>
  <summary>No issues found</summary>
</review_findings>
`,
			wantErr:   false,
			wantRound: 1,
			wantIssue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings, err := ParseReviewFindings(tt.response)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseReviewFindings() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseReviewFindings() unexpected error: %v", err)
				return
			}
			if findings.Round != tt.wantRound {
				t.Errorf("Round = %d, want %d", findings.Round, tt.wantRound)
			}
			if len(findings.Issues) != tt.wantIssue {
				t.Errorf("Issues count = %d, want %d", len(findings.Issues), tt.wantIssue)
			}
		})
	}
}

func TestParseReviewFindingsDetails(t *testing.T) {
	response := `
<review_findings>
  <round>1</round>
  <summary>Review complete</summary>
  <issues>
    <issue severity="high">
      <file>internal/api/server.go</file>
      <line>100</line>
      <description>SQL injection vulnerability</description>
      <suggestion>Use parameterized queries</suggestion>
    </issue>
  </issues>
  <questions>
    <question>Is input validation handled elsewhere?</question>
  </questions>
  <positives>
    <positive>Good separation of concerns</positive>
  </positives>
</review_findings>
`

	findings, err := ParseReviewFindings(response)
	if err != nil {
		t.Fatalf("ParseReviewFindings() error: %v", err)
	}

	if findings.Summary != "Review complete" {
		t.Errorf("Summary = %q, want %q", findings.Summary, "Review complete")
	}

	if len(findings.Issues) != 1 {
		t.Fatalf("Issues count = %d, want 1", len(findings.Issues))
	}

	issue := findings.Issues[0]
	if issue.Severity != "high" {
		t.Errorf("Issue.Severity = %q, want %q", issue.Severity, "high")
	}
	if issue.File != "internal/api/server.go" {
		t.Errorf("Issue.File = %q, want %q", issue.File, "internal/api/server.go")
	}
	if issue.Line != 100 {
		t.Errorf("Issue.Line = %d, want %d", issue.Line, 100)
	}
	if issue.Description != "SQL injection vulnerability" {
		t.Errorf("Issue.Description = %q, want %q", issue.Description, "SQL injection vulnerability")
	}
	if issue.Suggestion != "Use parameterized queries" {
		t.Errorf("Issue.Suggestion = %q, want %q", issue.Suggestion, "Use parameterized queries")
	}

	if len(findings.Questions) != 1 {
		t.Fatalf("Questions count = %d, want 1", len(findings.Questions))
	}
	if findings.Questions[0] != "Is input validation handled elsewhere?" {
		t.Errorf("Question = %q, want %q", findings.Questions[0], "Is input validation handled elsewhere?")
	}

	if len(findings.Positives) != 1 {
		t.Fatalf("Positives count = %d, want 1", len(findings.Positives))
	}
	if findings.Positives[0] != "Good separation of concerns" {
		t.Errorf("Positive = %q, want %q", findings.Positives[0], "Good separation of concerns")
	}
}

func TestParseReviewDecision(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		wantErr    bool
		wantStatus ReviewDecisionStatus
	}{
		{
			name:     "no review_decision block",
			response: "Random response",
			wantErr:  true,
		},
		{
			name: "pass decision",
			response: `
<review_decision>
  <status>pass</status>
  <gaps_addressed>true</gaps_addressed>
  <summary>All issues resolved</summary>
  <issues_resolved>
    <issue>Fixed SQL injection</issue>
    <issue>Added error handling</issue>
  </issues_resolved>
  <recommendation>Ready to merge</recommendation>
</review_decision>
`,
			wantErr:    false,
			wantStatus: ReviewStatusPass,
		},
		{
			name: "fail decision",
			response: `
<review_decision>
  <status>fail</status>
  <gaps_addressed>false</gaps_addressed>
  <summary>Issues remain</summary>
  <remaining_issues>
    <issue severity="high">SQL injection not fixed</issue>
  </remaining_issues>
  <recommendation>Fix remaining issues</recommendation>
</review_decision>
`,
			wantErr:    false,
			wantStatus: ReviewStatusFail,
		},
		{
			name: "needs_user_input decision",
			response: `
<review_decision>
  <status>needs_user_input</status>
  <gaps_addressed>false</gaps_addressed>
  <summary>Need clarification</summary>
  <user_questions>
    <question>Should we use OAuth or API keys?</question>
  </user_questions>
  <recommendation>Await user decision</recommendation>
</review_decision>
`,
			wantErr:    false,
			wantStatus: ReviewStatusNeedsUserInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ParseReviewDecision(tt.response)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseReviewDecision() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseReviewDecision() unexpected error: %v", err)
				return
			}
			if decision.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", decision.Status, tt.wantStatus)
			}
		})
	}
}

func TestParseReviewDecisionDetails(t *testing.T) {
	response := `
<review_decision>
  <status>pass</status>
  <gaps_addressed>true</gaps_addressed>
  <summary>All identified issues have been addressed</summary>
  <issues_resolved>
    <issue>SQL injection fixed with parameterized queries</issue>
    <issue>Added proper error handling</issue>
  </issues_resolved>
  <remaining_issues>
    <issue severity="low">Minor style issue</issue>
  </remaining_issues>
  <user_questions>
    <question>Consider adding more tests?</question>
  </user_questions>
  <recommendation>Ready to proceed to QA</recommendation>
</review_decision>
`

	decision, err := ParseReviewDecision(response)
	if err != nil {
		t.Fatalf("ParseReviewDecision() error: %v", err)
	}

	if decision.Status != ReviewStatusPass {
		t.Errorf("Status = %q, want %q", decision.Status, ReviewStatusPass)
	}
	if !decision.GapsAddressed {
		t.Error("GapsAddressed = false, want true")
	}
	if decision.Summary != "All identified issues have been addressed" {
		t.Errorf("Summary = %q, want %q", decision.Summary, "All identified issues have been addressed")
	}
	if len(decision.IssuesResolved) != 2 {
		t.Errorf("IssuesResolved count = %d, want 2", len(decision.IssuesResolved))
	}
	if len(decision.RemainingIssues) != 1 {
		t.Errorf("RemainingIssues count = %d, want 1", len(decision.RemainingIssues))
	}
	if decision.RemainingIssues[0].Severity != "low" {
		t.Errorf("RemainingIssue.Severity = %q, want %q", decision.RemainingIssues[0].Severity, "low")
	}
	if len(decision.UserQuestions) != 1 {
		t.Errorf("UserQuestions count = %d, want 1", len(decision.UserQuestions))
	}
	if decision.Recommendation != "Ready to proceed to QA" {
		t.Errorf("Recommendation = %q, want %q", decision.Recommendation, "Ready to proceed to QA")
	}
}

func TestHasHighSeverityIssues(t *testing.T) {
	tests := []struct {
		name     string
		findings *ReviewFindings
		expected bool
	}{
		{
			name:     "empty issues",
			findings: &ReviewFindings{Issues: []ReviewFinding{}},
			expected: false,
		},
		{
			name: "no high severity",
			findings: &ReviewFindings{
				Issues: []ReviewFinding{
					{Severity: "medium"},
					{Severity: "low"},
				},
			},
			expected: false,
		},
		{
			name: "has high severity",
			findings: &ReviewFindings{
				Issues: []ReviewFinding{
					{Severity: "low"},
					{Severity: "high"},
					{Severity: "medium"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.findings.HasHighSeverityIssues()
			if result != tt.expected {
				t.Errorf("HasHighSeverityIssues() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCountBySeverity(t *testing.T) {
	findings := &ReviewFindings{
		Issues: []ReviewFinding{
			{Severity: "high"},
			{Severity: "high"},
			{Severity: "medium"},
			{Severity: "low"},
			{Severity: "low"},
			{Severity: "low"},
		},
	}

	counts := findings.CountBySeverity()

	if counts["high"] != 2 {
		t.Errorf("high count = %d, want 2", counts["high"])
	}
	if counts["medium"] != 1 {
		t.Errorf("medium count = %d, want 1", counts["medium"])
	}
	if counts["low"] != 3 {
		t.Errorf("low count = %d, want 3", counts["low"])
	}
}

func TestFormatFindingsForRound2(t *testing.T) {
	t.Run("nil findings", func(t *testing.T) {
		result := FormatFindingsForRound2(nil)
		if result != "No findings from Round 1." {
			t.Errorf("FormatFindingsForRound2(nil) = %q, want %q", result, "No findings from Round 1.")
		}
	})

	t.Run("findings with issues", func(t *testing.T) {
		findings := &ReviewFindings{
			Summary: "Found some issues",
			Issues: []ReviewFinding{
				{
					Severity:    "high",
					File:        "main.go",
					Line:        42,
					Description: "Missing error check",
					Suggestion:  "Add error handling",
				},
				{
					Severity:    "medium",
					Description: "Consider refactoring",
				},
			},
			Positives: []string{"Good test coverage"},
		}

		result := FormatFindingsForRound2(findings)

		// Check key elements are present
		if !strings.Contains(result, "Round 1 Summary: Found some issues") {
			t.Error("Missing summary in output")
		}
		if !strings.Contains(result, "1 high") {
			t.Error("Missing high count in output")
		}
		if !strings.Contains(result, "1 medium") {
			t.Error("Missing medium count in output")
		}
		if !strings.Contains(result, "[HIGH] Missing error check") {
			t.Error("Missing high severity issue in output")
		}
		if !strings.Contains(result, "(in main.go:42)") {
			t.Error("Missing file location in output")
		}
		if !strings.Contains(result, "Suggested fix: Add error handling") {
			t.Error("Missing suggestion in output")
		}
		if !strings.Contains(result, "Good test coverage") {
			t.Error("Missing positive in output")
		}
	})
}
