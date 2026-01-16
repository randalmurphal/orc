package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// subtaskRequest is the request body for creating a subtask.
type subtaskRequest struct {
	ParentTaskID string `json:"parent_task_id"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	ProposedBy   string `json:"proposed_by,omitempty"`
}

// subtaskApproveRequest is the request body for approving a subtask.
type subtaskApproveRequest struct {
	ApprovedBy string `json:"approved_by,omitempty"`
}

// subtaskRejectRequest is the request body for rejecting a subtask.
type subtaskRejectRequest struct {
	Reason string `json:"reason,omitempty"`
}

// handleListSubtasks returns all subtasks for a parent task.
func (s *Server) handleListSubtasks(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	subtasks, err := pdb.ListAllSubtasks(taskID)
	if err != nil {
		s.jsonError(w, "failed to list subtasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if subtasks == nil {
		subtasks = []*db.Subtask{}
	}

	s.jsonResponse(w, subtasks)
}

// handleListPendingSubtasks returns only pending subtasks for a parent task.
func (s *Server) handleListPendingSubtasks(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	subtasks, err := pdb.ListPendingSubtasks(taskID)
	if err != nil {
		s.jsonError(w, "failed to list pending subtasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if subtasks == nil {
		subtasks = []*db.Subtask{}
	}

	s.jsonResponse(w, subtasks)
}

// handleCreateSubtask creates a new subtask in the queue.
func (s *Server) handleCreateSubtask(w http.ResponseWriter, r *http.Request) {
	var req subtaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ParentTaskID == "" {
		s.jsonError(w, "parent_task_id required", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		s.jsonError(w, "title required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Check pending count against config limit
	cfg, _ := config.Load()
	if cfg != nil && cfg.Subtasks.MaxPending > 0 {
		count, err := pdb.CountPendingSubtasks(req.ParentTaskID)
		if err != nil {
			s.jsonError(w, "failed to count pending subtasks: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if count >= cfg.Subtasks.MaxPending {
			s.jsonError(w, "max pending subtasks reached", http.StatusConflict)
			return
		}
	}

	subtask, err := pdb.QueueSubtask(req.ParentTaskID, req.Title, req.Description, req.ProposedBy)
	if err != nil {
		s.jsonError(w, "failed to create subtask: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, subtask)
}

// handleGetSubtask returns a specific subtask.
func (s *Server) handleGetSubtask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	subtask, err := pdb.GetSubtask(id)
	if err != nil {
		s.jsonError(w, "failed to get subtask: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if subtask == nil {
		s.jsonError(w, "subtask not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, subtask)
}

// handleApproveSubtask approves a pending subtask.
func (s *Server) handleApproveSubtask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req subtaskApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body is optional, continue with empty approvedBy
		req.ApprovedBy = ""
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	subtask, err := pdb.ApproveSubtask(id, req.ApprovedBy)
	if err != nil {
		s.jsonError(w, "failed to approve subtask: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, subtask)
}

// handleRejectSubtask rejects a pending subtask.
func (s *Server) handleRejectSubtask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req subtaskRejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Body is optional
		req.Reason = ""
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.RejectSubtask(id, req.Reason); err != nil {
		s.jsonError(w, "failed to reject subtask: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteSubtask deletes a subtask from the queue.
func (s *Server) handleDeleteSubtask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.DeleteSubtask(id); err != nil {
		s.jsonError(w, "failed to delete subtask: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
