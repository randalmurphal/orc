package gitlab

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/randalmurphal/orc/internal/hosting"
)

// Compile-time interface check.
var _ hosting.Provider = (*GitLabProvider)(nil)

func init() {
	hosting.RegisterProvider(hosting.ProviderGitLab, newProvider)
}

// GitLabProvider implements hosting.Provider using the go-gitlab library.
type GitLabProvider struct {
	client    *gogitlab.Client
	projectID string // URL-encoded "owner/repo" path used as project identifier
	owner     string
	repo      string
}

// newProvider creates a new GitLabProvider from the working directory and config.
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

	// Project ID is the full path: "owner/repo" or "group/subgroup/repo".
	projectID := owner + "/" + repo

	var client *gogitlab.Client
	if cfg.BaseURL != "" {
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
		client, err = gogitlab.NewClient(token, gogitlab.WithBaseURL(baseURL+"/api/v4"))
	} else {
		client, err = gogitlab.NewClient(token)
	}
	if err != nil {
		return nil, fmt.Errorf("create GitLab client: %w", err)
	}

	return &GitLabProvider{
		client:    client,
		projectID: projectID,
		owner:     owner,
		repo:      repo,
	}, nil
}

// Name returns the provider type.
func (g *GitLabProvider) Name() hosting.ProviderType {
	return hosting.ProviderGitLab
}

// OwnerRepo returns the owner and repository name.
// For nested GitLab groups, owner may be "group/subgroup".
func (g *GitLabProvider) OwnerRepo() (string, string) {
	return g.owner, g.repo
}

// CheckAuth validates the token by fetching the authenticated user.
func (g *GitLabProvider) CheckAuth(ctx context.Context) error {
	_, _, err := g.client.Users.CurrentUser(gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("check auth: %w", err)
	}
	return nil
}

// CreatePR creates a merge request.
func (g *GitLabProvider) CreatePR(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error) {
	title := opts.Title
	if opts.Draft {
		title = "Draft: " + title
	}

	createOpts := &gogitlab.CreateMergeRequestOptions{
		Title:              gogitlab.Ptr(title),
		Description:        gogitlab.Ptr(opts.Body),
		SourceBranch:       gogitlab.Ptr(opts.Head),
		TargetBranch:       gogitlab.Ptr(opts.Base),
		AllowCollaboration: gogitlab.Ptr(opts.MaintainerCanModify),
		RemoveSourceBranch: gogitlab.Ptr(true),
	}

	if len(opts.Labels) > 0 {
		labels := gogitlab.LabelOptions(opts.Labels)
		createOpts.Labels = &labels
	}

	if len(opts.Reviewers) > 0 {
		reviewerIDs, lookupErr := g.resolveUserIDs(ctx, opts.Reviewers)
		if lookupErr != nil {
			slog.Warn("failed to resolve reviewer usernames to IDs",
				"reviewers", opts.Reviewers,
				"error", lookupErr)
		} else if len(reviewerIDs) > 0 {
			createOpts.ReviewerIDs = &reviewerIDs
		}
	}

	if len(opts.Assignees) > 0 {
		assigneeIDs, lookupErr := g.resolveUserIDs(ctx, opts.Assignees)
		if lookupErr != nil {
			slog.Warn("failed to resolve assignee usernames to IDs",
				"assignees", opts.Assignees,
				"error", lookupErr)
		} else if len(assigneeIDs) > 0 {
			createOpts.AssigneeIDs = &assigneeIDs
		}
	}

	mr, _, err := g.client.MergeRequests.CreateMergeRequest(g.projectID, createOpts, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("create MR: %w", err)
	}

	return g.GetPR(ctx, int(mr.IID))
}

// GetPR gets a merge request by IID.
func (g *GitLabProvider) GetPR(ctx context.Context, number int) (*hosting.PR, error) {
	mr, _, err := g.client.MergeRequests.GetMergeRequest(g.projectID, int64(number), nil, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get MR %d: %w", number, err)
	}
	return mapMR(mr), nil
}

// UpdatePR updates a merge request's title, description, or state.
func (g *GitLabProvider) UpdatePR(ctx context.Context, number int, opts hosting.PRUpdateOptions) error {
	updateOpts := &gogitlab.UpdateMergeRequestOptions{}

	if opts.Title != "" {
		updateOpts.Title = gogitlab.Ptr(opts.Title)
	}
	if opts.Body != "" {
		updateOpts.Description = gogitlab.Ptr(opts.Body)
	}
	switch opts.State {
	case "closed":
		updateOpts.StateEvent = gogitlab.Ptr("close")
	case "open":
		updateOpts.StateEvent = gogitlab.Ptr("reopen")
	}

	_, _, err := g.client.MergeRequests.UpdateMergeRequest(g.projectID, int64(number), updateOpts, gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("update MR %d: %w", number, err)
	}
	return nil
}

// MergePR accepts (merges) a merge request.
func (g *GitLabProvider) MergePR(ctx context.Context, number int, opts hosting.PRMergeOptions) error {
	acceptOpts := &gogitlab.AcceptMergeRequestOptions{}

	// Build merge commit message: title + body.
	if opts.CommitTitle != "" {
		msg := opts.CommitTitle
		if opts.CommitMessage != "" {
			msg = opts.CommitTitle + "\n\n" + opts.CommitMessage
		}
		acceptOpts.MergeCommitMessage = gogitlab.Ptr(msg)
	}
	if opts.Method == "squash" {
		acceptOpts.Squash = gogitlab.Ptr(true)
		if opts.SquashCommitMessage != "" {
			acceptOpts.SquashCommitMessage = gogitlab.Ptr(opts.SquashCommitMessage)
		} else if opts.CommitTitle != "" {
			acceptOpts.SquashCommitMessage = gogitlab.Ptr(opts.CommitTitle)
		}
	}
	if opts.SHA != "" {
		acceptOpts.SHA = gogitlab.Ptr(opts.SHA)
	}
	if opts.DeleteBranch {
		acceptOpts.ShouldRemoveSourceBranch = gogitlab.Ptr(true)
	}

	_, _, err := g.client.MergeRequests.AcceptMergeRequest(g.projectID, int64(number), acceptOpts, gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("merge MR %d: %w", number, err)
	}
	return nil
}

// FindPRByBranch finds an open merge request for a given source branch.
func (g *GitLabProvider) FindPRByBranch(ctx context.Context, branch string) (*hosting.PR, error) {
	mrs, _, err := g.client.MergeRequests.ListProjectMergeRequests(g.projectID, &gogitlab.ListProjectMergeRequestsOptions{
		SourceBranch: gogitlab.Ptr(branch),
		State:        gogitlab.Ptr("opened"),
		ListOptions:  gogitlab.ListOptions{PerPage: 1},
	}, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("find MR by branch %q: %w", branch, err)
	}

	if len(mrs) == 0 {
		return nil, hosting.ErrNoPRFound
	}

	return mapBasicMR(mrs[0]), nil
}

// ListPRComments lists all discussion notes on a merge request.
func (g *GitLabProvider) ListPRComments(ctx context.Context, number int) ([]hosting.PRComment, error) {
	var allComments []hosting.PRComment
	opts := &gogitlab.ListMergeRequestDiscussionsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 100},
	}

	for {
		discussions, resp, err := g.client.Discussions.ListMergeRequestDiscussions(g.projectID, int64(number), opts, gogitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("list MR %d discussions: %w", number, err)
		}

		for _, d := range discussions {
			for _, note := range d.Notes {
				if note.System {
					continue
				}
				allComments = append(allComments, mapNote(note, d.ID))
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allComments, nil
}

// CreatePRComment creates a comment on a merge request.
// If Path is set, creates a discussion with a file position (inline comment).
// Otherwise, creates a simple note.
func (g *GitLabProvider) CreatePRComment(ctx context.Context, number int, comment hosting.PRCommentCreate) (*hosting.PRComment, error) {
	if comment.Path != "" {
		return g.createInlineComment(ctx, number, comment)
	}
	return g.createGeneralComment(ctx, number, comment.Body)
}

func (g *GitLabProvider) createInlineComment(ctx context.Context, number int, comment hosting.PRCommentCreate) (*hosting.PRComment, error) {
	// Get the MR to find the diff refs for positioning.
	mr, _, err := g.client.MergeRequests.GetMergeRequest(g.projectID, int64(number), nil, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get MR %d for inline comment: %w", number, err)
	}

	position := &gogitlab.PositionOptions{
		PositionType: gogitlab.Ptr("text"),
		NewPath:      gogitlab.Ptr(comment.Path),
		NewLine:      gogitlab.Ptr(int64(comment.Line)),
		BaseSHA:      gogitlab.Ptr(mr.DiffRefs.BaseSha),
		HeadSHA:      gogitlab.Ptr(mr.DiffRefs.HeadSha),
		StartSHA:     gogitlab.Ptr(mr.DiffRefs.StartSha),
	}

	discussionOpts := &gogitlab.CreateMergeRequestDiscussionOptions{
		Body:     gogitlab.Ptr(comment.Body),
		Position: position,
	}

	discussion, _, err := g.client.Discussions.CreateMergeRequestDiscussion(g.projectID, int64(number), discussionOpts, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("create inline comment on MR %d: %w", number, err)
	}

	if len(discussion.Notes) == 0 {
		return nil, fmt.Errorf("create inline comment on MR %d: discussion created but no notes returned", number)
	}

	mapped := mapNote(discussion.Notes[0], discussion.ID)
	return &mapped, nil
}

func (g *GitLabProvider) createGeneralComment(ctx context.Context, number int, body string) (*hosting.PRComment, error) {
	note, _, err := g.client.Notes.CreateMergeRequestNote(g.projectID, int64(number), &gogitlab.CreateMergeRequestNoteOptions{
		Body: gogitlab.Ptr(body),
	}, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("create comment on MR %d: %w", number, err)
	}

	return &hosting.PRComment{
		ID:        note.ID,
		Body:      note.Body,
		Author:    note.Author.Username,
		CreatedAt: note.CreatedAt.Format(time.RFC3339),
	}, nil
}

// ReplyToComment replies to a discussion thread on a merge request.
// The threadID is the first note ID of the discussion. We search for the
// discussion containing that note, then reply to it.
func (g *GitLabProvider) ReplyToComment(ctx context.Context, number int, threadID int64, body string) (*hosting.PRComment, error) {
	discussionID, err := g.findDiscussionID(ctx, number, threadID)
	if err != nil {
		return nil, fmt.Errorf("find discussion for note %d on MR %d: %w", threadID, number, err)
	}

	note, _, err := g.client.Discussions.AddMergeRequestDiscussionNote(g.projectID, int64(number), discussionID, &gogitlab.AddMergeRequestDiscussionNoteOptions{
		Body: gogitlab.Ptr(body),
	}, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("reply to comment %d on MR %d: %w", threadID, number, err)
	}

	mapped := mapNote(note, discussionID)
	return &mapped, nil
}

// findDiscussionID searches discussions for one containing the given note ID.
func (g *GitLabProvider) findDiscussionID(ctx context.Context, mrNumber int, noteID int64) (string, error) {
	opts := &gogitlab.ListMergeRequestDiscussionsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 100},
	}

	for {
		discussions, resp, err := g.client.Discussions.ListMergeRequestDiscussions(g.projectID, int64(mrNumber), opts, gogitlab.WithContext(ctx))
		if err != nil {
			return "", err
		}

		for _, d := range discussions {
			for _, note := range d.Notes {
				if note.ID == noteID {
					return d.ID, nil
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return "", fmt.Errorf("no discussion found containing note %d", noteID)
}

// GetPRComment fetches a single MR note by ID.
func (g *GitLabProvider) GetPRComment(ctx context.Context, prNumber int, commentID int64) (*hosting.PRComment, error) {
	note, _, err := g.client.Notes.GetMergeRequestNote(g.projectID, int64(prNumber), int64(commentID), gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get comment %d on MR %d: %w", commentID, prNumber, err)
	}

	mapped := mapNote(note, "")
	return &mapped, nil
}

// GetCheckRuns gets CI pipeline jobs for a ref, mapped to unified CheckRun format.
func (g *GitLabProvider) GetCheckRuns(ctx context.Context, ref string) ([]hosting.CheckRun, error) {
	// Get the latest pipeline for the ref.
	pipelines, _, err := g.client.Pipelines.ListProjectPipelines(g.projectID, &gogitlab.ListProjectPipelinesOptions{
		Ref:         gogitlab.Ptr(ref),
		ListOptions: gogitlab.ListOptions{PerPage: 1},
	}, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list pipelines for ref %q: %w", ref, err)
	}

	if len(pipelines) == 0 {
		return nil, nil
	}

	// Get jobs for the latest pipeline.
	jobs, _, err := g.client.Jobs.ListPipelineJobs(g.projectID, pipelines[0].ID, nil, gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("list pipeline jobs for ref %q: %w", ref, err)
	}

	checks := make([]hosting.CheckRun, 0, len(jobs))
	for _, job := range jobs {
		status, conclusion := mapJobStatus(job.Status)
		checks = append(checks, hosting.CheckRun{
			ID:         job.ID,
			Name:       job.Name,
			Status:     status,
			Conclusion: conclusion,
		})
	}
	return checks, nil
}

// GetPRReviews gets approval state for a merge request.
func (g *GitLabProvider) GetPRReviews(ctx context.Context, number int) ([]hosting.PRReview, error) {
	approvalState, _, err := g.client.MergeRequestApprovals.GetApprovalState(g.projectID, int64(number), gogitlab.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("get approval state for MR %d: %w", number, err)
	}

	var reviews []hosting.PRReview
	for _, rule := range approvalState.Rules {
		for _, approver := range rule.ApprovedBy {
			reviews = append(reviews, hosting.PRReview{
				ID:     approver.ID,
				Author: approver.Username,
				State:  "APPROVED",
			})
		}
	}

	return reviews, nil
}

// ApprovePR approves a merge request.
func (g *GitLabProvider) ApprovePR(ctx context.Context, number int, _ string) error {
	_, _, err := g.client.MergeRequestApprovals.ApproveMergeRequest(g.projectID, int64(number), nil, gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("approve MR %d: %w", number, err)
	}
	return nil
}

// GetPRStatusSummary fetches and summarizes MR status including reviews and pipeline checks.
func (g *GitLabProvider) GetPRStatusSummary(ctx context.Context, pr *hosting.PR) (*hosting.PRStatusSummary, error) {
	summary := &hosting.PRStatusSummary{
		ReviewStatus: "pending_review",
		Mergeable:    pr.Mergeable,
	}

	// Get reviews.
	reviews, err := g.GetPRReviews(ctx, pr.Number)
	if err != nil {
		return nil, fmt.Errorf("get reviews: %w", err)
	}

	// Track unique approvers.
	approvers := make(map[string]bool)
	for _, r := range reviews {
		if r.State == "APPROVED" {
			approvers[r.Author] = true
		}
	}

	summary.ReviewCount = len(approvers)
	summary.ApprovalCount = len(approvers)

	if len(approvers) > 0 {
		summary.ReviewStatus = "approved"
	}

	// Get check runs for pipeline status.
	checks, err := g.GetCheckRuns(ctx, pr.HeadBranch)
	if err != nil {
		summary.ChecksStatus = "unknown"
		return summary, nil
	}

	// Analyze checks.
	var failed, pending int
	for _, check := range checks {
		switch check.Status {
		case "completed":
			switch check.Conclusion {
			case "failure", "cancelled":
				failed++
			}
		default:
			pending++
		}
	}

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

// EnableAutoMerge enables merge_when_pipeline_succeeds for a merge request.
func (g *GitLabProvider) EnableAutoMerge(ctx context.Context, number int, method string) error {
	opts := &gogitlab.AcceptMergeRequestOptions{
		MergeWhenPipelineSucceeds: gogitlab.Ptr(true),
	}
	if method == "squash" {
		opts.Squash = gogitlab.Ptr(true)
	}
	_, _, err := g.client.MergeRequests.AcceptMergeRequest(g.projectID, int64(number), opts, gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("enable auto-merge for MR %d: %w", number, err)
	}
	return nil
}

// UpdatePRBranch rebases the merge request branch with the latest target branch changes.
func (g *GitLabProvider) UpdatePRBranch(ctx context.Context, number int) error {
	_, err := g.client.MergeRequests.RebaseMergeRequest(g.projectID, int64(number), nil, gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("rebase MR %d: %w", number, err)
	}
	return nil
}

// DeleteBranch deletes a branch from the remote.
func (g *GitLabProvider) DeleteBranch(ctx context.Context, branch string) error {
	_, err := g.client.Branches.DeleteBranch(g.projectID, branch, gogitlab.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("delete branch %q: %w", branch, err)
	}
	return nil
}

// resolveUserIDs converts a list of usernames to GitLab user IDs.
func (g *GitLabProvider) resolveUserIDs(ctx context.Context, usernames []string) ([]int64, error) {
	var ids []int64
	for _, username := range usernames {
		users, _, err := g.client.Users.ListUsers(&gogitlab.ListUsersOptions{
			Username: gogitlab.Ptr(username),
		}, gogitlab.WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("lookup user %q: %w", username, err)
		}
		if len(users) > 0 {
			ids = append(ids, users[0].ID)
		}
	}
	return ids, nil
}

// mapMR converts a go-gitlab MergeRequest to a hosting.PR.
func mapMR(mr *gogitlab.MergeRequest) *hosting.PR {
	state := mr.State
	switch state {
	case "opened":
		state = "open"
	}

	draft := mr.Draft
	mergeable := mr.DetailedMergeStatus == "mergeable"

	var createdAt, updatedAt string
	if mr.CreatedAt != nil {
		createdAt = mr.CreatedAt.Format(time.RFC3339)
	}
	if mr.UpdatedAt != nil {
		updatedAt = mr.UpdatedAt.Format(time.RFC3339)
	}

	// Extract labels.
	var labels []string
	for _, l := range mr.Labels {
		labels = append(labels, l)
	}

	// Extract assignees.
	var assignees []string
	for _, a := range mr.Assignees {
		if a.Username != "" {
			assignees = append(assignees, a.Username)
		}
	}

	return &hosting.PR{
		Number:     int(mr.IID),
		Title:      mr.Title,
		Body:       mr.Description,
		State:      state,
		HeadBranch: mr.SourceBranch,
		BaseBranch: mr.TargetBranch,
		HTMLURL:    mr.WebURL,
		Draft:      draft,
		Mergeable:  mergeable,
		HeadSHA:    mr.SHA,
		Labels:     labels,
		Assignees:  assignees,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

// mapBasicMR converts a go-gitlab BasicMergeRequest to a hosting.PR.
func mapBasicMR(mr *gogitlab.BasicMergeRequest) *hosting.PR {
	state := mr.State
	switch state {
	case "opened":
		state = "open"
	}

	mergeable := mr.DetailedMergeStatus == "mergeable"

	var createdAt, updatedAt string
	if mr.CreatedAt != nil {
		createdAt = mr.CreatedAt.Format(time.RFC3339)
	}
	if mr.UpdatedAt != nil {
		updatedAt = mr.UpdatedAt.Format(time.RFC3339)
	}

	// Extract labels.
	var labels []string
	for _, l := range mr.Labels {
		labels = append(labels, l)
	}

	// Extract assignees.
	var assignees []string
	for _, a := range mr.Assignees {
		if a.Username != "" {
			assignees = append(assignees, a.Username)
		}
	}

	return &hosting.PR{
		Number:     int(mr.IID),
		Title:      mr.Title,
		Body:       mr.Description,
		State:      state,
		HeadBranch: mr.SourceBranch,
		BaseBranch: mr.TargetBranch,
		HTMLURL:    mr.WebURL,
		Draft:      mr.Draft,
		Mergeable:  mergeable,
		HeadSHA:    mr.SHA,
		Labels:     labels,
		Assignees:  assignees,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

// mapNote converts a go-gitlab Note to a hosting.PRComment.
func mapNote(note *gogitlab.Note, _ string) hosting.PRComment {
	comment := hosting.PRComment{
		ID:        note.ID,
		Body:      note.Body,
		Author:    note.Author.Username,
		ThreadID:  note.ID,
		CreatedAt: note.CreatedAt.Format(time.RFC3339),
	}

	if note.Position != nil {
		comment.Path = note.Position.NewPath
		comment.Line = int(note.Position.NewLine)
	}

	return comment
}

// mapJobStatus converts a GitLab job status to unified (status, conclusion) pair.
func mapJobStatus(gitlabStatus string) (status, conclusion string) {
	switch gitlabStatus {
	case "success":
		return "completed", "success"
	case "failed":
		return "completed", "failure"
	case "canceled":
		return "completed", "cancelled"
	case "skipped":
		return "completed", "skipped"
	case "running":
		return "in_progress", "running"
	case "pending", "created":
		return "queued", ""
	case "manual":
		return "queued", ""
	default:
		return "queued", ""
	}
}
