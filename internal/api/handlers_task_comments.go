package api

import (
	"encoding/json"
	"net/http"

	"github.com/randalmurphal/orc/internal/db"
)

// createTaskCommentRequest is the request body for creating a task comment.
type createTaskCommentRequest struct {
	Author     string `json:"author"`
	AuthorType string `json:"author_type"`
	Content    string `json:"content"`
	Phase      string `json:"phase"`
}

// updateTaskCommentRequest is the request body for updating a task comment.
type updateTaskCommentRequest struct {
	Content string `json:"content"`
	Phase   string `json:"phase"`
}

// handleListTaskComments returns all comments for a task.
func (s *Server) handleListTaskComments(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
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

	// Filter by author_type if provided
	authorType := r.URL.Query().Get("author_type")
	phase := r.URL.Query().Get("phase")

	var comments []db.TaskComment

	if authorType != "" {
		comments, err = pdb.ListTaskCommentsByAuthorType(taskID, db.AuthorType(authorType))
	} else if phase != "" {
		comments, err = pdb.ListTaskCommentsByPhase(taskID, phase)
	} else {
		comments, err = pdb.ListTaskComments(taskID)
	}

	if err != nil {
		s.jsonError(w, "failed to list task comments: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure we return empty array, not null
	if comments == nil {
		comments = []db.TaskComment{}
	}

	s.jsonResponse(w, comments)
}

// handleCreateTaskComment creates a new task comment.
func (s *Server) handleCreateTaskComment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task_id required", http.StatusBadRequest)
		return
	}

	var req createTaskCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		s.jsonError(w, "content is required", http.StatusBadRequest)
		return
	}

	// Validate author type
	authorType := db.AuthorType(req.AuthorType)
	if authorType == "" {
		authorType = db.AuthorTypeHuman
	} else if authorType != db.AuthorTypeHuman && authorType != db.AuthorTypeAgent && authorType != db.AuthorTypeSystem {
		s.jsonError(w, "invalid author_type: must be human, agent, or system", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	// Ensure task exists in database for foreign key constraint
	if err := s.syncTaskToDB(pdb, taskID); err != nil {
		s.jsonError(w, "failed to sync task: "+err.Error(), http.StatusInternalServerError)
		return
	}

	comment := &db.TaskComment{
		TaskID:     taskID,
		Author:     req.Author,
		AuthorType: authorType,
		Content:    req.Content,
		Phase:      req.Phase,
	}

	if err := pdb.CreateTaskComment(comment); err != nil {
		s.jsonError(w, "failed to create task comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, comment)
}

// handleGetTaskComment retrieves a single task comment.
func (s *Server) handleGetTaskComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if commentID == "" {
		s.jsonError(w, "comment_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetTaskComment(commentID)
	if err != nil {
		s.jsonError(w, "failed to get task comment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if comment == nil {
		s.jsonError(w, "comment not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, comment)
}

// handleUpdateTaskComment updates a task comment.
func (s *Server) handleUpdateTaskComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if commentID == "" {
		s.jsonError(w, "comment_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	comment, err := pdb.GetTaskComment(commentID)
	if err != nil {
		s.jsonError(w, "failed to get task comment: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if comment == nil {
		s.jsonError(w, "comment not found", http.StatusNotFound)
		return
	}

	var req updateTaskCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content != "" {
		comment.Content = req.Content
	}
	// Allow clearing phase by setting it explicitly
	comment.Phase = req.Phase

	if err := pdb.UpdateTaskComment(comment); err != nil {
		s.jsonError(w, "failed to update task comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, comment)
}

// handleDeleteTaskComment removes a task comment.
func (s *Server) handleDeleteTaskComment(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if commentID == "" {
		s.jsonError(w, "comment_id required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.DeleteTaskComment(commentID); err != nil {
		s.jsonError(w, "failed to delete task comment: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetTaskCommentStats returns statistics about comments for a task.
func (s *Server) handleGetTaskCommentStats(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
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

	stats, err := pdb.GetTaskCommentStats(taskID)
	if err != nil {
		s.jsonError(w, "failed to get comment stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"task_id":        taskID,
		"total_comments": stats.Total,
		"human_count":    stats.HumanCount,
		"agent_count":    stats.AgentCount,
		"system_count":   stats.SystemCount,
	}

	s.jsonResponse(w, response)
}
