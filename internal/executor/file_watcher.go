// Package executor provides the flowgraph-based execution engine for orc.
// This file contains file watching functionality for real-time change detection.
package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
)

// DefaultFileWatchInterval is the default interval for file change polling.
const DefaultFileWatchInterval = 10 * time.Second

// FileChangeDetector detects file changes in a git worktree.
type FileChangeDetector interface {
	Detect(ctx context.Context, worktreePath, baseRef string) ([]events.ChangedFile, error)
}

// GitDiffDetector implements FileChangeDetector using git diff.
type GitDiffDetector struct {
	diffService *diff.Service
}

// NewGitDiffDetector creates a new GitDiffDetector.
func NewGitDiffDetector(worktreePath string) *GitDiffDetector {
	return &GitDiffDetector{
		diffService: diff.NewService(worktreePath, nil), // No cache needed for file watcher
	}
}

// Detect detects file changes by comparing HEAD against the base reference.
// Only returns committed/staged changes (not untracked files).
func (d *GitDiffDetector) Detect(ctx context.Context, worktreePath, baseRef string) ([]events.ChangedFile, error) {
	// Use diff.Service.GetFileList to get changes between base and HEAD
	// HEAD is empty string to compare against current HEAD
	files, err := d.diffService.GetFileList(ctx, baseRef, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get file list: %w", err)
	}

	// Convert diff.FileDiff to events.ChangedFile
	result := make([]events.ChangedFile, 0, len(files))
	for _, f := range files {
		result = append(result, events.ChangedFile{
			Path:      f.Path,
			Status:    f.Status,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}

	return result, nil
}

// FileWatcher polls for file changes and publishes events.
type FileWatcher struct {
	detector  FileChangeDetector
	publisher *PublishHelper
	interval  time.Duration
	taskID    string
	workDir   string
	baseRef   string
	logger    *slog.Logger

	mu        sync.Mutex
	lastState string // Hash of last known file state for deduplication
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewFileWatcher creates a new file watcher.
func NewFileWatcher(detector FileChangeDetector, publisher *PublishHelper, taskID, workDir, baseRef string, logger *slog.Logger) *FileWatcher {
	return &FileWatcher{
		detector:  detector,
		publisher: publisher,
		interval:  DefaultFileWatchInterval,
		taskID:    taskID,
		workDir:   workDir,
		baseRef:   baseRef,
		logger:    logger,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start begins the file watching loop in a goroutine.
func (w *FileWatcher) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the file watching loop to stop and waits for it to finish.
func (w *FileWatcher) Stop() {
	close(w.stopCh)
	<-w.doneCh
}

// run is the main file watching loop.
func (w *FileWatcher) run(ctx context.Context) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Debug("file watcher stopping due to context cancellation", "task", w.taskID)
			return
		case <-w.stopCh:
			w.logger.Debug("file watcher stopping due to stop signal", "task", w.taskID)
			return
		case <-ticker.C:
			w.checkForChanges(ctx)
		}
	}
}

// checkForChanges polls for file changes and publishes an event if changes are detected.
func (w *FileWatcher) checkForChanges(ctx context.Context) {
	files, err := w.detector.Detect(ctx, w.workDir, w.baseRef)
	if err != nil {
		w.logger.Warn("failed to detect file changes",
			"task", w.taskID,
			"error", err,
		)
		return
	}

	// Calculate hash of current state for deduplication
	currentState := w.hashFileState(files)

	w.mu.Lock()
	defer w.mu.Unlock()

	// Skip if state hasn't changed
	if currentState == w.lastState {
		return
	}

	w.lastState = currentState

	// Only publish if there are files
	if len(files) == 0 {
		return
	}

	// Calculate totals
	totalAdditions := 0
	totalDeletions := 0
	for _, f := range files {
		totalAdditions += f.Additions
		totalDeletions += f.Deletions
	}

	// Publish event
	w.publisher.FilesChanged(w.taskID, events.FilesChangedUpdate{
		Files:          files,
		TotalAdditions: totalAdditions,
		TotalDeletions: totalDeletions,
		Timestamp:      time.Now(),
	})

	w.logger.Debug("published files_changed event",
		"task", w.taskID,
		"file_count", len(files),
		"additions", totalAdditions,
		"deletions", totalDeletions,
	)
}

// hashFileState creates a hash of the file state for deduplication.
// The hash includes path, status, additions, and deletions for each file.
func (w *FileWatcher) hashFileState(files []events.ChangedFile) string {
	if len(files) == 0 {
		return ""
	}

	// Sort files by path for consistent hashing
	sorted := make([]events.ChangedFile, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})

	// Create hash from sorted file list
	h := sha256.New()
	for _, f := range sorted {
		_, _ = fmt.Fprintf(h, "%s:%s:%d:%d\n", f.Path, f.Status, f.Additions, f.Deletions)
	}

	return hex.EncodeToString(h.Sum(nil))
}
