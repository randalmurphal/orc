package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withResumeTestDir creates a temp directory with orc initialized and changes to it
func withResumeTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("failed to init orc: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(origWd)
	})

	return tmpDir
}

// createResumeTestBackend creates a backend in the given directory.
func createResumeTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestResumeCommand_TaskNotFound(t *testing.T) {
	withResumeTestDir(t)

	// Run resume command for non-existent task
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-999"})

	var stderr bytes.Buffer
	cmd.SetErr(&stderr)

	err := cmd.Execute()
	if err == nil {
		t.Error("Resume should fail for non-existent task")
	}

	if err != nil && !contains([]string{err.Error()}, "task TASK-999 not found") {
		t.Errorf("Expected 'task not found' error, got: %v", err)
	}
}

// TestValidateTaskResumable tests the validation logic directly without running the executor.
func TestValidateTaskResumable(t *testing.T) {
	tests := []struct {
		name        string
		status      task.Status
		setLivePID  bool // If true, sets ExecutorPID to current process (alive)
		forceResume bool
		wantErr     bool
		errContains string
	}{
		// Non-resumable statuses
		{
			name:        "completed task not resumable",
			status:      task.StatusCompleted,
			wantErr:     true,
			errContains: "cannot be resumed",
		},
		{
			name:        "created task not resumable",
			status:      task.StatusCreated,
			wantErr:     true,
			errContains: "cannot be resumed",
		},
		// Resumable statuses
		{
			name:    "paused task is resumable",
			status:  task.StatusPaused,
			wantErr: false,
		},
		{
			name:    "blocked task is resumable",
			status:  task.StatusBlocked,
			wantErr: false,
		},
		{
			name:    "failed task is resumable",
			status:  task.StatusFailed,
			wantErr: false,
		},
		// Running task cases
		{
			name:        "running task not resumable without force",
			status:      task.StatusRunning,
			setLivePID:  true, // Need a live PID to be detected as truly running (not orphaned)
			forceResume: false,
			wantErr:     true,
			errContains: "currently running",
		},
		{
			name:        "running task resumable with force",
			status:      task.StatusRunning,
			setLivePID:  true,
			forceResume: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := task.New("TASK-001", "Test task")
			tk.Status = tt.status
			tk.CurrentPhase = "implement"
			if tt.setLivePID {
				tk.ExecutorPID = os.Getpid() // Set to current process (alive)
			}

			result, err := ValidateTaskResumable(tk, tt.forceResume)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errContains)
					return
				}
				if !contains([]string{err.Error()}, tt.errContains) {
					t.Errorf("Expected error containing %q, got: %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if result == nil {
					t.Error("Expected validation result, got nil")
				}
			}
		})
	}
}

// TestValidateTaskResumable_OrphanedTask tests orphan detection in validation.
// This tests the fix where we check task.ExecutorPID (source of truth).
func TestValidateTaskResumable_OrphanedTask(t *testing.T) {
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusRunning
	tk.CurrentPhase = "implement"
	// Set a PID that doesn't exist (process is dead = orphaned)
	// This is the key field checked by task.CheckOrphaned()
	tk.ExecutorPID = 999999

	result, err := ValidateTaskResumable(tk, false)

	if err != nil {
		t.Errorf("Orphaned task should be resumable, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result, got nil")
	}
	if !result.IsOrphaned {
		t.Error("Expected IsOrphaned=true for dead PID")
	}
	if !result.RequiresStateUpdate {
		t.Error("Expected RequiresStateUpdate=true for orphaned task")
	}
}

// TestValidateTaskResumable_ForceRunning tests force flag with running task.
func TestValidateTaskResumable_ForceRunning(t *testing.T) {
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusRunning
	tk.CurrentPhase = "implement"
	// Set our own PID so it appears as a live process
	tk.ExecutorPID = os.Getpid()

	// Without force - should fail (task appears to still be running)
	_, err := ValidateTaskResumable(tk, false)
	if err == nil {
		t.Error("Expected error for running task without force")
	}

	// With force - should succeed
	result, err := ValidateTaskResumable(tk, true)
	if err != nil {
		t.Errorf("Expected no error with force flag, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected validation result, got nil")
	}
	if !result.RequiresStateUpdate {
		t.Error("Expected RequiresStateUpdate=true for force-resumed task")
	}
}

func TestResumeCommand_FromWorktreeDirectory(t *testing.T) {
	// Create main project structure
	tmpDir := t.TempDir()
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("failed to init orc: %v", err)
	}

	// Create a task in the main repo via backend
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to tmpDir: %v", err)
	}

	// Create test data via backend - use completed status so it fails validation
	// early (before trying to run executor)
	backend := createResumeTestBackend(t, tmpDir)

	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusCompleted // Will fail with "cannot be resumed"
	tk.Weight = task.WeightSmall
	tk.CurrentPhase = "implement"
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	_ = backend.Close()

	// Create a "worktree-like" subdirectory (simulating worktree context)
	worktreeDir := filepath.Join(tmpDir, ".orc", "worktrees", "orc-TASK-001")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// Create minimal .orc in worktree (like a real worktree would have)
	worktreeOrcDir := filepath.Join(worktreeDir, ".orc")
	if err := os.MkdirAll(worktreeOrcDir, 0755); err != nil {
		t.Fatalf("failed to create worktree .orc dir: %v", err)
	}

	// Change to worktree directory (no tasks here!)
	if err := os.Chdir(worktreeDir); err != nil {
		t.Fatalf("failed to chdir to worktree: %v", err)
	}

	// The resume command should find the task in the main repo via FindProjectRoot
	cmd := newResumeCmd()
	cmd.SetArgs([]string{"TASK-001"})

	err = cmd.Execute()

	// Should get "cannot be resumed" (because completed), NOT "task not found"
	// This verifies the task was found from the worktree directory
	if err == nil {
		t.Error("Expected error for completed task")
	}
	if err != nil && contains([]string{err.Error()}, "task TASK-001 not found") {
		t.Errorf("Resume from worktree should find task in main repo, got: %v", err)
	}
}

// contains helper to check if any string in the slice contains the substring
func contains(strs []string, substr string) bool {
	for _, s := range strs {
		if s != "" && len(s) > 0 {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
