package executor

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// TestSIGUSR1CancelsContext verifies that receiving SIGUSR1 cancels the execution context.
// This tests the pause signal handling mechanism used by the WorkflowExecutor.
func TestSIGUSR1CancelsContext(t *testing.T) {
	// Skip in parallel mode since signal handling is process-global
	// t.Parallel() - intentionally not parallel due to signal handling

	// Set up a signal channel like the executor does
	pauseCh := make(chan os.Signal, 1)
	signal.Notify(pauseCh, syscall.SIGUSR1)
	defer func() {
		signal.Stop(pauseCh)
		// Drain any remaining signals
		select {
		case <-pauseCh:
		default:
		}
	}()

	// Create a cancellable context
	execCtx, execCancel := context.WithCancel(context.Background())
	defer execCancel()

	// Start the signal handler goroutine (mirrors workflow_executor.go logic)
	signalReceived := make(chan struct{})
	go func() {
		select {
		case <-pauseCh:
			close(signalReceived)
			execCancel()
		case <-execCtx.Done():
			return
		}
	}()

	// Send SIGUSR1 to ourselves
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("find process: %v", err)
	}

	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		t.Fatalf("send SIGUSR1: %v", err)
	}

	// Wait for signal to be received and context to be cancelled
	select {
	case <-signalReceived:
		// Signal was received - good
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SIGUSR1 to be received")
	}

	// Verify context was cancelled
	select {
	case <-execCtx.Done():
		// Context was cancelled - good
	case <-time.After(100 * time.Millisecond):
		t.Error("context should be cancelled after SIGUSR1")
	}
}

// TestSIGUSR1CleanupOnContextCancel verifies that the signal handler goroutine
// exits cleanly when the context is cancelled without receiving a signal.
func TestSIGUSR1CleanupOnContextCancel(t *testing.T) {
	// Skip in parallel mode since signal handling is process-global
	// t.Parallel() - intentionally not parallel

	pauseCh := make(chan os.Signal, 1)
	signal.Notify(pauseCh, syscall.SIGUSR1)
	defer func() {
		signal.Stop(pauseCh)
		select {
		case <-pauseCh:
		default:
		}
	}()

	execCtx, execCancel := context.WithCancel(context.Background())

	// Track when goroutine exits
	goroutineExited := make(chan struct{})
	go func() {
		defer close(goroutineExited)
		select {
		case <-pauseCh:
			execCancel()
		case <-execCtx.Done():
			return
		}
	}()

	// Cancel the context (simulating normal task completion)
	execCancel()

	// Goroutine should exit cleanly
	select {
	case <-goroutineExited:
		// Good - goroutine exited
	case <-time.After(1 * time.Second):
		t.Error("signal handler goroutine should exit when context is cancelled")
	}
}

// TestSignalChannelDrain verifies that the cleanup code properly drains
// any pending signals from the channel.
func TestSignalChannelDrain(t *testing.T) {
	t.Parallel()

	pauseCh := make(chan os.Signal, 1)

	// Simulate a signal that arrived but wasn't consumed
	pauseCh <- syscall.SIGUSR1

	// Cleanup code from workflow_executor.go
	signal.Stop(pauseCh)
	select {
	case <-pauseCh:
		// Drained successfully
	default:
		// No signal to drain - also fine
	}

	// Channel should be empty and safe to close
	close(pauseCh)
}

// TestMultipleSIGUSR1Handling verifies that only the first SIGUSR1 triggers the cancel,
// and subsequent signals don't cause issues.
func TestMultipleSIGUSR1Handling(t *testing.T) {
	// Skip in parallel mode since signal handling is process-global
	// t.Parallel() - intentionally not parallel

	pauseCh := make(chan os.Signal, 1)
	signal.Notify(pauseCh, syscall.SIGUSR1)
	defer func() {
		signal.Stop(pauseCh)
		select {
		case <-pauseCh:
		default:
		}
	}()

	execCtx, execCancel := context.WithCancel(context.Background())
	defer execCancel()

	cancelCount := 0
	signalHandled := make(chan struct{})

	go func() {
		select {
		case <-pauseCh:
			cancelCount++
			close(signalHandled)
			execCancel()
		case <-execCtx.Done():
			return
		}
	}()

	// Send first SIGUSR1
	proc, _ := os.FindProcess(os.Getpid())
	if err := proc.Signal(syscall.SIGUSR1); err != nil {
		t.Fatalf("send first SIGUSR1: %v", err)
	}

	// Wait for first signal to be handled
	select {
	case <-signalHandled:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first SIGUSR1")
	}

	// Send second SIGUSR1 (should not cause panic or issues)
	// Note: The signal handler goroutine has already exited, so this goes to the channel
	// but is cleaned up by the defer
	// Send signal - may fail on some systems if the signal disposition changed.
	// That's OK - we just want to verify no panic occurs.
	_ = proc.Signal(syscall.SIGUSR1)

	// Small delay to ensure no panic
	time.Sleep(50 * time.Millisecond)

	if cancelCount != 1 {
		t.Errorf("expected 1 cancel, got %d", cancelCount)
	}
}

// TestContextCancellationPropagation verifies that when the execution context
// is cancelled (via SIGUSR1), it propagates correctly to child operations.
func TestContextCancellationPropagation(t *testing.T) {
	// Skip in parallel mode since signal handling is process-global
	// t.Parallel() - intentionally not parallel

	parentCtx, parentCancel := context.WithCancel(context.Background())
	defer parentCancel()

	execCtx, execCancel := context.WithCancel(parentCtx)
	defer execCancel()

	// Create a child context (like what phases would use)
	phaseCtx, phaseCancel := context.WithCancel(execCtx)
	defer phaseCancel()

	// Cancel the execution context (simulating SIGUSR1)
	execCancel()

	// Both exec and phase contexts should be done
	select {
	case <-execCtx.Done():
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Error("execCtx should be done")
	}

	select {
	case <-phaseCtx.Done():
		// Good - cancellation propagated
	case <-time.After(100 * time.Millisecond):
		t.Error("phaseCtx should be done (cancellation should propagate)")
	}
}
