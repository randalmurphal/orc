// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// truncate truncates a string for logging purposes.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// handleRunTask starts task execution.
func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Debug: log task fields to trace description injection
	s.logger.Debug("handleRunTask: task loaded",
		"task_id", t.ID,
		"title", t.Title,
		"description_len", len(t.Description),
		"description_preview", truncate(t.Description, 100),
	)

	if !t.CanRun() {
		s.jsonError(w, fmt.Sprintf("task cannot run in status: %s", t.Status), http.StatusBadRequest)
		return
	}

	// Check for incomplete blockers
	if len(t.BlockedBy) > 0 {
		// Check if force=true query param is set
		force := r.URL.Query().Get("force") == "true"

		if !force {
			// Load all tasks to check blocker status
			tasksDir := filepath.Join(s.workDir, task.OrcDir, task.TasksDir)
			allTasks, err := task.LoadAllFrom(tasksDir)
			if err != nil {
				s.logger.Warn("failed to load tasks for dependency check", "error", err)
				// Continue anyway - don't block on dependency check failure
			} else {
				// Build task map
				taskMap := make(map[string]*task.Task)
				for _, tsk := range allTasks {
					taskMap[tsk.ID] = tsk
				}

				// Get incomplete blockers
				blockers := t.GetIncompleteBlockers(taskMap)
				if len(blockers) > 0 {
					// Return 409 Conflict with blocker details
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusConflict)
					json.NewEncoder(w).Encode(map[string]any{
						"error":           "task_blocked",
						"message":         "Task is blocked by incomplete dependencies",
						"blocked_by":      blockers,
						"force_available": true,
					})
					return
				}
			}
		}
	}

	// Load plan and state
	p, err := plan.LoadFrom(s.workDir, id)
	if err != nil {
		s.jsonError(w, "plan not found", http.StatusNotFound)
		return
	}

	st, err := state.LoadFrom(s.workDir, id)
	if err != nil {
		// Create new state if it doesn't exist
		st = state.New(id)
	}

	// Update task status and phase to running BEFORE spawning executor.
	// This ensures:
	// 1. The UI sees the correct status immediately when it reloads
	// 2. The file watcher broadcasts task_updated (not task_deleted)
	// 3. No race condition where the task appears deleted during executor startup
	// 4. Task shows in the correct column based on current_phase (not stuck in Queued)
	t.Status = task.StatusRunning
	if len(p.Phases) > 0 {
		t.CurrentPhase = p.Phases[0].ID
	}
	if err := t.SaveTo(task.TaskDirIn(s.workDir, id)); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function for later cancellation
	s.runningTasksMu.Lock()
	s.runningTasks[id] = cancel
	s.runningTasksMu.Unlock()

	// Start execution in background goroutine
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, id)
			s.runningTasksMu.Unlock()
		}()

		execCfg := executor.ConfigFromOrc(s.orcConfig)
		execCfg.WorkDir = s.workDir
		exec := executor.NewWithConfig(execCfg, s.orcConfig)
		exec.SetPublisher(s.publisher)

		// Execute with event publishing
		err := exec.ExecuteTask(ctx, t, p, st)
		if err != nil {
			s.logger.Error("task execution failed", "task", id, "error", err)
			s.Publish(id, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", id)
			s.Publish(id, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}

		// Reload and publish final state
		if finalState, err := state.LoadFrom(s.workDir, id); err == nil {
			s.Publish(id, Event{Type: "state", Data: finalState})
		}
	}()

	// Return task with updated status so frontend can update store immediately
	s.jsonResponse(w, map[string]any{"status": "started", "task_id": id, "task": t})
}

// handlePauseTask pauses task execution.
func (s *Server) handlePauseTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	t.Status = task.StatusPaused
	if err := t.SaveTo(task.TaskDirIn(s.workDir, id)); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "paused", "task_id": id})
}

// handleResumeTask resumes task execution.
func (s *Server) handleResumeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.LoadFrom(s.workDir, id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	t.Status = task.StatusRunning
	if err := t.SaveTo(task.TaskDirIn(s.workDir, id)); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "resumed", "task_id": id})
}

// handleStream handles SSE streaming for a task.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify task exists
	if !task.ExistsIn(s.workDir, id) {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create subscriber channel
	ch := make(chan Event, 100)

	s.subscribersMu.Lock()
	s.subscribers[id] = append(s.subscribers[id], ch)
	s.subscribersMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		s.subscribersMu.Lock()
		subs := s.subscribers[id]
		for i, sub := range subs {
			if sub == ch {
				s.subscribers[id] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		s.subscribersMu.Unlock()
		close(ch)
	}()

	// Send initial state
	if st, err := state.LoadFrom(s.workDir, id); err == nil {
		data, _ := json.Marshal(st)
		fmt.Fprintf(w, "event: state\ndata: %s\n\n", data)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}
