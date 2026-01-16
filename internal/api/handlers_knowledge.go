package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/randalmurphal/orc/internal/db"
)

// knowledgeRequest is the request body for creating a knowledge entry.
type knowledgeRequest struct {
	Type        db.KnowledgeType `json:"type"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	SourceTask  string           `json:"source_task,omitempty"`
	ProposedBy  string           `json:"proposed_by,omitempty"`
}

// knowledgeApproveRequest is the request body for approving knowledge.
type knowledgeApproveRequest struct {
	ApprovedBy string `json:"approved_by,omitempty"`
}

// knowledgeValidateRequest is the request body for validating knowledge.
type knowledgeValidateRequest struct {
	ValidatedBy string `json:"validated_by,omitempty"`
}

// knowledgeRejectRequest is the request body for rejecting knowledge.
type knowledgeRejectRequest struct {
	Reason string `json:"reason,omitempty"`
}

// knowledgeStatusResponse is the response for knowledge status.
type knowledgeStatusResponse struct {
	PendingCount  int `json:"pending_count"`
	StaleCount    int `json:"stale_count"`
	ApprovedCount int `json:"approved_count"`
}

// handleListKnowledge returns all knowledge entries with optional status filter.
func (s *Server) handleListKnowledge(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	ktype := r.URL.Query().Get("type")

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	var entries []*db.KnowledgeEntry

	if status == "pending" {
		entries, err = pdb.ListPendingKnowledge()
	} else if ktype != "" && status != "" {
		entries, err = pdb.ListKnowledgeByType(db.KnowledgeType(ktype), db.KnowledgeStatus(status))
	} else {
		// Return all by querying each type/status combination
		pending, _ := pdb.ListPendingKnowledge()
		entries = append(entries, pending...)

		for _, kt := range []db.KnowledgeType{db.KnowledgePattern, db.KnowledgeGotcha, db.KnowledgeDecision} {
			approved, _ := pdb.ListKnowledgeByType(kt, db.KnowledgeApproved)
			rejected, _ := pdb.ListKnowledgeByType(kt, db.KnowledgeRejected)
			entries = append(entries, approved...)
			entries = append(entries, rejected...)
		}
	}

	if err != nil {
		s.jsonError(w, "failed to list knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []*db.KnowledgeEntry{}
	}

	s.jsonResponse(w, entries)
}

// handleListStaleKnowledge returns stale knowledge entries.
func (s *Server) handleListStaleKnowledge(w http.ResponseWriter, r *http.Request) {
	stalenessDays := 90 // Default
	if days := r.URL.Query().Get("days"); days != "" {
		if d, err := strconv.Atoi(days); err == nil && d > 0 {
			stalenessDays = d
		}
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	entries, err := pdb.ListStaleKnowledge(stalenessDays)
	if err != nil {
		s.jsonError(w, "failed to list stale knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if entries == nil {
		entries = []*db.KnowledgeEntry{}
	}

	s.jsonResponse(w, entries)
}

// handleGetKnowledgeStatus returns knowledge queue statistics.
func (s *Server) handleGetKnowledgeStatus(w http.ResponseWriter, r *http.Request) {
	stalenessDays := 90 // Default

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	pendingCount, _ := pdb.CountPendingKnowledge()
	staleCount, _ := pdb.CountStaleKnowledge(stalenessDays)

	// Count approved entries
	approvedCount := 0
	for _, kt := range []db.KnowledgeType{db.KnowledgePattern, db.KnowledgeGotcha, db.KnowledgeDecision} {
		entries, _ := pdb.ListKnowledgeByType(kt, db.KnowledgeApproved)
		approvedCount += len(entries)
	}

	s.jsonResponse(w, knowledgeStatusResponse{
		PendingCount:  pendingCount,
		StaleCount:    staleCount,
		ApprovedCount: approvedCount,
	})
}

// handleCreateKnowledge creates a new knowledge entry in the queue.
func (s *Server) handleCreateKnowledge(w http.ResponseWriter, r *http.Request) {
	var req knowledgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type == "" {
		s.jsonError(w, "type required", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		s.jsonError(w, "name required", http.StatusBadRequest)
		return
	}
	if req.Description == "" {
		s.jsonError(w, "description required", http.StatusBadRequest)
		return
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	entry, err := pdb.QueueKnowledge(req.Type, req.Name, req.Description, req.SourceTask, req.ProposedBy)
	if err != nil {
		s.jsonError(w, "failed to create knowledge entry: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, entry)
}

// handleGetKnowledge returns a specific knowledge entry.
func (s *Server) handleGetKnowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	entry, err := pdb.GetKnowledgeEntry(id)
	if err != nil {
		s.jsonError(w, "failed to get knowledge entry: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if entry == nil {
		s.jsonError(w, "knowledge entry not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, entry)
}

// handleApproveKnowledge approves a pending knowledge entry.
func (s *Server) handleApproveKnowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req knowledgeApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ApprovedBy = "web"
	}
	if req.ApprovedBy == "" {
		req.ApprovedBy = "web"
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	entry, err := pdb.ApproveKnowledge(id, req.ApprovedBy)
	if err != nil {
		s.jsonError(w, "failed to approve knowledge: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, entry)
}

// handleApproveAllKnowledge approves all pending knowledge entries.
func (s *Server) handleApproveAllKnowledge(w http.ResponseWriter, r *http.Request) {
	var req knowledgeApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ApprovedBy = "web"
	}
	if req.ApprovedBy == "" {
		req.ApprovedBy = "web"
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	count, err := pdb.ApproveAllPending(req.ApprovedBy)
	if err != nil {
		s.jsonError(w, "failed to approve all knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]int{"approved_count": count})
}

// handleValidateKnowledge validates (confirms still relevant) an approved knowledge entry.
func (s *Server) handleValidateKnowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req knowledgeValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ValidatedBy = "web"
	}
	if req.ValidatedBy == "" {
		req.ValidatedBy = "web"
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	entry, err := pdb.ValidateKnowledge(id, req.ValidatedBy)
	if err != nil {
		s.jsonError(w, "failed to validate knowledge: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.jsonResponse(w, entry)
}

// handleRejectKnowledge rejects a pending knowledge entry.
func (s *Server) handleRejectKnowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req knowledgeRejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = ""
	}

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.RejectKnowledge(id, req.Reason); err != nil {
		s.jsonError(w, "failed to reject knowledge: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteKnowledge deletes a knowledge entry.
func (s *Server) handleDeleteKnowledge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	pdb, err := db.OpenProject(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "failed to open database: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = pdb.Close() }()

	if err := pdb.DeleteKnowledge(id); err != nil {
		s.jsonError(w, "failed to delete knowledge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
