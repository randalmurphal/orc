package store

import (
	"context"
	"fmt"
	"time"
)

// RedisClient abstracts Redis cache operations for testing.
type RedisClient interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Delete(ctx context.Context, key string) error
	Connect(ctx context.Context) error
	Close() error
}

// CacheStore provides cache operations.
type CacheStore struct {
	client RedisClient
}

// CacheStoreOption configures a CacheStore.
type CacheStoreOption func(*CacheStore)

// NewCacheStore creates a new cache store.
func NewCacheStore(opts ...CacheStoreOption) *CacheStore {
	s := &CacheStore{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithRedisClient sets a custom Redis client (for testing).
func WithRedisClient(client RedisClient) CacheStoreOption {
	return func(s *CacheStore) {
		s.client = client
	}
}

// Connect establishes connection to Redis.
func (s *CacheStore) Connect(ctx context.Context) error {
	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	return nil
}

// Close closes the connection.
func (s *CacheStore) Close() error {
	return s.client.Close()
}

// Set stores a value with TTL. Zero TTL means no expiry.
func (s *CacheStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := s.client.Set(ctx, key, value, ttl); err != nil {
		return fmt.Errorf("cache set %s: %w", key, err)
	}
	return nil
}

// Get retrieves a value. Returns (nil, nil) on cache miss.
func (s *CacheStore) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("cache get %s: %w", key, err)
	}
	return val, nil
}

// Delete removes a key. Idempotent — no error if key doesn't exist.
func (s *CacheStore) Delete(ctx context.Context, key string) error {
	if err := s.client.Delete(ctx, key); err != nil {
		return fmt.Errorf("cache delete %s: %w", key, err)
	}
	return nil
}

// EmbeddingCacheTTL is the TTL for embedding cache entries (30 days).
const EmbeddingCacheTTL = 30 * 24 * time.Hour

// QueryCacheTTL is the TTL for query cache entries (10 minutes).
const QueryCacheTTL = 10 * time.Minute

// SetEmbedding stores an embedding with 30-day TTL.
func (s *CacheStore) SetEmbedding(ctx context.Context, hash string, data []byte) error {
	return s.Set(ctx, "embed:"+hash, data, EmbeddingCacheTTL)
}

// SetQuery stores a query result with 10-minute TTL.
func (s *CacheStore) SetQuery(ctx context.Context, hash string, data []byte) error {
	return s.Set(ctx, "query:"+hash, data, QueryCacheTTL)
}
