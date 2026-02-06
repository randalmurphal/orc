// Package store provides storage backends for the knowledge layer.
package store

import (
	"context"
	"fmt"
)

// Node represents a graph node.
type Node struct {
	ID         string
	Labels     []string
	Properties map[string]interface{}
}

// Neo4jDriver abstracts Neo4j database operations for testing.
type Neo4jDriver interface {
	CreateNode(ctx context.Context, labels []string, props map[string]interface{}) (string, error)
	QueryNodes(ctx context.Context, label string, props map[string]interface{}) ([]Node, error)
	CreateRelationship(ctx context.Context, fromID, toID, relType string, props map[string]interface{}) error
	ExecuteCypher(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error)
	Connect(ctx context.Context) error
	Close() error
}

// GraphStore provides graph database operations.
type GraphStore struct {
	driver Neo4jDriver
}

// GraphStoreOption configures a GraphStore.
type GraphStoreOption func(*GraphStore)

// NewGraphStore creates a new graph store.
func NewGraphStore(opts ...GraphStoreOption) *GraphStore {
	s := &GraphStore{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithNeo4jDriver sets a custom Neo4j driver (for testing).
func WithNeo4jDriver(driver Neo4jDriver) GraphStoreOption {
	return func(s *GraphStore) {
		s.driver = driver
	}
}

// Connect establishes connection to Neo4j.
func (s *GraphStore) Connect(ctx context.Context) error {
	if err := s.driver.Connect(ctx); err != nil {
		return fmt.Errorf("connect to neo4j: %w", err)
	}
	return nil
}

// Close closes the connection.
func (s *GraphStore) Close() error {
	return s.driver.Close()
}

// CreateNode creates a node and returns its ID.
func (s *GraphStore) CreateNode(ctx context.Context, node Node) (string, error) {
	id, err := s.driver.CreateNode(ctx, node.Labels, node.Properties)
	if err != nil {
		return "", fmt.Errorf("create node: %w", err)
	}
	return id, nil
}

// QueryNodes queries nodes by label and properties.
func (s *GraphStore) QueryNodes(ctx context.Context, label string, props map[string]interface{}) ([]Node, error) {
	nodes, err := s.driver.QueryNodes(ctx, label, props)
	if err != nil {
		return nil, fmt.Errorf("query nodes: %w", err)
	}
	return nodes, nil
}

// CreateRelationship creates a relationship between two nodes.
func (s *GraphStore) CreateRelationship(ctx context.Context, fromID, toID, relType string, props map[string]interface{}) error {
	if err := s.driver.CreateRelationship(ctx, fromID, toID, relType, props); err != nil {
		return fmt.Errorf("create relationship: %w", err)
	}
	return nil
}

// ExecuteCypher executes a raw Cypher query.
func (s *GraphStore) ExecuteCypher(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	results, err := s.driver.ExecuteCypher(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("execute cypher %q: %w", query, err)
	}
	return results, nil
}
