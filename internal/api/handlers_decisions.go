// Package api provides the REST API and SSE server for orc.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// DecisionRequest represents the request body for decision resolution.
type DecisionRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
}

// DecisionResponse represents the response for decision resolution.
type DecisionResponse struct {
	DecisionID string `json:"decision_id"`
	TaskID     string `json:"task_id"`
	Approved   bool   `json:"approved"`
	NewStatus  string `json:"new_status"`
}

// handlePostDecision handles POST /api/decisions/{id}
func (s *Server) handlePostDecision(w http.ResponseWriter, r *http.Request) {
	decisionID := r.PathValue("id")

	// Parse request body
	var req DecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Get pending decision
	decision, ok := s.pendingDecisions.Get(decisionID)
	if !ok {
		s.jsonError(w, fmt.Sprintf("decision not found: %s", decisionID), http.StatusNotFound)
		return
	}

	// Load task
	t, err := s.backend.LoadTask(decision.TaskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Verify task is blocked
	if t.Status != task.StatusBlocked {
		s.jsonError(w, fmt.Sprintf("task is not blocked (status: %s)", t.Status), http.StatusBadRequest)
		return
	}

	// Load state to record decision
	st, err := s.backend.LoadState(decision.TaskID)
	if err != nil {
		s.jsonError(w, "state not found", http.StatusNotFound)
		return
	}

	// Record gate decision in state
	st.RecordGateDecision(decision.Phase, decision.GateType, req.Approved, req.Reason)

	// Save state
	if err := s.backend.SaveState(st); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save state: %v", err), http.StatusInternalServerError)
		return
	}

	// Record in database if available
	if dbBackend, ok := s.backend.(*storage.DatabaseBackend); ok {
		dbDecision := &db.GateDecision{
			TaskID:   decision.TaskID,
			Phase:    decision.Phase,
			GateType: decision.GateType,
			Approved: req.Approved,
			Reason:   req.Reason,
		}
		if err := dbBackend.DB().AddGateDecision(dbDecision); err != nil {
			s.logger.Warn("failed to record gate decision in database", "error", err)
			// Don't fail the request - database recording is optional
		}
	}

	// Update task status based on approval
	var newStatus task.Status
	if req.Approved {
		newStatus = task.StatusPlanned
	} else {
		newStatus = task.StatusFailed
	}

	t.Status = newStatus
	if err := s.backend.SaveTask(t); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save task: %v", err), http.StatusInternalServerError)
		return
	}

	// Emit decision_resolved event
	resolvedData := events.DecisionResolvedData{
		DecisionID: decisionID,
		TaskID:     decision.TaskID,
		Phase:      decision.Phase,
		Approved:   req.Approved,
		Reason:     req.Reason,
		ResolvedBy: "api",
		ResolvedAt: time.Now(),
	}

	s.publisher.Publish(events.Event{
		Type:   events.EventDecisionResolved,
		TaskID: decision.TaskID,
		Data:   resolvedData,
		Time:   time.Now(),
	})

	// Remove decision from store
	s.pendingDecisions.Remove(decisionID)

	// Return success response
	s.jsonResponse(w, DecisionResponse{
		DecisionID: decisionID,
		TaskID:     decision.TaskID,
		Approved:   req.Approved,
		NewStatus:  string(newStatus),
	})
}
