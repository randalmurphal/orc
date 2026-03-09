package bench

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
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

	// Ensure common virtual environment directories are gitignored.
	// Models may run `python -m venv .venv` during build steps. If the repo's
	// .gitignore only has `venv*/` (which doesn't match `.venv/`), the venv
	// gets committed and `git diff` produces massive output (60MB+) that can
	// crash the stream-json scanner.
	ensureBenchGitignore(worktreePath)

	return worktreePath, nil
}

// benchGitignoreEntries are patterns appended to .gitignore in bench worktrees
// to prevent models from accidentally committing large generated directories.
var benchGitignoreEntries = []string{
	".venv/",
	"__pycache__/",
	"*.pyc",
	"node_modules/",
	".tox/",
	".mypy_cache/",
	".pytest_cache/",
	".ruff_cache/",
}

// ensureBenchGitignore appends common generated-directory patterns to .gitignore
// if they're not already present. This is idempotent — safe to call multiple times.
func ensureBenchGitignore(worktreePath string) {
	gitignorePath := filepath.Join(worktreePath, ".gitignore")

	existing, _ := os.ReadFile(gitignorePath)
	existingStr := string(existing)

	var toAdd []string
	for _, entry := range benchGitignoreEntries {
		if !containsLine(existingStr, entry) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return // Best-effort, don't fail the run
	}
	defer f.Close()

	// Ensure we start on a new line
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return
		}
	}
	if _, err := f.WriteString("# Added by orc bench (prevent large diffs from generated dirs)\n"); err != nil {
		return
	}
	for _, entry := range toAdd {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return
		}
	}
}

// containsLine checks if a gitignore file already contains a specific pattern.
func containsLine(content, pattern string) bool {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	return slices.Contains(lines, pattern)
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
