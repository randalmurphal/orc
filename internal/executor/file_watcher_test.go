package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/events"
)

// MockFileChangeDetector is a mock implementation of FileChangeDetector for testing.
type MockFileChangeDetector struct {
	files []events.ChangedFile
	err   error
}

func (m *MockFileChangeDetector) Detect(ctx context.Context, worktreePath, baseRef string) ([]events.ChangedFile, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.files, nil
}

// MockPublisher is a mock implementation of events.Publisher for testing.
type MockPublisher struct {
	mu     sync.Mutex
	events []events.Event
}

func (m *MockPublisher) Publish(ev events.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, ev)
}

func (m *MockPublisher) Subscribe(taskID string) <-chan events.Event {
	// Not needed for these tests
	return make(<-chan events.Event)
}

func (m *MockPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {
	// Not needed for these tests
}

func (m *MockPublisher) Close() {
	// Not needed for these tests
}

func (m *MockPublisher) GetEvents() []events.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]events.Event, len(m.events))
	copy(result, m.events)
	return result
}

func (m *MockPublisher) GetFilesChangedEvents() []events.FilesChangedUpdate {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []events.FilesChangedUpdate
	for _, ev := range m.events {
		if ev.Type == events.EventFilesChanged {
			if data, ok := ev.Data.(events.FilesChangedUpdate); ok {
				result = append(result, data)
			}
		}
	}
	return result
}

func TestFileWatcher_StartStop(t *testing.T) {
	detector := &MockFileChangeDetector{
		files: []events.ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
		},
	}
	mockPub := &MockPublisher{}
	publisher := NewEventPublisher(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	watcher := NewFileWatcher(detector, publisher, "TASK-001", "/tmp/worktree", "main", logger)

	ctx := context.Background()
	watcher.Start(ctx)

	// Give it a moment to potentially start
	time.Sleep(100 * time.Millisecond)

	// Stop the watcher
	watcher.Stop()

	// Should complete without hanging
}

func TestFileWatcher_EmitsEventOnChange(t *testing.T) {
	detector := &MockFileChangeDetector{
		files: []events.ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			{Path: "file2.go", Status: "added", Additions: 20, Deletions: 0},
		},
	}
	mockPub := &MockPublisher{}
	publisher := NewEventPublisher(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use very short interval for testing
	watcher := NewFileWatcher(detector, publisher, "TASK-001", "/tmp/worktree", "main", logger)
	watcher.interval = 50 * time.Millisecond

	ctx := context.Background()
	watcher.Start(ctx)
	defer watcher.Stop()

	// Wait for at least one poll
	time.Sleep(100 * time.Millisecond)

	events := mockPub.GetFilesChangedEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one files_changed event, got none")
	}

	event := events[0]
	if len(event.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(event.Files))
	}
	if event.TotalAdditions != 30 {
		t.Errorf("expected total additions 30, got %d", event.TotalAdditions)
	}
	if event.TotalDeletions != 5 {
		t.Errorf("expected total deletions 5, got %d", event.TotalDeletions)
	}
}

func TestFileWatcher_Dedupe(t *testing.T) {
	detector := &MockFileChangeDetector{
		files: []events.ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
		},
	}
	mockPub := &MockPublisher{}
	publisher := NewEventPublisher(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use very short interval for testing
	watcher := NewFileWatcher(detector, publisher, "TASK-001", "/tmp/worktree", "main", logger)
	watcher.interval = 50 * time.Millisecond

	ctx := context.Background()
	watcher.Start(ctx)
	defer watcher.Stop()

	// Wait for multiple polls (should see same state multiple times)
	time.Sleep(250 * time.Millisecond)

	events := mockPub.GetFilesChangedEvents()
	// Should only get one event since state doesn't change
	if len(events) != 1 {
		t.Errorf("expected exactly 1 event (deduped), got %d", len(events))
	}
}

func TestFileWatcher_NoEventWhenEmpty(t *testing.T) {
	detector := &MockFileChangeDetector{
		files: []events.ChangedFile{}, // No files
	}
	mockPub := &MockPublisher{}
	publisher := NewEventPublisher(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use very short interval for testing
	watcher := NewFileWatcher(detector, publisher, "TASK-001", "/tmp/worktree", "main", logger)
	watcher.interval = 50 * time.Millisecond

	ctx := context.Background()
	watcher.Start(ctx)
	defer watcher.Stop()

	// Wait for at least one poll
	time.Sleep(100 * time.Millisecond)

	events := mockPub.GetFilesChangedEvents()
	if len(events) != 0 {
		t.Errorf("expected no events when file list is empty, got %d", len(events))
	}
}

func TestFileWatcher_GitError(t *testing.T) {
	detector := &MockFileChangeDetector{
		err: fmt.Errorf("git command failed"),
	}
	mockPub := &MockPublisher{}
	publisher := NewEventPublisher(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use very short interval for testing
	watcher := NewFileWatcher(detector, publisher, "TASK-001", "/tmp/worktree", "main", logger)
	watcher.interval = 50 * time.Millisecond

	ctx := context.Background()
	watcher.Start(ctx)
	defer watcher.Stop()

	// Wait for at least one poll
	time.Sleep(100 * time.Millisecond)

	// Should not crash, but also should not emit events
	events := mockPub.GetFilesChangedEvents()
	if len(events) != 0 {
		t.Errorf("expected no events on error, got %d", len(events))
	}
}

func TestFileWatcher_ContextCancel(t *testing.T) {
	detector := &MockFileChangeDetector{
		files: []events.ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
		},
	}
	mockPub := &MockPublisher{}
	publisher := NewEventPublisher(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	watcher := NewFileWatcher(detector, publisher, "TASK-001", "/tmp/worktree", "main", logger)
	watcher.interval = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	watcher.Start(ctx)

	// Cancel context
	cancel()

	// Should complete quickly without needing explicit Stop()
	done := make(chan struct{})
	go func() {
		watcher.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("watcher did not stop after context cancellation")
	}
}

func TestFileWatcher_HashFileState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	watcher := NewFileWatcher(nil, nil, "TASK-001", "/tmp", "main", logger)

	tests := []struct {
		name     string
		files1   []events.ChangedFile
		files2   []events.ChangedFile
		sameHash bool
	}{
		{
			name: "identical files produce same hash",
			files1: []events.ChangedFile{
				{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			},
			files2: []events.ChangedFile{
				{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			},
			sameHash: true,
		},
		{
			name: "different order produces same hash",
			files1: []events.ChangedFile{
				{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
				{Path: "file2.go", Status: "added", Additions: 20, Deletions: 0},
			},
			files2: []events.ChangedFile{
				{Path: "file2.go", Status: "added", Additions: 20, Deletions: 0},
				{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			},
			sameHash: true,
		},
		{
			name: "different additions produce different hash",
			files1: []events.ChangedFile{
				{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			},
			files2: []events.ChangedFile{
				{Path: "file1.go", Status: "modified", Additions: 15, Deletions: 5},
			},
			sameHash: false,
		},
		{
			name: "different status produces different hash",
			files1: []events.ChangedFile{
				{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
			},
			files2: []events.ChangedFile{
				{Path: "file1.go", Status: "added", Additions: 10, Deletions: 5},
			},
			sameHash: false,
		},
		{
			name:     "empty files produce empty hash",
			files1:   []events.ChangedFile{},
			files2:   []events.ChangedFile{},
			sameHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash1 := watcher.hashFileState(tt.files1)
			hash2 := watcher.hashFileState(tt.files2)

			if tt.sameHash && hash1 != hash2 {
				t.Errorf("expected same hash, got %s and %s", hash1, hash2)
			}
			if !tt.sameHash && hash1 == hash2 {
				t.Errorf("expected different hashes, both got %s", hash1)
			}
		})
	}
}

func TestFileWatcher_NilPublisher(t *testing.T) {
	detector := &MockFileChangeDetector{
		files: []events.ChangedFile{
			{Path: "file1.go", Status: "modified", Additions: 10, Deletions: 5},
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use nil publisher (should be safe)
	watcher := NewFileWatcher(detector, nil, "TASK-001", "/tmp/worktree", "main", logger)
	watcher.interval = 50 * time.Millisecond

	ctx := context.Background()
	watcher.Start(ctx)
	defer watcher.Stop()

	// Should not crash
	time.Sleep(100 * time.Millisecond)
}
