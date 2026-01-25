// Package executor provides the flowgraph-based execution engine for orc.
// This file contains heartbeat functionality for orphan detection.
package executor

import (
	"context"
	"log/slog"
	"time"
)

// HeartbeatUpdater is a minimal interface for updating task heartbeats.
type HeartbeatUpdater interface {
	UpdateTaskHeartbeat(taskID string) error
}

// DefaultHeartbeatInterval is the default interval for heartbeat updates during phase execution.
// This ensures orphan detection has fresh heartbeat data even during long-running phases.
const DefaultHeartbeatInterval = 2 * time.Minute

// HeartbeatRunner manages periodic heartbeat updates during task execution.
// This ensures that long-running phases don't trigger false positive orphan detection.
type HeartbeatRunner struct {
	updater  HeartbeatUpdater
	taskID   string
	logger   *slog.Logger
	interval time.Duration

	// Channel to signal stop
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewHeartbeatRunner creates a new heartbeat runner for a task.
// The updater parameter should implement HeartbeatUpdater (storage.Backend satisfies this).
func NewHeartbeatRunner(updater HeartbeatUpdater, taskID string, logger *slog.Logger) *HeartbeatRunner {
	return &HeartbeatRunner{
		updater:  updater,
		taskID:   taskID,
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

// updateHeartbeat updates the heartbeat timestamp in the database.
func (h *HeartbeatRunner) updateHeartbeat() {
	if err := h.updater.UpdateTaskHeartbeat(h.taskID); err != nil {
		h.logger.Warn("failed to update heartbeat",
			"error", err,
			"task", h.taskID,
		)
		// Continue running - a single update failure shouldn't stop heartbeats
		return
	}

	h.logger.Debug("heartbeat updated", "task", h.taskID)
}
