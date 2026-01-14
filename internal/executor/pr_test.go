package executor

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

func TestBuildPRBody_IncludesTaskTitle(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:     "TEST-001",
		Title:  "Implement feature X",
		Weight: "medium",
	}

	body := e.buildPRBody(tsk)

	if !strings.Contains(body, "Implement feature X") {
		t.Errorf("expected body to contain task title, got: %s", body)
	}
	if !strings.Contains(body, "TEST-001") {
		t.Errorf("expected body to contain task ID, got: %s", body)
	}
}

func TestBuildPRBody_IncludesPhases(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:          "TEST-002",
		Title:       "Add new API endpoint",
		Description: "Create POST /api/widgets endpoint with validation",
		Weight:      "large",
	}

	body := e.buildPRBody(tsk)

	// Should include description when present
	if !strings.Contains(body, "Create POST /api/widgets endpoint") {
		t.Errorf("expected body to contain description, got: %s", body)
	}

	// Should include weight
	if !strings.Contains(body, "large") {
		t.Errorf("expected body to contain weight, got: %s", body)
	}

	// Should have standard sections
	if !strings.Contains(body, "## Summary") {
		t.Errorf("expected body to contain Summary section, got: %s", body)
	}
	if !strings.Contains(body, "## Task Details") {
		t.Errorf("expected body to contain Task Details section, got: %s", body)
	}
	if !strings.Contains(body, "## Test Plan") {
		t.Errorf("expected body to contain Test Plan section, got: %s", body)
	}
}

func TestBuildPRBody_UsesDescription(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:          "TEST-003",
		Title:       "Short title",
		Description: "This is a longer description that explains the task in detail",
		Weight:      "small",
	}

	body := e.buildPRBody(tsk)

	// Description should be in summary, not title
	if !strings.Contains(body, "This is a longer description") {
		t.Errorf("expected body to use description in summary, got: %s", body)
	}
}

func TestBuildPRBody_FallsBackToTitle(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:          "TEST-004",
		Title:       "Title only task",
		Description: "", // Empty description
		Weight:      "trivial",
	}

	body := e.buildPRBody(tsk)

	// Should use title when description is empty
	if !strings.Contains(body, "Title only task") {
		t.Errorf("expected body to fall back to title, got: %s", body)
	}
}

func TestBuildPRBody_HasOrcFooter(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:     "TEST-005",
		Title:  "Any task",
		Weight: "small",
	}

	body := e.buildPRBody(tsk)

	if !strings.Contains(body, "Created by [orc]") {
		t.Errorf("expected body to have orc footer, got: %s", body)
	}
	if !strings.Contains(body, "github.com/randalmurphal/orc") {
		t.Errorf("expected body to have orc link, got: %s", body)
	}
}

func TestIsLabelError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "label not found",
			err:      errors.New("could not add label: automated not found"),
			expected: true,
		},
		{
			name:     "label not found uppercase",
			err:      errors.New("Could not add Label: AUTOMATED not found"),
			expected: true,
		},
		{
			name:     "multiple labels not found",
			err:      errors.New("could not add label: bug-fix not found"),
			expected: true,
		},
		{
			name:     "gh cli error with label",
			err:      errors.New("gh pr create: label 'automated' not found: exit status 1"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("network timeout"),
			expected: false,
		},
		{
			name:     "auth error",
			err:      errors.New("gh: not authenticated"),
			expected: false,
		},
		{
			name:     "branch not found",
			err:      errors.New("branch not found: feature-branch"),
			expected: false,
		},
		{
			name:     "generic not found without label",
			err:      errors.New("repository not found"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLabelError(tt.err)
			if got != tt.expected {
				t.Errorf("isLabelError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrSyncConflict(t *testing.T) {
	// ErrSyncConflict should be defined and usable
	if ErrSyncConflict == nil {
		t.Error("ErrSyncConflict should not be nil")
	}
	if ErrSyncConflict.Error() != "sync conflict detected" {
		t.Errorf("ErrSyncConflict.Error() = %s, want 'sync conflict detected'", ErrSyncConflict.Error())
	}
}

func TestSyncPhaseConstants(t *testing.T) {
	// Verify sync phase constants are defined
	if SyncPhaseStart != "start" {
		t.Errorf("SyncPhaseStart = %s, want 'start'", SyncPhaseStart)
	}
	if SyncPhaseCompletion != "completion" {
		t.Errorf("SyncPhaseCompletion = %s, want 'completion'", SyncPhaseCompletion)
	}
}

func TestIsAuthError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "not logged in",
			err:      errors.New("gh: not logged in"),
			expected: true,
		},
		{
			name:     "not authenticated",
			err:      errors.New("gh not authenticated: You are not logged into any GitHub hosts"),
			expected: true,
		},
		{
			name:     "authentication required",
			err:      errors.New("authentication required"),
			expected: true,
		},
		{
			name:     "failed to authenticate",
			err:      errors.New("failed to authenticate with GitHub"),
			expected: true,
		},
		{
			name:     "401 unauthorized",
			err:      errors.New("HTTP 401: Unauthorized"),
			expected: true,
		},
		{
			name:     "lowercase unauthorized",
			err:      errors.New("request unauthorized"),
			expected: true,
		},
		{
			name:     "auth token error",
			err:      errors.New("invalid auth token"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("network timeout"),
			expected: false,
		},
		{
			name:     "label error is not auth",
			err:      errors.New("could not add label: automated not found"),
			expected: false,
		},
		{
			name:     "branch not found",
			err:      errors.New("branch not found: feature-branch"),
			expected: false,
		},
		{
			name:     "repository not found",
			err:      errors.New("repository not found"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("exit status 1"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isAuthError(tt.err)
			if got != tt.expected {
				t.Errorf("isAuthError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrGHNotAuthenticated(t *testing.T) {
	// ErrGHNotAuthenticated should be defined and usable
	if ErrGHNotAuthenticated == nil {
		t.Error("ErrGHNotAuthenticated should not be nil")
	}
	if ErrGHNotAuthenticated.Error() != "GitHub CLI not authenticated" {
		t.Errorf("ErrGHNotAuthenticated.Error() = %s, want 'GitHub CLI not authenticated'",
			ErrGHNotAuthenticated.Error())
	}
}

func TestPRReviewResult(t *testing.T) {
	// Test that PRReviewResult struct works correctly
	result := PRReviewResult{
		Approved: true,
		Comment:  "Test comment",
	}

	if !result.Approved {
		t.Error("expected Approved to be true")
	}
	if result.Comment != "Test comment" {
		t.Errorf("expected Comment to be 'Test comment', got %s", result.Comment)
	}
}

func TestReviewAndApprove_FailsOnCheckFailure(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:    "TEST-001",
		Title: "Test task",
	}

	result, err := e.reviewAndApprove(nil, tsk, "", false, "Tests failed: 5 failures")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Approved {
		t.Error("expected not approved when checks fail")
	}
	if !strings.Contains(result.Comment, "CI checks have not passed") {
		t.Errorf("expected comment to mention CI checks, got: %s", result.Comment)
	}
}

func TestReviewAndApprove_ApprovesOnCheckSuccess(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:    "TEST-002",
		Title: "Test task that should pass",
	}

	result, err := e.reviewAndApprove(nil, tsk, "diff content here", true, "All checks passed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Approved {
		t.Error("expected approved when checks pass")
	}
	if !strings.Contains(result.Comment, "Auto-approved by orc") {
		t.Errorf("expected comment to mention auto-approval, got: %s", result.Comment)
	}
	if !strings.Contains(result.Comment, "All checks passed") {
		t.Errorf("expected comment to include check status, got: %s", result.Comment)
	}
}

func TestReviewAndApprove_IncludesTaskTitle(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:    "TEST-003",
		Title: "My awesome feature",
	}

	result, err := e.reviewAndApprove(nil, tsk, "diff", true, "Success")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Comment, "My awesome feature") {
		t.Errorf("expected comment to include task title, got: %s", result.Comment)
	}
}

func TestReviewAndApprove_ApprovesWithPendingChecks(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:    "TEST-004",
		Title: "Task with pending checks",
	}

	// When checksOK is true but some are pending, should still approve
	result, err := e.reviewAndApprove(nil, tsk, "diff", true, "Some checks still pending")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Approved {
		t.Error("expected approved when checksOK is true (even with pending)")
	}
}

func TestParsePRChecks_UsesCorrectFields(t *testing.T) {
	// Test that we correctly parse the gh pr checks JSON output format
	// gh pr checks --json returns bucket field (pass/fail/pending/skipping/cancel)
	// not conclusion field (which was the old expected format)

	tests := []struct {
		name           string
		jsonOutput     string
		expectPassed   bool
		expectDetails  string
		expectErr      bool
	}{
		{
			name: "all checks pass",
			jsonOutput: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "test", "state": "completed", "bucket": "pass"}
			]`,
			expectPassed:  true,
			expectDetails: "All checks passed",
		},
		{
			name: "some checks fail",
			jsonOutput: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "test", "state": "completed", "bucket": "fail"}
			]`,
			expectPassed:  false,
			expectDetails: "Failed checks: test",
		},
		{
			name: "checks pending",
			jsonOutput: `[
				{"name": "build", "state": "pending", "bucket": "pending"},
				{"name": "test", "state": "completed", "bucket": "pass"}
			]`,
			expectPassed:  true,
			expectDetails: "Some checks still pending",
		},
		{
			name: "skipped checks are ok",
			jsonOutput: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "optional", "state": "completed", "bucket": "skipping"}
			]`,
			expectPassed:  true,
			expectDetails: "All checks passed",
		},
		{
			name: "cancelled checks are ok",
			jsonOutput: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "cancelled", "state": "completed", "bucket": "cancel"}
			]`,
			expectPassed:  true,
			expectDetails: "All checks passed",
		},
		{
			name:          "empty checks list",
			jsonOutput:    `[]`,
			expectPassed:  true,
			expectDetails: "All checks passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can't directly test checkPRStatus because it calls runGH,
			// but we can verify the parsing logic would work with the expected fields
			var checks []struct {
				Name   string `json:"name"`
				State  string `json:"state"`
				Bucket string `json:"bucket"`
			}
			err := json.Unmarshal([]byte(tt.jsonOutput), &checks)
			if err != nil {
				if !tt.expectErr {
					t.Fatalf("unexpected parse error: %v", err)
				}
				return
			}

			// Replicate the logic from checkPRStatus
			var failedChecks []string
			pending := false
			for _, c := range checks {
				switch c.Bucket {
				case "fail":
					failedChecks = append(failedChecks, c.Name)
				case "pending":
					pending = true
				}
			}

			var passed bool
			var details string
			if len(failedChecks) > 0 {
				passed = false
				details = "Failed checks: " + strings.Join(failedChecks, ", ")
			} else if pending {
				passed = true
				details = "Some checks still pending"
			} else {
				passed = true
				details = "All checks passed"
			}

			if passed != tt.expectPassed {
				t.Errorf("passed = %v, want %v", passed, tt.expectPassed)
			}
			if details != tt.expectDetails {
				t.Errorf("details = %q, want %q", details, tt.expectDetails)
			}
		})
	}
}
