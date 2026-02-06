package store

import (
	"context"
	"fmt"
	"sort"
)

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

// QdrantClient abstracts Qdrant vector database operations for testing.
type QdrantClient interface {
	CreateCollection(ctx context.Context, name string, dimension int) error
	Upsert(ctx context.Context, collection string, vectors []Vector) error
	Search(ctx context.Context, collection string, queryVec []float32, limit int) ([]ScoredVector, error)
	Connect(ctx context.Context) error
	Close() error
}

// VectorStore provides vector database operations.
type VectorStore struct {
	client            QdrantClient
	expectedDimension int
}

// VectorStoreOption configures a VectorStore.
type VectorStoreOption func(*VectorStore)

// NewVectorStore creates a new vector store.
func NewVectorStore(opts ...VectorStoreOption) *VectorStore {
	s := &VectorStore{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithQdrantClient sets a custom Qdrant client (for testing).
func WithQdrantClient(client QdrantClient) VectorStoreOption {
	return func(s *VectorStore) {
		s.client = client
		// If the client has an expected dimension, propagate it
		if mc, ok := client.(interface{ ExpectedDimension() int }); ok {
			s.expectedDimension = mc.ExpectedDimension()
		}
	}
}

// Connect establishes connection to Qdrant.
func (s *VectorStore) Connect(ctx context.Context) error {
	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("connect to qdrant: %w", err)
	}
	return nil
}

// Close closes the connection.
func (s *VectorStore) Close() error {
	return s.client.Close()
}

// CreateCollection creates a vector collection with given dimensions.
func (s *VectorStore) CreateCollection(ctx context.Context, name string, dimension int) error {
	if err := s.client.CreateCollection(ctx, name, dimension); err != nil {
		return fmt.Errorf("create collection %s: %w", name, err)
	}
	return nil
}

// Upsert inserts or updates vectors in a collection.
func (s *VectorStore) Upsert(ctx context.Context, collection string, vectors []Vector) error {
	if s.expectedDimension > 0 {
		for _, v := range vectors {
			if len(v.Values) != s.expectedDimension {
				return fmt.Errorf("upsert vector: dimension mismatch: expected %d, got %d",
					s.expectedDimension, len(v.Values))
			}
		}
	}
	if err := s.client.Upsert(ctx, collection, vectors); err != nil {
		return fmt.Errorf("upsert vectors to %s: %w", collection, err)
	}
	return nil
}

// Search performs similarity search and returns scored results.
func (s *VectorStore) Search(ctx context.Context, collection string, queryVec []float32, limit int) ([]ScoredVector, error) {
	results, err := s.client.Search(ctx, collection, queryVec, limit)
	if err != nil {
		return nil, fmt.Errorf("search %s: %w", collection, err)
	}
	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	return results, nil
}
