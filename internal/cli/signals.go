// Package cli provides the command-line interface for orc.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// SetupSignalHandler returns a context that is cancelled on SIGINT/SIGTERM
func SetupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\n⚠️  Received %s, saving state and exiting gracefully...\n", sig)
		cancel()

		// Second signal forces immediate exit
		sig = <-sigChan
		fmt.Printf("\n🛑 Received %s again, forcing exit\n", sig)
		os.Exit(1)
	}()

	return ctx, cancel
}

// GracefulShutdown saves current state before exit
func GracefulShutdown(backend storage.Backend, t *task.Task, s *state.State, phase string) error {
	// Mark phase as interrupted (not failed - can be resumed)
	s.InterruptPhase(phase)

	if err := backend.SaveState(s); err != nil {
		return fmt.Errorf("save state on interrupt: %w", err)
	}

	// Update task status to interrupted so it can be resumed
	t.Status = task.StatusBlocked
	if err := backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task on interrupt: %w", err)
	}

	fmt.Printf("✅ State saved. Resume with: orc resume %s\n", t.ID)
	return nil
}

// InterruptHandler manages interrupt signals during task execution
type InterruptHandler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	backend   storage.Backend
	task      *task.Task
	state     *state.State
	lastPhase string
}

// NewInterruptHandler creates a new interrupt handler
func NewInterruptHandler(backend storage.Backend, t *task.Task, s *state.State) *InterruptHandler {
	ctx, cancel := SetupSignalHandler()
	return &InterruptHandler{
		ctx:     ctx,
		cancel:  cancel,
		backend: backend,
		task:    t,
		state:   s,
	}
}

// Context returns the cancellable context
func (h *InterruptHandler) Context() context.Context {
	return h.ctx
}

// SetCurrentPhase updates the current phase for state saving
func (h *InterruptHandler) SetCurrentPhase(phase string) {
	h.lastPhase = phase
}

// Cleanup saves state if interrupted
func (h *InterruptHandler) Cleanup() {
	if h.ctx.Err() != nil && h.lastPhase != "" {
		_ = GracefulShutdown(h.backend, h.task, h.state, h.lastPhase)
	}
	h.cancel()
}

// WasInterrupted returns true if the context was cancelled
func (h *InterruptHandler) WasInterrupted() bool {
	return h.ctx.Err() != nil
}

// WaitWithTimeout waits for a duration while respecting interrupts
func WaitWithTimeout(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
