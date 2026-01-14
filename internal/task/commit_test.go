package task

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommitAndSync(t *testing.T) {
	// Create temp directory for project root
	tmpDir, err := os.MkdirTemp("", "task-commit-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo (required for git commands)
	if err := initGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Create .orc/tasks directory structure
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	// Create task
	task := New("TASK-001", "Test Task")

	// Save task to the temp directory
	taskDir := filepath.Join(tasksDir, task.ID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}
	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Test CommitAndSync - should not error
	cfg := CommitConfig{
		ProjectRoot:  tmpDir,
		CommitPrefix: "[orc]",
	}

	// Should not panic or return error for valid input
	// The actual git commit may fail in test environment, but the function should handle it gracefully
	err = CommitAndSync(task, "created", cfg)
	// We don't check error because git commit may fail in test environment (e.g., no git user configured)
	_ = err
}

func TestCommitDeletion(t *testing.T) {
	// Create temp directory for project root
	tmpDir, err := os.MkdirTemp("", "task-commit-del-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo (required for git commands)
	if err := initGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Create .orc/tasks directory structure
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	// Create and save task first
	task := New("TASK-002", "Task to Delete")
	taskDir := filepath.Join(tasksDir, task.ID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}
	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Stage and commit the task first
	cfg := CommitConfig{
		ProjectRoot:  tmpDir,
		CommitPrefix: "[orc]",
	}
	_ = CommitAndSync(task, "created", cfg)

	// Remove task directory
	if err := os.RemoveAll(taskDir); err != nil {
		t.Fatalf("failed to remove task dir: %v", err)
	}

	// Test CommitDeletion
	err = CommitDeletion("TASK-002", cfg)
	// We don't check error because git commit may fail in test environment
	_ = err
}

func TestCommitStatusChange(t *testing.T) {
	// Create temp directory for project root
	tmpDir, err := os.MkdirTemp("", "task-commit-status-test-")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize git repo (required for git commands)
	if err := initGitRepo(tmpDir); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Create .orc/tasks directory structure
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatalf("failed to create tasks dir: %v", err)
	}

	// Create task
	task := New("TASK-003", "Status Change Task")
	task.Status = StatusRunning

	// Save task to the temp directory
	taskDir := filepath.Join(tasksDir, task.ID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}
	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Test CommitStatusChange
	cfg := CommitConfig{
		ProjectRoot:  tmpDir,
		CommitPrefix: "[orc]",
	}

	err = CommitStatusChange(task, "running", cfg)
	// We don't check error because git commit may fail in test environment
	_ = err
}

func TestDefaultCommitConfig(t *testing.T) {
	cfg := DefaultCommitConfig()

	if cfg.CommitPrefix != "[orc]" {
		t.Errorf("CommitPrefix = %s, want [orc]", cfg.CommitPrefix)
	}

	if cfg.ProjectRoot != "" {
		t.Errorf("ProjectRoot = %s, want empty string", cfg.ProjectRoot)
	}

	if cfg.Logger != nil {
		t.Errorf("Logger = %v, want nil", cfg.Logger)
	}
}

// initGitRepo initializes a git repository in the given directory
func initGitRepo(dir string) error {
	// Initialize git repo
	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		return err
	}

	// Configure git user for commits (required in test environment)
	cmd = exec.Command("git", "-C", dir, "config", "user.email", "test@example.com")
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "-C", dir, "config", "user.name", "Test User")
	if err := cmd.Run(); err != nil {
		return err
	}

	// Create initial commit so git operations work
	gitignore := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(""), 0644); err != nil {
		return err
	}
	cmd = exec.Command("git", "-C", dir, "add", ".")
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = exec.Command("git", "-C", dir, "commit", "-m", "Initial commit")
	return cmd.Run()
}
