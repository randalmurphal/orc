package store

import (
	"context"
	"errors"
	"testing"
)

// SC-9: Graph store provides node CRUD against Neo4j.
func TestGraphStore_CreateNode(t *testing.T) {
	mock := &mockNeo4jDriver{}
	store := NewGraphStore(WithNeo4jDriver(mock))

	node := Node{
		Labels:     []string{"File"},
		Properties: map[string]interface{}{"path": "/main.go", "language": "go"},
	}

	id, err := store.CreateNode(context.Background(), node)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	if id == "" {
		t.Error("CreateNode should return a non-empty node ID")
	}

	// Verify the driver received correct call
	if mock.createNodeCalls != 1 {
		t.Errorf("driver create calls = %d, want 1", mock.createNodeCalls)
	}
	if mock.lastLabels[0] != "File" {
		t.Errorf("driver labels = %v, want [File]", mock.lastLabels)
	}
}

// SC-9: Graph store queries nodes.
func TestGraphStore_QueryNodes(t *testing.T) {
	mock := &mockNeo4jDriver{
		queryResult: []Node{
			{ID: "1", Labels: []string{"File"}, Properties: map[string]interface{}{"path": "/main.go"}},
			{ID: "2", Labels: []string{"File"}, Properties: map[string]interface{}{"path": "/util.go"}},
		},
	}
	store := NewGraphStore(WithNeo4jDriver(mock))

	nodes, err := store.QueryNodes(context.Background(), "File", map[string]interface{}{"language": "go"})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}

	if len(nodes) != 2 {
		t.Errorf("QueryNodes returned %d nodes, want 2", len(nodes))
	}
}

// SC-9: Graph store creates relationships.
func TestGraphStore_CreateRelationship(t *testing.T) {
	mock := &mockNeo4jDriver{}
	store := NewGraphStore(WithNeo4jDriver(mock))

	err := store.CreateRelationship(context.Background(), "node-1", "node-2", "IMPORTS", nil)
	if err != nil {
		t.Fatalf("CreateRelationship: %v", err)
	}

	if mock.createRelCalls != 1 {
		t.Errorf("driver create relationship calls = %d, want 1", mock.createRelCalls)
	}
	if mock.lastRelType != "IMPORTS" {
		t.Errorf("relationship type = %s, want IMPORTS", mock.lastRelType)
	}
}

// SC-9: Graph store executes Cypher queries.
func TestGraphStore_ExecuteCypher(t *testing.T) {
	mock := &mockNeo4jDriver{
		cypherResult: []map[string]interface{}{
			{"count": 42},
		},
	}
	store := NewGraphStore(WithNeo4jDriver(mock))

	results, err := store.ExecuteCypher(context.Background(), "MATCH (n) RETURN count(n) as count", nil)
	if err != nil {
		t.Fatalf("ExecuteCypher: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("ExecuteCypher returned %d results, want 1", len(results))
	}
	if results[0]["count"] != 42 {
		t.Errorf("count = %v, want 42", results[0]["count"])
	}
}

// SC-9 error path: Connection refused wraps original error.
func TestGraphStore_ConnectionRefused(t *testing.T) {
	mock := &mockNeo4jDriver{
		connectErr: errors.New("connection refused"),
	}
	store := NewGraphStore(WithNeo4jDriver(mock))

	err := store.Connect(context.Background())
	if err == nil {
		t.Fatal("Connect should return error when connection refused")
	}

	if !errors.Is(err, mock.connectErr) {
		t.Errorf("error should wrap original: %v", err)
	}
}

// SC-9 error path: Query syntax error includes query context.
func TestGraphStore_CypherSyntaxError(t *testing.T) {
	mock := &mockNeo4jDriver{
		cypherErr: errors.New("syntax error at position 5"),
	}
	store := NewGraphStore(WithNeo4jDriver(mock))

	_, err := store.ExecuteCypher(context.Background(), "INVALID CYPHER", nil)
	if err == nil {
		t.Fatal("ExecuteCypher should return error on syntax error")
	}
}

// --- Types and stubs ---

// Node represents a graph node.
type Node struct {
	ID         string
	Labels     []string
	Properties map[string]interface{}
}

// GraphStore provides graph database operations.
type GraphStore struct{}

// GraphStoreOption configures a GraphStore.
type GraphStoreOption func(*GraphStore)

// NewGraphStore creates a new graph store.
func NewGraphStore(opts ...GraphStoreOption) *GraphStore {
	return nil
}

// WithNeo4jDriver sets a custom Neo4j driver (for testing).
func WithNeo4jDriver(driver *mockNeo4jDriver) GraphStoreOption {
	return func(s *GraphStore) {}
}

// Connect establishes connection to Neo4j.
func (s *GraphStore) Connect(ctx context.Context) error {
	return errors.New("not implemented")
}

// Close closes the connection.
func (s *GraphStore) Close() error {
	return errors.New("not implemented")
}

// CreateNode creates a node and returns its ID.
func (s *GraphStore) CreateNode(ctx context.Context, node Node) (string, error) {
	return "", errors.New("not implemented")
}

// QueryNodes queries nodes by label and properties.
func (s *GraphStore) QueryNodes(ctx context.Context, label string, props map[string]interface{}) ([]Node, error) {
	return nil, errors.New("not implemented")
}

// CreateRelationship creates a relationship between two nodes.
func (s *GraphStore) CreateRelationship(ctx context.Context, fromID, toID, relType string, props map[string]interface{}) error {
	return errors.New("not implemented")
}

// ExecuteCypher executes a raw Cypher query.
func (s *GraphStore) ExecuteCypher(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

type mockNeo4jDriver struct {
	createNodeCalls int
	createRelCalls  int
	lastLabels      []string
	lastRelType     string
	queryResult     []Node
	cypherResult    []map[string]interface{}
	connectErr      error
	cypherErr       error
}
