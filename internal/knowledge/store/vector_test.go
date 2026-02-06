package store

import (
	"context"
	"errors"
	"testing"
)

// SC-10: Vector store upserts vectors and verifies correct gRPC calls.
func TestVectorStore_Upsert(t *testing.T) {
	mock := &mockQdrantClient{}
	store := NewVectorStore(WithQdrantClient(mock))

	vector := Vector{
		ID:      "file-1",
		Values:  make([]float32, 1024),
		Payload: map[string]interface{}{"path": "/main.go"},
	}

	err := store.Upsert(context.Background(), "code_chunks", []Vector{vector})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if mock.upsertCalls != 1 {
		t.Errorf("upsert calls = %d, want 1", mock.upsertCalls)
	}
	if mock.lastCollection != "code_chunks" {
		t.Errorf("collection = %s, want code_chunks", mock.lastCollection)
	}
}

// SC-10: Vector store performs similarity search and returns sorted results.
func TestVectorStore_Search(t *testing.T) {
	mock := &mockQdrantClient{
		searchResults: []ScoredVector{
			{ID: "file-1", Score: 0.95, Payload: map[string]interface{}{"path": "/main.go"}},
			{ID: "file-2", Score: 0.80, Payload: map[string]interface{}{"path": "/util.go"}},
		},
	}
	store := NewVectorStore(WithQdrantClient(mock))

	queryVec := make([]float32, 1024)
	results, err := store.Search(context.Background(), "code_chunks", queryVec, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Search returned %d results, want 2", len(results))
	}

	// Results should be sorted by similarity (highest first)
	if results[0].Score < results[1].Score {
		t.Error("Search results should be sorted by score descending")
	}
	if results[0].ID != "file-1" {
		t.Errorf("top result ID = %s, want file-1", results[0].ID)
	}
}

// SC-10: Vector store manages collections.
func TestVectorStore_CreateCollection(t *testing.T) {
	mock := &mockQdrantClient{}
	store := NewVectorStore(WithQdrantClient(mock))

	err := store.CreateCollection(context.Background(), "code_chunks", 1024)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	if mock.createCollectionCalls != 1 {
		t.Errorf("create collection calls = %d, want 1", mock.createCollectionCalls)
	}
	if mock.lastDimension != 1024 {
		t.Errorf("dimension = %d, want 1024", mock.lastDimension)
	}
}

// SC-10 error path: Dimension mismatch on upsert returns descriptive error.
func TestVectorStore_Upsert_DimensionMismatch(t *testing.T) {
	mock := &mockQdrantClient{
		expectedDimension: 1024,
	}
	store := NewVectorStore(WithQdrantClient(mock))

	// Vector with wrong dimensions (768 instead of 1024)
	vector := Vector{
		ID:     "file-1",
		Values: make([]float32, 768),
	}

	err := store.Upsert(context.Background(), "code_chunks", []Vector{vector})
	if err == nil {
		t.Fatal("Upsert should return error on dimension mismatch")
	}

	// Error should mention expected vs actual dimensions
	errStr := err.Error()
	if !containsString(errStr, "1024") || !containsString(errStr, "768") {
		t.Errorf("error %q should mention expected (1024) and actual (768) dimensions", errStr)
	}
}

// SC-10 error path: Connection error wrapped.
func TestVectorStore_ConnectionError(t *testing.T) {
	mock := &mockQdrantClient{
		connectErr: errors.New("connection refused"),
	}
	store := NewVectorStore(WithQdrantClient(mock))

	err := store.Connect(context.Background())
	if err == nil {
		t.Fatal("Connect should return error when connection refused")
	}
}

// --- Types and stubs ---

// Vector represents a vector with payload.
type Vector struct {
	ID      string
	Values  []float32
	Payload map[string]interface{}
}

// ScoredVector represents a search result with similarity score.
type ScoredVector struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

// VectorStore provides vector database operations.
type VectorStore struct{}

// VectorStoreOption configures a VectorStore.
type VectorStoreOption func(*VectorStore)

// NewVectorStore creates a new vector store.
func NewVectorStore(opts ...VectorStoreOption) *VectorStore {
	return nil
}

// WithQdrantClient sets a custom Qdrant client (for testing).
func WithQdrantClient(client *mockQdrantClient) VectorStoreOption {
	return func(s *VectorStore) {}
}

// Connect establishes connection to Qdrant.
func (s *VectorStore) Connect(ctx context.Context) error {
	return errors.New("not implemented")
}

// Close closes the connection.
func (s *VectorStore) Close() error {
	return errors.New("not implemented")
}

// CreateCollection creates a vector collection with given dimensions.
func (s *VectorStore) CreateCollection(ctx context.Context, name string, dimension int) error {
	return errors.New("not implemented")
}

// Upsert inserts or updates vectors in a collection.
func (s *VectorStore) Upsert(ctx context.Context, collection string, vectors []Vector) error {
	return errors.New("not implemented")
}

// Search performs similarity search and returns scored results.
func (s *VectorStore) Search(ctx context.Context, collection string, queryVec []float32, limit int) ([]ScoredVector, error) {
	return nil, errors.New("not implemented")
}

type mockQdrantClient struct {
	upsertCalls           int
	createCollectionCalls int
	lastCollection        string
	lastDimension         int
	expectedDimension     int
	searchResults         []ScoredVector
	connectErr            error
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
