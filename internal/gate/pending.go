// Package gate provides gate evaluation for orc phase transitions.
package gate

import (
	"fmt"
	"sync"
	"time"
)

// PendingDecision represents a gate decision awaiting approval/rejection.
type PendingDecision struct {
	ProjectID   string
	DecisionID  string
	TaskID      string
	TaskTitle   string
	Phase       string
	GateType    string
	Question    string
	Context     string
	Options     []PendingDecisionOption
	RequestedAt time.Time
}

// PendingDecisionOption represents a selectable choice on a pending decision.
type PendingDecisionOption struct {
	ID          string
	Label       string
	Description string
	Recommended bool
}

// PendingDecisionStore manages pending gate decisions.
type PendingDecisionStore struct {
	mu      sync.RWMutex
	pending map[string]*PendingDecision
}

// NewPendingDecisionStore creates a new pending decision store.
func NewPendingDecisionStore() *PendingDecisionStore {
	return &PendingDecisionStore{
		pending: make(map[string]*PendingDecision),
	}
}

// Add stores a pending decision.
func (s *PendingDecisionStore) Add(decision *PendingDecision) error {
	if decision == nil {
		return fmt.Errorf("pending decision is required")
	}
	if decision.DecisionID == "" {
		return fmt.Errorf("pending decision id is required")
	}
	if decision.ProjectID == "" {
		return fmt.Errorf("pending decision project id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[pendingDecisionKey(decision.ProjectID, decision.DecisionID)] = decision
	return nil
}

// Get retrieves a pending decision by ID.
func (s *PendingDecisionStore) Get(projectID string, decisionID string) (*PendingDecision, bool) {
	if projectID == "" || decisionID == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	decision, ok := s.pending[pendingDecisionKey(projectID, decisionID)]
	if !ok {
		return nil, false
	}
	return decision, true
}

// Remove deletes a pending decision.
func (s *PendingDecisionStore) Remove(projectID string, decisionID string) {
	if projectID == "" || decisionID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, pendingDecisionKey(projectID, decisionID))
}

// List returns all pending decisions.
func (s *PendingDecisionStore) List(projectID string) []*PendingDecision {
	if projectID == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	decisions := make([]*PendingDecision, 0, len(s.pending))
	for _, d := range s.pending {
		if projectID != "" && d.ProjectID != projectID {
			continue
		}
		decisions = append(decisions, d)
	}
	return decisions
}

func pendingDecisionKey(projectID string, decisionID string) string {
	return projectID + "::" + decisionID
}
