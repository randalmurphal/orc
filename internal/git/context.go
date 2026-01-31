package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Context manages git operations for a repository.
// Absorbed from devflow/git to eliminate the external dependency.
type Context struct {
	repoPath    string        // Path to the main repository
	worktreeDir string        // Directory where worktrees are created
	workDir     string        // Current working directory for commands (defaults to repoPath)
	runner      CommandRunner // Command runner (defaults to ExecRunner)
}

// ContextOption configures Context.
type ContextOption func(*Context)

// NewContext creates a new git context for the repository.
// It validates that the path is a git repository and applies any options.
func NewContext(repoPath string, opts ...ContextOption) (*Context, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	// Verify it's a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = absPath
	if err := cmd.Run(); err != nil {
		return nil, ErrNotGitRepo
	}

	g := &Context{
		repoPath:    absPath,
		worktreeDir: ".worktrees",
		workDir:     absPath,
		runner:      NewExecRunner(),
	}

	for _, opt := range opts {
		opt(g)
	}

	return g, nil
}

// WithWorktreeDir sets the directory where worktrees are created.
// Default is ".worktrees" relative to the repository root.
func WithWorktreeDir(dir string) ContextOption {
	return func(g *Context) {
		g.worktreeDir = dir
	}
}

// WithRunner sets a custom command runner for git operations.
// This is primarily used for testing to inject mock command execution.
func WithRunner(runner CommandRunner) ContextOption {
	return func(g *Context) {
		g.runner = runner
	}
}

// RepoPath returns the path to the main repository.
func (g *Context) RepoPath() string {
	return g.repoPath
}

// WorkDir returns the current working directory for git commands.
// This is the repo path unless working in a worktree.
func (g *Context) WorkDir() string {
	return g.workDir
}

// WorktreeDirPath returns the path to the worktrees directory.
func (g *Context) WorktreeDirPath() string {
	if filepath.IsAbs(g.worktreeDir) {
		return g.worktreeDir
	}
	return filepath.Join(g.repoPath, g.worktreeDir)
}

// InWorktree returns a new Context that operates in the specified worktree.
func (g *Context) InWorktree(worktreePath string) *Context {
	return &Context{
		repoPath:    g.repoPath,
		worktreeDir: g.worktreeDir,
		workDir:     worktreePath,
		runner:      g.runner,
	}
}

// CurrentBranch returns the current branch name.
func (g *Context) CurrentBranch() (string, error) {
	branch, err := g.runGit("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", &GitError{Op: "get current branch", Err: err}
	}
	return branch, nil
}

// Checkout switches to the specified ref (branch, tag, or commit).
func (g *Context) Checkout(ref string) error {
	if _, err := g.runGit("checkout", ref); err != nil {
		return &GitError{Op: "checkout", Err: err}
	}
	return nil
}

// CreateBranch creates a new branch at HEAD.
func (g *Context) CreateBranch(name string) error {
	if _, err := g.runGit("branch", name); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return ErrBranchExists
		}
		return &GitError{Op: "create branch", Err: err}
	}
	return nil
}

// DeleteBranch deletes a branch. If force is true, uses -D instead of -d.
func (g *Context) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	if _, err := g.runGit("branch", flag, name); err != nil {
		return &GitError{Op: "delete branch", Err: err}
	}
	return nil
}

// BranchExists checks if a branch exists.
func (g *Context) BranchExists(name string) bool {
	_, err := g.runGit("rev-parse", "--verify", name)
	return err == nil
}

// Stage adds files to the staging area.
func (g *Context) Stage(files ...string) error {
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, files...)
	if _, err := g.runGit(args...); err != nil {
		return &GitError{Op: "stage files", Err: err}
	}
	return nil
}

// StageAll stages all changes (git add -A).
func (g *Context) StageAll() error {
	if _, err := g.runGit("add", "-A"); err != nil {
		return &GitError{Op: "stage all", Err: err}
	}
	return nil
}

// Commit creates a commit with the given message.
// Returns ErrNothingToCommit if there are no staged changes.
func (g *Context) Commit(message string) error {
	output, err := g.runGit("commit", "-m", message)
	if err != nil {
		if strings.Contains(output, "nothing to commit") ||
			strings.Contains(err.Error(), "nothing to commit") {
			return ErrNothingToCommit
		}
		return &GitError{Op: "commit", Output: output, Err: err}
	}
	return nil
}

// Push pushes the branch to the remote.
// If setUpstream is true, uses -u to set upstream tracking.
func (g *Context) Push(remote, branch string, setUpstream bool) error {
	args := []string{"push"}
	if setUpstream {
		args = append(args, "-u")
	}
	args = append(args, remote, branch)

	if _, err := g.runGit(args...); err != nil {
		return &GitError{Op: "push", Err: err}
	}
	return nil
}

// Pull pulls changes from the remote.
func (g *Context) Pull(remote, branch string) error {
	if _, err := g.runGit("pull", remote, branch); err != nil {
		return &GitError{Op: "pull", Err: err}
	}
	return nil
}

// Fetch fetches updates from the remote.
func (g *Context) Fetch(remote string) error {
	if _, err := g.runGit("fetch", remote); err != nil {
		return &GitError{Op: "fetch", Err: err}
	}
	return nil
}

// Diff returns the diff between two refs.
func (g *Context) Diff(base, head string) (string, error) {
	diff, err := g.runGit("diff", base+"..."+head)
	if err != nil {
		return "", &GitError{Op: "diff", Err: err}
	}
	return diff, nil
}

// DiffStaged returns the diff of staged changes.
func (g *Context) DiffStaged() (string, error) {
	diff, err := g.runGit("diff", "--cached")
	if err != nil {
		return "", &GitError{Op: "diff staged", Err: err}
	}
	return diff, nil
}

// Status returns the working tree status in short format.
func (g *Context) Status() (string, error) {
	status, err := g.runGit("status", "--short")
	if err != nil {
		return "", &GitError{Op: "status", Err: err}
	}
	return status, nil
}

// IsClean returns true if the working tree has no uncommitted changes.
func (g *Context) IsClean() (bool, error) {
	status, err := g.Status()
	if err != nil {
		return false, err
	}
	return status == "", nil
}

// HeadCommit returns the current HEAD commit SHA.
func (g *Context) HeadCommit() (string, error) {
	sha, err := g.runGit("rev-parse", "HEAD")
	if err != nil {
		return "", &GitError{Op: "get HEAD commit", Err: err}
	}
	return sha, nil
}

// IsBranchPushed checks if the branch exists on the remote.
func (g *Context) IsBranchPushed(branch string) bool {
	_, err := g.runGit("rev-parse", "--verify", "origin/"+branch)
	return err == nil
}

// GetRemoteURL returns the URL of the specified remote.
func (g *Context) GetRemoteURL(remote string) (string, error) {
	url, err := g.runGit("remote", "get-url", remote)
	if err != nil {
		return "", &GitError{Op: "get remote URL", Err: err}
	}
	return url, nil
}

// runGit executes a git command and returns stdout.
func (g *Context) runGit(args ...string) (string, error) {
	return g.runner.Run(g.workDir, "git", args...)
}

// RunGit executes a git command and returns stdout.
// This is the public version of runGit for use by external packages.
func (g *Context) RunGit(args ...string) (string, error) {
	return g.runGit(args...)
}

// --- Worktree operations ---

// WorktreeInfo represents an active git worktree.
type WorktreeInfo struct {
	Path   string // Filesystem path to the worktree
	Branch string // Branch checked out in the worktree
	Commit string // HEAD commit SHA
}

// CreateContextWorktree creates an isolated worktree for the branch.
// If the branch doesn't exist, it will be created.
// Returns the path to the worktree directory.
func (g *Context) CreateContextWorktree(branch string) (string, error) {
	safeName := SanitizeBranchName(branch)
	worktreeBase := g.WorktreeDirPath()
	worktreePath := filepath.Join(worktreeBase, safeName)

	if _, err := os.Stat(worktreePath); err == nil {
		return "", ErrWorktreeExists
	}

	worktreesDir := worktreeBase
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", fmt.Errorf("create worktrees dir: %w", err)
	}

	_, err := g.runGit("worktree", "add", "-b", branch, worktreePath, "HEAD")
	if err != nil {
		_, err = g.runGit("worktree", "add", worktreePath, branch)
		if err != nil {
			if strings.Contains(err.Error(), "not a valid reference") ||
				strings.Contains(err.Error(), "invalid reference") {
				return "", fmt.Errorf("branch %q does not exist and could not be created: %w", branch, err)
			}
			return "", &GitError{Op: "create worktree", Err: err}
		}
	}

	return worktreePath, nil
}

// CleanupWorktree removes a worktree and its registration.
func (g *Context) CleanupWorktree(worktreePath string) error {
	_, err := g.runGit("worktree", "remove", worktreePath)
	if err != nil {
		_, err = g.runGit("worktree", "remove", "--force", worktreePath)
		if err != nil {
			return &GitError{Op: "cleanup worktree", Err: err}
		}
	}
	return nil
}

// ListWorktrees returns all active worktrees.
func (g *Context) ListWorktrees() ([]WorktreeInfo, error) {
	output, err := g.runGit("worktree", "list", "--porcelain")
	if err != nil {
		return nil, &GitError{Op: "list worktrees", Err: err}
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			current.Branch = "(detached)"
		}
	}

	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// GetWorktree returns information about a specific worktree by branch name.
func (g *Context) GetWorktree(branch string) (*WorktreeInfo, error) {
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return nil, err
	}

	for _, wt := range worktrees {
		if wt.Branch == branch {
			return &wt, nil
		}
	}

	return nil, ErrWorktreeNotFound
}

// GetWorktreeByPath returns information about a specific worktree by path.
func (g *Context) GetWorktreeByPath(path string) (*WorktreeInfo, error) {
	worktrees, err := g.ListWorktrees()
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	for _, wt := range worktrees {
		wtAbs, err := filepath.Abs(wt.Path)
		if err != nil {
			continue
		}
		if wtAbs == absPath {
			return &wt, nil
		}
	}

	return nil, ErrWorktreeNotFound
}

// PruneWorktrees removes stale worktree administrative files.
func (g *Context) PruneWorktrees() error {
	if _, err := g.runGit("worktree", "prune"); err != nil {
		return &GitError{Op: "prune worktrees", Err: err}
	}
	return nil
}

// SanitizeBranchName converts a branch name to a safe directory name.
func SanitizeBranchName(branch string) string {
	safe := strings.ReplaceAll(branch, "/", "-")
	safe = strings.ToLower(safe)
	safe = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(safe, "")
	safe = regexp.MustCompile(`-+`).ReplaceAllString(safe, "-")
	safe = strings.Trim(safe, "-")
	return safe
}
