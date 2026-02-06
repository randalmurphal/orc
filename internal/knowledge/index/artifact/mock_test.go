package artifact

import (
	"context"
	"fmt"
	"sync"

	"github.com/randalmurphal/orc/internal/knowledge/index"
	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// Verify interface compliance.
var _ index.GraphStorer = (*mockGraphStore)(nil)

// mockGraphStore implements index.GraphStorer as an in-memory fake
// that tracks all operations for test assertions.
type mockGraphStore struct {
	mu sync.Mutex

	// State
	nodes  []store.Node
	rels   []relRecord
	nextID int

	// Cypher tracking
	cypherCalls []cypherCall

	// Delete tracking
	deleteCalls []deleteCall

	// Pre-configured query results (for simulating existing data)
	queryResultsByLabel map[string][]store.Node

	// Error injection (global)
	createNodeErr error
	createRelErr  error
	queryErr      error
	cypherErr     error
	deleteErr     error

	// Error injection (per-label for CreateNode)
	createNodeErrForLabel map[string]error
}

type relRecord struct {
	fromID  string
	toID    string
	relType string
	props   map[string]interface{}
}

type cypherCall struct {
	query  string
	params map[string]interface{}
}

type deleteCall struct {
	label string
	prop  string
	value string
}

func newMockGraphStore() *mockGraphStore {
	return &mockGraphStore{
		queryResultsByLabel:   make(map[string][]store.Node),
		createNodeErrForLabel: make(map[string]error),
	}
}

func (m *mockGraphStore) CreateNode(_ context.Context, node store.Node) (string, error) {
	if m.createNodeErr != nil {
		return "", m.createNodeErr
	}
	// Check label-specific errors.
	for _, label := range node.Labels {
		if err, ok := m.createNodeErrForLabel[label]; ok {
			return "", err
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("node-%d", m.nextID)
	node.ID = id
	m.nodes = append(m.nodes, node)
	return id, nil
}

func (m *mockGraphStore) QueryNodes(_ context.Context, label string, props map[string]interface{}) ([]store.Node, error) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check pre-configured results first.
	if results, ok := m.queryResultsByLabel[label]; ok {
		return results, nil
	}

	// Search created nodes.
	var matches []store.Node
	for _, n := range m.nodes {
		if hasLabel(n, label) && matchesProps(n, props) {
			matches = append(matches, n)
		}
	}
	return matches, nil
}

func (m *mockGraphStore) CreateRelationship(_ context.Context, fromID, toID, relType string, props map[string]interface{}) error {
	if m.createRelErr != nil {
		return m.createRelErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rels = append(m.rels, relRecord{fromID, toID, relType, props})
	return nil
}

func (m *mockGraphStore) ExecuteCypher(_ context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	if m.cypherErr != nil {
		return nil, m.cypherErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cypherCalls = append(m.cypherCalls, cypherCall{query, params})
	return nil, nil
}

func (m *mockGraphStore) DeleteNodesByProperty(_ context.Context, label, prop, value string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls = append(m.deleteCalls, deleteCall{label, prop, value})

	// Actually remove matching nodes so subsequent queries reflect deletion.
	filtered := m.nodes[:0]
	for _, n := range m.nodes {
		if hasLabel(n, label) {
			if pv, ok := n.Properties[prop]; ok && fmt.Sprint(pv) == value {
				continue
			}
		}
		filtered = append(filtered, n)
	}
	m.nodes = filtered
	return nil
}

// --- Test helper functions ---

func hasLabel(n store.Node, label string) bool {
	for _, l := range n.Labels {
		if l == label {
			return true
		}
	}
	return false
}

func matchesProps(n store.Node, props map[string]interface{}) bool {
	for k, v := range props {
		if nv, ok := n.Properties[k]; !ok || nv != v {
			return false
		}
	}
	return true
}

func (m *mockGraphStore) nodesWithLabel(label string) []store.Node {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []store.Node
	for _, n := range m.nodes {
		if hasLabel(n, label) {
			result = append(result, n)
		}
	}
	return result
}

func (m *mockGraphStore) relsOfType(relType string) []relRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []relRecord
	for _, r := range m.rels {
		if r.relType == relType {
			result = append(result, r)
		}
	}
	return result
}


func (m *mockGraphStore) allCypherCalls() []cypherCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]cypherCall, len(m.cypherCalls))
	copy(result, m.cypherCalls)
	return result
}

// Helper: string pointer for proto optional fields.
func strPtr(s string) *string { return &s }

// Helper: int32 pointer for proto optional fields.
func int32Ptr(i int32) *int32 { return &i }
