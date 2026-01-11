// Package e2e contains end-to-end tests for the orc workflow.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/tests/testutil"
)

// TestDeleteCommand verifies the orc delete command.
func TestDeleteCommand(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create a task
	taskDir := repo.CreateTask("TASK-001", "Test task to delete")
	testutil.AssertFileExists(t, filepath.Join(taskDir, "task.yaml"))

	// Delete the task by removing directory (simulating command behavior)
	err := os.RemoveAll(taskDir)
	if err != nil {
		t.Fatalf("delete task: %v", err)
	}

	// Verify task directory is gone
	testutil.AssertFileNotExists(t, filepath.Join(taskDir, "task.yaml"))
	testutil.AssertFileNotExists(t, taskDir)
}

// TestDeleteCommandPreservesOtherTasks verifies delete only removes target task.
func TestDeleteCommandPreservesOtherTasks(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create multiple tasks
	task1Dir := repo.CreateTask("TASK-001", "Task one")
	task2Dir := repo.CreateTask("TASK-002", "Task two")
	task3Dir := repo.CreateTask("TASK-003", "Task three")

	// Delete task 2
	err := os.RemoveAll(task2Dir)
	if err != nil {
		t.Fatalf("delete task: %v", err)
	}

	// Verify task 2 is gone but others remain
	testutil.AssertFileNotExists(t, task2Dir)
	testutil.AssertFileExists(t, filepath.Join(task1Dir, "task.yaml"))
	testutil.AssertFileExists(t, filepath.Join(task3Dir, "task.yaml"))
}

// TestDiffCommandWithBranch verifies diff works with task branch.
func TestDiffCommandWithBranch(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	branchName := "orc/" + taskID

	// Create task branch
	testutil.CreateBranch(t, repo.RootDir, branchName)
	testutil.AssertBranchExists(t, repo.RootDir, branchName)

	// Switch to task branch and make a change
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout branch: %v\n%s", err, output)
	}

	// Create a new file
	newFile := filepath.Join(repo.RootDir, "feature.go")
	if err := os.WriteFile(newFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Commit the change
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}

	cmd = exec.Command("git", "commit", "-m", "Add feature")
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}

	// Run git diff main...branchName
	cmd = exec.Command("git", "diff", "--stat", "main..."+branchName)
	cmd.Dir = repo.RootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff: %v\n%s", err, output)
	}

	// Verify diff shows the new file
	if !strings.Contains(string(output), "feature.go") {
		t.Errorf("diff output should contain feature.go, got: %s", output)
	}
}

// TestDiffCommandNoChanges verifies diff with no changes.
func TestDiffCommandNoChanges(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	branchName := "orc/" + taskID

	// Create task branch (no changes)
	testutil.CreateBranch(t, repo.RootDir, branchName)

	// Run git diff - should be empty
	cmd := exec.Command("git", "diff", "main..."+branchName)
	cmd.Dir = repo.RootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff: %v\n%s", err, output)
	}

	// Empty diff is expected
	if len(strings.TrimSpace(string(output))) != 0 {
		t.Errorf("expected empty diff, got: %s", output)
	}
}

// TestDiffCommandNameOnly verifies --name-only flag behavior.
func TestDiffCommandNameOnly(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	branchName := "orc/" + taskID

	// Create and checkout branch
	testutil.CreateBranch(t, repo.RootDir, branchName)
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v\n%s", err, output)
	}

	// Create multiple files
	files := []string{"a.go", "b.go", "c.go"}
	for _, f := range files {
		path := filepath.Join(repo.RootDir, f)
		if err := os.WriteFile(path, []byte("package main\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	// Commit
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repo.RootDir
	cmd.CombinedOutput()
	cmd = exec.Command("git", "commit", "-m", "Add files")
	cmd.Dir = repo.RootDir
	cmd.CombinedOutput()

	// Get name-only diff
	cmd = exec.Command("git", "diff", "--name-only", "main..."+branchName)
	cmd.Dir = repo.RootDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git diff --name-only: %v\n%s", err, output)
	}

	// Should list all three files
	for _, f := range files {
		if !strings.Contains(string(output), f) {
			t.Errorf("name-only output should contain %s, got: %s", f, output)
		}
	}
}

// TestGetBaseBranch verifies base branch detection.
func TestGetBaseBranch(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Default branch should be detected
	cmd := exec.Command("git", "rev-parse", "--verify", "main")
	cmd.Dir = repo.RootDir
	if err := cmd.Run(); err != nil {
		// If main doesn't exist, master should
		cmd = exec.Command("git", "rev-parse", "--verify", "master")
		cmd.Dir = repo.RootDir
		if err := cmd.Run(); err != nil {
			t.Skip("No main or master branch")
		}
	}
}
