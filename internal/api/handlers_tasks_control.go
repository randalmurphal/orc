// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"fmt"
	"net/http"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
)

// truncate truncates a string for logging purposes.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// handleRunTask starts task execution using WorkflowExecutor.
func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	description := task.GetDescriptionProto(t)
	s.logger.Debug("handleRunTask: task loaded",
		"task_id", t.Id,
		"title", t.Title,
		"description_len", len(description),
		"description_preview", truncate(description, 100),
	)

	if !task.CanRunProto(t) {
		s.jsonError(w, fmt.Sprintf("task cannot run in status: %s", t.Status), http.StatusBadRequest)
		return
	}

	// Check for incomplete blockers
	if len(t.BlockedBy) > 0 {
		force := r.URL.Query().Get("force") == "true"
		if !force {
			allTasks, err := s.backend.LoadAllTasksProto()
			if err != nil {
				s.logger.Warn("failed to load tasks for dependency check", "error", err)
			} else {
				taskMap := make(map[string]*orcv1.Task)
				for _, tsk := range allTasks {
					taskMap[tsk.Id] = tsk
				}
				blockers := task.GetIncompleteBlockersProto(t, taskMap)
				if len(blockers) > 0 {
					s.jsonResponseWithStatus(w, http.StatusConflict, map[string]any{
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

	// Get workflow ID from task - MUST be set
	workflowID := task.GetWorkflowIDProto(t)
	if workflowID == "" {
		s.jsonError(w, fmt.Sprintf("task %s has no workflow_id set - cannot run", id), http.StatusBadRequest)
		return
	}

	// Get first phase of workflow to set current_phase
	phases, err := s.backend.GetWorkflowPhases(workflowID)
	if err == nil && len(phases) > 0 {
		task.SetCurrentPhaseProto(t, phases[0].PhaseTemplateID)
	}

	// Update task status to running BEFORE spawning executor
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := s.backend.SaveTaskProto(t); err != nil {
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

		// Create git operations
		gitOps, err := git.New(s.workDir, git.DefaultConfig())
		if err != nil {
			s.logger.Error("failed to create git ops", "error", err)
			s.Publish(id, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
			s.ensureTaskStatusConsistent(id, err)
			return
		}

		// Create WorkflowExecutor
		we := executor.NewWorkflowExecutor(
			s.backend,
			s.projectDB,
			s.orcConfig,
			s.workDir,
			executor.WithWorkflowGitOps(gitOps),
			executor.WithWorkflowPublisher(s.publisher),
			executor.WithWorkflowLogger(s.logger),
			executor.WithWorkflowAutomationService(s.automationSvc),
		)

		// Build run options
		opts := executor.WorkflowRunOptions{
			ContextType: executor.ContextTask,
			TaskID:      id,
			Prompt:      description,
			Category:    t.Category,
		}

		// Execute workflow
		result, err := we.Run(ctx, workflowID, opts)
		if err != nil {
			s.logger.Error("task execution failed", "task", id, "error", err)
			s.Publish(id, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", id, "run_id", result.RunID)
			s.Publish(id, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}

		s.ensureTaskStatusConsistent(id, err)
	}()

	// Return task with updated status
	s.jsonResponse(w, map[string]any{"status": "started", "task_id": id, "task": t})
}

// handlePauseTask pauses task execution.
func (s *Server) handlePauseTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := s.backend.SaveTaskProto(t); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "paused", "task_id": id})
}

// handleResumeTask resumes task execution.
// Uses the same smart retry logic as CLI and WebSocket handlers.
func (s *Server) handleResumeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Use the shared resumeTask function which handles all the retry logic
	result, err := s.resumeTask(id)
	if err != nil {
		// Return 404 for task not found, 400 for other errors
		if err.Error() == "task not found" {
			s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
			return
		}
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, result)
}

// handleSkipBlock clears the blocked_by dependencies for a task.
// This allows a blocked task to become ready for execution.
func (s *Server) handleSkipBlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.backend.LoadTaskProto(id)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(id))
		return
	}

	// Store the blockers we're clearing for the response
	clearedBlockers := t.BlockedBy

	// Clear blockers
	t.BlockedBy = nil
	t.IsBlocked = false
	t.UnmetBlockers = nil

	// If task was in blocked status, reset to planned so it can be run
	if t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	}

	if err := s.backend.SaveTaskProto(t); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]any{
		"status":           "success",
		"task_id":          id,
		"message":          "Block skipped successfully",
		"cleared_blockers": clearedBlockers,
	})
}


