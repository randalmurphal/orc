package hosting

import "errors"

// Hosting provider errors.
var (
	// ErrNoPRFound is returned when no PR/MR exists for the given branch.
	ErrNoPRFound = errors.New("no pull request found for branch")

	// ErrAuthFailed is returned when authentication fails.
	ErrAuthFailed = errors.New("authentication failed")

	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = errors.New("not found")

	// ErrAutoMergeNotSupported is returned when the provider doesn't support auto-merge.
	ErrAutoMergeNotSupported = errors.New("auto-merge not supported by this provider")
)
