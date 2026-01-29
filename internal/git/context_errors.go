package git

import "errors"

// Git context operation errors.
// These were absorbed from devflow/git to eliminate the external dependency.
//
// Note: ErrMergeConflict is defined in git_status.go (orc's original).
var (
	// ErrNotGitRepo indicates the path is not a git repository.
	ErrNotGitRepo = errors.New("not a git repository")

	// ErrWorktreeExists indicates a worktree already exists for the branch.
	ErrWorktreeExists = errors.New("worktree already exists for this branch")

	// ErrWorktreeNotFound indicates the worktree does not exist.
	ErrWorktreeNotFound = errors.New("worktree not found")

	// ErrBranchExists indicates the branch already exists.
	ErrBranchExists = errors.New("branch already exists")

	// ErrBranchNotFound indicates the branch does not exist.
	ErrBranchNotFound = errors.New("branch not found")

	// ErrGitDirty indicates the working directory has uncommitted changes.
	ErrGitDirty = errors.New("working directory has uncommitted changes")

	// ErrNothingToCommit indicates there are no staged changes to commit.
	ErrNothingToCommit = errors.New("nothing to commit")

	// ErrPushFailed indicates a push operation failed.
	ErrPushFailed = errors.New("push failed")
)

// GitError wraps a git command error with context.
// Named GitError (not Error) to avoid collision with the builtin error interface.
type GitError struct {
	Op     string // Operation that failed (e.g., "commit", "push")
	Cmd    string // Git command that was run
	Output string // Combined stdout/stderr output
	Err    error  // Underlying error
}

func (e *GitError) Error() string {
	if e.Output != "" {
		return e.Op + ": " + e.Output
	}
	return e.Op + ": " + e.Err.Error()
}

func (e *GitError) Unwrap() error {
	return e.Err
}
