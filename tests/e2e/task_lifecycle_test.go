// Package e2e contains end-to-end tests for the orc workflow.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestTaskLifecycleCreation verifies the full task creation lifecycle.
func TestTaskLifecycleCreation(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create tasks directory
	tasksDir := filepath.Join(repo.OrcDir, "tasks")

	// Create sequence store
	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	// Create task ID generator
	gen := task.NewTaskIDGenerator(task.ModeSolo, "",
		task.WithSequenceStore(store),
	)

	// Generate task ID
	taskID, err := gen.Next()
	if err != nil {
		t.Fatalf("generate task ID: %v", err)
	}

	if taskID != "TASK-001" {
		t.Errorf("first task ID = %q, want TASK-001", taskID)
	}

	// Create task directory
	taskDir := filepath.Join(tasksDir, taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("create task dir: %v", err)
	}

	// Create task.yaml
	taskData := map[string]any{
		"id":          taskID,
		"title":       "Test task for lifecycle",
		"description": "This is a test task to verify the lifecycle",
		"weight":      "small",
		"status":      "pending",
	}
	testutil.WriteYAML(t, filepath.Join(taskDir, "task.yaml"), taskData)

	// Verify task was created
	testutil.AssertFileExists(t, filepath.Join(taskDir, "task.yaml"))

	// Verify task.yaml content
	savedTask := testutil.ReadYAML(t, filepath.Join(taskDir, "task.yaml"))
	if savedTask["id"] != taskID {
		t.Errorf("saved task id = %v, want %s", savedTask["id"], taskID)
	}
	if savedTask["title"] != "Test task for lifecycle" {
		t.Errorf("saved task title = %v", savedTask["title"])
	}
}

// TestTaskLifecycleBranch verifies branch creation for tasks.
func TestTaskLifecycleBranch(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	executorPrefix := ""

	// Expected branch name in solo mode
	branchName := git.BranchName(taskID, executorPrefix)
	expectedBranch := "orc/TASK-001"

	if branchName != expectedBranch {
		t.Errorf("branch name = %q, want %q", branchName, expectedBranch)
	}

	// Create the branch
	testutil.CreateBranch(t, repo.RootDir, branchName)

	// Verify branch exists
	testutil.AssertBranchExists(t, repo.RootDir, branchName)
}

// TestTaskLifecycleWorktree verifies worktree creation for tasks.
func TestTaskLifecycleWorktree(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	executorPrefix := ""

	// Expected worktree directory
	worktreeDir := git.WorktreeDirName(taskID, executorPrefix)
	expectedDir := "orc-TASK-001"

	if worktreeDir != expectedDir {
		t.Errorf("worktree dir = %q, want %q", worktreeDir, expectedDir)
	}

	// Full worktree path
	worktreePath := filepath.Join(repo.OrcDir, "worktrees", worktreeDir)

	// Create worktree directory (simulating worktree setup)
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	// Verify worktree exists
	testutil.AssertWorktreeExists(t, worktreePath)
}

// TestTaskLifecycleState verifies state tracking during task execution.
func TestTaskLifecycleState(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"

	// Create task directory
	taskDir := filepath.Join(repo.OrcDir, "tasks", taskID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("create task dir: %v", err)
	}

	// Create initial state
	initialState := map[string]any{
		"current_phase": "pending",
		"iteration":     0,
		"started_at":    "2025-01-11T10:00:00Z",
	}
	testutil.WriteYAML(t, filepath.Join(taskDir, "state.yaml"), initialState)

	// Simulate phase transition
	runningState := map[string]any{
		"current_phase": "implement",
		"iteration":     1,
		"started_at":    "2025-01-11T10:00:00Z",
	}
	testutil.WriteYAML(t, filepath.Join(taskDir, "state.yaml"), runningState)

	// Verify state was updated
	state := testutil.ReadYAML(t, filepath.Join(taskDir, "state.yaml"))
	if state["current_phase"] != "implement" {
		t.Errorf("current_phase = %v, want implement", state["current_phase"])
	}
	if state["iteration"] != 1 {
		t.Errorf("iteration = %v, want 1", state["iteration"])
	}
}

// TestTaskLifecyclePlan verifies plan generation for tasks.
func TestTaskLifecyclePlan(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	taskDir := filepath.Join(repo.OrcDir, "tasks", taskID)

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("create task dir: %v", err)
	}

	// Create plan.yaml (simulating plan generation for "small" weight)
	plan := map[string]any{
		"task_id": taskID,
		"weight":  "small",
		"phases": []map[string]any{
			{
				"id":       "implement",
				"name":     "Implementation",
				"gate":     "auto",
				"template": "implement",
			},
			{
				"id":       "test",
				"name":     "Testing",
				"gate":     "auto",
				"template": "test",
			},
		},
	}
	testutil.WriteYAML(t, filepath.Join(taskDir, "plan.yaml"), plan)

	// Verify plan
	savedPlan := testutil.ReadYAML(t, filepath.Join(taskDir, "plan.yaml"))
	if savedPlan["weight"] != "small" {
		t.Errorf("plan weight = %v, want small", savedPlan["weight"])
	}

	phases, ok := savedPlan["phases"].([]interface{})
	if !ok {
		t.Fatal("phases should be an array")
	}
	if len(phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(phases))
	}
}

// TestTaskLifecycleGitCommit verifies git commit creation after phase completion.
func TestTaskLifecycleGitCommit(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"

	// Create and checkout branch
	branchName := git.BranchName(taskID, "")
	testutil.CreateBranch(t, repo.RootDir, branchName)

	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout branch: %v\n%s", err, output)
	}

	// Create a file to commit
	testFile := filepath.Join(repo.RootDir, "test_file.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	// Stage and commit
	cmd = exec.Command("git", "add", "test_file.txt")
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, output)
	}

	commitMsg := "[orc] " + taskID + ": implement - completed"
	cmd = exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = repo.RootDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, output)
	}

	// Verify commit exists
	cmd = exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = repo.RootDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}

	testutil.AssertFileContains(t, testFile, "test content")
	if len(output) == 0 {
		t.Error("expected commit to exist")
	}
}

// TestTaskLifecycleTranscript verifies transcript storage.
func TestTaskLifecycleTranscript(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"
	taskDir := filepath.Join(repo.OrcDir, "tasks", taskID)
	transcriptsDir := filepath.Join(taskDir, "transcripts")

	if err := os.MkdirAll(transcriptsDir, 0755); err != nil {
		t.Fatalf("create transcripts dir: %v", err)
	}

	// Create transcript file
	transcript := `User: Implement the feature
Assistant: I'll help implement this feature.
[Tool call: Read file...]
Assistant: Done implementing the feature.
`
	transcriptPath := filepath.Join(transcriptsDir, "implement_001.md")
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0644); err != nil {
		t.Fatalf("write transcript: %v", err)
	}

	// Verify transcript
	testutil.AssertFileExists(t, transcriptPath)
	testutil.AssertFileContains(t, transcriptPath, "Implement the feature")
}

// TestTaskLifecycleComplete verifies the complete task lifecycle flow.
func TestTaskLifecycleComplete(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// 1. Generate task ID
	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)
	gen := task.NewTaskIDGenerator(task.ModeSolo, "", task.WithSequenceStore(store))

	taskID, err := gen.Next()
	if err != nil {
		t.Fatalf("generate task ID: %v", err)
	}

	// 2. Create task structure
	taskDir := filepath.Join(repo.OrcDir, "tasks", taskID)
	transcriptsDir := filepath.Join(taskDir, "transcripts")
	if err := os.MkdirAll(transcriptsDir, 0755); err != nil {
		t.Fatalf("create task dirs: %v", err)
	}

	// 3. Create task.yaml
	taskData := map[string]any{
		"id":     taskID,
		"title":  "Complete lifecycle test",
		"weight": "small",
		"status": "pending",
	}
	testutil.WriteYAML(t, filepath.Join(taskDir, "task.yaml"), taskData)

	// 4. Create plan.yaml
	plan := map[string]any{
		"task_id": taskID,
		"weight":  "small",
		"phases": []map[string]any{
			{"id": "implement", "gate": "auto"},
			{"id": "test", "gate": "auto"},
		},
	}
	testutil.WriteYAML(t, filepath.Join(taskDir, "plan.yaml"), plan)

	// 5. Create branch
	branchName := git.BranchName(taskID, "")
	testutil.CreateBranch(t, repo.RootDir, branchName)

	// 6. Create worktree directory
	worktreePath := filepath.Join(repo.OrcDir, "worktrees", git.WorktreeDirName(taskID, ""))
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	// 7. Update state through phases
	states := []map[string]any{
		{"current_phase": "implement", "iteration": 1},
		{"current_phase": "implement", "iteration": 2, "completed": true},
		{"current_phase": "test", "iteration": 1},
		{"current_phase": "test", "iteration": 1, "completed": true},
	}

	for _, state := range states {
		testutil.WriteYAML(t, filepath.Join(taskDir, "state.yaml"), state)
	}

	// 8. Mark task as complete
	taskData["status"] = "completed"
	testutil.WriteYAML(t, filepath.Join(taskDir, "task.yaml"), taskData)

	// Verify final state
	finalTask := testutil.ReadYAML(t, filepath.Join(taskDir, "task.yaml"))
	if finalTask["status"] != "completed" {
		t.Errorf("final status = %v, want completed", finalTask["status"])
	}

	// Verify all files exist
	testutil.AssertFileExists(t, filepath.Join(taskDir, "task.yaml"))
	testutil.AssertFileExists(t, filepath.Join(taskDir, "plan.yaml"))
	testutil.AssertFileExists(t, filepath.Join(taskDir, "state.yaml"))
	testutil.AssertBranchExists(t, repo.RootDir, branchName)
	testutil.AssertWorktreeExists(t, worktreePath)
}

// TestTaskLifecycleCleanup verifies cleanup after task completion.
func TestTaskLifecycleCleanup(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	taskID := "TASK-001"

	// Create worktree
	worktreePath := filepath.Join(repo.OrcDir, "worktrees", git.WorktreeDirName(taskID, ""))
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	// Verify worktree exists
	testutil.AssertWorktreeExists(t, worktreePath)

	// Simulate cleanup (cleanup_on_complete: true)
	if err := os.RemoveAll(worktreePath); err != nil {
		t.Fatalf("remove worktree: %v", err)
	}

	// Verify worktree removed
	testutil.AssertWorktreeNotExists(t, worktreePath)
}
