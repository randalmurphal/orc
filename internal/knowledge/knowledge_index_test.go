//go:build tdd_pending

package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/randalmurphal/orc/internal/knowledge/index"
)

// --- SC-13: knowledge.Service.IndexProject ---

// SC-13: IndexProject calls through to the indexer pipeline and returns populated result.
func TestServiceIndexProject_Success(t *testing.T) {
	mock := &mockIndexComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		indexResult: &index.IndexResult{
			FilesProcessed:    5,
			ChunksStored:      12,
			PatternsFound:     1,
			ErrorsEncountered: nil,
		},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(mock))

	result, err := svc.IndexProject(context.Background(), "/tmp/test-project", index.IndexOptions{})
	if err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	if result == nil {
		t.Fatal("IndexProject should return non-nil result")
	}
	if result.FilesProcessed != 5 {
		t.Errorf("FilesProcessed = %d, want 5", result.FilesProcessed)
	}
	if result.ChunksStored != 12 {
		t.Errorf("ChunksStored = %d, want 12", result.ChunksStored)
	}
	if result.PatternsFound != 1 {
		t.Errorf("PatternsFound = %d, want 1", result.PatternsFound)
	}
}

// SC-13: IndexProject returns error when service is not available (disabled).
func TestServiceIndexProject_Disabled(t *testing.T) {
	svc := NewService(ServiceConfig{Enabled: false})

	_, err := svc.IndexProject(context.Background(), "/tmp/test", index.IndexOptions{})
	if err == nil {
		t.Fatal("IndexProject should return error when service is disabled")
	}
}

// SC-13: IndexProject returns error when components are nil.
func TestServiceIndexProject_NilComponents(t *testing.T) {
	svc := NewService(ServiceConfig{Enabled: true})
	// comps is nil

	_, err := svc.IndexProject(context.Background(), "/tmp/test", index.IndexOptions{})
	if err == nil {
		t.Fatal("IndexProject should return error when components are nil")
	}
}

// SC-13: IndexProject returns error when infrastructure is unhealthy.
func TestServiceIndexProject_Unhealthy(t *testing.T) {
	mock := &mockIndexComponents{
		neo4jHealthy:  false,
		qdrantHealthy: true,
		redisHealthy:  true,
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(mock))

	_, err := svc.IndexProject(context.Background(), "/tmp/test", index.IndexOptions{})
	if err == nil {
		t.Fatal("IndexProject should return error when infrastructure is unhealthy")
	}
}

// SC-13: IndexProject propagates indexer pipeline errors.
func TestServiceIndexProject_IndexerError(t *testing.T) {
	mock := &mockIndexComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		indexErr:      errors.New("graph store error: connection lost"),
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(mock))

	_, err := svc.IndexProject(context.Background(), "/tmp/test", index.IndexOptions{})
	if err == nil {
		t.Fatal("IndexProject should propagate indexer error")
	}
}

// SC-13: IndexProject passes incremental option through.
func TestServiceIndexProject_IncrementalOption(t *testing.T) {
	mock := &mockIndexComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		indexResult:   &index.IndexResult{FilesProcessed: 1},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(mock))

	_, err := svc.IndexProject(context.Background(), "/tmp/test", index.IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	if !mock.lastOpts.Incremental {
		t.Error("incremental option should be passed through to indexer")
	}
}

// SC-13: IndexProject passes project root to indexer.
func TestServiceIndexProject_PassesRoot(t *testing.T) {
	mock := &mockIndexComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
		indexResult:   &index.IndexResult{},
	}

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(mock))

	_, err := svc.IndexProject(context.Background(), "/my/project", index.IndexOptions{})
	if err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	if mock.lastRoot != "/my/project" {
		t.Errorf("indexer received root %q, want /my/project", mock.lastRoot)
	}
}

// --- Test doubles ---

// mockIndexComponents extends mockComponents to support IndexProject.
type mockIndexComponents struct {
	callOrder         []string
	healthCheckCalled bool
	neo4jHealthy      bool
	qdrantHealthy     bool
	redisHealthy      bool

	// Indexing mock state
	indexResult *index.IndexResult
	indexErr    error
	lastRoot    string
	lastOpts    index.IndexOptions
}

func (m *mockIndexComponents) InfraStart(_ context.Context) error {
	m.callOrder = append(m.callOrder, "infra.Start")
	return nil
}

func (m *mockIndexComponents) InfraStop(_ context.Context) error {
	m.callOrder = append(m.callOrder, "infra.Stop")
	return nil
}

func (m *mockIndexComponents) GraphConnect(_ context.Context) error {
	m.callOrder = append(m.callOrder, "graph.Connect")
	return nil
}

func (m *mockIndexComponents) GraphClose() error {
	m.callOrder = append(m.callOrder, "graph.Close")
	return nil
}

func (m *mockIndexComponents) VectorConnect(_ context.Context) error {
	m.callOrder = append(m.callOrder, "vector.Connect")
	return nil
}

func (m *mockIndexComponents) VectorClose() error {
	m.callOrder = append(m.callOrder, "vector.Close")
	return nil
}

func (m *mockIndexComponents) CacheConnect(_ context.Context) error {
	m.callOrder = append(m.callOrder, "cache.Connect")
	return nil
}

func (m *mockIndexComponents) CacheClose() error {
	m.callOrder = append(m.callOrder, "cache.Close")
	return nil
}

func (m *mockIndexComponents) IsHealthy() (neo4j, qdrant, redis bool) {
	m.healthCheckCalled = true
	return m.neo4jHealthy, m.qdrantHealthy, m.redisHealthy
}
