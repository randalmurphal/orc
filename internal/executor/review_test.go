package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

func TestShouldRunReview(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	tests := []struct {
		name      string
		response  string
		wantErr   bool
		wantRound int
		wantIssue int
	}{
		{
			name:     "invalid JSON",
			response: "Some random response without valid JSON",
			wantErr:  true,
		},
		{
			name: "valid findings with issues",
			response: `{
				"round": 1,
				"summary": "Code looks good overall with minor issues",
				"issues": [
					{
						"severity": "high",
						"file": "main.go",
						"line": 42,
						"description": "Missing error handling",
						"suggestion": "Add error check after db.Query()"
					},
					{
						"severity": "medium",
						"file": "utils.go",
						"line": 15,
						"description": "Unused variable",
						"suggestion": "Remove unused variable x"
					},
					{
						"severity": "low",
						"description": "Consider adding comments"
					}
				],
				"questions": ["Should we use a different database driver?"],
				"positives": ["Good test coverage", "Clean code structure"]
			}`,
			wantErr:   false,
			wantRound: 1,
			wantIssue: 3,
		},
		{
			name: "empty findings",
			response: `{
				"round": 1,
				"summary": "No issues found",
				"issues": []
			}`,
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
	t.Parallel()
	response := `{
		"round": 1,
		"summary": "Review complete",
		"issues": [
			{
				"severity": "high",
				"file": "internal/api/server.go",
				"line": 100,
				"description": "SQL injection vulnerability",
				"suggestion": "Use parameterized queries"
			}
		],
		"questions": ["Is input validation handled elsewhere?"],
		"positives": ["Good separation of concerns"]
	}`

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
	t.Parallel()
	tests := []struct {
		name       string
		response   string
		wantErr    bool
		wantStatus ReviewDecisionStatus
	}{
		{
			name:     "invalid JSON",
			response: "Random response",
			wantErr:  true,
		},
		{
			name: "pass decision",
			response: `{
				"status": "pass",
				"gaps_addressed": true,
				"summary": "All issues resolved",
				"issues_resolved": ["Fixed SQL injection", "Added error handling"],
				"recommendation": "Ready to merge"
			}`,
			wantErr:    false,
			wantStatus: ReviewStatusPass,
		},
		{
			name: "fail decision",
			response: `{
				"status": "fail",
				"gaps_addressed": false,
				"summary": "Issues remain",
				"remaining_issues": [{"severity": "high", "description": "SQL injection not fixed"}],
				"recommendation": "Fix remaining issues"
			}`,
			wantErr:    false,
			wantStatus: ReviewStatusFail,
		},
		{
			name: "needs_user_input decision",
			response: `{
				"status": "needs_user_input",
				"gaps_addressed": false,
				"summary": "Need clarification",
				"user_questions": ["Should we use OAuth or API keys?"],
				"recommendation": "Await user decision"
			}`,
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
	t.Parallel()
	response := `{
		"status": "pass",
		"gaps_addressed": true,
		"summary": "All identified issues have been addressed",
		"issues_resolved": [
			"SQL injection fixed with parameterized queries",
			"Added proper error handling"
		],
		"remaining_issues": [
			{"severity": "low", "description": "Minor style issue"}
		],
		"user_questions": ["Consider adding more tests?"],
		"recommendation": "Ready to proceed to QA"
	}`

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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// -----------------------------------------------------------------------------
// Parallel Review Tests
// -----------------------------------------------------------------------------

func TestParallelReviewResult_Merge(t *testing.T) {
	t.Parallel()
	pr := NewParallelReviewResult([]ReviewerPerspective{
		PerspectiveCorrectness,
		PerspectiveArchitecture,
	})

	// Add correctness findings
	pr.AddFindings(PerspectiveCorrectness, &ReviewFindings{
		Round:   1,
		Summary: "Correctness review complete",
		Issues: []ReviewFinding{
			{Severity: "high", File: "api.go", Line: 10, Description: "Missing error check"},
			{Severity: "low", Description: "Could simplify condition"},
		},
		Positives: []string{"Good error messages"},
	})

	// Add architecture findings with some overlap
	pr.AddFindings(PerspectiveArchitecture, &ReviewFindings{
		Round:   1,
		Summary: "Architecture review complete",
		Issues: []ReviewFinding{
			{Severity: "medium", File: "service.go", Line: 20, Description: "Violates SRP"},
			{Severity: "low", Description: "could simplify condition"}, // Similar to above (dedup test)
		},
		Positives: []string{"Good error messages", "Clean interfaces"}, // One duplicate
	})

	merged := pr.Merge()

	// Check merged summary contains both perspectives
	if !strings.Contains(merged.Summary, "[correctness]") {
		t.Error("Merged summary missing correctness perspective")
	}
	if !strings.Contains(merged.Summary, "[architecture]") {
		t.Error("Merged summary missing architecture perspective")
	}

	// Check issues are sorted by severity (high first)
	if len(merged.Issues) == 0 {
		t.Fatal("Expected issues in merged result")
	}
	if merged.Issues[0].Severity != "high" {
		t.Errorf("First issue should be high severity, got %s", merged.Issues[0].Severity)
	}

	// Check deduplication of positives
	positiveCount := len(merged.Positives)
	if positiveCount != 2 {
		t.Errorf("Expected 2 unique positives, got %d", positiveCount)
	}

	// Check perspective is tagged on issues
	for _, issue := range merged.Issues {
		if issue.Perspective == "" {
			t.Error("Issue should have perspective tagged")
		}
	}
}

func TestParallelReviewResult_HasHighSeverityIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		findings map[ReviewerPerspective]*ReviewFindings
		expected bool
	}{
		{
			name:     "no findings",
			findings: map[ReviewerPerspective]*ReviewFindings{},
			expected: false,
		},
		{
			name: "only low/medium issues",
			findings: map[ReviewerPerspective]*ReviewFindings{
				PerspectiveCorrectness: {
					Issues: []ReviewFinding{{Severity: "medium"}, {Severity: "low"}},
				},
			},
			expected: false,
		},
		{
			name: "has high severity issue",
			findings: map[ReviewerPerspective]*ReviewFindings{
				PerspectiveArchitecture: {
					Issues: []ReviewFinding{{Severity: "high"}},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pr := &ParallelReviewResult{
				Perspectives: []ReviewerPerspective{PerspectiveCorrectness, PerspectiveArchitecture},
				Findings:     tt.findings,
			}
			if pr.HasHighSeverityIssues() != tt.expected {
				t.Errorf("HasHighSeverityIssues() = %v, want %v", pr.HasHighSeverityIssues(), tt.expected)
			}
		})
	}
}

func TestShouldRunParallelReview(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      *config.Config
		weight   string
		expected bool
	}{
		{
			name:     "nil config returns false",
			cfg:      nil,
			weight:   "large",
			expected: false,
		},
		{
			name: "disabled returns false",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{Enabled: false},
				},
			},
			weight:   "large",
			expected: false,
		},
		{
			name: "trivial weight returns false",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{Enabled: true},
				},
			},
			weight:   "trivial",
			expected: false,
		},
		{
			name: "small weight returns false",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{Enabled: true},
				},
			},
			weight:   "small",
			expected: false,
		},
		{
			name: "medium weight with parallel enabled returns true",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{Enabled: true},
				},
			},
			weight:   "medium",
			expected: true,
		},
		{
			name: "large weight with parallel enabled returns true",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{Enabled: true},
				},
			},
			weight:   "large",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRunParallelReview(tt.cfg, taskWeightFromString(tt.weight))
			if result != tt.expected {
				t.Errorf("ShouldRunParallelReview() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetReviewPerspectives(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      *config.Config
		expected []ReviewerPerspective
	}{
		{
			name:     "nil config returns defaults",
			cfg:      nil,
			expected: DefaultReviewPerspectives(),
		},
		{
			name: "empty perspectives returns defaults",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{
						Perspectives: []string{},
					},
				},
			},
			expected: DefaultReviewPerspectives(),
		},
		{
			name: "custom perspectives",
			cfg: &config.Config{
				Review: config.ReviewConfig{
					Parallel: config.ParallelReviewConfig{
						Perspectives: []string{"security", "performance"},
					},
				},
			},
			expected: []ReviewerPerspective{PerspectiveSecurity, PerspectivePerformance},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReviewPerspectives(tt.cfg)
			if len(result) != len(tt.expected) {
				t.Errorf("GetReviewPerspectives() length = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, p := range result {
				if p != tt.expected[i] {
					t.Errorf("GetReviewPerspectives()[%d] = %v, want %v", i, p, tt.expected[i])
				}
			}
		})
	}
}

func TestGetPerspectivePromptContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		perspective ReviewerPerspective
		contains    string
	}{
		{PerspectiveCorrectness, "correctness"},
		{PerspectiveArchitecture, "architecture"},
		{PerspectiveSecurity, "security"},
		{PerspectivePerformance, "performance"},
	}

	for _, tt := range tests {
		t.Run(string(tt.perspective), func(t *testing.T) {
			result := GetPerspectivePromptContext(tt.perspective)
			if result == "" {
				t.Error("Expected non-empty prompt context")
			}
			if !strings.Contains(strings.ToLower(result), tt.contains) {
				t.Errorf("Expected prompt to contain %q", tt.contains)
			}
		})
	}

	t.Run("unknown perspective returns empty", func(t *testing.T) {
		result := GetPerspectivePromptContext("unknown")
		if result != "" {
			t.Errorf("Expected empty string for unknown perspective, got %q", result)
		}
	})
}

func TestFormatParallelReviewSummary(t *testing.T) {
	t.Parallel()
	t.Run("nil result", func(t *testing.T) {
		result := FormatParallelReviewSummary(nil)
		if !strings.Contains(result, "No parallel review") {
			t.Error("Expected nil message")
		}
	})

	t.Run("with findings", func(t *testing.T) {
		pr := NewParallelReviewResult([]ReviewerPerspective{
			PerspectiveCorrectness,
			PerspectiveArchitecture,
		})
		pr.AddFindings(PerspectiveCorrectness, &ReviewFindings{
			Issues: []ReviewFinding{
				{Severity: "high"},
				{Severity: "medium"},
			},
		})

		result := FormatParallelReviewSummary(pr)

		if !strings.Contains(result, "Parallel Review Summary") {
			t.Error("Missing header")
		}
		if !strings.Contains(result, "correctness") {
			t.Error("Missing correctness perspective")
		}
		if !strings.Contains(result, "Merged Totals") {
			t.Error("Missing merged totals")
		}
	})
}

func TestIssueDeduplication(t *testing.T) {
	t.Parallel()
	// Test that similar issues are deduplicated
	pr := NewParallelReviewResult([]ReviewerPerspective{
		PerspectiveCorrectness,
		PerspectiveArchitecture,
	})

	// Add same issue from different perspectives (slightly different wording)
	pr.AddFindings(PerspectiveCorrectness, &ReviewFindings{
		Issues: []ReviewFinding{
			{Severity: "high", File: "api.go", Line: 42, Description: "Missing error handling"},
		},
	})
	pr.AddFindings(PerspectiveArchitecture, &ReviewFindings{
		Issues: []ReviewFinding{
			{Severity: "high", File: "api.go", Line: 42, Description: "missing error handling"}, // Same, different case
			{Severity: "medium", File: "service.go", Line: 10, Description: "Different issue"},
		},
	})

	merged := pr.Merge()

	// Should have 2 unique issues (the api.go:42 issues deduplicated)
	if len(merged.Issues) != 2 {
		t.Errorf("Expected 2 unique issues after dedup, got %d", len(merged.Issues))
	}
}

// Helper to convert string weight to task.Weight
func taskWeightFromString(s string) task.Weight {
	switch s {
	case "trivial":
		return task.WeightTrivial
	case "small":
		return task.WeightSmall
	case "medium":
		return task.WeightMedium
	case "large":
		return task.WeightLarge
	default:
		return task.WeightSmall
	}
}


