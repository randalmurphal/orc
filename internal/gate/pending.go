// Package gate provides gate evaluation for orc phase transitions.
package gate

import (
	"sync"
	"time"
)

// PendingDecision represents a gate decision awaiting approval/rejection.
type PendingDecision struct {
	DecisionID  string
	TaskID      string
	TaskTitle   string
	Phase       string
	GateType    string
	Question    string
	Context     string
	RequestedAt time.Time
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
func (s *PendingDecisionStore) Add(decision *PendingDecision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[decision.DecisionID] = decision
}

// Get retrieves a pending decision by ID.
func (s *PendingDecisionStore) Get(decisionID string) (*PendingDecision, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	decision, ok := s.pending[decisionID]
	return decision, ok
}

// Remove deletes a pending decision.
func (s *PendingDecisionStore) Remove(decisionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, decisionID)
}

// List returns all pending decisions.
func (s *PendingDecisionStore) List() []*PendingDecision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	decisions := make([]*PendingDecision, 0, len(s.pending))
	for _, d := range s.pending {
		decisions = append(decisions, d)
	}
	return decisions
}
