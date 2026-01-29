package api

import (
	"sync"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"golang.org/x/sync/singleflight"
)

// dashboardCache provides a TTL-based cache for LoadAllTasks() results,
// with singleflight coalescing to prevent redundant concurrent loads.
type dashboardCache struct {
	mu       sync.RWMutex
	tasks    []*orcv1.Task
	loadedAt time.Time
	ttl      time.Duration
	group    singleflight.Group
	backend  storage.Backend
}

// newDashboardCache creates a new dashboard cache with the given backend and TTL.
func newDashboardCache(backend storage.Backend, ttl time.Duration) *dashboardCache {
	return &dashboardCache{
		backend: backend,
		ttl:     ttl,
	}
}

// Tasks returns cached tasks or loads them from the backend.
// Concurrent callers share a single LoadAllTasks() call via singleflight.
func (c *dashboardCache) Tasks() ([]*orcv1.Task, error) {
	// Fast path: check if cache is valid
	c.mu.RLock()
	if c.tasks != nil && time.Since(c.loadedAt) < c.ttl {
		tasks := c.tasks
		c.mu.RUnlock()
		return tasks, nil
	}
	c.mu.RUnlock()

	// Slow path: load via singleflight to coalesce concurrent requests
	result, err, _ := c.group.Do("load", func() (any, error) {
		// Double-check cache after acquiring singleflight slot
		c.mu.RLock()
		if c.tasks != nil && time.Since(c.loadedAt) < c.ttl {
			tasks := c.tasks
			c.mu.RUnlock()
			return tasks, nil
		}
		c.mu.RUnlock()

		tasks, err := c.backend.LoadAllTasks()
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.tasks = tasks
		c.loadedAt = time.Now()
		c.mu.Unlock()

		return tasks, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]*orcv1.Task), nil
}

// Invalidate clears the cache, forcing the next Tasks() call to reload.
func (c *dashboardCache) Invalidate() {
	c.mu.Lock()
	c.tasks = nil
	c.loadedAt = time.Time{}
	c.mu.Unlock()
}
