// Package git provides git integration for orc checkpointing.
package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Checkpoint represents a git checkpoint (commit) for a phase.
type Checkpoint struct {
	TaskID    string    `yaml:"task_id" json:"task_id"`
	Phase     string    `yaml:"phase" json:"phase"`
	CommitSHA string    `yaml:"commit_sha" json:"commit_sha"`
	Message   string    `yaml:"message" json:"message"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// Git provides git operations for orc.
type Git struct {
	workDir      string
	branchPrefix string
	commitPrefix string
}

// New creates a new Git instance.
func New(workDir string) *Git {
	return &Git{
		workDir:      workDir,
		branchPrefix: "orc/",
		commitPrefix: "[orc]",
	}
}

// CreateBranch creates a new branch for a task.
func (g *Git) CreateBranch(taskID string) error {
	branchName := g.branchPrefix + taskID

	// Create and checkout branch
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w\n%s", branchName, err, output)
	}

	return nil
}

// SwitchBranch switches to an existing branch.
func (g *Git) SwitchBranch(taskID string) error {
	branchName := g.branchPrefix + taskID

	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch to branch %s: %w\n%s", branchName, err, output)
	}

	return nil
}

// CreateCheckpoint creates a checkpoint commit for a phase.
func (g *Git) CreateCheckpoint(taskID, phase, message string) (*Checkpoint, error) {
	// Stage all changes
	stageCmd := exec.Command("git", "add", "-A")
	stageCmd.Dir = g.workDir
	if output, err := stageCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to stage changes: %w\n%s", err, output)
	}

	// Check if there are changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = g.workDir
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to check status: %w", err)
	}

	// If no changes, still create an empty commit for the checkpoint
	commitMsg := fmt.Sprintf("%s %s: %s - %s", g.commitPrefix, taskID, phase, message)

	var commitCmd *exec.Cmd
	if len(strings.TrimSpace(string(statusOutput))) == 0 {
		// Allow empty commit for checkpoint
		commitCmd = exec.Command("git", "commit", "--allow-empty", "-m", commitMsg)
	} else {
		commitCmd = exec.Command("git", "commit", "-m", commitMsg)
	}
	commitCmd.Dir = g.workDir

	if output, err := commitCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create commit: %w\n%s", err, output)
	}

	// Get the commit SHA
	shaCmd := exec.Command("git", "rev-parse", "HEAD")
	shaCmd.Dir = g.workDir
	shaOutput, err := shaCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit SHA: %w", err)
	}

	return &Checkpoint{
		TaskID:    taskID,
		Phase:     phase,
		CommitSHA: strings.TrimSpace(string(shaOutput)),
		Message:   message,
		CreatedAt: time.Now(),
	}, nil
}

// Rewind resets the branch to a specific checkpoint.
func (g *Git) Rewind(commitSHA string) error {
	cmd := exec.Command("git", "reset", "--hard", commitSHA)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to rewind to %s: %w\n%s", commitSHA, err, output)
	}

	return nil
}

// GetCheckpoints returns all checkpoint commits for a task.
func (g *Git) GetCheckpoints(taskID string) ([]Checkpoint, error) {
	branchName := g.branchPrefix + taskID

	// Get commits with orc prefix
	cmd := exec.Command("git", "log", branchName, "--oneline", "--grep", g.commitPrefix)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get checkpoints: %w\n%s", err, output)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	checkpoints := make([]Checkpoint, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 2 {
			checkpoints = append(checkpoints, Checkpoint{
				TaskID:    taskID,
				CommitSHA: parts[0],
				Message:   parts[1],
			})
		}
	}

	return checkpoints, nil
}

// GetCurrentBranch returns the current branch name.
func (g *Git) GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = g.workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// IsClean returns true if the working directory is clean.
func (g *Git) IsClean() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = g.workDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check status: %w", err)
	}

	return len(strings.TrimSpace(string(output))) == 0, nil
}

// CreateWorktree creates an isolated worktree for parallel task execution.
func (g *Git) CreateWorktree(taskID, baseBranch string) (string, error) {
	worktreePath := fmt.Sprintf(".orc/worktrees/%s", taskID)
	branchName := g.branchPrefix + taskID

	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create worktree: %w\n%s", err, output)
	}

	return worktreePath, nil
}

// RemoveWorktree removes a worktree.
func (g *Git) RemoveWorktree(taskID string) error {
	worktreePath := fmt.Sprintf(".orc/worktrees/%s", taskID)

	cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = g.workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %w\n%s", err, output)
	}

	return nil
}
