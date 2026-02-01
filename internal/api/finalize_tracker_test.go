package api

import (
	"errors"
	"strings"
	"testing"
)

func TestBuildFinalizeRetryContext_NilError(t *testing.T) {
	result := buildFinalizeRetryContext(nil)
	if result != "" {
		t.Errorf("expected empty string for nil error, got %q", result)
	}
}

func TestBuildFinalizeRetryContext_ConflictError(t *testing.T) {
	err := errors.New("merge conflict in file.go")
	result := buildFinalizeRetryContext(err)

	if !strings.Contains(result, "merge conflicts") {
		t.Errorf("expected conflict context, got %q", result)
	}
	if !strings.Contains(result, "merge conflict in file.go") {
		t.Errorf("expected original error in context, got %q", result)
	}
	if !strings.Contains(result, "On resume") {
		t.Errorf("expected resume guidance, got %q", result)
	}
}

func TestBuildFinalizeRetryContext_TestError(t *testing.T) {
	err := errors.New("test failures: 3 tests failed")
	result := buildFinalizeRetryContext(err)

	if !strings.Contains(result, "test failures") {
		t.Errorf("expected test failure context, got %q", result)
	}
	if !strings.Contains(result, "3 tests failed") {
		t.Errorf("expected original error in context, got %q", result)
	}
	if !strings.Contains(result, "On resume") {
		t.Errorf("expected resume guidance, got %q", result)
	}
}

func TestBuildFinalizeRetryContext_RebaseError(t *testing.T) {
	err := errors.New("rebase failed: diverged branches")
	result := buildFinalizeRetryContext(err)

	if !strings.Contains(result, "during rebase") {
		t.Errorf("expected rebase context, got %q", result)
	}
	if !strings.Contains(result, "diverged branches") {
		t.Errorf("expected original error in context, got %q", result)
	}
	if !strings.Contains(result, "merge strategy") {
		t.Errorf("expected merge strategy suggestion, got %q", result)
	}
}

func TestBuildFinalizeRetryContext_GitError(t *testing.T) {
	err := errors.New("git push failed: remote rejected")
	result := buildFinalizeRetryContext(err)

	if !strings.Contains(result, "git operation") {
		t.Errorf("expected git operation context, got %q", result)
	}
	if !strings.Contains(result, "remote rejected") {
		t.Errorf("expected original error in context, got %q", result)
	}
	if !strings.Contains(result, "worktree state") {
		t.Errorf("expected worktree guidance, got %q", result)
	}
}

func TestBuildFinalizeRetryContext_GenericError(t *testing.T) {
	err := errors.New("unexpected network timeout")
	result := buildFinalizeRetryContext(err)

	if !strings.HasPrefix(result, "Finalize failed:") {
		t.Errorf("expected generic prefix, got %q", result)
	}
	if !strings.Contains(result, "unexpected network timeout") {
		t.Errorf("expected original error in context, got %q", result)
	}
}

func TestBuildFinalizeRetryContext_CaseInsensitive(t *testing.T) {
	// Test that matching is case-insensitive
	testCases := []struct {
		name     string
		errMsg   string
		contains string
	}{
		{"uppercase CONFLICT", "CONFLICT detected", "merge conflicts"},
		{"uppercase TEST", "TEST failures", "test failures"},
		{"uppercase REBASE", "REBASE error", "during rebase"},
		{"uppercase GIT", "GIT error", "git operation"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := errors.New(tc.errMsg)
			result := buildFinalizeRetryContext(err)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("expected %q in context for error %q, got %q", tc.contains, tc.errMsg, result)
			}
		})
	}
}
