package brief

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Cache manages brief caching to avoid regeneration on every phase.
type Cache struct {
	path string
}

// NewCache creates a cache backed by the given file path.
func NewCache(path string) *Cache {
	return &Cache{path: path}
}

// Store persists a brief to the cache file.
func (c *Cache) Store(b *Brief) error {
	data, err := json.Marshal(b)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// Load reads a cached brief. Returns nil without error if the cache
// file is missing or corrupted.
func (c *Cache) Load() (*Brief, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var b Brief
	if err := json.Unmarshal(data, &b); err != nil {
		// Treat corrupt cache as missing
		return nil, nil
	}
	return &b, nil
}

// IsStale returns true if the cache is stale based on task count delta.
func (c *Cache) IsStale(currentTaskCount, threshold int) bool {
	cached, err := c.Load()
	if err != nil || cached == nil {
		return true
	}
	return currentTaskCount-cached.TaskCount >= threshold
}

// Invalidate removes the cache file.
func (c *Cache) Invalidate() error {
	err := os.Remove(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
