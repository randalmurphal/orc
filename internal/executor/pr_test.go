package executor

import (
	"github.com/randalmurphal/orc/internal/git"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrSyncConflict(t *testing.T) {
	t.Parallel()
	// ErrSyncConflict should be defined and usable
	if ErrSyncConflict == nil {
		t.Error("ErrSyncConflict should not be nil")
	}
	if ErrSyncConflict.Error() != "sync conflict detected" {
		t.Errorf("ErrSyncConflict.Error() = %s, want 'sync conflict detected'", ErrSyncConflict.Error())
	}
}

func TestSyncPhaseConstants(t *testing.T) {
	t.Parallel()
	// Verify sync phase constants are defined
	if SyncPhaseStart != "start" {
		t.Errorf("SyncPhaseStart = %s, want 'start'", SyncPhaseStart)
	}
	if SyncPhaseCompletion != "completion" {
		t.Errorf("SyncPhaseCompletion = %s, want 'completion'", SyncPhaseCompletion)
	}
}

func TestErrTaskBlocked(t *testing.T) {
	t.Parallel()
	// ErrTaskBlocked should be defined and usable
	if ErrTaskBlocked == nil {
		t.Error("ErrTaskBlocked should not be nil")
	}
	if ErrTaskBlocked.Error() != "task blocked" {
		t.Errorf("ErrTaskBlocked.Error() = %s, want 'task blocked'", ErrTaskBlocked.Error())
	}
}

func TestErrTaskBlocked_WrappingWithFmtErrorf(t *testing.T) {
	t.Parallel()
	// Test that ErrTaskBlocked can be properly wrapped using fmt.Errorf
	// This matches the pattern used in completeTask:
	//   return fmt.Errorf("%w: sync conflict - resolve conflicts then run 'orc resume %s'", ErrTaskBlocked, t.ID)
	taskID := "TASK-123"
	wrapped := fmt.Errorf("%w: sync conflict - resolve conflicts then run 'orc resume %s'", ErrTaskBlocked, taskID)

	// errors.Is should work with wrapped errors
	if !errors.Is(wrapped, ErrTaskBlocked) {
		t.Error("errors.Is(wrapped, ErrTaskBlocked) should return true")
	}

	// The error message should include the task ID
	if !strings.Contains(wrapped.Error(), taskID) {
		t.Errorf("wrapped error should contain task ID, got: %s", wrapped.Error())
	}

	// The error message should include the original sentinel error message
	if !strings.Contains(wrapped.Error(), ErrTaskBlocked.Error()) {
		t.Errorf("wrapped error should contain sentinel error message, got: %s", wrapped.Error())
	}
}

func TestIsNonFastForwardError(t *testing.T) {
	t.Parallel()
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
			name:     "non-fast-forward explicit",
			err:      errors.New("error: failed to push some refs to 'origin'\nhint: Updates were rejected because the tip of your current branch is behind\n ! [rejected] orc/TASK-001 -> orc/TASK-001 (non-fast-forward)"),
			expected: true,
		},
		{
			name:     "non-fast-forward lowercase",
			err:      errors.New("non-fast-forward"),
			expected: true,
		},
		{
			name:     "rejected with fetch first",
			err:      errors.New("rejected: fetch first"),
			expected: true,
		},
		{
			name:     "failed to push behind",
			err:      errors.New("error: failed to push some refs, your branch is behind"),
			expected: true,
		},
		{
			name:     "failed to push behind different format",
			err:      errors.New("To github.com:user/repo.git\n ! [rejected]        orc/TASK-001 -> orc/TASK-001 (non-fast-forward)\nerror: failed to push some refs to 'github.com:user/repo.git'\nhint: Updates were rejected because the tip of your current branch is behind\nhint: its remote counterpart."),
			expected: true,
		},
		{
			name:     "unrelated network error",
			err:      errors.New("network timeout"),
			expected: false,
		},
		{
			name:     "auth error is not fast-forward",
			err:      errors.New("gh: not authenticated"),
			expected: false,
		},
		{
			name:     "remote not found",
			err:      errors.New("remote origin not found"),
			expected: false,
		},
		{
			name:     "branch not found",
			err:      errors.New("branch not found: orc/TASK-001"),
			expected: false,
		},
		{
			name:     "generic git error",
			err:      errors.New("exit status 128"),
			expected: false,
		},
		{
			name:     "permission denied",
			err:      errors.New("permission denied (publickey)"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := git.IsNonFastForwardError(tt.err)
			if got != tt.expected {
				t.Errorf("git.IsNonFastForwardError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestErrMergeFailed(t *testing.T) {
	t.Parallel()
	// ErrMergeFailed should be defined and usable
	if ErrMergeFailed == nil {
		t.Error("ErrMergeFailed should not be nil")
	}
	if ErrMergeFailed.Error() != "PR merge failed" {
		t.Errorf("ErrMergeFailed.Error() = %s, want 'PR merge failed'", ErrMergeFailed.Error())
	}
}

func TestErrMergeFailed_WrappingWithFmtErrorf(t *testing.T) {
	t.Parallel()
	// Test that ErrMergeFailed can be properly wrapped using fmt.Errorf
	// This matches the pattern used in completeTask:
	//   return fmt.Errorf("%w: merge failed - run 'orc resume %s' after resolving", ErrTaskBlocked, t.ID)
	taskID := "TASK-456"
	wrapped := fmt.Errorf("%w: max retries exceeded for %s", ErrMergeFailed, taskID)

	// errors.Is should work with wrapped errors
	if !errors.Is(wrapped, ErrMergeFailed) {
		t.Error("errors.Is(wrapped, ErrMergeFailed) should return true")
	}

	// The error message should include the task ID
	if !strings.Contains(wrapped.Error(), taskID) {
		t.Errorf("wrapped error should contain task ID, got: %s", wrapped.Error())
	}

	// The error message should include the original sentinel error message
	if !strings.Contains(wrapped.Error(), ErrMergeFailed.Error()) {
		t.Errorf("wrapped error should contain sentinel error message, got: %s", wrapped.Error())
	}
}

func TestErrMergeFailed_NestedWrapping(t *testing.T) {
	t.Parallel()
	// Test that ErrMergeFailed can be detected through multiple layers of wrapping
	// This is important because the error flows through multiple function calls:
	// MergePR -> WaitForCIAndMerge -> createPR -> runCompletion -> completeTask

	// Simulate the wrapping chain
	mergeErr := fmt.Errorf("%w: max retries (3) exceeded: HTTP 405", ErrMergeFailed)
	ciMergeErr := fmt.Errorf("merge PR: %w", mergeErr)
	completionErr := fmt.Errorf("completion action: %w", ciMergeErr)

	// errors.Is should still find ErrMergeFailed through the chain
	if !errors.Is(completionErr, ErrMergeFailed) {
		t.Errorf("errors.Is should find ErrMergeFailed through wrapping chain, got error: %s", completionErr)
	}
}

func TestErrDirectMergeBlocked(t *testing.T) {
	t.Parallel()
	// ErrDirectMergeBlocked should be defined and usable
	if ErrDirectMergeBlocked == nil {
		t.Error("ErrDirectMergeBlocked should not be nil")
	}
	if ErrDirectMergeBlocked.Error() != "direct merge to protected branch blocked" {
		t.Errorf("ErrDirectMergeBlocked.Error() = %s, want 'direct merge to protected branch blocked'", ErrDirectMergeBlocked.Error())
	}
}
