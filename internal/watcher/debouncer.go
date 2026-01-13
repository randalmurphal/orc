package watcher

import (
	"os"
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

// deleteEntry tracks a pending delete verification.
type deleteEntry struct {
	timer *time.Timer
	path  string
}

// Debouncer coalesces rapid file change events.
// It waits for a quiet period before firing the callback.
type Debouncer struct {
	mu             sync.Mutex
	pending        map[debounceKey]*debounceEntry
	pendingDeletes map[string]*deleteEntry // keyed by taskID
	interval       time.Duration
	deleteInterval time.Duration // shorter interval for delete verification
	callback       func(taskID string, fileType FileType, path string)
	deleteCallback func(taskID string)
	stopped        bool
}

// NewDebouncer creates a debouncer with the given interval in milliseconds.
func NewDebouncer(intervalMs int, callback func(taskID string, fileType FileType, path string)) *Debouncer {
	return &Debouncer{
		pending:        make(map[debounceKey]*debounceEntry),
		pendingDeletes: make(map[string]*deleteEntry),
		interval:       time.Duration(intervalMs) * time.Millisecond,
		deleteInterval: 100 * time.Millisecond, // Short delay to catch rename scenarios
		callback:       callback,
	}
}

// SetDeleteCallback sets the callback for verified delete events.
func (d *Debouncer) SetDeleteCallback(callback func(taskID string)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deleteCallback = callback
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

// TriggerDelete schedules a delete verification for a task.
// After the delay, it verifies the task.yaml file is actually gone before firing.
// This handles false positives from rename operations, atomic saves, and git operations.
func (d *Debouncer) TriggerDelete(taskID string, path string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.stopped {
		return
	}

	// If already pending, reset the timer
	if entry, exists := d.pendingDeletes[taskID]; exists {
		entry.timer.Stop()
		entry.path = path
		entry.timer = time.AfterFunc(d.deleteInterval, func() {
			d.fireDelete(taskID)
		})
		return
	}

	// Create new entry
	d.pendingDeletes[taskID] = &deleteEntry{
		path: path,
		timer: time.AfterFunc(d.deleteInterval, func() {
			d.fireDelete(taskID)
		}),
	}
}

// CancelDelete cancels a pending delete verification.
// Called when a Create event comes in for a file that was just "deleted".
func (d *Debouncer) CancelDelete(taskID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if entry, exists := d.pendingDeletes[taskID]; exists {
		entry.timer.Stop()
		delete(d.pendingDeletes, taskID)
	}
}

// fireDelete verifies the deletion and fires the callback if confirmed.
func (d *Debouncer) fireDelete(taskID string) {
	d.mu.Lock()
	entry, exists := d.pendingDeletes[taskID]
	if !exists || d.stopped {
		d.mu.Unlock()
		return
	}
	path := entry.path
	callback := d.deleteCallback
	delete(d.pendingDeletes, taskID)
	d.mu.Unlock()

	// Verify the file is actually gone
	if _, err := os.Stat(path); err == nil {
		// File still exists - this was a false positive (likely rename or atomic save)
		return
	}

	// File is confirmed gone, fire the callback
	if callback != nil {
		callback(taskID)
	}
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

	for taskID, entry := range d.pendingDeletes {
		entry.timer.Stop()
		delete(d.pendingDeletes, taskID)
	}
}

// PendingCount returns the number of pending debounced events.
// Useful for testing.
func (d *Debouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}
