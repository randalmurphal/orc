package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/randalmurphal/orc/internal/events"
)

// testPublisher captures published events for testing (thread-safe).
type testPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (p *testPublisher) Publish(event events.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *testPublisher) Subscribe(taskID string) <-chan events.Event {
	return make(chan events.Event)
}

func (p *testPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {}

func (p *testPublisher) Close() {}

func (p *testPublisher) getEvents() []events.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Return a copy to avoid races
	result := make([]events.Event, len(p.events))
	copy(result, p.events)
	return result
}

func (p *testPublisher) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = nil
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc/tasks structure
	tasksDir := filepath.Join(tmpDir, ".orc", "tasks")
	require.NoError(t, os.MkdirAll(tasksDir, 0755))

	return tmpDir
}

func createTask(t *testing.T, workDir, taskID string) {
	t.Helper()
	taskDir := filepath.Join(workDir, ".orc", "tasks", taskID)
	require.NoError(t, os.MkdirAll(taskDir, 0755))

	taskYaml := filepath.Join(taskDir, "task.yaml")
	content := []byte(`id: ` + taskID + `
title: Test Task
status: created
weight: small
`)
	require.NoError(t, os.WriteFile(taskYaml, content, 0644))
}

func TestNew(t *testing.T) {
	t.Run("creates watcher with valid config", func(t *testing.T) {
		pub := &testPublisher{}
		cfg := &Config{
			WorkDir:    t.TempDir(),
			Publisher:  pub,
			DebounceMs: 100,
		}

		w, err := New(cfg)
		require.NoError(t, err)
		assert.NotNil(t, w)

		w.Stop()
	})

	t.Run("returns error with nil config", func(t *testing.T) {
		_, err := New(nil)
		assert.Error(t, err)
	})

	t.Run("returns error with nil publisher", func(t *testing.T) {
		_, err := New(&Config{WorkDir: t.TempDir()})
		assert.Error(t, err)
	})

	t.Run("uses default debounce if not specified", func(t *testing.T) {
		pub := &testPublisher{}
		cfg := &Config{
			WorkDir:   t.TempDir(),
			Publisher: pub,
		}

		w, err := New(cfg)
		require.NoError(t, err)
		assert.NotNil(t, w.debouncer)

		w.Stop()
	})
}

func TestDebouncer(t *testing.T) {
	t.Run("triggers callback after interval", func(t *testing.T) {
		var mu sync.Mutex
		var called bool
		var calledTaskID string
		var calledFileType FileType

		d := NewDebouncer(50, func(taskID string, fileType FileType, path string) {
			mu.Lock()
			defer mu.Unlock()
			called = true
			calledTaskID = taskID
			calledFileType = fileType
		})

		d.Trigger("TASK-001", FileTypeTask, "/test/path")

		// Should not be called immediately
		mu.Lock()
		notCalledYet := !called
		mu.Unlock()
		assert.True(t, notCalledYet)

		// Wait for debounce
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		wasCalled := called
		taskID := calledTaskID
		fileType := calledFileType
		mu.Unlock()

		assert.True(t, wasCalled)
		assert.Equal(t, "TASK-001", taskID)
		assert.Equal(t, FileTypeTask, fileType)

		d.Stop()
	})

	t.Run("resets timer on repeated triggers", func(t *testing.T) {
		var mu sync.Mutex
		callCount := 0

		d := NewDebouncer(100, func(taskID string, fileType FileType, path string) {
			mu.Lock()
			defer mu.Unlock()
			callCount++
		})

		// Trigger multiple times in quick succession
		d.Trigger("TASK-001", FileTypeTask, "/path1")
		time.Sleep(30 * time.Millisecond)
		d.Trigger("TASK-001", FileTypeTask, "/path2")
		time.Sleep(30 * time.Millisecond)
		d.Trigger("TASK-001", FileTypeTask, "/path3")

		// Wait for debounce
		time.Sleep(150 * time.Millisecond)

		// Should only be called once
		mu.Lock()
		count := callCount
		mu.Unlock()
		assert.Equal(t, 1, count)

		d.Stop()
	})

	t.Run("handles multiple task IDs independently", func(t *testing.T) {
		var mu sync.Mutex
		calls := make(map[string]int)

		d := NewDebouncer(50, func(taskID string, fileType FileType, path string) {
			mu.Lock()
			defer mu.Unlock()
			calls[taskID]++
		})

		d.Trigger("TASK-001", FileTypeTask, "/path1")
		d.Trigger("TASK-002", FileTypeTask, "/path2")

		// Wait for debounce
		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		call1 := calls["TASK-001"]
		call2 := calls["TASK-002"]
		mu.Unlock()

		assert.Equal(t, 1, call1)
		assert.Equal(t, 1, call2)

		d.Stop()
	})

	t.Run("stop cancels pending timers", func(t *testing.T) {
		var mu sync.Mutex
		called := false

		d := NewDebouncer(100, func(taskID string, fileType FileType, path string) {
			mu.Lock()
			defer mu.Unlock()
			called = true
		})

		d.Trigger("TASK-001", FileTypeTask, "/path")
		d.Stop()

		// Wait past debounce interval
		time.Sleep(150 * time.Millisecond)

		mu.Lock()
		wasCalled := called
		mu.Unlock()
		assert.False(t, wasCalled)
	})
}

func TestWatcher_ExtractTaskID(t *testing.T) {
	w := &Watcher{
		tasksDir: "/project/.orc/tasks",
	}

	tests := []struct {
		path     string
		expected string
	}{
		{"/project/.orc/tasks/TASK-001/task.yaml", "TASK-001"},
		{"/project/.orc/tasks/TASK-002/state.yaml", "TASK-002"},
		{"/project/.orc/tasks/TASK-003/spec.md", "TASK-003"},
		{"/project/.orc/tasks/TASK-123/plan.yaml", "TASK-123"},
		{"/project/.orc/tasks/invalid/task.yaml", ""},
		{"/project/.orc/config.yaml", ""},
		{"/other/path", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := w.extractTaskID(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWatcher_ClassifyFile(t *testing.T) {
	w := &Watcher{}

	tests := []struct {
		path     string
		expected FileType
	}{
		{"/path/task.yaml", FileTypeTask},
		{"/path/state.yaml", FileTypeState},
		{"/path/plan.yaml", FileTypePlan},
		{"/path/spec.md", FileTypeSpec},
		{"/path/other.txt", FileTypeUnknown},
		{"/path/readme.md", FileTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(filepath.Base(tt.path), func(t *testing.T) {
			result := w.classifyFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDebouncer_Delete(t *testing.T) {
	t.Run("verifies file is actually deleted before callback", func(t *testing.T) {
		var mu sync.Mutex
		var deletedTaskID string

		d := NewDebouncer(50, func(taskID string, fileType FileType, path string) {})
		d.SetDeleteCallback(func(taskID string) {
			mu.Lock()
			defer mu.Unlock()
			deletedTaskID = taskID
		})

		// Create a temp file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "task.yaml")
		require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0644))

		// Trigger delete for this file
		d.TriggerDelete("TASK-001", tmpFile)

		// Wait for delete check
		time.Sleep(200 * time.Millisecond)

		// Callback should NOT have fired because file still exists
		mu.Lock()
		taskID := deletedTaskID
		mu.Unlock()
		assert.Empty(t, taskID, "should not fire callback for existing file")

		d.Stop()
	})

	t.Run("fires callback when file is actually deleted", func(t *testing.T) {
		var mu sync.Mutex
		var deletedTaskID string

		d := NewDebouncer(50, func(taskID string, fileType FileType, path string) {})
		d.SetDeleteCallback(func(taskID string) {
			mu.Lock()
			defer mu.Unlock()
			deletedTaskID = taskID
		})

		// Reference a non-existent file
		tmpDir := t.TempDir()
		nonExistentFile := filepath.Join(tmpDir, "task.yaml")

		// Trigger delete for non-existent file
		d.TriggerDelete("TASK-002", nonExistentFile)

		// Wait for delete check
		time.Sleep(200 * time.Millisecond)

		// Callback should fire because file doesn't exist
		mu.Lock()
		taskID := deletedTaskID
		mu.Unlock()
		assert.Equal(t, "TASK-002", taskID)

		d.Stop()
	})

	t.Run("cancels pending delete when CancelDelete is called", func(t *testing.T) {
		var mu sync.Mutex
		var deletedTaskID string

		d := NewDebouncer(50, func(taskID string, fileType FileType, path string) {})
		d.SetDeleteCallback(func(taskID string) {
			mu.Lock()
			defer mu.Unlock()
			deletedTaskID = taskID
		})

		// Reference a non-existent file
		tmpDir := t.TempDir()
		nonExistentFile := filepath.Join(tmpDir, "task.yaml")

		// Trigger delete, then cancel before it fires
		d.TriggerDelete("TASK-003", nonExistentFile)
		d.CancelDelete("TASK-003")

		// Wait for would-be delete check
		time.Sleep(200 * time.Millisecond)

		// Callback should NOT fire because it was cancelled
		mu.Lock()
		taskID := deletedTaskID
		mu.Unlock()
		assert.Empty(t, taskID, "should not fire callback for cancelled delete")

		d.Stop()
	})

	t.Run("handles rename scenario (Remove then Create)", func(t *testing.T) {
		var mu sync.Mutex
		var deletedTaskID string
		var createdTaskID string

		d := NewDebouncer(50, func(taskID string, fileType FileType, path string) {
			mu.Lock()
			defer mu.Unlock()
			createdTaskID = taskID
		})
		d.SetDeleteCallback(func(taskID string) {
			mu.Lock()
			defer mu.Unlock()
			deletedTaskID = taskID
		})

		// Create a temp file
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "task.yaml")
		require.NoError(t, os.WriteFile(tmpFile, []byte("test"), 0644))

		// Simulate rename: TriggerDelete then immediately CancelDelete and Trigger
		d.TriggerDelete("TASK-004", tmpFile)
		// Simulate Create event cancelling the delete
		d.CancelDelete("TASK-004")
		d.Trigger("TASK-004", FileTypeTask, tmpFile)

		// Wait for events to process
		time.Sleep(200 * time.Millisecond)

		mu.Lock()
		deleted := deletedTaskID
		created := createdTaskID
		mu.Unlock()

		// Delete callback should NOT have fired
		assert.Empty(t, deleted, "should not fire delete for rename scenario")
		// Regular callback should have fired
		assert.Equal(t, "TASK-004", created, "should fire create for rename scenario")

		d.Stop()
	})
}

func TestWatcher_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("detects new task creation", func(t *testing.T) {
		workDir := setupTestDir(t)
		pub := &testPublisher{}

		w, err := New(&Config{
			WorkDir:    workDir,
			Publisher:  pub,
			DebounceMs: 50,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		go w.Start(ctx)

		// Give watcher time to initialize and set up watches
		time.Sleep(100 * time.Millisecond)

		// Create a new task directory first, then the file
		// This mimics how orc actually creates tasks
		taskDir := filepath.Join(workDir, ".orc", "tasks", "TASK-001")
		require.NoError(t, os.MkdirAll(taskDir, 0755))

		// Give time for directory watch to be added
		time.Sleep(100 * time.Millisecond)

		// Now write the task.yaml file
		taskYaml := filepath.Join(taskDir, "task.yaml")
		content := []byte(`id: TASK-001
title: Test Task
status: created
weight: small
`)
		require.NoError(t, os.WriteFile(taskYaml, content, 0644))

		// Wait for debounce + processing
		time.Sleep(200 * time.Millisecond)

		cancel()
		w.Stop()

		// Check that an event was published
		evts := pub.getEvents()
		require.NotEmpty(t, evts, "expected at least one event")

		// Find task created/updated event
		var found bool
		for _, e := range evts {
			if e.TaskID == "TASK-001" &&
				(e.Type == events.EventTaskCreated || e.Type == events.EventTaskUpdated) {
				found = true
				break
			}
		}
		assert.True(t, found, "expected task created/updated event for TASK-001")
	})

	t.Run("detects task modification", func(t *testing.T) {
		workDir := setupTestDir(t)
		pub := &testPublisher{}

		// Create initial task
		createTask(t, workDir, "TASK-002")

		w, err := New(&Config{
			WorkDir:    workDir,
			Publisher:  pub,
			DebounceMs: 50,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		go w.Start(ctx)

		// Give watcher time to initialize and process initial state
		time.Sleep(100 * time.Millisecond)
		pub.reset()

		// Modify the task
		taskPath := filepath.Join(workDir, ".orc", "tasks", "TASK-002", "task.yaml")
		newContent := []byte(`id: TASK-002
title: Modified Task
status: running
weight: small
`)
		require.NoError(t, os.WriteFile(taskPath, newContent, 0644))

		// Wait for debounce + processing
		time.Sleep(200 * time.Millisecond)

		cancel()
		w.Stop()

		// Check that an event was published
		evts := pub.getEvents()
		require.NotEmpty(t, evts, "expected at least one event after modification")

		// Find task updated event
		var found bool
		for _, e := range evts {
			if e.TaskID == "TASK-002" && e.Type == events.EventTaskUpdated {
				found = true
				break
			}
		}
		assert.True(t, found, "expected task updated event for TASK-002")
	})

	t.Run("ignores unchanged content", func(t *testing.T) {
		workDir := setupTestDir(t)
		pub := &testPublisher{}

		// Create initial task
		createTask(t, workDir, "TASK-003")

		w, err := New(&Config{
			WorkDir:    workDir,
			Publisher:  pub,
			DebounceMs: 50,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		go w.Start(ctx)

		// Give watcher time to initialize
		time.Sleep(100 * time.Millisecond)

		// Read the content
		taskPath := filepath.Join(workDir, ".orc", "tasks", "TASK-003", "task.yaml")
		content, err := os.ReadFile(taskPath)
		require.NoError(t, err)

		// Trigger a hash check by accessing the file
		_, _ = w.hasContentChanged(taskPath)

		pub.reset()

		// Write the same content again
		require.NoError(t, os.WriteFile(taskPath, content, 0644))

		// Wait for debounce + processing
		time.Sleep(200 * time.Millisecond)

		cancel()
		w.Stop()

		// Should not publish an event for unchanged content
		evts := pub.getEvents()
		assert.Empty(t, evts, "expected no events for unchanged content")
	})

	t.Run("no spurious delete on atomic save", func(t *testing.T) {
		workDir := setupTestDir(t)
		pub := &testPublisher{}

		// Create initial task
		createTask(t, workDir, "TASK-ATOMIC")

		w, err := New(&Config{
			WorkDir:    workDir,
			Publisher:  pub,
			DebounceMs: 50,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		go w.Start(ctx)

		// Give watcher time to initialize
		time.Sleep(100 * time.Millisecond)
		pub.reset()

		// Simulate atomic save: write to temp, remove original, rename temp
		taskPath := filepath.Join(workDir, ".orc", "tasks", "TASK-ATOMIC", "task.yaml")
		tempPath := taskPath + ".tmp"

		// Write new content to temp
		newContent := []byte(`id: TASK-ATOMIC
title: Atomically Saved Task
status: running
weight: medium
`)
		require.NoError(t, os.WriteFile(tempPath, newContent, 0644))

		// Remove original (this triggers fsnotify Remove)
		require.NoError(t, os.Remove(taskPath))

		// Immediately rename temp to original (this triggers fsnotify Create/Rename)
		require.NoError(t, os.Rename(tempPath, taskPath))

		// Wait for events to process
		time.Sleep(300 * time.Millisecond)

		cancel()
		w.Stop()

		// Check events - should NOT have delete
		evts := pub.getEvents()
		var hasDelete bool
		for _, e := range evts {
			if e.TaskID == "TASK-ATOMIC" && e.Type == events.EventTaskDeleted {
				hasDelete = true
				break
			}
		}

		assert.False(t, hasDelete, "should NOT publish delete event for atomic save")
		// Note: We might or might not get an update depending on timing,
		// the important thing is no spurious delete
	})

	t.Run("actual task deletion publishes delete event", func(t *testing.T) {
		workDir := setupTestDir(t)
		pub := &testPublisher{}

		// Create initial task
		createTask(t, workDir, "TASK-DELETE")

		w, err := New(&Config{
			WorkDir:    workDir,
			Publisher:  pub,
			DebounceMs: 50,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		go w.Start(ctx)

		// Give watcher time to initialize
		time.Sleep(100 * time.Millisecond)
		pub.reset()

		// Actually delete the task.yaml file
		taskPath := filepath.Join(workDir, ".orc", "tasks", "TASK-DELETE", "task.yaml")
		require.NoError(t, os.Remove(taskPath))

		// Wait for delete verification
		time.Sleep(300 * time.Millisecond)

		cancel()
		w.Stop()

		// Check that delete event was published
		evts := pub.getEvents()
		var hasDelete bool
		for _, e := range evts {
			if e.TaskID == "TASK-DELETE" && e.Type == events.EventTaskDeleted {
				hasDelete = true
				break
			}
		}

		assert.True(t, hasDelete, "should publish delete event for actual deletion")
	})
}
