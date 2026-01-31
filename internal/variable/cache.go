package variable

import (
	"sync"
	"time"
)

// Cache provides TTL-based caching for resolved variables.
// It is safe for concurrent use.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	value     string
	expiresAt time.Time
}

// NewCache creates a new variable cache.
func NewCache() *Cache {
	return &Cache{
		entries: make(map[string]*cacheEntry),
	}
}

// Get retrieves a cached value if it exists and hasn't expired.
// Returns the value and true if found, or empty string and false if not found or expired.
func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		return "", false
	}

	return entry.value, true
}

// Set stores a value in the cache with the given TTL.
// If ttl is 0 or negative, the value is not cached.
func (c *Cache) Set(key, value string, ttl time.Duration) {
	if ttl <= 0 {
		return // Don't cache zero/negative TTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a cached value.
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

// CacheKey generates a cache key for a variable definition.
// The key includes the source type and relevant config to ensure
// different configurations produce different cache entries.
func CacheKey(def *Definition, ctx *ResolutionContext) string {
	// Base key from variable name
	key := def.Name

	// Add context-specific suffix for phase outputs
	if def.SourceType == SourcePhaseOutput {
		if ctx != nil && ctx.TaskID != "" {
			key = ctx.TaskID + ":" + key
		}
	}

	return key
}
