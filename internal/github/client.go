package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
)

// ErrNoPRFound is returned when no PR exists for the given branch.
var ErrNoPRFound = errors.New("no pull request found for branch")

// Client implements Provider using the gh CLI.
type Client struct {
	repoPath string
	owner    string
	repo     string
}

// Ensure Client implements Provider.
var _ Provider = (*Client)(nil)

// NewClient creates a new GitHub client.
// It extracts owner/repo from the git remote URL.
func NewClient(repoPath string) (*Client, error) {
	client := &Client{repoPath: repoPath}

	// Get owner/repo from git remote
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get remote URL: %w", err)
	}

	// Parse owner/repo from URL
	// Handles multiple formats:
	// - git@github.com:owner/repo.git
	// - git@github.company.com:org/repo.git (GitHub Enterprise)
	// - ssh://git@github.com:22/org/repo.git (SSH with port)
	// - https://github.com/owner/repo.git
	// - github.com/org/subgroup/repo (nested paths - takes last two segments)
	rawURL := strings.TrimSpace(string(output))
	rawURL = strings.TrimSuffix(rawURL, ".git")

	client.owner, client.repo = parseOwnerRepo(rawURL)

	if client.owner == "" || client.repo == "" {
		return nil, fmt.Errorf("could not parse owner/repo from remote URL: %s", rawURL)
	}

	return client, nil
}

// parseOwnerRepo extracts owner and repo from various git remote URL formats.
// Supports:
// - git@github.com:owner/repo
// - git@github.company.com:org/repo (GitHub Enterprise)
// - ssh://git@github.com:22/org/repo (SSH with port)
// - https://github.com/owner/repo
// - Nested paths like github.com/org/subgroup/repo (takes last two segments)
func parseOwnerRepo(rawURL string) (owner, repo string) {
	// SSH URL pattern: git@host:path or ssh://git@host[:port]/path
	// Match git@<host>:<path> format (most common SSH format)
	sshPattern := regexp.MustCompile(`^git@[^:]+:(.+)$`)
	if matches := sshPattern.FindStringSubmatch(rawURL); len(matches) == 2 {
		return extractLastTwoSegments(matches[1])
	}

	// Match ssh://git@host[:port]/path format
	sshURLPattern := regexp.MustCompile(`^ssh://[^/]+/(.+)$`)
	if matches := sshURLPattern.FindStringSubmatch(rawURL); len(matches) == 2 {
		return extractLastTwoSegments(matches[1])
	}

	// HTTPS/HTTP URL pattern: https://host/path
	httpsPattern := regexp.MustCompile(`^https?://[^/]+/(.+)$`)
	if matches := httpsPattern.FindStringSubmatch(rawURL); len(matches) == 2 {
		return extractLastTwoSegments(matches[1])
	}

	// Fallback: try to find path segments after any hostname-like pattern
	// This handles edge cases like "github.com/owner/repo" without protocol
	hostPathPattern := regexp.MustCompile(`[^/]+\.[^/]+/(.+)$`)
	if matches := hostPathPattern.FindStringSubmatch(rawURL); len(matches) == 2 {
		return extractLastTwoSegments(matches[1])
	}

	return "", ""
}

// extractLastTwoSegments takes a path like "org/subgroup/repo" or "owner/repo"
// and returns the last two segments as owner and repo.
func extractLastTwoSegments(path string) (owner, repo string) {
	// Clean up any leading/trailing slashes
	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")

	if len(segments) < 2 {
		return "", ""
	}

	// Take the last two segments (handles nested paths)
	return segments[len(segments)-2], segments[len(segments)-1]
}

// Owner returns the repository owner.
func (c *Client) Owner() string {
	return c.owner
}

// Repo returns the repository name.
func (c *Client) Repo() string {
	return c.repo
}

// runGH executes a gh command and returns output.
func (c *Client) runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = c.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh %s: %s: %w", strings.Join(args, " "), stderr.String(), err)
	}

	return stdout.Bytes(), nil
}

// CreatePR creates a pull request.
func (c *Client) CreatePR(ctx context.Context, opts PRCreateOptions) (*PR, error) {
	args := []string{"pr", "create",
		"--title", opts.Title,
		"--body", opts.Body,
		"--head", opts.Head,
		"--base", opts.Base,
	}

	if opts.Draft {
		args = append(args, "--draft")
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	for _, reviewer := range opts.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}

	output, err := c.runGH(ctx, args...)
	if err != nil {
		return nil, err
	}

	// gh pr create outputs the PR URL
	prURL := strings.TrimSpace(string(output))

	// Get PR details
	return c.GetPRByURL(ctx, prURL)
}

// GetPR gets a PR by number.
func (c *Client) GetPR(ctx context.Context, number int) (*PR, error) {
	output, err := c.runGH(ctx, "pr", "view", fmt.Sprintf("%d", number), "--json",
		"number,title,body,state,headRefName,baseRefName,url,isDraft,mergeable,createdAt,updatedAt")
	if err != nil {
		return nil, err
	}

	var raw struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		Body        string `json:"body"`
		State       string `json:"state"`
		HeadRefName string `json:"headRefName"`
		BaseRefName string `json:"baseRefName"`
		URL         string `json:"url"`
		IsDraft     bool   `json:"isDraft"`
		Mergeable   string `json:"mergeable"`
		CreatedAt   string `json:"createdAt"`
		UpdatedAt   string `json:"updatedAt"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parse PR: %w", err)
	}

	return &PR{
		Number:     raw.Number,
		Title:      raw.Title,
		Body:       raw.Body,
		State:      raw.State,
		HeadBranch: raw.HeadRefName,
		BaseBranch: raw.BaseRefName,
		HTMLURL:    raw.URL,
		Draft:      raw.IsDraft,
		Mergeable:  raw.Mergeable == "MERGEABLE",
		CreatedAt:  raw.CreatedAt,
		UpdatedAt:  raw.UpdatedAt,
	}, nil
}

// GetPRByURL gets a PR by URL.
func (c *Client) GetPRByURL(ctx context.Context, url string) (*PR, error) {
	// Extract PR number from URL
	parts := strings.Split(url, "/pull/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid PR URL: %s", url)
	}

	var number int
	if _, err := fmt.Sscanf(parts[1], "%d", &number); err != nil {
		return nil, fmt.Errorf("parse PR number from URL %s: %w", url, err)
	}

	return c.GetPR(ctx, number)
}

// FindPRByBranch finds a PR for a given branch.
func (c *Client) FindPRByBranch(ctx context.Context, branch string) (*PR, error) {
	output, err := c.runGH(ctx, "pr", "list",
		"--head", branch,
		"--json", "number",
		"--limit", "1")
	if err != nil {
		return nil, err
	}

	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(output, &prs); err != nil {
		return nil, fmt.Errorf("parse PR list: %w", err)
	}

	if len(prs) == 0 {
		return nil, ErrNoPRFound
	}

	return c.GetPR(ctx, prs[0].Number)
}

// ListPRComments lists comments on a PR.
func (c *Client) ListPRComments(ctx context.Context, number int) ([]PRComment, error) {
	// Get review comments (inline on code)
	output, err := c.runGH(ctx, "api",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", c.owner, c.repo, number))
	if err != nil {
		return nil, err
	}

	var rawComments []struct {
		ID              int64  `json:"id"`
		Body            string `json:"body"`
		Path            string `json:"path"`
		OriginalLine    int    `json:"original_line"`
		Line            int    `json:"line"`
		Side            string `json:"side"`
		InReplyToID     int64  `json:"in_reply_to_id"`
		User            struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
	}

	if err := json.Unmarshal(output, &rawComments); err != nil {
		return nil, fmt.Errorf("parse PR comments: %w", err)
	}

	comments := make([]PRComment, 0, len(rawComments))
	for _, rc := range rawComments {
		line := rc.Line
		if line == 0 {
			line = rc.OriginalLine
		}
		comments = append(comments, PRComment{
			ID:        rc.ID,
			Body:      rc.Body,
			Path:      rc.Path,
			Line:      line,
			Side:      rc.Side,
			ThreadID:  rc.InReplyToID,
			Author:    rc.User.Login,
			CreatedAt: rc.CreatedAt,
		})
	}

	return comments, nil
}

// CreatePRComment creates a comment on a PR.
func (c *Client) CreatePRComment(ctx context.Context, number int, comment PRCommentCreate) (*PRComment, error) {
	if comment.Path != "" {
		// Inline comment on a file - need to use API
		// First, get the latest commit SHA for the PR
		prOutput, err := c.runGH(ctx, "pr", "view", fmt.Sprintf("%d", number),
			"--json", "headRefOid")
		if err != nil {
			return nil, fmt.Errorf("get PR head commit: %w", err)
		}

		var pr struct {
			HeadRefOid string `json:"headRefOid"`
		}
		if err := json.Unmarshal(prOutput, &pr); err != nil {
			return nil, fmt.Errorf("parse PR head commit: %w", err)
		}

		// Create inline comment via API using -f flags (gh api supports -f for string fields, -F for non-string)
		side := "RIGHT"
		if comment.Side != "" {
			side = comment.Side
		}
		output, err := c.runGH(ctx, "api",
			fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", c.owner, c.repo, number),
			"-X", "POST",
			"-f", fmt.Sprintf("body=%s", comment.Body),
			"-f", fmt.Sprintf("commit_id=%s", pr.HeadRefOid),
			"-f", fmt.Sprintf("path=%s", comment.Path),
			"-F", fmt.Sprintf("line=%d", comment.Line),
			"-f", fmt.Sprintf("side=%s", side))
		if err != nil {
			return nil, fmt.Errorf("create inline comment: %w", err)
		}

		var created struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(output, &created); err != nil {
			slog.Warn("comment created but could not parse response ID",
				"path", comment.Path,
				"line", comment.Line,
				"error", err)
		} else if created.ID == 0 {
			slog.Warn("comment created but response contained no ID",
				"path", comment.Path,
				"line", comment.Line)
		}

		return &PRComment{
			ID:   created.ID, // May be 0 if parsing failed
			Body: comment.Body,
			Path: comment.Path,
			Line: comment.Line,
		}, nil
	}

	// General comment on the PR - use API to get the comment ID back
	output, err := c.runGH(ctx, "api",
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", c.owner, c.repo, number),
		"-X", "POST",
		"-f", fmt.Sprintf("body=%s", comment.Body))
	if err != nil {
		return nil, fmt.Errorf("create PR comment: %w", err)
	}

	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(output, &created); err != nil {
		slog.Warn("PR comment created but could not parse response ID",
			"pr", number,
			"error", err)
	} else if created.ID == 0 {
		slog.Warn("PR comment created but response contained no ID",
			"pr", number)
	}

	return &PRComment{
		ID:   created.ID, // May be 0 if parsing failed
		Body: comment.Body,
	}, nil
}

// ReplyToComment replies to a comment thread.
func (c *Client) ReplyToComment(ctx context.Context, number int, threadID int64, body string) (*PRComment, error) {
	output, err := c.runGH(ctx, "api",
		fmt.Sprintf("/repos/%s/%s/pulls/%d/comments/%d/replies", c.owner, c.repo, number, threadID),
		"-X", "POST",
		"-f", fmt.Sprintf("body=%s", body))
	if err != nil {
		return nil, err
	}

	var created struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(output, &created); err == nil {
		return &PRComment{
			ID:       created.ID,
			Body:     body,
			ThreadID: threadID,
		}, nil
	}

	return &PRComment{
		Body:     body,
		ThreadID: threadID,
	}, nil
}

// MergePR merges a pull request.
func (c *Client) MergePR(ctx context.Context, number int, opts PRMergeOptions) error {
	args := []string{"pr", "merge", fmt.Sprintf("%d", number)}

	switch opts.Method {
	case "squash":
		args = append(args, "--squash")
	case "rebase":
		args = append(args, "--rebase")
	default:
		args = append(args, "--merge")
	}

	if opts.DeleteBranch {
		args = append(args, "--delete-branch")
	}

	if opts.CommitTitle != "" {
		args = append(args, "--subject", opts.CommitTitle)
	}

	// gh pr merge doesn't need --yes flag - it auto-confirms when not in TTY
	_, err := c.runGH(ctx, args...)
	return err
}

// UpdatePR updates a PR.
func (c *Client) UpdatePR(ctx context.Context, number int, opts PRUpdateOptions) error {
	args := []string{"pr", "edit", fmt.Sprintf("%d", number)}

	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	if opts.Body != "" {
		args = append(args, "--body", opts.Body)
	}

	_, err := c.runGH(ctx, args...)
	return err
}

// GetCheckRuns gets CI check runs for a ref.
func (c *Client) GetCheckRuns(ctx context.Context, ref string) ([]CheckRun, error) {
	output, err := c.runGH(ctx, "api",
		fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs", c.owner, c.repo, ref))
	if err != nil {
		return nil, err
	}

	var response struct {
		CheckRuns []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
		} `json:"check_runs"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("parse check runs: %w", err)
	}

	checks := make([]CheckRun, 0, len(response.CheckRuns))
	for _, rc := range response.CheckRuns {
		checks = append(checks, CheckRun{
			ID:         rc.ID,
			Name:       rc.Name,
			Status:     rc.Status,
			Conclusion: rc.Conclusion,
		})
	}

	return checks, nil
}

// CheckGHAuth checks if gh CLI is authenticated.
func CheckGHAuth(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "gh", "auth", "status")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh not authenticated: %s", stderr.String())
	}
	return nil
}
