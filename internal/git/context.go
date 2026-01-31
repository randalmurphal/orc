package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

// RepoPath returns the path to the main repository.
func (g *Context) RepoPath() string {
	return g.repoPath
}

// WorkDir returns the current working directory for git commands.
// This is the repo path unless working in a worktree.
func (g *Context) WorkDir() string {
	return g.workDir
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

// Fetch fetches updates from the remote.
func (g *Context) Fetch(remote string) error {
	if _, err := g.runGit("fetch", remote); err != nil {
		return &GitError{Op: "fetch", Err: err}
	}
	return nil
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

