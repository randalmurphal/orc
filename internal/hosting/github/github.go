package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	gogithub "github.com/google/go-github/v82/github"

	"github.com/randalmurphal/orc/internal/hosting"
)

// Compile-time interface check.
var _ hosting.Provider = (*GitHubProvider)(nil)

func init() {
	hosting.RegisterProvider(hosting.ProviderGitHub, newProvider)
}

// GitHubProvider implements hosting.Provider using the go-github library.
type GitHubProvider struct {
	client *gogithub.Client
	owner  string
	repo   string
}

// newProvider creates a new GitHubProvider from the working directory and config.
func newProvider(workDir string, cfg hosting.Config) (hosting.Provider, error) {
	token, err := resolveToken(cfg)
	if err != nil {
		return nil, err
	}

	// Get remote URL from git.
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))
	owner, repo := hosting.ParseOwnerRepo(remoteURL)
	if owner == "" || repo == "" {
		return nil, fmt.Errorf("could not parse owner/repo from remote URL: %s", remoteURL)
	}

	// Create authenticated HTTP client and go-github client.
	httpClient := &http.Client{
		Transport: &oauth2Transport{token: token},
	}

	client := gogithub.NewClient(httpClient)

	// GitHub Enterprise: override base URL.
	if cfg.BaseURL != "" {
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		var parseErr error
		client.BaseURL, parseErr = client.BaseURL.Parse(baseURL + "/api/v3/")
		if parseErr != nil {
			return nil, fmt.Errorf("parse base URL %q: %w", cfg.BaseURL, parseErr)
		}
		client.UploadURL, parseErr = client.UploadURL.Parse(baseURL + "/api/uploads/")
		if parseErr != nil {
			return nil, fmt.Errorf("parse upload URL %q: %w", cfg.BaseURL, parseErr)
		}
	}

	return &GitHubProvider{
		client: client,
		owner:  owner,
		repo:   repo,
	}, nil
}

// oauth2Transport adds an Authorization header to every request.
type oauth2Transport struct {
	token string
	base  http.RoundTripper
}

func (t *oauth2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req2)
}

// Name returns the provider type.
func (g *GitHubProvider) Name() hosting.ProviderType {
	return hosting.ProviderGitHub
}

// OwnerRepo returns the owner and repository name.
func (g *GitHubProvider) OwnerRepo() (string, string) {
	return g.owner, g.repo
}

// CheckAuth validates the token by fetching the authenticated user.
func (g *GitHubProvider) CheckAuth(ctx context.Context) error {
	_, _, err := g.client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("check auth: %w", err)
	}
	return nil
}

// CreatePR creates a pull request.
func (g *GitHubProvider) CreatePR(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error) {
	newPR := &gogithub.NewPullRequest{
		Title:               gogithub.Ptr(opts.Title),
		Body:                gogithub.Ptr(opts.Body),
		Head:                gogithub.Ptr(opts.Head),
		Base:                gogithub.Ptr(opts.Base),
		Draft:               gogithub.Ptr(opts.Draft),
		MaintainerCanModify: gogithub.Ptr(opts.MaintainerCanModify),
	}

	created, _, err := g.client.PullRequests.Create(ctx, g.owner, g.repo, newPR)
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}

	prNumber := created.GetNumber()

	// Add labels (best-effort).
	if len(opts.Labels) > 0 {
		_, _, labelErr := g.client.Issues.AddLabelsToIssue(ctx, g.owner, g.repo, prNumber, opts.Labels)
		if labelErr != nil {
			slog.Warn("failed to add labels to PR",
				"pr", prNumber,
				"labels", opts.Labels,
				"error", labelErr)
		}
	}

	// Request reviewers (best-effort).
	if len(opts.Reviewers) > 0 || len(opts.TeamReviewers) > 0 {
		reviewersReq := gogithub.ReviewersRequest{
			Reviewers:     opts.Reviewers,
			TeamReviewers: opts.TeamReviewers,
		}
		_, _, revErr := g.client.PullRequests.RequestReviewers(ctx, g.owner, g.repo, prNumber, reviewersReq)
		if revErr != nil {
			slog.Warn("failed to request reviewers for PR",
				"pr", prNumber,
				"reviewers", opts.Reviewers,
				"team_reviewers", opts.TeamReviewers,
				"error", revErr)
		}
	}

	// Add assignees (best-effort).
	if len(opts.Assignees) > 0 {
		_, _, assignErr := g.client.Issues.AddAssignees(ctx, g.owner, g.repo, prNumber, opts.Assignees)
		if assignErr != nil {
			slog.Warn("failed to add assignees to PR",
				"pr", prNumber,
				"assignees", opts.Assignees,
				"error", assignErr)
		}
	}

	return g.GetPR(ctx, prNumber)
}

// GetPR gets a pull request by number.
func (g *GitHubProvider) GetPR(ctx context.Context, number int) (*hosting.PR, error) {
	pr, _, err := g.client.PullRequests.Get(ctx, g.owner, g.repo, number)
	if err != nil {
		return nil, fmt.Errorf("get PR %d: %w", number, err)
	}
	return mapPR(pr), nil
}

// UpdatePR updates a pull request's title, body, or state.
func (g *GitHubProvider) UpdatePR(ctx context.Context, number int, opts hosting.PRUpdateOptions) error {
	update := &gogithub.PullRequest{}

	if opts.Title != "" {
		update.Title = gogithub.Ptr(opts.Title)
	}
	if opts.Body != "" {
		update.Body = gogithub.Ptr(opts.Body)
	}
	if opts.State == "closed" || opts.State == "open" {
		update.State = gogithub.Ptr(opts.State)
	}

	_, _, err := g.client.PullRequests.Edit(ctx, g.owner, g.repo, number, update)
	if err != nil {
		return fmt.Errorf("update PR %d: %w", number, err)
	}
	return nil
}

// MergePR merges a pull request.
func (g *GitHubProvider) MergePR(ctx context.Context, number int, opts hosting.PRMergeOptions) error {
	mergeMethod := "merge"
	switch opts.Method {
	case "squash":
		mergeMethod = "squash"
	case "rebase":
		mergeMethod = "rebase"
	}

	mergeOpts := &gogithub.PullRequestOptions{
		MergeMethod: mergeMethod,
		CommitTitle: opts.CommitTitle,
		SHA:         opts.SHA,
	}

	// For squash merges, use SquashCommitMessage if provided, otherwise CommitMessage.
	commitBody := opts.CommitMessage
	if mergeMethod == "squash" && opts.SquashCommitMessage != "" {
		commitBody = opts.SquashCommitMessage
	}

	_, _, err := g.client.PullRequests.Merge(ctx, g.owner, g.repo, number, commitBody, mergeOpts)
	if err != nil {
		return fmt.Errorf("merge PR %d: %w", number, err)
	}

	if opts.DeleteBranch {
		// Get the PR to find the head branch.
		pr, _, getErr := g.client.PullRequests.Get(ctx, g.owner, g.repo, number)
		if getErr != nil {
			slog.Warn("merged PR but failed to get head branch for deletion",
				"pr", number, "error", getErr)
			return nil
		}
		if delErr := g.DeleteBranch(ctx, pr.GetHead().GetRef()); delErr != nil {
			slog.Warn("merged PR but failed to delete branch",
				"pr", number, "branch", pr.GetHead().GetRef(), "error", delErr)
		}
	}

	return nil
}

// FindPRByBranch finds a PR for a given branch.
func (g *GitHubProvider) FindPRByBranch(ctx context.Context, branch string) (*hosting.PR, error) {
	prs, _, err := g.client.PullRequests.List(ctx, g.owner, g.repo, &gogithub.PullRequestListOptions{
		Head:        g.owner + ":" + branch,
		State:       "open",
		ListOptions: gogithub.ListOptions{PerPage: 1},
	})
	if err != nil {
		return nil, fmt.Errorf("find PR by branch %q: %w", branch, err)
	}

	if len(prs) == 0 {
		return nil, hosting.ErrNoPRFound
	}

	return mapPR(prs[0]), nil
}

// ListPRComments lists review comments on a PR.
func (g *GitHubProvider) ListPRComments(ctx context.Context, number int) ([]hosting.PRComment, error) {
	var allComments []*gogithub.PullRequestComment
	opts := &gogithub.PullRequestListCommentsOptions{
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := g.client.PullRequests.ListComments(ctx, g.owner, g.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list PR %d comments: %w", number, err)
		}
		allComments = append(allComments, comments...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]hosting.PRComment, 0, len(allComments))
	for _, c := range allComments {
		result = append(result, mapPRComment(c))
	}
	return result, nil
}

// CreatePRComment creates a comment on a PR.
func (g *GitHubProvider) CreatePRComment(ctx context.Context, number int, comment hosting.PRCommentCreate) (*hosting.PRComment, error) {
	if comment.Path != "" {
		return g.createInlineComment(ctx, number, comment)
	}
	return g.createGeneralComment(ctx, number, comment.Body)
}

func (g *GitHubProvider) createInlineComment(ctx context.Context, number int, comment hosting.PRCommentCreate) (*hosting.PRComment, error) {
	// Get head SHA for the commit_id field.
	pr, _, err := g.client.PullRequests.Get(ctx, g.owner, g.repo, number)
	if err != nil {
		return nil, fmt.Errorf("get PR %d head commit: %w", number, err)
	}

	side := "RIGHT"
	if comment.Side != "" {
		side = comment.Side
	}

	reviewComment := &gogithub.PullRequestComment{
		Body:     gogithub.Ptr(comment.Body),
		Path:     gogithub.Ptr(comment.Path),
		Line:     gogithub.Ptr(comment.Line),
		Side:     gogithub.Ptr(side),
		CommitID: gogithub.Ptr(pr.GetHead().GetSHA()),
	}

	created, _, err := g.client.PullRequests.CreateComment(ctx, g.owner, g.repo, number, reviewComment)
	if err != nil {
		return nil, fmt.Errorf("create inline comment on PR %d: %w", number, err)
	}

	mapped := mapPRComment(created)
	return &mapped, nil
}

func (g *GitHubProvider) createGeneralComment(ctx context.Context, number int, body string) (*hosting.PRComment, error) {
	issueComment := &gogithub.IssueComment{
		Body: gogithub.Ptr(body),
	}

	created, _, err := g.client.Issues.CreateComment(ctx, g.owner, g.repo, number, issueComment)
	if err != nil {
		return nil, fmt.Errorf("create comment on PR %d: %w", number, err)
	}

	return &hosting.PRComment{
		ID:        created.GetID(),
		Body:      created.GetBody(),
		Author:    created.GetUser().GetLogin(),
		CreatedAt: created.GetCreatedAt().Format(time.RFC3339),
	}, nil
}

// ReplyToComment replies to a comment thread.
func (g *GitHubProvider) ReplyToComment(ctx context.Context, number int, threadID int64, body string) (*hosting.PRComment, error) {
	created, _, err := g.client.PullRequests.CreateCommentInReplyTo(ctx, g.owner, g.repo, number, body, threadID)
	if err != nil {
		return nil, fmt.Errorf("reply to comment %d on PR %d: %w", threadID, number, err)
	}

	mapped := mapPRComment(created)
	return &mapped, nil
}

// GetPRComment fetches a single PR review comment by ID.
func (g *GitHubProvider) GetPRComment(ctx context.Context, _ int, commentID int64) (*hosting.PRComment, error) {
	comment, _, err := g.client.PullRequests.GetComment(ctx, g.owner, g.repo, commentID)
	if err != nil {
		return nil, fmt.Errorf("get comment %d: %w", commentID, err)
	}

	mapped := mapPRComment(comment)
	return &mapped, nil
}

// GetCheckRuns gets CI check runs for a ref.
func (g *GitHubProvider) GetCheckRuns(ctx context.Context, ref string) ([]hosting.CheckRun, error) {
	result, _, err := g.client.Checks.ListCheckRunsForRef(ctx, g.owner, g.repo, ref, nil)
	if err != nil {
		return nil, fmt.Errorf("get check runs for %q: %w", ref, err)
	}

	checks := make([]hosting.CheckRun, 0, len(result.CheckRuns))
	for _, cr := range result.CheckRuns {
		checks = append(checks, hosting.CheckRun{
			ID:         cr.GetID(),
			Name:       cr.GetName(),
			Status:     cr.GetStatus(),
			Conclusion: cr.GetConclusion(),
		})
	}
	return checks, nil
}

// GetPRReviews gets reviews for a PR.
func (g *GitHubProvider) GetPRReviews(ctx context.Context, number int) ([]hosting.PRReview, error) {
	var allReviews []*gogithub.PullRequestReview
	opts := &gogithub.ListOptions{PerPage: 100}

	for {
		reviews, resp, err := g.client.PullRequests.ListReviews(ctx, g.owner, g.repo, number, opts)
		if err != nil {
			return nil, fmt.Errorf("list reviews for PR %d: %w", number, err)
		}
		allReviews = append(allReviews, reviews...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	result := make([]hosting.PRReview, 0, len(allReviews))
	for _, r := range allReviews {
		result = append(result, hosting.PRReview{
			ID:        r.GetID(),
			Author:    r.GetUser().GetLogin(),
			State:     r.GetState(),
			Body:      r.GetBody(),
			CreatedAt: r.GetSubmittedAt().Format(time.RFC3339),
		})
	}
	return result, nil
}

// ApprovePR approves a pull request.
func (g *GitHubProvider) ApprovePR(ctx context.Context, number int, body string) error {
	review := &gogithub.PullRequestReviewRequest{
		Event: gogithub.Ptr("APPROVE"),
		Body:  gogithub.Ptr(body),
	}

	_, _, err := g.client.PullRequests.CreateReview(ctx, g.owner, g.repo, number, review)
	if err != nil {
		return fmt.Errorf("approve PR %d: %w", number, err)
	}
	return nil
}

// GetPRStatusSummary fetches and summarizes PR status including reviews and checks.
func (g *GitHubProvider) GetPRStatusSummary(ctx context.Context, pr *hosting.PR) (*hosting.PRStatusSummary, error) {
	summary := &hosting.PRStatusSummary{
		ReviewStatus: "pending_review",
		Mergeable:    pr.Mergeable,
	}

	// Get reviews.
	reviews, err := g.GetPRReviews(ctx, pr.Number)
	if err != nil {
		return nil, fmt.Errorf("get reviews: %w", err)
	}

	// Track the most recent review state per author (to handle re-reviews).
	latestReviewByAuthor := make(map[string]string)
	for _, r := range reviews {
		// Skip COMMENTED and PENDING states - they don't affect approval status.
		if r.State == "COMMENTED" || r.State == "PENDING" {
			continue
		}
		latestReviewByAuthor[r.Author] = r.State
	}

	summary.ReviewCount = len(latestReviewByAuthor)

	// Count approvals and changes requested.
	var approvals, changesRequested int
	for _, state := range latestReviewByAuthor {
		switch state {
		case "APPROVED":
			approvals++
		case "CHANGES_REQUESTED":
			changesRequested++
		}
	}
	summary.ApprovalCount = approvals

	// Determine review status (changes_requested takes precedence).
	if changesRequested > 0 {
		summary.ReviewStatus = "changes_requested"
	} else if approvals > 0 {
		summary.ReviewStatus = "approved"
	}

	// Get check runs for checks status.
	checks, err := g.GetCheckRuns(ctx, pr.HeadBranch)
	if err != nil {
		// Don't fail on check run errors - just mark as unknown.
		summary.ChecksStatus = "unknown"
		return summary, nil
	}

	// Analyze checks.
	var passed, failed, pending int
	for _, check := range checks {
		switch check.Status {
		case "completed":
			switch check.Conclusion {
			case "success", "neutral", "skipped":
				passed++
			case "failure", "timed_out", "cancelled", "action_required":
				failed++
			}
		default:
			pending++
		}
	}
	_ = passed // used for clarity in the switch logic

	// Determine overall checks status.
	if len(checks) == 0 {
		summary.ChecksStatus = "none"
	} else if failed > 0 {
		summary.ChecksStatus = "failure"
	} else if pending > 0 {
		summary.ChecksStatus = "pending"
	} else {
		summary.ChecksStatus = "success"
	}

	return summary, nil
}

// EnableAutoMerge returns ErrAutoMergeNotSupported because GitHub's REST API
// does not support enabling auto-merge (requires GraphQL).
func (g *GitHubProvider) EnableAutoMerge(_ context.Context, _ int, _ string) error {
	return hosting.ErrAutoMergeNotSupported
}

// UpdatePRBranch updates the PR branch with the latest base branch changes.
func (g *GitHubProvider) UpdatePRBranch(ctx context.Context, number int) error {
	_, _, err := g.client.PullRequests.UpdateBranch(ctx, g.owner, g.repo, number, nil)
	if err != nil {
		return fmt.Errorf("update branch for PR %d: %w", number, err)
	}
	return nil
}

// DeleteBranch deletes a branch from the remote.
func (g *GitHubProvider) DeleteBranch(ctx context.Context, branch string) error {
	_, err := g.client.Git.DeleteRef(ctx, g.owner, g.repo, "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("delete branch %q: %w", branch, err)
	}
	return nil
}

// mapPR converts a go-github PullRequest to a hosting.PR.
func mapPR(pr *gogithub.PullRequest) *hosting.PR {
	state := pr.GetState()
	if pr.GetMerged() {
		state = "merged"
	}

	var createdAt, updatedAt string
	if t := pr.GetCreatedAt(); !t.IsZero() {
		createdAt = t.Format(time.RFC3339)
	}
	if t := pr.GetUpdatedAt(); !t.IsZero() {
		updatedAt = t.Format(time.RFC3339)
	}

	// Extract labels.
	var labels []string
	for _, l := range pr.Labels {
		if name := l.GetName(); name != "" {
			labels = append(labels, name)
		}
	}

	// Extract assignees.
	var assignees []string
	for _, a := range pr.Assignees {
		if login := a.GetLogin(); login != "" {
			assignees = append(assignees, login)
		}
	}

	return &hosting.PR{
		Number:     pr.GetNumber(),
		Title:      pr.GetTitle(),
		Body:       pr.GetBody(),
		State:      state,
		HeadBranch: pr.GetHead().GetRef(),
		BaseBranch: pr.GetBase().GetRef(),
		HTMLURL:    pr.GetHTMLURL(),
		Draft:      pr.GetDraft(),
		Mergeable:  pr.GetMergeable(),
		HeadSHA:    pr.GetHead().GetSHA(),
		Labels:     labels,
		Assignees:  assignees,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

// mapPRComment converts a go-github PullRequestComment to a hosting.PRComment.
func mapPRComment(c *gogithub.PullRequestComment) hosting.PRComment {
	line := c.GetLine()
	if line == 0 {
		line = c.GetOriginalLine()
	}

	return hosting.PRComment{
		ID:        c.GetID(),
		Body:      c.GetBody(),
		Path:      c.GetPath(),
		Line:      line,
		Side:      c.GetSide(),
		ThreadID:  int64(c.GetInReplyTo()),
		Author:    c.GetUser().GetLogin(),
		CreatedAt: c.GetCreatedAt().Format(time.RFC3339),
	}
}
