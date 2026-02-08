package bench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Workspace manages git operations for benchmark runs.
// Each run gets its own worktree checked out at the pre-fix commit.
type Workspace struct {
	// BaseDir is the root directory for all bench data (~/.orc/bench/).
	BaseDir string
	// ReposDir is where repos are cloned (~/.orc/bench/repos/).
	ReposDir string
	// RunsDir is where per-run worktrees live (~/.orc/bench/runs/).
	RunsDir string
}

// NewWorkspace creates a new workspace manager.
func NewWorkspace(baseDir string) *Workspace {
	return &Workspace{
		BaseDir:  baseDir,
		ReposDir: filepath.Join(baseDir, "repos"),
		RunsDir:  filepath.Join(baseDir, "runs"),
	}
}

// DefaultWorkspace returns a workspace at the default location.
func DefaultWorkspace() (*Workspace, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}
	return NewWorkspace(filepath.Join(home, ".orc", "bench")), nil
}

// EnsureRepo clones a repo if it doesn't exist locally.
// Returns the path to the local clone.
func (w *Workspace) EnsureRepo(project *Project) (string, error) {
	repoDir := filepath.Join(w.ReposDir, project.ID)

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
		// Already cloned, fetch latest
		cmd := exec.Command("git", "fetch", "--all")
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git fetch %s: %s: %w", project.ID, string(out), err)
		}
		return repoDir, nil
	}

	// Clone
	if err := os.MkdirAll(w.ReposDir, 0755); err != nil {
		return "", fmt.Errorf("create repos dir: %w", err)
	}

	cmd := exec.Command("git", "clone", project.RepoURL, repoDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone %s: %s: %w", project.RepoURL, string(out), err)
	}

	return repoDir, nil
}

// SetupRun creates a worktree for a specific benchmark run.
// The worktree is checked out at the task's pre-fix commit.
// Returns the worktree path.
func (w *Workspace) SetupRun(runID string, project *Project, task *Task) (string, error) {
	repoDir, err := w.EnsureRepo(project)
	if err != nil {
		return "", err
	}

	worktreePath := filepath.Join(w.RunsDir, runID)

	// Remove if exists (stale from previous failed run)
	if _, err := os.Stat(worktreePath); err == nil {
		w.CleanupRun(runID, repoDir)
	}

	if err := os.MkdirAll(w.RunsDir, 0755); err != nil {
		return "", fmt.Errorf("create runs dir: %w", err)
	}

	// Create worktree at pre-fix commit (detached HEAD)
	cmd := exec.Command("git", "worktree", "add", "--detach", worktreePath, task.PreFixCommit)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create worktree for run %s at %s: %s: %w", runID, task.PreFixCommit, string(out), err)
	}

	return worktreePath, nil
}

// CleanupRun removes a run's worktree.
func (w *Workspace) CleanupRun(runID string, repoDir string) {
	worktreePath := filepath.Join(w.RunsDir, runID)

	// Remove git worktree reference
	cmd := exec.Command("git", "worktree", "remove", "--force", worktreePath)
	cmd.Dir = repoDir
	_ = cmd.Run()

	// Remove directory if it still exists
	_ = os.RemoveAll(worktreePath)
}

