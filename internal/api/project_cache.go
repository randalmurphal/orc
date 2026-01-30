// Package api provides the REST API and Connect RPC server for orc.
package api

import (
	"fmt"
	"sync"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/project"
)

// ProjectCache provides LRU-cached access to project databases.
// Thread-safe for concurrent access.
type ProjectCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	order   []string // LRU order: oldest at front
	maxSize int
}

type cacheEntry struct {
	db   *db.ProjectDB
	path string
}

// NewProjectCache creates a cache with the given maximum size.
func NewProjectCache(maxSize int) *ProjectCache {
	if maxSize < 1 {
		maxSize = 10
	}
	return &ProjectCache{
		entries: make(map[string]*cacheEntry),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// Get returns the ProjectDB for the given project ID.
// Opens the database if not cached, evicting LRU entry if at capacity.
func (c *ProjectCache) Get(projectID string) (*db.ProjectDB, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache
	if entry, ok := c.entries[projectID]; ok {
		c.touch(projectID)
		return entry.db, nil
	}

	// Load project from registry
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, fmt.Errorf("load registry: %w", err)
	}
	proj, err := reg.Get(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// Open database
	pdb, err := db.OpenProject(proj.Path)
	if err != nil {
		return nil, fmt.Errorf("open project db: %w", err)
	}

	// Evict if at capacity
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	// Add to cache
	c.entries[projectID] = &cacheEntry{db: pdb, path: proj.Path}
	c.order = append(c.order, projectID)

	return pdb, nil
}

// Contains checks if a project is in the cache (for testing).
func (c *ProjectCache) Contains(projectID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.entries[projectID]
	return ok
}

// touch moves projectID to end of order (most recently used).
func (c *ProjectCache) touch(projectID string) {
	for i, id := range c.order {
		if id == projectID {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, projectID)
			return
		}
	}
}

// evictOldest removes the least recently used entry.
func (c *ProjectCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	c.order = c.order[1:]
	if entry, ok := c.entries[oldest]; ok {
		_ = entry.db.Close()
		delete(c.entries, oldest)
	}
}

// Close closes all cached databases.
func (c *ProjectCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.entries {
		_ = entry.db.Close()
	}
	c.entries = make(map[string]*cacheEntry)
	c.order = nil
	return nil
}

// GetProjectPath returns the filesystem path for a cached project.
func (c *ProjectCache) GetProjectPath(projectID string) (string, error) {
	c.mu.RLock()
	if entry, ok := c.entries[projectID]; ok {
		c.mu.RUnlock()
		return entry.path, nil
	}
	c.mu.RUnlock()

	// Not cached, look up from registry
	reg, err := project.LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}
	proj, err := reg.Get(projectID)
	if err != nil {
		return "", fmt.Errorf("project not found: %w", err)
	}
	return proj.Path, nil
}

// Size returns the current number of cached databases.
func (c *ProjectCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
