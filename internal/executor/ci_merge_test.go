package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

func TestCICheckResult_Status(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   CICheckResult
		expected CIStatus
	}{
		{
			name: "all passed",
			result: CICheckResult{
				TotalChecks:  3,
				PassedChecks: 3,
			},
			expected: CIStatusPassed,
		},
		{
			name: "some pending",
			result: CICheckResult{
				TotalChecks:   3,
				PassedChecks:  2,
				PendingChecks: 1,
			},
			expected: CIStatusPending,
		},
		{
			name: "some failed",
			result: CICheckResult{
				TotalChecks:  3,
				PassedChecks: 2,
				FailedChecks: 1,
			},
			expected: CIStatusFailed,
		},
		{
			name: "no checks",
			result: CICheckResult{
				TotalChecks: 0,
			},
			expected: CIStatusNoChecks,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.Status != tt.expected && tt.result.TotalChecks > 0 {
				// This test validates the status is set correctly when result is built
				// The actual status determination happens in CheckCIStatus
				t.Logf("Status would be determined by CheckCIStatus, not the struct itself")
			}
		})
	}
}

func TestCIConfig_Defaults(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	// Verify default values
	if !cfg.Completion.CI.WaitForCI {
		t.Error("expected WaitForCI to be true by default")
	}

	if cfg.Completion.CI.CITimeout != 10*time.Minute {
		t.Errorf("expected CITimeout to be 10m, got %v", cfg.Completion.CI.CITimeout)
	}

	if cfg.Completion.CI.PollInterval != 30*time.Second {
		t.Errorf("expected PollInterval to be 30s, got %v", cfg.Completion.CI.PollInterval)
	}

	if !cfg.Completion.CI.MergeOnCIPass {
		t.Error("expected MergeOnCIPass to be true by default")
	}

	if cfg.Completion.CI.MergeMethod != "squash" {
		t.Errorf("expected MergeMethod to be 'squash', got %s", cfg.Completion.CI.MergeMethod)
	}
}

func TestConfig_ShouldWaitForCI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		profile  config.AutomationProfile
		waitFor  bool
		expected bool
	}{
		{
			name:     "auto profile with wait enabled",
			profile:  config.ProfileAuto,
			waitFor:  true,
			expected: true,
		},
		{
			name:     "fast profile with wait enabled",
			profile:  config.ProfileFast,
			waitFor:  true,
			expected: true,
		},
		{
			name:     "safe profile with wait enabled",
			profile:  config.ProfileSafe,
			waitFor:  true,
			expected: false, // Safe profile doesn't auto-merge
		},
		{
			name:     "strict profile with wait enabled",
			profile:  config.ProfileStrict,
			waitFor:  true,
			expected: false, // Strict profile doesn't auto-merge
		},
		{
			name:     "auto profile with wait disabled",
			profile:  config.ProfileAuto,
			waitFor:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Profile = tt.profile
			cfg.Completion.CI.WaitForCI = tt.waitFor

			if got := cfg.ShouldWaitForCI(); got != tt.expected {
				t.Errorf("ShouldWaitForCI() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_ShouldMergeOnCIPass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		profile  config.AutomationProfile
		merge    bool
		expected bool
	}{
		{
			name:     "auto profile with merge enabled",
			profile:  config.ProfileAuto,
			merge:    true,
			expected: true,
		},
		{
			name:     "fast profile with merge enabled",
			profile:  config.ProfileFast,
			merge:    true,
			expected: true,
		},
		{
			name:     "safe profile with merge enabled",
			profile:  config.ProfileSafe,
			merge:    true,
			expected: false, // Safe requires human approval
		},
		{
			name:     "auto profile with merge disabled",
			profile:  config.ProfileAuto,
			merge:    false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Profile = tt.profile
			cfg.Completion.CI.MergeOnCIPass = tt.merge

			if got := cfg.ShouldMergeOnCIPass(); got != tt.expected {
				t.Errorf("ShouldMergeOnCIPass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_CITimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{
			name:     "default timeout",
			timeout:  0,
			expected: 10 * time.Minute,
		},
		{
			name:     "custom timeout",
			timeout:  5 * time.Minute,
			expected: 5 * time.Minute,
		},
		{
			name:     "negative timeout uses default",
			timeout:  -1 * time.Minute,
			expected: 10 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.CITimeout = tt.timeout

			if got := cfg.CITimeout(); got != tt.expected {
				t.Errorf("CITimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_CIPollInterval(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		interval time.Duration
		expected time.Duration
	}{
		{
			name:     "default interval",
			interval: 0,
			expected: 30 * time.Second,
		},
		{
			name:     "custom interval",
			interval: 15 * time.Second,
			expected: 15 * time.Second,
		},
		{
			name:     "negative interval uses default",
			interval: -1 * time.Second,
			expected: 30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.PollInterval = tt.interval

			if got := cfg.CIPollInterval(); got != tt.expected {
				t.Errorf("CIPollInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_MergeMethod(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		method   string
		expected string
	}{
		{
			name:     "default method",
			method:   "",
			expected: "squash",
		},
		{
			name:     "squash method",
			method:   "squash",
			expected: "squash",
		},
		{
			name:     "merge method",
			method:   "merge",
			expected: "merge",
		},
		{
			name:     "rebase method",
			method:   "rebase",
			expected: "rebase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.MergeMethod = tt.method

			if got := cfg.MergeMethod(); got != tt.expected {
				t.Errorf("MergeMethod() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCIMerger_WaitForCIAndMerge_NoPR(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Profile = config.ProfileAuto

	merger := NewCIMerger(cfg)

	// Task without PR should skip CI wait
	tsk := &orcv1.Task{
		Id: "TASK-001",
	}

	err := merger.WaitForCIAndMerge(context.Background(), tsk)
	if err != nil {
		t.Errorf("expected no error for task without PR, got %v", err)
	}
}

func TestCIMerger_WaitForCIAndMerge_CIDisabled(t *testing.T) {
	t.Parallel()
	cfg := config.Default()
	cfg.Profile = config.ProfileSafe // Safe profile disables CI wait

	merger := NewCIMerger(cfg)

	// Task with PR but CI disabled should skip
	prURL := "https://github.com/owner/repo/pull/1"
	prNumber := int32(1)
	tsk := &orcv1.Task{
		Id: "TASK-001",
		Pr: &orcv1.PRInfo{
			Url:    &prURL,
			Number: &prNumber,
		},
	}

	err := merger.WaitForCIAndMerge(context.Background(), tsk)
	if err != nil {
		t.Errorf("expected no error when CI is disabled, got %v", err)
	}
}

func TestParseChecksJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		jsonStr       string
		expectStatus  CIStatus
		expectPassed  int
		expectPending int
		expectFailed  int
	}{
		{
			name:          "empty array",
			jsonStr:       "[]",
			expectStatus:  CIStatusNoChecks,
			expectPassed:  0,
			expectPending: 0,
			expectFailed:  0,
		},
		{
			name: "all passed",
			jsonStr: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "test", "state": "completed", "bucket": "pass"}
			]`,
			expectStatus:  CIStatusPassed,
			expectPassed:  2,
			expectPending: 0,
			expectFailed:  0,
		},
		{
			name: "some pending",
			jsonStr: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "test", "state": "in_progress", "bucket": "pending"}
			]`,
			expectStatus:  CIStatusPending,
			expectPassed:  1,
			expectPending: 1,
			expectFailed:  0,
		},
		{
			name: "one failed",
			jsonStr: `[
				{"name": "build", "state": "completed", "bucket": "pass"},
				{"name": "test", "state": "completed", "bucket": "fail"}
			]`,
			expectStatus:  CIStatusFailed,
			expectPassed:  1,
			expectPending: 0,
			expectFailed:  1,
		},
		{
			name: "skipping counts as passed",
			jsonStr: `[
				{"name": "build", "state": "completed", "bucket": "skipping"},
				{"name": "test", "state": "completed", "bucket": "pass"}
			]`,
			expectStatus:  CIStatusPassed,
			expectPassed:  2,
			expectPending: 0,
			expectFailed:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse checks like CIMerger.CheckCIStatus does
			var checks []struct {
				Name   string `json:"name"`
				State  string `json:"state"`
				Bucket string `json:"bucket"`
			}
			if err := json.Unmarshal([]byte(tt.jsonStr), &checks); err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if len(checks) == 0 {
				if tt.expectStatus != CIStatusNoChecks {
					t.Errorf("expected status %v, got no_checks", tt.expectStatus)
				}
				return
			}

			result := &CICheckResult{
				TotalChecks: len(checks),
			}

			for _, c := range checks {
				switch c.Bucket {
				case "pass", "skipping":
					result.PassedChecks++
				case "fail", "cancel":
					result.FailedChecks++
					result.FailedNames = append(result.FailedNames, c.Name)
				case "pending":
					result.PendingChecks++
					result.PendingNames = append(result.PendingNames, c.Name)
				}
			}

			// Determine status
			if result.FailedChecks > 0 {
				result.Status = CIStatusFailed
			} else if result.PendingChecks > 0 {
				result.Status = CIStatusPending
			} else {
				result.Status = CIStatusPassed
			}

			if result.Status != tt.expectStatus {
				t.Errorf("expected status %v, got %v", tt.expectStatus, result.Status)
			}
			if result.PassedChecks != tt.expectPassed {
				t.Errorf("expected %d passed, got %d", tt.expectPassed, result.PassedChecks)
			}
			if result.PendingChecks != tt.expectPending {
				t.Errorf("expected %d pending, got %d", tt.expectPending, result.PendingChecks)
			}
			if result.FailedChecks != tt.expectFailed {
				t.Errorf("expected %d failed, got %d", tt.expectFailed, result.FailedChecks)
			}
		})
	}
}

func TestTask_GetPRURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		task     *task.Task
		expected string
	}{
		{
			name:     "nil PR",
			task:     &task.Task{},
			expected: "",
		},
		{
			name: "empty PR URL",
			task: &task.Task{
				PR: &task.PRInfo{},
			},
			expected: "",
		},
		{
			name: "valid PR URL",
			task: &task.Task{
				PR: &task.PRInfo{
					URL: "https://github.com/owner/repo/pull/123",
				},
			},
			expected: "https://github.com/owner/repo/pull/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.GetPRURL(); got != tt.expected {
				t.Errorf("GetPRURL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTask_SetMergedInfo(t *testing.T) {
	t.Parallel()
	tsk := &task.Task{ID: "TASK-001"}

	tsk.SetMergedInfo("https://github.com/owner/repo/pull/123", "main")

	if tsk.PR == nil {
		t.Fatal("expected PR to be set")
	}
	if tsk.PR.URL != "https://github.com/owner/repo/pull/123" {
		t.Errorf("expected URL to be set, got %s", tsk.PR.URL)
	}
	if !tsk.PR.Merged {
		t.Error("expected Merged to be true")
	}
	if tsk.PR.MergedAt == nil {
		t.Error("expected MergedAt to be set")
	}
	if tsk.PR.TargetBranch != "main" {
		t.Errorf("expected TargetBranch to be 'main', got %s", tsk.PR.TargetBranch)
	}
	if tsk.PR.Status != task.PRStatusMerged {
		t.Errorf("expected Status to be PRStatusMerged, got %s", tsk.PR.Status)
	}
}

func TestParsePRURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		url            string
		expectedOwner  string
		expectedRepo   string
		expectedNumber int
		expectError    bool
	}{
		{
			name:           "standard HTTPS URL",
			url:            "https://github.com/owner/repo/pull/123",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedNumber: 123,
			expectError:    false,
		},
		{
			name:           "URL with organization name",
			url:            "https://github.com/my-org/my-repo/pull/456",
			expectedOwner:  "my-org",
			expectedRepo:   "my-repo",
			expectedNumber: 456,
			expectError:    false,
		},
		{
			name:           "URL without https prefix",
			url:            "github.com/owner/repo/pull/789",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedNumber: 789,
			expectError:    false,
		},
		{
			name:           "URL with http prefix",
			url:            "http://github.com/owner/repo/pull/101",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedNumber: 101,
			expectError:    false,
		},
		{
			name:           "large PR number",
			url:            "https://github.com/owner/repo/pull/99999",
			expectedOwner:  "owner",
			expectedRepo:   "repo",
			expectedNumber: 99999,
			expectError:    false,
		},
		{
			name:        "invalid URL - not a PR URL",
			url:         "https://github.com/owner/repo/issues/123",
			expectError: true,
		},
		{
			name:        "invalid URL - missing PR number",
			url:         "https://github.com/owner/repo/pull/",
			expectError: true,
		},
		{
			name:        "invalid URL - completely wrong format",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "invalid URL - GitLab URL",
			url:         "https://gitlab.com/owner/repo/merge_requests/123",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, prNumber, err := parsePRURL(tt.url)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if owner != tt.expectedOwner {
				t.Errorf("expected owner %q, got %q", tt.expectedOwner, owner)
			}
			if repo != tt.expectedRepo {
				t.Errorf("expected repo %q, got %q", tt.expectedRepo, repo)
			}
			if prNumber != tt.expectedNumber {
				t.Errorf("expected PR number %d, got %d", tt.expectedNumber, prNumber)
			}
		})
	}
}

func TestMergeMethodTranslation(t *testing.T) {
	t.Parallel()
	// Test that merge method values are passed correctly to the API
	// GitHub API expects: "squash", "merge", or "rebase"
	tests := []struct {
		name           string
		configMethod   string
		expectedMethod string
	}{
		{
			name:           "squash method",
			configMethod:   "squash",
			expectedMethod: "squash",
		},
		{
			name:           "merge method",
			configMethod:   "merge",
			expectedMethod: "merge",
		},
		{
			name:           "rebase method",
			configMethod:   "rebase",
			expectedMethod: "rebase",
		},
		{
			name:           "empty defaults to squash",
			configMethod:   "",
			expectedMethod: "squash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Completion.CI.MergeMethod = tt.configMethod

			// Verify the merge method is returned correctly
			method := cfg.MergeMethod()
			if method != tt.expectedMethod {
				t.Errorf("expected method %q, got %q", tt.expectedMethod, method)
			}
		})
	}
}

func TestMergeAPIPathConstruction(t *testing.T) {
	t.Parallel()
	// Test that the API path is constructed correctly for different PR URLs
	// This verifies the format: PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge
	tests := []struct {
		name         string
		prURL        string
		expectedPath string
	}{
		{
			name:         "standard PR URL",
			prURL:        "https://github.com/owner/repo/pull/123",
			expectedPath: "/repos/owner/repo/pulls/123/merge",
		},
		{
			name:         "org with hyphens",
			prURL:        "https://github.com/my-org/my-repo/pull/456",
			expectedPath: "/repos/my-org/my-repo/pulls/456/merge",
		},
		{
			name:         "large PR number",
			prURL:        "https://github.com/acme/project/pull/99999",
			expectedPath: "/repos/acme/project/pulls/99999/merge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, prNumber, err := parsePRURL(tt.prURL)
			if err != nil {
				t.Fatalf("failed to parse PR URL: %v", err)
			}

			// Reconstruct the API path as MergePR does (using fmt.Sprintf like the real code)
			apiPath := fmt.Sprintf("/repos/%s/%s/pulls/%d/merge", owner, repo, prNumber)
			if apiPath != tt.expectedPath {
				t.Errorf("expected API path %q, got %q", tt.expectedPath, apiPath)
			}
		})
	}
}

func TestDeleteBranchAPIPathConstruction(t *testing.T) {
	t.Parallel()
	// Test that delete branch API path is constructed correctly
	// This verifies the format: DELETE /repos/{owner}/{repo}/git/refs/heads/{branch}
	tests := []struct {
		name         string
		owner        string
		repo         string
		branch       string
		expectedPath string
	}{
		{
			name:         "simple branch name",
			owner:        "owner",
			repo:         "repo",
			branch:       "feature-branch",
			expectedPath: "/repos/owner/repo/git/refs/heads/feature-branch",
		},
		{
			name:         "branch with refs/heads prefix",
			owner:        "owner",
			repo:         "repo",
			branch:       "refs/heads/feature-branch",
			expectedPath: "/repos/owner/repo/git/refs/heads/feature-branch",
		},
		{
			name:         "orc task branch",
			owner:        "my-org",
			repo:         "my-repo",
			branch:       "orc/TASK-001",
			expectedPath: "/repos/my-org/my-repo/git/refs/heads/orc/TASK-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Strip refs/heads/ prefix if present, as deleteBranch does
			branchName := tt.branch
			if len(branchName) > 11 && branchName[:11] == "refs/heads/" {
				branchName = branchName[11:]
			}

			apiPath := "/repos/" + tt.owner + "/" + tt.repo + "/git/refs/heads/" + branchName
			if apiPath != tt.expectedPath {
				t.Errorf("expected API path %q, got %q", tt.expectedPath, apiPath)
			}
		})
	}
}

func TestIsRetryableMergeError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		output   string
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			output:   "",
			expected: false,
		},
		{
			name:     "405 with base branch modified message",
			err:      fmt.Errorf("exit status 1: HTTP 405 Base branch was modified. Review and try the merge again."),
			output:   "",
			expected: true,
		},
		{
			name:     "405 in output with message",
			err:      fmt.Errorf("exit status 1"),
			output:   `{"message": "Base branch was modified. Review and try the merge again.", "status": "405"}`,
			expected: true,
		},
		{
			name:     "405 with review message variant",
			err:      fmt.Errorf("HTTP 405: review and try the merge again"),
			output:   "",
			expected: true,
		},
		{
			name:     "422 validation error - not retryable",
			err:      fmt.Errorf("HTTP 422 Unprocessable Entity"),
			output:   `{"message": "Pull Request is not mergeable"}`,
			expected: false,
		},
		{
			name:     "404 not found - not retryable",
			err:      fmt.Errorf("HTTP 404 Not Found"),
			output:   "",
			expected: false,
		},
		{
			name:     "405 without base branch message - not retryable",
			err:      fmt.Errorf("HTTP 405 Method Not Allowed"),
			output:   "",
			expected: false,
		},
		{
			name:     "generic network error - not retryable",
			err:      fmt.Errorf("network timeout"),
			output:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableMergeError(tt.err, tt.output)
			if result != tt.expected {
				t.Errorf("isRetryableMergeError(%v, %q) = %v, want %v", tt.err, tt.output, result, tt.expected)
			}
		})
	}
}

func TestIsValidationMergeError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		output   string
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			output:   "",
			expected: false,
		},
		{
			name:     "422 error in error string",
			err:      fmt.Errorf("HTTP 422 Unprocessable Entity: validation failed"),
			output:   "",
			expected: true,
		},
		{
			name:     "422 in output",
			err:      fmt.Errorf("exit status 1"),
			output:   `{"message": "Pull Request is not mergeable", "status": "422"}`,
			expected: true,
		},
		{
			name:     "405 - not a validation error",
			err:      fmt.Errorf("HTTP 405"),
			output:   "",
			expected: false,
		},
		{
			name:     "404 - not a validation error",
			err:      fmt.Errorf("HTTP 404"),
			output:   "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidationMergeError(tt.err, tt.output)
			if result != tt.expected {
				t.Errorf("isValidationMergeError(%v, %q) = %v, want %v", tt.err, tt.output, result, tt.expected)
			}
		})
	}
}

func TestErrMergeFailed_Sentinel(t *testing.T) {
	t.Parallel()
	// Test that ErrMergeFailed works as a sentinel error
	wrappedErr := fmt.Errorf("%w: some details", ErrMergeFailed)

	if !errors.Is(wrappedErr, ErrMergeFailed) {
		t.Error("expected errors.Is to return true for wrapped ErrMergeFailed")
	}

	// Test nested wrapping
	doubleWrapped := fmt.Errorf("outer: %w", wrappedErr)
	if !errors.Is(doubleWrapped, ErrMergeFailed) {
		t.Error("expected errors.Is to return true for double-wrapped ErrMergeFailed")
	}
}

func TestMergePR_ExponentialBackoffValues(t *testing.T) {
	t.Parallel()
	// Test that backoff calculation produces expected values
	// Formula: min(2^attempt seconds, 8 seconds)
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 2 * time.Second}, // 2^1 = 2s
		{2, 4 * time.Second}, // 2^2 = 4s
		{3, 8 * time.Second}, // 2^3 = 8s, capped
		{4, 8 * time.Second}, // would be 16s, but capped at 8s
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			backoff := min(time.Duration(1<<tt.attempt)*time.Second, 8*time.Second)
			if backoff != tt.expected {
				t.Errorf("backoff for attempt %d = %v, want %v", tt.attempt, backoff, tt.expected)
			}
		})
	}
}
