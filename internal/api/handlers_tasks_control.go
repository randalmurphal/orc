// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// handleRunTask starts task execution.
func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	if !t.CanRun() {
		s.jsonError(w, fmt.Sprintf("task cannot run in status: %s", t.Status), http.StatusBadRequest)
		return
	}

	// Load plan and state
	p, err := plan.Load(id)
	if err != nil {
		s.jsonError(w, "plan not found", http.StatusNotFound)
		return
	}

	st, err := state.Load(id)
	if err != nil {
		// Create new state if it doesn't exist
		st = state.New(id)
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

		exec := executor.New(executor.DefaultConfig())
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
		if finalState, err := state.Load(id); err == nil {
			s.Publish(id, Event{Type: "state", Data: finalState})
		}
	}()

	s.jsonResponse(w, map[string]string{"status": "started", "task_id": id})
}

// handlePauseTask pauses task execution.
func (s *Server) handlePauseTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	t.Status = task.StatusPaused
	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "paused", "task_id": id})
}

// handleResumeTask resumes task execution.
func (s *Server) handleResumeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	t.Status = task.StatusRunning
	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "resumed", "task_id": id})
}

// handleStream handles SSE streaming for a task.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify task exists
	if !task.Exists(id) {
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
	if st, err := state.Load(id); err == nil {
		data, _ := json.Marshal(st)
		fmt.Fprintf(w, "event: state\ndata: %s\n\n", data)
		w.(http.Flusher).Flush()
	}

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			w.(http.Flusher).Flush()
		}
	}
}
