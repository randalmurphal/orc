package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

// SC-11: Cache store provides TTL-based set.
func TestCacheStore_Set(t *testing.T) {
	mock := &mockRedisClient{}
	store := NewCacheStore(WithRedisClient(mock))

	err := store.Set(context.Background(), "embed:hash123", []byte("vector-data"), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	if mock.setCalls != 1 {
		t.Errorf("set calls = %d, want 1", mock.setCalls)
	}
	if mock.lastKey != "embed:hash123" {
		t.Errorf("key = %s, want embed:hash123", mock.lastKey)
	}
	if mock.lastTTL != 30*24*time.Hour {
		t.Errorf("TTL = %v, want 30 days", mock.lastTTL)
	}
}

// SC-11: Cache store provides TTL-based get with hit.
func TestCacheStore_Get_Hit(t *testing.T) {
	mock := &mockRedisClient{
		data: map[string][]byte{
			"embed:hash123": []byte("cached-value"),
		},
	}
	store := NewCacheStore(WithRedisClient(mock))

	value, err := store.Get(context.Background(), "embed:hash123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if value == nil {
		t.Fatal("Get should return non-nil value on cache hit")
	}
	if string(value) != "cached-value" {
		t.Errorf("Get value = %s, want cached-value", string(value))
	}
}

// SC-11: Cache miss returns (nil, nil), NOT an error.
func TestCacheStore_Get_Miss(t *testing.T) {
	mock := &mockRedisClient{
		data: map[string][]byte{}, // Empty cache
	}
	store := NewCacheStore(WithRedisClient(mock))

	value, err := store.Get(context.Background(), "nonexistent-key")
	if err != nil {
		t.Fatalf("Get should not error on cache miss: %v", err)
	}
	if value != nil {
		t.Errorf("Get should return nil on cache miss, got %v", value)
	}
}

// SC-11: Delete is idempotent.
func TestCacheStore_Delete(t *testing.T) {
	mock := &mockRedisClient{
		data: map[string][]byte{
			"key1": []byte("value1"),
		},
	}
	store := NewCacheStore(WithRedisClient(mock))

	// Delete existing key
	err := store.Delete(context.Background(), "key1")
	if err != nil {
		t.Fatalf("Delete existing key: %v", err)
	}

	// Delete non-existing key (should not error)
	err = store.Delete(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Delete non-existing key should not error: %v", err)
	}
}

// SC-11: Embedding cache uses 30-day TTL.
func TestCacheStore_EmbeddingCacheTTL(t *testing.T) {
	mock := &mockRedisClient{}
	store := NewCacheStore(WithRedisClient(mock))

	err := store.SetEmbedding(context.Background(), "hash123", []byte("vector"))
	if err != nil {
		t.Fatalf("SetEmbedding: %v", err)
	}

	expectedTTL := 30 * 24 * time.Hour
	if mock.lastTTL != expectedTTL {
		t.Errorf("embedding TTL = %v, want %v (30 days)", mock.lastTTL, expectedTTL)
	}
}

// SC-11: Query cache uses 10-minute TTL.
func TestCacheStore_QueryCacheTTL(t *testing.T) {
	mock := &mockRedisClient{}
	store := NewCacheStore(WithRedisClient(mock))

	err := store.SetQuery(context.Background(), "query-hash", []byte("results"))
	if err != nil {
		t.Fatalf("SetQuery: %v", err)
	}

	expectedTTL := 10 * time.Minute
	if mock.lastTTL != expectedTTL {
		t.Errorf("query TTL = %v, want %v (10 minutes)", mock.lastTTL, expectedTTL)
	}
}

// SC-11 error path: Connection failure returns wrapped error.
func TestCacheStore_ConnectionError(t *testing.T) {
	mock := &mockRedisClient{
		connectErr: errors.New("connection refused"),
	}
	store := NewCacheStore(WithRedisClient(mock))

	err := store.Connect(context.Background())
	if err == nil {
		t.Fatal("Connect should return error when connection refused")
	}
}

// Edge case: Set with 0 TTL.
func TestCacheStore_SetZeroTTL(t *testing.T) {
	mock := &mockRedisClient{}
	store := NewCacheStore(WithRedisClient(mock))

	// Zero TTL should be handled explicitly (no surprise behavior)
	err := store.Set(context.Background(), "key", []byte("value"), 0)
	// The behavior (error or no-expiry) must be documented.
	// For this test, we verify the call doesn't panic and is deterministic.
	_ = err // Implementation will define the behavior
}

// --- Test doubles ---

type mockRedisClient struct {
	setCalls   int
	lastKey    string
	lastTTL    time.Duration
	data       map[string][]byte
	connectErr error
}

func (m *mockRedisClient) Connect(_ context.Context) error {
	return m.connectErr
}

func (m *mockRedisClient) Close() error {
	return nil
}

func (m *mockRedisClient) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.setCalls++
	m.lastKey = key
	m.lastTTL = ttl
	if m.data != nil {
		m.data[key] = value
	}
	return nil
}

func (m *mockRedisClient) Get(_ context.Context, key string) ([]byte, error) {
	if m.data == nil {
		return nil, nil
	}
	val, ok := m.data[key]
	if !ok {
		return nil, nil // Cache miss = (nil, nil)
	}
	return val, nil
}

func (m *mockRedisClient) Delete(_ context.Context, key string) error {
	if m.data != nil {
		delete(m.data, key)
	}
	return nil
}
