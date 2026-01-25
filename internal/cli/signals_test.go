package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// createSignalsTestBackend creates a backend in the given directory.
func createSignalsTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestSetupSignalHandler(t *testing.T) {
	ctx, cancel := SetupSignalHandler()
	defer cancel()

	if ctx == nil {
		t.Fatal("SetupSignalHandler() returned nil context")
	}

	// Context should not be cancelled initially
	select {
	case <-ctx.Done():
		t.Error("context should not be cancelled initially")
	default:
		// expected
	}

	// Cancel and verify
	cancel()

	select {
	case <-ctx.Done():
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Error("context should be cancelled after cancel()")
	}
}

func TestInterruptHandler(t *testing.T) {
	// Use temp directory to avoid pollution from Cleanup's GracefulShutdown
	tmpDir := t.TempDir()

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	// Create backend
	backend := createSignalsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tsk := task.New("TASK-001", "Test task")

	h := NewInterruptHandler(backend, tsk)
	defer h.Cleanup()

	if h.Context() == nil {
		t.Error("Context() returned nil")
	}

	if h.WasInterrupted() {
		t.Error("WasInterrupted() should be false initially")
	}

	h.SetCurrentPhase("implement")

	// Cancel the context
	h.cancel()

	if !h.WasInterrupted() {
		t.Error("WasInterrupted() should be true after cancel")
	}
}

func TestWaitWithTimeout(t *testing.T) {
	// Test normal timeout
	ctx := context.Background()
	start := time.Now()
	err := WaitWithTimeout(ctx, 50*time.Millisecond)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("WaitWithTimeout() returned error: %v", err)
	}

	if elapsed < 40*time.Millisecond {
		t.Errorf("WaitWithTimeout() returned too quickly: %v", elapsed)
	}

	// Test cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = WaitWithTimeout(ctx, 1*time.Second)
	if err == nil {
		t.Error("WaitWithTimeout() should return error for cancelled context")
	}
}

func TestGracefulShutdown(t *testing.T) {
	// Use a temp directory to test graceful shutdown behavior
	tmpDir := t.TempDir()

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	// Create backend
	backend := createSignalsTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	tsk := &task.Task{ID: "TASK-001", Title: "Test task", Weight: task.WeightSmall}
	// Initialize execution state
	tsk.Execution.Phases = make(map[string]*task.PhaseState)
	tsk.Execution.StartPhase("implement")

	// Save task to backend first
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// GracefulShutdown saves state via task and updates task status
	err := GracefulShutdown(backend, tsk, "implement")

	// Should succeed (creates directories as needed)
	if err != nil {
		t.Errorf("GracefulShutdown failed: %v", err)
	}

	// Verify task status was updated to blocked (task.Status is the single source of truth)
	if tsk.Status != task.StatusBlocked {
		t.Errorf("expected task status %v, got %v", task.StatusBlocked, tsk.Status)
	}

	// Verify phase was interrupted in execution state
	ps := tsk.Execution.Phases["implement"]
	if ps == nil {
		t.Fatal("expected implement phase to exist after interrupt")
	}
	if ps.Status != task.PhaseStatusInterrupted {
		t.Errorf("expected phase status %v, got %v", task.PhaseStatusInterrupted, ps.Status)
	}
}
