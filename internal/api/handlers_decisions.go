// Package api provides the REST API and SSE server for orc.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// PendingDecisionItem represents a pending decision in the list response.
type PendingDecisionItem struct {
	DecisionID  string    `json:"decision_id"`
	TaskID      string    `json:"task_id"`
	TaskTitle   string    `json:"task_title"`
	Phase       string    `json:"phase"`
	GateType    string    `json:"gate_type"`
	Question    string    `json:"question"`
	Context     string    `json:"context,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
}

// handleListDecisions handles GET /api/decisions
func (s *Server) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	decisions := s.pendingDecisions.List()

	items := make([]PendingDecisionItem, len(decisions))
	for i, d := range decisions {
		items[i] = PendingDecisionItem{
			DecisionID:  d.DecisionID,
			TaskID:      d.TaskID,
			TaskTitle:   d.TaskTitle,
			Phase:       d.Phase,
			GateType:    d.GateType,
			Question:    d.Question,
			Context:     d.Context,
			RequestedAt: d.RequestedAt,
		}
	}

	s.jsonResponse(w, items)
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
	t, err := s.backend.LoadTaskProto(decision.TaskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Verify task is blocked
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		s.jsonError(w, fmt.Sprintf("task is not blocked (status: %s)", t.Status), http.StatusBadRequest)
		return
	}

	// Verify phase matches current task phase to prevent stale decisions
	currentPhase := task.GetCurrentPhaseProto(t)
	if currentPhase != decision.Phase {
		s.jsonError(w, fmt.Sprintf("decision phase mismatch: task is at phase %q, decision is for phase %q", currentPhase, decision.Phase), http.StatusConflict)
		return
	}

	// Record gate decision in task execution state
	task.EnsureExecutionProto(t)
	gateDecision := &orcv1.GateDecision{
		Phase:     decision.Phase,
		GateType:  decision.GateType,
		Approved:  req.Approved,
		Timestamp: timestamppb.Now(),
	}
	if req.Reason != "" {
		gateDecision.Reason = &req.Reason
	}
	t.Execution.Gates = append(t.Execution.Gates, gateDecision)

	// Save task
	if err := s.backend.SaveTaskProto(t); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save task: %v", err), http.StatusInternalServerError)
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
	var newStatus orcv1.TaskStatus
	if req.Approved {
		newStatus = orcv1.TaskStatus_TASK_STATUS_PLANNED
	} else {
		newStatus = orcv1.TaskStatus_TASK_STATUS_FAILED
	}

	t.Status = newStatus
	if err := s.backend.SaveTaskProto(t); err != nil {
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
		NewStatus:  task.StatusFromProto(newStatus),
	})
}
