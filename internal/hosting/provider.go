// Package hosting provides a unified interface for git hosting providers (GitHub, GitLab).
package hosting

import (
	"context"
)

// ProviderType identifies which hosting provider is in use.
type ProviderType string

const (
	ProviderGitHub  ProviderType = "github"
	ProviderGitLab  ProviderType = "gitlab"
	ProviderUnknown ProviderType = "unknown"
)

// Provider is the interface for git hosting providers.
// Implementations exist for GitHub (go-github) and GitLab (go-gitlab).
type Provider interface {
	// PR / Merge Request operations
	CreatePR(ctx context.Context, opts PRCreateOptions) (*PR, error)
	GetPR(ctx context.Context, number int) (*PR, error)
	UpdatePR(ctx context.Context, number int, opts PRUpdateOptions) error
	MergePR(ctx context.Context, number int, opts PRMergeOptions) error
	FindPRByBranch(ctx context.Context, branch string) (*PR, error)

	// Comments
	ListPRComments(ctx context.Context, number int) ([]PRComment, error)
	CreatePRComment(ctx context.Context, number int, comment PRCommentCreate) (*PRComment, error)
	ReplyToComment(ctx context.Context, number int, threadID int64, body string) (*PRComment, error)
	GetPRComment(ctx context.Context, prNumber int, commentID int64) (*PRComment, error)

	// CI status (GitHub check runs / GitLab pipelines → unified)
	GetCheckRuns(ctx context.Context, ref string) ([]CheckRun, error)

	// Reviews / Approvals
	GetPRReviews(ctx context.Context, number int) ([]PRReview, error)
	ApprovePR(ctx context.Context, number int, body string) error

	// Status summary
	GetPRStatusSummary(ctx context.Context, pr *PR) (*PRStatusSummary, error)

	// Auto-merge
	// EnableAutoMerge enables auto-merge for a PR with the given merge method.
	// GitHub: returns ErrAutoMergeNotSupported (requires GraphQL, not REST).
	// GitLab: sets merge_when_pipeline_succeeds via AcceptMergeRequest.
	EnableAutoMerge(ctx context.Context, number int, method string) error

	// Branch updates
	// UpdatePRBranch updates the PR branch with the latest base branch changes.
	// GitHub: PullRequests.UpdateBranch(). GitLab: MergeRequests.RebaseMergeRequest().
	UpdatePRBranch(ctx context.Context, number int) error

	// Branch operations
	DeleteBranch(ctx context.Context, branch string) error

	// Auth + metadata
	CheckAuth(ctx context.Context) error
	Name() ProviderType
	OwnerRepo() (string, string)
}

// PR represents a pull request / merge request.
type PR struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	State      string `json:"state"` // open, closed, merged
	HeadBranch string `json:"head_branch"`
	BaseBranch string `json:"base_branch"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Mergeable  bool     `json:"mergeable"`
	HeadSHA    string   `json:"head_sha,omitempty"`  // HEAD SHA for merge verification
	Labels     []string `json:"labels,omitempty"`    // Current labels
	Assignees  []string `json:"assignees,omitempty"` // Current assignees
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// PRCreateOptions for creating a PR / merge request.
type PRCreateOptions struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Head      string   `json:"head"` // Source branch
	Base      string   `json:"base"` // Target branch
	Draft     bool     `json:"draft"`
	Labels    []string `json:"labels,omitempty"`
	Reviewers           []string `json:"reviewers,omitempty"`
	TeamReviewers       []string `json:"team_reviewers,omitempty"`        // Team slugs (GitHub) / group paths (GitLab)
	Assignees           []string `json:"assignees,omitempty"`             // Usernames to assign
	MaintainerCanModify bool     `json:"maintainer_can_modify,omitempty"` // GitHub: maintainer_can_modify, GitLab: allow_collaboration
}

// PRUpdateOptions for updating a PR / merge request.
type PRUpdateOptions struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	State string `json:"state,omitempty"` // open, closed
}

// PRMergeOptions for merging a PR / merge request.
type PRMergeOptions struct {
	Method       string `json:"method"` // merge, squash, rebase
	CommitTitle         string `json:"commit_title,omitempty"`
	CommitMessage       string `json:"commit_message,omitempty"`        // Merge commit body
	SquashCommitMessage string `json:"squash_commit_message,omitempty"` // Squash-specific commit message
	SHA                 string `json:"sha,omitempty"`                   // HEAD SHA to verify before merge
	DeleteBranch        bool   `json:"delete_branch"`
}

// PRComment represents a PR comment / MR note.
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

// PRCommentCreate for creating a comment / note.
type PRCommentCreate struct {
	Body string `json:"body"`
	Path string `json:"path,omitempty"`
	Line int    `json:"line,omitempty"`
	Side string `json:"side,omitempty"`
}

// CheckRun represents a CI check (GitHub check run / GitLab pipeline job).
type CheckRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`               // queued, in_progress, completed
	Conclusion string `json:"conclusion,omitempty"` // success, failure, neutral, etc.
}

// PRReview represents a pull request review / merge request approval.
type PRReview struct {
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	State     string `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, DISMISSED, PENDING
	Body      string `json:"body,omitempty"`
	CreatedAt string `json:"created_at"`
}

// PRStatusSummary aggregates PR status information.
type PRStatusSummary struct {
	ReviewStatus  string // pending_review, changes_requested, approved
	ReviewCount   int
	ApprovalCount int
	ChecksStatus  string // pending, success, failure
	Mergeable     bool
}
