// Package github provides GitHub PR integration using the gh CLI.
package github

import (
	"context"
)

// Provider is the interface for git hosting providers.
// Currently GitHub, designed for future GitLab/Bitbucket support.
type Provider interface {
	// PR Operations
	CreatePR(ctx context.Context, opts PRCreateOptions) (*PR, error)
	GetPR(ctx context.Context, number int) (*PR, error)
	UpdatePR(ctx context.Context, number int, opts PRUpdateOptions) error
	MergePR(ctx context.Context, number int, opts PRMergeOptions) error

	// Comments
	ListPRComments(ctx context.Context, number int) ([]PRComment, error)
	CreatePRComment(ctx context.Context, number int, comment PRCommentCreate) (*PRComment, error)
	ReplyToComment(ctx context.Context, number int, threadID int64, body string) (*PRComment, error)

	// Checks/Status
	GetCheckRuns(ctx context.Context, ref string) ([]CheckRun, error)

	// Reviews
	GetPRReviews(ctx context.Context, number int) ([]PRReview, error)

	// PR discovery
	FindPRByBranch(ctx context.Context, branch string) (*PR, error)
}

// PR represents a pull request.
type PR struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	State      string `json:"state"` // open, closed, merged
	HeadBranch string `json:"head_branch"`
	BaseBranch string `json:"base_branch"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Mergeable  bool   `json:"mergeable"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// PRCreateOptions for creating a PR.
type PRCreateOptions struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Head      string   `json:"head"` // Source branch
	Base      string   `json:"base"` // Target branch
	Draft     bool     `json:"draft"`
	Labels    []string `json:"labels,omitempty"`
	Reviewers []string `json:"reviewers,omitempty"`
}

// PRUpdateOptions for updating a PR.
type PRUpdateOptions struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	State string `json:"state,omitempty"` // open, closed
}

// PRMergeOptions for merging a PR.
type PRMergeOptions struct {
	Method       string `json:"method"` // merge, squash, rebase
	CommitTitle  string `json:"commit_title,omitempty"`
	DeleteBranch bool   `json:"delete_branch"`
}

// PRComment represents a PR comment.
type PRComment struct {
	ID        int64  `json:"id"`
	Body      string `json:"body"`
	Path      string `json:"path,omitempty"` // File path for inline comments
	Line      int    `json:"line,omitempty"`
	Side      string `json:"side,omitempty"` // LEFT or RIGHT
	ThreadID  int64  `json:"thread_id,omitempty"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
}

// PRCommentCreate for creating a comment.
type PRCommentCreate struct {
	Body string `json:"body"`
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
	Side string `json:"side,omitempty"`
}

// CheckRun represents a CI check.
type CheckRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`               // queued, in_progress, completed
	Conclusion string `json:"conclusion,omitempty"` // success, failure, neutral, etc.
}

// PRReview represents a pull request review.
type PRReview struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	State     string `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	Body      string `json:"body,omitempty"`
	CreatedAt string `json:"created_at"`
}
