package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestShouldRunQA(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      *config.Config
		weight   string
		expected bool
	}{
		{
			name:     "nil config skips trivial",
			cfg:      nil,
			weight:   "trivial",
			expected: false,
		},
		{
			name:     "nil config runs for small",
			cfg:      nil,
			weight:   "small",
			expected: true,
		},
		{
			name:     "nil config runs for medium",
			cfg:      nil,
			weight:   "medium",
			expected: true,
		},
		{
			name: "disabled config returns false",
			cfg: &config.Config{
				QA: config.QAConfig{Enabled: false},
			},
			weight:   "medium",
			expected: false,
		},
		{
			name: "enabled with skip list",
			cfg: &config.Config{
				QA: config.QAConfig{
					Enabled:        true,
					SkipForWeights: []string{"trivial", "small"},
				},
			},
			weight:   "small",
			expected: false,
		},
		{
			name: "enabled not in skip list",
			cfg: &config.Config{
				QA: config.QAConfig{
					Enabled:        true,
					SkipForWeights: []string{"trivial"},
				},
			},
			weight:   "medium",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldRunQA(tt.cfg, tt.weight)
			if result != tt.expected {
				t.Errorf("ShouldRunQA() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseQAResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		response   string
		wantErr    bool
		wantStatus QAStatus
	}{
		{
			name:     "invalid JSON",
			response: "Some random response without valid JSON",
			wantErr:  true,
		},
		{
			name: "pass status",
			response: `{
				"status": "pass",
				"summary": "All tests pass",
				"recommendation": "Ready for release"
			}`,
			wantErr:    false,
			wantStatus: QAStatusPass,
		},
		{
			name: "fail status",
			response: `{
				"status": "fail",
				"summary": "Tests failing",
				"issues": [{"severity": "high", "description": "E2E test fails"}],
				"recommendation": "Fix failing tests"
			}`,
			wantErr:    false,
			wantStatus: QAStatusFail,
		},
		{
			name: "needs_attention status",
			response: `{
				"status": "needs_attention",
				"summary": "Minor items to address",
				"recommendation": "Follow up on documentation"
			}`,
			wantErr:    false,
			wantStatus: QAStatusNeedsAttention,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseQAResult(tt.response)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseQAResult() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseQAResult() unexpected error: %v", err)
				return
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
		})
	}
}

func TestParseQAResultDetails(t *testing.T) {
	t.Parallel()
	response := `{
		"status": "pass",
		"summary": "QA session complete. All tests pass with good coverage.",
		"tests_written": [
			{
				"file": "internal/api/server_test.go",
				"description": "E2E tests for new endpoint",
				"type": "e2e"
			},
			{
				"file": "internal/executor/qa_test.go",
				"description": "Unit tests for QA parsing",
				"type": "unit"
			}
		],
		"tests_run": {
			"total": 42,
			"passed": 40,
			"failed": 0,
			"skipped": 2
		},
		"coverage": {
			"percentage": 85.5,
			"uncovered_areas": "Error handling in edge cases"
		},
		"documentation": [
			{
				"file": "docs/api/new-endpoint.md",
				"type": "api"
			}
		],
		"issues": [
			{
				"severity": "low",
				"description": "Consider adding more edge case tests",
				"reproduction": "N/A"
			}
		],
		"recommendation": "Ready for production deployment"
	}`

	result, err := ParseQAResult(response)
	if err != nil {
		t.Fatalf("ParseQAResult() error: %v", err)
	}

	// Check status
	if result.Status != QAStatusPass {
		t.Errorf("Status = %q, want %q", result.Status, QAStatusPass)
	}

	// Check summary
	if !strings.Contains(result.Summary, "QA session complete") {
		t.Errorf("Summary = %q, want to contain 'QA session complete'", result.Summary)
	}

	// Check tests written
	if len(result.TestsWritten) != 2 {
		t.Errorf("TestsWritten count = %d, want 2", len(result.TestsWritten))
	} else {
		if result.TestsWritten[0].File != "internal/api/server_test.go" {
			t.Errorf("TestsWritten[0].File = %q, want %q", result.TestsWritten[0].File, "internal/api/server_test.go")
		}
		if result.TestsWritten[0].Type != "e2e" {
			t.Errorf("TestsWritten[0].Type = %q, want %q", result.TestsWritten[0].Type, "e2e")
		}
	}

	// Check tests run
	if result.TestsRun == nil {
		t.Fatal("TestsRun is nil")
	}
	if result.TestsRun.Total != 42 {
		t.Errorf("TestsRun.Total = %d, want 42", result.TestsRun.Total)
	}
	if result.TestsRun.Passed != 40 {
		t.Errorf("TestsRun.Passed = %d, want 40", result.TestsRun.Passed)
	}
	if result.TestsRun.Failed != 0 {
		t.Errorf("TestsRun.Failed = %d, want 0", result.TestsRun.Failed)
	}
	if result.TestsRun.Skipped != 2 {
		t.Errorf("TestsRun.Skipped = %d, want 2", result.TestsRun.Skipped)
	}

	// Check coverage
	if result.Coverage == nil {
		t.Fatal("Coverage is nil")
	}
	if result.Coverage.Percentage != 85.5 {
		t.Errorf("Coverage.Percentage = %f, want 85.5", result.Coverage.Percentage)
	}
	if result.Coverage.UncoveredAreas != "Error handling in edge cases" {
		t.Errorf("Coverage.UncoveredAreas = %q", result.Coverage.UncoveredAreas)
	}

	// Check documentation
	if len(result.Documentation) != 1 {
		t.Errorf("Documentation count = %d, want 1", len(result.Documentation))
	} else {
		if result.Documentation[0].File != "docs/api/new-endpoint.md" {
			t.Errorf("Documentation[0].File = %q", result.Documentation[0].File)
		}
		if result.Documentation[0].Type != "api" {
			t.Errorf("Documentation[0].Type = %q", result.Documentation[0].Type)
		}
	}

	// Check issues
	if len(result.Issues) != 1 {
		t.Errorf("Issues count = %d, want 1", len(result.Issues))
	} else {
		if result.Issues[0].Severity != "low" {
			t.Errorf("Issues[0].Severity = %q, want %q", result.Issues[0].Severity, "low")
		}
	}

	// Check recommendation
	if result.Recommendation != "Ready for production deployment" {
		t.Errorf("Recommendation = %q", result.Recommendation)
	}
}

func TestQAResultHasHighSeverityIssues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   *QAResult
		expected bool
	}{
		{
			name:     "no issues",
			result:   &QAResult{Issues: []QAIssue{}},
			expected: false,
		},
		{
			name: "only low severity",
			result: &QAResult{
				Issues: []QAIssue{
					{Severity: "low"},
					{Severity: "medium"},
				},
			},
			expected: false,
		},
		{
			name: "has high severity",
			result: &QAResult{
				Issues: []QAIssue{
					{Severity: "low"},
					{Severity: "high"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.HasHighSeverityIssues()
			if result != tt.expected {
				t.Errorf("HasHighSeverityIssues() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQAResultAllTestsPassed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   *QAResult
		expected bool
	}{
		{
			name:     "no tests run",
			result:   &QAResult{TestsRun: nil},
			expected: true,
		},
		{
			name: "all passed",
			result: &QAResult{
				TestsRun: &QATestRun{Total: 10, Passed: 10, Failed: 0},
			},
			expected: true,
		},
		{
			name: "some failed",
			result: &QAResult{
				TestsRun: &QATestRun{Total: 10, Passed: 8, Failed: 2},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.AllTestsPassed()
			if result != tt.expected {
				t.Errorf("AllTestsPassed() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatQAResultSummary(t *testing.T) {
	t.Parallel()
	t.Run("nil result", func(t *testing.T) {
		result := FormatQAResultSummary(nil)
		if result != "No QA result available." {
			t.Errorf("FormatQAResultSummary(nil) = %q", result)
		}
	})

	t.Run("full result", func(t *testing.T) {
		result := &QAResult{
			Status:  QAStatusPass,
			Summary: "All good",
			TestsRun: &QATestRun{
				Total: 10, Passed: 9, Failed: 0, Skipped: 1,
			},
			Coverage: &QACoverage{
				Percentage: 80.5,
			},
			TestsWritten: []QATest{
				{File: "test.go", Type: "unit", Description: "Test things"},
			},
			Documentation: []QADoc{
				{File: "README.md", Type: "feature"},
			},
			Issues: []QAIssue{
				{Severity: "low", Description: "Minor thing"},
			},
			Recommendation: "Ship it",
		}

		output := FormatQAResultSummary(result)

		checks := []string{
			"QA Status: PASS",
			"Summary: All good",
			"Tests: 10 total",
			"Coverage: 80.5%",
			"Tests Written: 1",
			"Documentation: 1 files",
			"Issues: 1",
			"[LOW] Minor thing",
			"Recommendation: Ship it",
		}

		for _, check := range checks {
			if !strings.Contains(output, check) {
				t.Errorf("Output missing %q", check)
			}
		}
	})
}

// =============================================================================
// QA E2E Tests
// =============================================================================

func TestParseQAE2ETestResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		response     string
		wantErr      bool
		wantFindings int
	}{
		{
			name:     "invalid JSON",
			response: "not valid json",
			wantErr:  true,
		},
		{
			name: "no findings",
			response: `{
				"status": "complete",
				"summary": "All tests passed",
				"findings": []
			}`,
			wantErr:      false,
			wantFindings: 0,
		},
		{
			name: "with findings",
			response: `{
				"status": "complete",
				"summary": "Found 2 issues",
				"findings": [
					{
						"id": "QA-001",
						"severity": "high",
						"confidence": 95,
						"category": "functional",
						"title": "Form validation fails",
						"steps_to_reproduce": ["Step 1", "Step 2"],
						"expected": "Error message",
						"actual": "Silent failure"
					},
					{
						"id": "QA-002",
						"severity": "low",
						"confidence": 80,
						"category": "visual",
						"title": "Minor spacing issue",
						"steps_to_reproduce": ["Load page"],
						"expected": "16px margin",
						"actual": "12px margin"
					}
				],
				"verification": {
					"scenarios_tested": 10,
					"viewports_tested": ["desktop", "mobile"]
				}
			}`,
			wantErr:      false,
			wantFindings: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseQAE2ETestResult(tt.response)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseQAE2ETestResult() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseQAE2ETestResult() unexpected error: %v", err)
				return
			}
			if len(result.Findings) != tt.wantFindings {
				t.Errorf("Findings count = %d, want %d", len(result.Findings), tt.wantFindings)
			}
		})
	}
}

func TestParseQAE2EFixResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		response      string
		wantErr       bool
		wantFixed     int
		wantDeferred  int
	}{
		{
			name:     "invalid JSON",
			response: "not json",
			wantErr:  true,
		},
		{
			name: "all fixed",
			response: `{
				"status": "complete",
				"summary": "Fixed 2 issues",
				"fixes_applied": [
					{"finding_id": "QA-001", "status": "fixed", "files_modified": ["a.go"], "change_description": "Fixed it"},
					{"finding_id": "QA-002", "status": "fixed", "files_modified": ["b.go"], "change_description": "Fixed it too"}
				]
			}`,
			wantErr:      false,
			wantFixed:    2,
			wantDeferred: 0,
		},
		{
			name: "some deferred",
			response: `{
				"status": "complete",
				"summary": "Fixed 1 of 2 issues",
				"fixes_applied": [
					{"finding_id": "QA-001", "status": "fixed", "files_modified": ["a.go"], "change_description": "Fixed it"}
				],
				"issues_deferred": [
					{"finding_id": "QA-002", "reason": "Requires design decision"}
				]
			}`,
			wantErr:      false,
			wantFixed:    1,
			wantDeferred: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseQAE2EFixResult(tt.response)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseQAE2EFixResult() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseQAE2EFixResult() unexpected error: %v", err)
				return
			}
			if len(result.FixesApplied) != tt.wantFixed {
				t.Errorf("FixesApplied count = %d, want %d", len(result.FixesApplied), tt.wantFixed)
			}
			if len(result.IssuesDeferred) != tt.wantDeferred {
				t.Errorf("IssuesDeferred count = %d, want %d", len(result.IssuesDeferred), tt.wantDeferred)
			}
		})
	}
}

func TestQAE2ETestResult_HasFindings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   *QAE2ETestResult
		expected bool
	}{
		{
			name:     "nil findings",
			result:   &QAE2ETestResult{Findings: nil},
			expected: false,
		},
		{
			name:     "empty findings",
			result:   &QAE2ETestResult{Findings: []QAE2EFinding{}},
			expected: false,
		},
		{
			name: "has findings",
			result: &QAE2ETestResult{
				Findings: []QAE2EFinding{
					{ID: "QA-001", Title: "Test"},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.HasFindings()
			if result != tt.expected {
				t.Errorf("HasFindings() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQAE2ETestResult_HighSeverityCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   *QAE2ETestResult
		expected int
	}{
		{
			name:     "no findings",
			result:   &QAE2ETestResult{Findings: []QAE2EFinding{}},
			expected: 0,
		},
		{
			name: "mixed severity",
			result: &QAE2ETestResult{
				Findings: []QAE2EFinding{
					{ID: "QA-001", Severity: QAE2ESeverityCritical},
					{ID: "QA-002", Severity: QAE2ESeverityHigh},
					{ID: "QA-003", Severity: QAE2ESeverityMedium},
					{ID: "QA-004", Severity: QAE2ESeverityLow},
				},
			},
			expected: 2, // 1 critical + 1 high
		},
		{
			name: "only low/medium",
			result: &QAE2ETestResult{
				Findings: []QAE2EFinding{
					{ID: "QA-001", Severity: QAE2ESeverityMedium},
					{ID: "QA-002", Severity: QAE2ESeverityLow},
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.result.HighSeverityCount()
			if result != tt.expected {
				t.Errorf("HighSeverityCount() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestQAE2ETestResult_FormatFindingsForFix(t *testing.T) {
	t.Parallel()
	t.Run("no findings", func(t *testing.T) {
		result := &QAE2ETestResult{Findings: []QAE2EFinding{}}
		output := result.FormatFindingsForFix()
		if output != "No findings to fix." {
			t.Errorf("FormatFindingsForFix() = %q, want %q", output, "No findings to fix.")
		}
	})

	t.Run("with findings", func(t *testing.T) {
		result := &QAE2ETestResult{
			Findings: []QAE2EFinding{
				{
					ID:              "QA-001",
					Severity:        QAE2ESeverityHigh,
					Confidence:      95,
					Category:        QAE2ECategoryFunctional,
					Title:           "Form fails silently",
					StepsToReproduce: []string{"Fill form", "Click submit"},
					Expected:        "Success message",
					Actual:          "Nothing happens",
					ScreenshotPath:  "/tmp/qa/bug-001.png",
					SuggestedFix:    "Check event handler",
				},
			},
		}
		output := result.FormatFindingsForFix()

		checks := []string{
			"QA-001",
			"[HIGH]",
			"Form fails silently",
			"Category:** functional",
			"Confidence:** 95",
			"1. Fill form",
			"2. Click submit",
			"Expected:** Success message",
			"Actual:** Nothing happens",
			"Screenshot:** /tmp/qa/bug-001.png",
			"Suggested Fix:** Check event handler",
		}

		for _, check := range checks {
			if !strings.Contains(output, check) {
				t.Errorf("Output missing %q\nGot: %s", check, output)
			}
		}
	})
}

func TestFormatQAE2EResultSummary(t *testing.T) {
	t.Parallel()
	t.Run("nil result", func(t *testing.T) {
		output := FormatQAE2EResultSummary(nil)
		if output != "No QA E2E result available." {
			t.Errorf("FormatQAE2EResultSummary(nil) = %q", output)
		}
	})

	t.Run("no findings", func(t *testing.T) {
		result := &QAE2ETestResult{
			Status:   "complete",
			Summary:  "All tests passed",
			Findings: []QAE2EFinding{},
			Verification: &QAE2EVerification{
				ScenariosTested: 15,
				ViewportsTested: []string{"desktop", "mobile"},
			},
		}
		output := FormatQAE2EResultSummary(result)
		if !strings.Contains(output, "No issues found - PASS") {
			t.Errorf("Expected PASS message, got: %s", output)
		}
		if !strings.Contains(output, "Scenarios Tested: 15") {
			t.Errorf("Expected scenario count, got: %s", output)
		}
	})

	t.Run("with findings", func(t *testing.T) {
		result := &QAE2ETestResult{
			Status:  "complete",
			Summary: "Found issues",
			Findings: []QAE2EFinding{
				{ID: "QA-001", Severity: QAE2ESeverityCritical, Title: "Critical bug"},
				{ID: "QA-002", Severity: QAE2ESeverityLow, Title: "Minor issue"},
			},
		}
		output := FormatQAE2EResultSummary(result)
		if !strings.Contains(output, "Findings: 2 total (1 critical/high)") {
			t.Errorf("Expected findings summary, got: %s", output)
		}
		if !strings.Contains(output, "[CRITICAL] QA-001: Critical bug") {
			t.Errorf("Expected critical finding, got: %s", output)
		}
	})
}
