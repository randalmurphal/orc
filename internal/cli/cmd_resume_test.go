package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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

// TestValidateTaskResumableProto tests the validation logic directly without running the executor.
func TestValidateTaskResumableProto(t *testing.T) {
	tests := []struct {
		name        string
		status      orcv1.TaskStatus
		forceResume bool
		wantErr     bool
		errContains string
	}{
		// Non-resumable statuses
		{
			name:        "completed task not resumable",
			status:      orcv1.TaskStatus_TASK_STATUS_COMPLETED,
			wantErr:     true,
			errContains: "cannot be resumed",
		},
		{
			name:        "created task not resumable",
			status:      orcv1.TaskStatus_TASK_STATUS_CREATED,
			wantErr:     true,
			errContains: "cannot be resumed",
		},
		// Resumable statuses
		{
			name:    "paused task is resumable",
			status:  orcv1.TaskStatus_TASK_STATUS_PAUSED,
			wantErr: false,
		},
		{
			name:    "blocked task is resumable",
			status:  orcv1.TaskStatus_TASK_STATUS_BLOCKED,
			wantErr: false,
		},
		{
			name:    "failed task is resumable",
			status:  orcv1.TaskStatus_TASK_STATUS_FAILED,
			wantErr: false,
		},
		// Running task cases - with executor tracking fields:
		// - ExecutorPid=0 means orphaned (resumable without force)
		// - ExecutorPid=alive PID means truly running (not resumable without force)
		{
			name:        "running task not resumable without force",
			status:      orcv1.TaskStatus_TASK_STATUS_RUNNING,
			forceResume: false,
			wantErr:     true,
			errContains: "currently running",
		},
		{
			name:        "running task resumable with force",
			status:      orcv1.TaskStatus_TASK_STATUS_RUNNING,
			forceResume: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tk := task.NewProtoTask("TASK-001", "Test task")
			tk.Status = tt.status
			task.SetCurrentPhaseProto(tk, "implement")

			// For running task tests, set a valid PID so the task appears truly running
			// (not orphaned). Otherwise CheckOrphanedProto would detect it as orphaned.
			if tt.status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
				tk.ExecutorPid = int32(os.Getpid())
			}

			result, err := ValidateTaskResumableProto(tk, tt.forceResume)

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
func TestValidateTaskResumable_OrphanedTask(t *testing.T) {
	// Create a running task with no executor info (ExecutorPid=0)
	// This should be detected as orphaned and be resumable without force
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement")
	// ExecutorPid defaults to 0, which means orphaned

	// Without force - should succeed because task is orphaned
	result, err := ValidateTaskResumableProto(tk, false)
	if err != nil {
		t.Errorf("Expected orphaned running task to be resumable without force, got: %v", err)
	}
	if result == nil {
		t.Error("Expected validation result, got nil")
	}
}

// TestValidateTaskResumable_ForceRunning tests force flag with running task.
// Uses proto types. Set ExecutorPid to a live PID so the task appears truly running.
func TestValidateTaskResumable_ForceRunning(t *testing.T) {
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task.SetCurrentPhaseProto(tk, "implement")
	// Set to current process PID so the task appears to be running (not orphaned)
	tk.ExecutorPid = int32(os.Getpid())

	// Without force - should fail (task appears to still be running)
	_, err := ValidateTaskResumableProto(tk, false)
	if err == nil {
		t.Error("Expected error for running task without force")
	}

	// With force - should succeed
	result, err := ValidateTaskResumableProto(tk, true)
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
