package diff

import (
	"container/list"
	"strings"
	"sync"
)

// Cache is an LRU cache for computed diffs.
type Cache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
}

type cacheEntry struct {
	key   string
	value *FileDiff
}

// NewCache creates a new LRU cache with the given capacity.
func NewCache(capacity int) *Cache {
	if capacity <= 0 {
		capacity = 100
	}
	return &Cache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get retrieves an item from the cache.
// Returns nil if the key is not found.
func (c *Cache) Get(key string) *FileDiff {
	c.mu.Lock()
	defer c.mu.Unlock()

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		entry := el.Value.(*cacheEntry)
		// Return a copy to prevent modification of cached value
		return copyFileDiff(entry.value)
	}
	return nil
}

// Set adds an item to the cache.
// If the key already exists, the value is updated and moved to front.
func (c *Cache) Set(key string, value *FileDiff) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store a copy to prevent external modification
	valueCopy := copyFileDiff(value)

	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		el.Value.(*cacheEntry).value = valueCopy
		return
	}

	// Evict oldest if at capacity
	if c.order.Len() >= c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*cacheEntry).key)
		}
	}

	entry := &cacheEntry{key: key, value: valueCopy}
	el := c.order.PushFront(entry)
	c.items[key] = el
}

// Invalidate removes all entries matching a prefix.
// Useful when new commits arrive and cached diffs may be stale.
func (c *Cache) Invalidate(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, el := range c.items {
		if strings.HasPrefix(key, prefix) {
			c.order.Remove(el)
			delete(c.items, key)
		}
	}
}

// Clear removes all entries from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.order = list.New()
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// copyFileDiff creates a deep copy of a FileDiff.
func copyFileDiff(fd *FileDiff) *FileDiff {
	if fd == nil {
		return nil
	}

	copy := &FileDiff{
		Path:      fd.Path,
		Status:    fd.Status,
		OldPath:   fd.OldPath,
		Additions: fd.Additions,
		Deletions: fd.Deletions,
		Binary:    fd.Binary,
		Syntax:    fd.Syntax,
	}

	if fd.Hunks != nil {
		copy.Hunks = make([]Hunk, len(fd.Hunks))
		for i, hunk := range fd.Hunks {
			copy.Hunks[i] = Hunk{
				OldStart: hunk.OldStart,
				OldLines: hunk.OldLines,
				NewStart: hunk.NewStart,
				NewLines: hunk.NewLines,
			}
			if hunk.Lines != nil {
				copy.Hunks[i].Lines = make([]Line, len(hunk.Lines))
				for j, line := range hunk.Lines {
					copy.Hunks[i].Lines[j] = Line{
						Type:    line.Type,
						Content: line.Content,
						OldLine: line.OldLine,
						NewLine: line.NewLine,
					}
				}
			}
		}
	}

	return copy
}
