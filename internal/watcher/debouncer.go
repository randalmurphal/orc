package watcher

import (
	"sync"
	"time"
)

// debounceKey uniquely identifies a debounce entry.
type debounceKey struct {
	taskID   string
	fileType FileType
}

// debounceEntry tracks a pending debounced event.
type debounceEntry struct {
	timer *time.Timer
	path  string
}

// Debouncer coalesces rapid file change events.
// It waits for a quiet period before firing the callback.
type Debouncer struct {
	mu       sync.Mutex
	pending  map[debounceKey]*debounceEntry
	interval time.Duration
	callback func(taskID string, fileType FileType, path string)
	stopped  bool
}

// NewDebouncer creates a debouncer with the given interval in milliseconds.
func NewDebouncer(intervalMs int, callback func(taskID string, fileType FileType, path string)) *Debouncer {
	return &Debouncer{
		pending:  make(map[debounceKey]*debounceEntry),
		interval: time.Duration(intervalMs) * time.Millisecond,
		callback: callback,
	}
}

// Trigger registers a file change event for debouncing.
// If an event for the same task+fileType is already pending, it resets the timer.
func (d *Debouncer) Trigger(taskID string, fileType FileType, path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	key := debounceKey{taskID: taskID, fileType: fileType}

	// If already pending, stop the old timer and update path
	if entry, exists := d.pending[key]; exists {
		entry.timer.Stop()
		entry.path = path
		entry.timer = time.AfterFunc(d.interval, func() {
			d.fire(key)
		})
		return
	}

	// Create new entry
	d.pending[key] = &debounceEntry{
		path: path,
		timer: time.AfterFunc(d.interval, func() {
			d.fire(key)
		}),
	}
}

// fire executes the callback for a debounced event.
func (d *Debouncer) fire(key debounceKey) {
	d.mu.Lock()
	entry, exists := d.pending[key]
	if !exists || d.stopped {
		d.mu.Unlock()
		return
	}
	path := entry.path
	delete(d.pending, key)
	d.mu.Unlock()

	// Call the callback outside the lock
	d.callback(key.taskID, key.fileType, path)
}

// Stop cancels all pending timers and prevents new events.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.stopped = true

	for key, entry := range d.pending {
		entry.timer.Stop()
		delete(d.pending, key)
	}
}

// PendingCount returns the number of pending debounced events.
// Useful for testing.
func (d *Debouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}
