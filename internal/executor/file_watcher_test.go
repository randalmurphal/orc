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
	publisher := events.NewPublishHelper(mockPub)
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
	publisher := events.NewPublishHelper(mockPub)
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
	publisher := events.NewPublishHelper(mockPub)
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
	publisher := events.NewPublishHelper(mockPub)
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
	publisher := events.NewPublishHelper(mockPub)
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
	publisher := events.NewPublishHelper(mockPub)
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

// Integration tests for GitDiffDetector

func TestGitDiffDetector_Integration(t *testing.T) {
	t.Parallel()

	// Create temp dir for test repo
	tmpDir, err := os.MkdirTemp("", "orc-git-diff-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize git repo
	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	// Create detector
	detector := NewGitDiffDetector(tmpDir)

	// Initially no changes (HEAD == main)
	files, err := detector.Detect(context.Background(), tmpDir, "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files when no changes, got %d", len(files))
	}

	// Make a change and commit
	testFile := tmpDir + "/test.go"
	if err := os.WriteFile(testFile, []byte("package test\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Add test.go"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Create a feature branch
	if err := runGitCmd(tmpDir, "checkout", "-b", "feature"); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Make another change
	if err := os.WriteFile(testFile, []byte("package test\n\nfunc Test() {}\n"), 0644); err != nil {
		t.Fatalf("failed to update test file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Update test.go"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Detect changes from feature to main
	files, err = detector.Detect(context.Background(), tmpDir, "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 changed file, got %d", len(files))
	}

	file := files[0]
	if file.Path != "test.go" {
		t.Errorf("expected path 'test.go', got %s", file.Path)
	}
	if file.Status != "modified" {
		t.Errorf("expected status 'modified', got %s", file.Status)
	}
	if file.Additions == 0 {
		t.Error("expected additions > 0")
	}
}

func TestGitDiffDetector_BinaryFiles(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "orc-git-diff-binary-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	detector := NewGitDiffDetector(tmpDir)

	// Create a binary file (image)
	binaryData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10} // JPEG header
	binFile := tmpDir + "/image.jpg"
	if err := os.WriteFile(binFile, binaryData, 0644); err != nil {
		t.Fatalf("failed to write binary file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Add binary file"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	if err := runGitCmd(tmpDir, "checkout", "-b", "feature"); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Modify binary file
	binaryData2 := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x20}
	if err := os.WriteFile(binFile, binaryData2, 0644); err != nil {
		t.Fatalf("failed to update binary file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Update binary file"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	files, err := detector.Detect(context.Background(), tmpDir, "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Binary files should still be reported
	if len(files) != 1 {
		t.Fatalf("expected 1 changed file, got %d", len(files))
	}
	if files[0].Path != "image.jpg" {
		t.Errorf("expected path 'image.jpg', got %s", files[0].Path)
	}
}

func TestGitDiffDetector_MultipleFiles(t *testing.T) {
	t.Parallel()

	tmpDir, err := os.MkdirTemp("", "orc-git-diff-multi-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	detector := NewGitDiffDetector(tmpDir)

	if err := runGitCmd(tmpDir, "checkout", "-b", "feature"); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Add multiple files
	for i := 1; i <= 3; i++ {
		filename := fmt.Sprintf(tmpDir+"/file%d.go", i)
		content := fmt.Sprintf("package file%d\n", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file%d: %v", i, err)
		}
	}

	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Add multiple files"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	files, err := detector.Detect(context.Background(), tmpDir, "main")
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 changed files, got %d", len(files))
	}

	// Verify all files are present
	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.Path] = true
	}
	for i := 1; i <= 3; i++ {
		expected := fmt.Sprintf("file%d.go", i)
		if !paths[expected] {
			t.Errorf("expected to find %s in changed files", expected)
		}
	}
}

// Integration test: FileWatcher + PublishHelper + GitDiffDetector
func TestFileWatcher_Integration(t *testing.T) {
	t.Parallel()

	// Create temp dir for test repo
	tmpDir, err := os.MkdirTemp("", "orc-filewatcher-integration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Initialize git repo
	if err := initTestRepo(tmpDir); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	// Create feature branch
	if err := runGitCmd(tmpDir, "checkout", "-b", "feature"); err != nil {
		t.Fatalf("failed to create feature branch: %v", err)
	}

	// Set up components
	detector := NewGitDiffDetector(tmpDir)
	mockPub := &MockPublisher{}
	publisher := events.NewPublishHelper(mockPub)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create watcher with very short interval
	watcher := NewFileWatcher(detector, publisher, "TASK-001", tmpDir, "main", logger)
	watcher.interval = 100 * time.Millisecond

	ctx := context.Background()
	watcher.Start(ctx)
	defer watcher.Stop()

	// Wait a bit to ensure first poll happens (no changes yet)
	time.Sleep(200 * time.Millisecond)

	// Should have no events yet
	events := mockPub.GetFilesChangedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events initially, got %d", len(events))
	}

	// Make a change and commit
	testFile := tmpDir + "/integration_test.go"
	if err := os.WriteFile(testFile, []byte("package test\n\nfunc IntegrationTest() {}\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Add integration test"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Wait for watcher to detect the change
	time.Sleep(300 * time.Millisecond)

	// Should have at least one event now
	events = mockPub.GetFilesChangedEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one files_changed event after commit, got none")
	}

	// Verify event content
	event := events[0]
	if len(event.Files) != 1 {
		t.Errorf("expected 1 file in event, got %d", len(event.Files))
	}
	if event.Files[0].Path != "integration_test.go" {
		t.Errorf("expected path 'integration_test.go', got %s", event.Files[0].Path)
	}
	if event.Files[0].Status != "added" {
		t.Errorf("expected status 'added', got %s", event.Files[0].Status)
	}
	if event.TotalAdditions == 0 {
		t.Error("expected total additions > 0")
	}

	// Verify timestamp is recent
	if time.Since(event.Timestamp) > 5*time.Second {
		t.Errorf("event timestamp too old: %v", event.Timestamp)
	}

	// Make another commit (should trigger another event)
	if err := os.WriteFile(testFile, []byte("package test\n\nfunc IntegrationTest() {}\n\nfunc AnotherTest() {}\n"), 0644); err != nil {
		t.Fatalf("failed to update test file: %v", err)
	}
	if err := runGitCmd(tmpDir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGitCmd(tmpDir, "commit", "-m", "Update integration test"); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}

	// Wait for another poll
	time.Sleep(300 * time.Millisecond)

	// Should have at least 2 events now
	events = mockPub.GetFilesChangedEvents()
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}
}
