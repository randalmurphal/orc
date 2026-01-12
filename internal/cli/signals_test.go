package cli

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	t.Cleanup(func() {
		os.Chdir(origDir)
	})

	tsk := task.New("TASK-001", "Test task")
	st := state.New("TASK-001")

	h := NewInterruptHandler(tsk, st)
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
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	t.Cleanup(func() {
		os.Chdir(origDir)
	})

	tsk := &task.Task{ID: "TASK-001"}
	st := state.New("TASK-001")
	st.StartPhase("implement")

	// GracefulShutdown saves state and task
	err := GracefulShutdown(tsk, st, "implement")

	// Should succeed (creates directories as needed)
	if err != nil {
		t.Errorf("GracefulShutdown failed: %v", err)
	}

	// Verify task status was updated to blocked
	if tsk.Status != task.StatusBlocked {
		t.Errorf("expected task status %v, got %v", task.StatusBlocked, tsk.Status)
	}

	// Verify state was interrupted
	if st.Status != state.StatusInterrupted {
		t.Errorf("expected state status %v, got %v", state.StatusInterrupted, st.Status)
	}
}
