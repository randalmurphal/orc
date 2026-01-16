// Package executor provides the flowgraph-based execution engine for orc.
// This file contains heartbeat functionality for orphan detection.
package executor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/state"
)

// StateSaver is a minimal interface for saving state, used by HeartbeatRunner.
// This allows for easier testing without implementing the full storage.Backend.
type StateSaver interface {
	SaveState(s *state.State) error
}

// DefaultHeartbeatInterval is the default interval for heartbeat updates during phase execution.
// This ensures orphan detection has fresh heartbeat data even during long-running phases.
const DefaultHeartbeatInterval = 2 * time.Minute

// HeartbeatRunner manages periodic heartbeat updates during task execution.
// This ensures that long-running phases don't trigger false positive orphan detection.
type HeartbeatRunner struct {
	saver    StateSaver
	logger   *slog.Logger
	interval time.Duration

	// Protects state access from concurrent reads/writes
	mu    sync.Mutex
	state *state.State

	// Channel to signal stop
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewHeartbeatRunner creates a new heartbeat runner.
// The saver parameter should implement StateSaver (storage.Backend satisfies this).
func NewHeartbeatRunner(saver StateSaver, s *state.State, logger *slog.Logger) *HeartbeatRunner {
	return &HeartbeatRunner{
		saver:    saver,
		state:    s,
		logger:   logger,
		interval: DefaultHeartbeatInterval,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

// Start begins the heartbeat loop in a goroutine.
// The heartbeat will run until Stop() is called or the context is canceled.
func (h *HeartbeatRunner) Start(ctx context.Context) {
	go h.run(ctx)
}

// Stop signals the heartbeat loop to stop and waits for it to finish.
func (h *HeartbeatRunner) Stop() {
	close(h.stopCh)
	<-h.doneCh
}

// run is the main heartbeat loop.
func (h *HeartbeatRunner) run(ctx context.Context) {
	defer close(h.doneCh)

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Debug("heartbeat stopping due to context cancellation")
			return
		case <-h.stopCh:
			h.logger.Debug("heartbeat stopping due to stop signal")
			return
		case <-ticker.C:
			h.updateHeartbeat()
		}
	}
}

// updateHeartbeat updates the heartbeat timestamp and persists it to the database.
func (h *HeartbeatRunner) updateHeartbeat() {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Update the heartbeat timestamp
	h.state.UpdateHeartbeat()

	// Persist to database
	if err := h.saver.SaveState(h.state); err != nil {
		h.logger.Warn("failed to save heartbeat update",
			"error", err,
			"task", h.state.TaskID,
		)
		// Continue running - a single save failure shouldn't stop heartbeats
		return
	}

	h.logger.Debug("heartbeat updated",
		"task", h.state.TaskID,
		"timestamp", h.state.Execution.LastHeartbeat,
	)
}

// UpdateState updates the state reference. This is useful when the state
// is modified outside the heartbeat runner (e.g., phase transitions).
func (h *HeartbeatRunner) UpdateState(s *state.State) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.state = s
}
