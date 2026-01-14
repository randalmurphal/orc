// Package watcher provides file system watching for task and initiative directory changes.
// It monitors .orc/tasks/ and .orc/initiatives/ and publishes events when files are created, modified, or deleted.
package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
)

// FileType represents the type of file being watched.
type FileType int

const (
	FileTypeTask FileType = iota
	FileTypeState
	FileTypePlan
	FileTypeSpec
	FileTypeInitiative
	FileTypeUnknown
)

// Config configures the file watcher.
type Config struct {
	WorkDir    string
	Publisher  events.Publisher
	Logger     *slog.Logger
	DebounceMs int // Debounce interval in milliseconds (default: 500)
}

// Watcher monitors the .orc/tasks and .orc/initiatives directories for file changes.
type Watcher struct {
	workDir        string
	tasksDir       string
	initiativesDir string
	publisher      events.Publisher
	logger         *slog.Logger

	fsWatcher           *fsnotify.Watcher
	debouncer           *Debouncer
	initiativeDebouncer *Debouncer

	// Content hashing to detect meaningful changes
	hashes   map[string]string
	hashesMu sync.RWMutex

	// Task weight tracking for detecting weight changes
	weights   map[string]task.Weight
	weightsMu sync.RWMutex

	// Lifecycle
	done chan struct{}
}

// New creates a new file watcher.
func New(cfg *Config) (*Watcher, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if cfg.Publisher == nil {
		return nil, fmt.Errorf("publisher is required")
	}

	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "."
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	debounceMs := cfg.DebounceMs
	if debounceMs <= 0 {
		debounceMs = 500
	}

	tasksDir := filepath.Join(workDir, ".orc", "tasks")
	initiativesDir := filepath.Join(workDir, ".orc", "initiatives")

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	w := &Watcher{
		workDir:        workDir,
		tasksDir:       tasksDir,
		initiativesDir: initiativesDir,
		publisher:      cfg.Publisher,
		logger:         logger,
		fsWatcher:      fsWatcher,
		hashes:         make(map[string]string),
		weights:        make(map[string]task.Weight),
		done:           make(chan struct{}),
	}

	// Create debouncer with callback for tasks
	w.debouncer = NewDebouncer(debounceMs, w.handleDebouncedEvent)
	w.debouncer.SetDeleteCallback(w.publishTaskDeleted)

	// Create debouncer with callback for initiatives
	w.initiativeDebouncer = NewDebouncer(debounceMs, w.handleDebouncedInitiativeEvent)
	w.initiativeDebouncer.SetDeleteCallback(w.publishInitiativeDeleted)

	return w, nil
}

// Start begins watching the tasks and initiatives directories.
// Blocks until context is cancelled or an error occurs.
func (w *Watcher) Start(ctx context.Context) error {
	// Watch the .orc directory so we can detect when tasks/ or initiatives/ is created
	orcDir := filepath.Dir(w.tasksDir)
	if err := w.fsWatcher.Add(orcDir); err != nil {
		w.logger.Warn("failed to watch .orc directory", "error", err)
	}

	// Ensure tasks directory exists
	if _, err := os.Stat(w.tasksDir); os.IsNotExist(err) {
		w.logger.Debug("tasks directory does not exist, will watch when created", "path", w.tasksDir)
	} else {
		// Add existing task directories to watch
		if err := w.addWatchRecursive(w.tasksDir); err != nil {
			w.logger.Warn("failed to add initial task watches", "error", err)
		}
	}

	// Watch initiatives directory if it exists
	if _, err := os.Stat(w.initiativesDir); os.IsNotExist(err) {
		w.logger.Debug("initiatives directory does not exist, will watch when created", "path", w.initiativesDir)
	} else {
		if err := w.addWatchRecursive(w.initiativesDir); err != nil {
			w.logger.Warn("failed to add initial initiative watches", "error", err)
		}
	}

	w.logger.Info("file watcher started", "tasksDir", w.tasksDir, "initiativesDir", w.initiativesDir)

	// Event processing loop
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("file watcher stopping", "reason", "context cancelled")
			w.Stop()
			return ctx.Err()

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}
			w.handleFSEvent(event)

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("fsnotify error", "error", err)
		}
	}
}

// Stop gracefully shuts down the watcher.
func (w *Watcher) Stop() error {
	select {
	case <-w.done:
		// Already stopped
		return nil
	default:
		close(w.done)
	}

	w.debouncer.Stop()
	w.initiativeDebouncer.Stop()

	if err := w.fsWatcher.Close(); err != nil {
		return fmt.Errorf("close fsnotify watcher: %w", err)
	}

	w.logger.Info("file watcher stopped")
	return nil
}

// Done returns a channel that's closed when the watcher stops.
func (w *Watcher) Done() <-chan struct{} {
	return w.done
}

// addWatchRecursive adds the directory and all subdirectories to the watch list.
func (w *Watcher) addWatchRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip paths with errors
		}
		if d.IsDir() {
			if err := w.fsWatcher.Add(path); err != nil {
				w.logger.Debug("failed to watch directory", "path", path, "error", err)
				return nil // Continue despite errors
			}
			w.logger.Debug("watching directory", "path", path)
		}
		return nil
	})
}

// handleFSEvent processes a raw fsnotify event.
func (w *Watcher) handleFSEvent(event fsnotify.Event) {
	path := event.Name

	// Check if tasks or initiatives directory was just created
	if event.Has(fsnotify.Create) {
		if path == w.tasksDir {
			w.logger.Info("tasks directory created, adding watches")
			if err := w.addWatchRecursive(w.tasksDir); err != nil {
				w.logger.Warn("failed to watch tasks directory", "error", err)
			}
			return
		}
		if path == w.initiativesDir {
			w.logger.Info("initiatives directory created, adding watches")
			if err := w.addWatchRecursive(w.initiativesDir); err != nil {
				w.logger.Warn("failed to watch initiatives directory", "error", err)
			}
			return
		}
	}

	// Route to appropriate handler based on path
	if strings.HasPrefix(path, w.tasksDir) {
		w.handleTaskFSEvent(event, path)
	} else if strings.HasPrefix(path, w.initiativesDir) {
		w.handleInitiativeFSEvent(event, path)
	}
}

// handleTaskFSEvent processes a task-related fsnotify event.
func (w *Watcher) handleTaskFSEvent(event fsnotify.Event, path string) {
	// Handle directory creation first (before checking file type)
	// This ensures we add watches for new task directories
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			w.logger.Debug("new task directory detected, adding watch", "path", path)
			if err := w.fsWatcher.Add(path); err != nil {
				w.logger.Debug("failed to watch new directory", "path", path, "error", err)
			}
			return // Directories themselves don't trigger task events
		}
	}

	// Extract task ID from path
	taskID := w.extractTaskID(path)
	if taskID == "" {
		return
	}

	// Determine file type
	fileType := w.classifyFile(path)
	if fileType == FileTypeUnknown {
		return
	}

	w.logger.Debug("task fs event",
		"op", event.Op.String(),
		"path", path,
		"taskID", taskID,
		"fileType", fileType,
	)

	// Handle file removal
	if event.Has(fsnotify.Remove) {
		w.removeHash(path)
		// If task.yaml was removed, the task might be deleted
		// But we need to verify - fsnotify sends Remove events for:
		// - Actual deletions
		// - File renames (Remove + Create)
		// - Atomic saves (temp write, remove original, rename temp)
		// - Git operations (checkout, worktree setup)
		if fileType == FileTypeTask {
			// Schedule verification after a short delay to handle rename scenarios
			w.debouncer.TriggerDelete(taskID, path)
			return
		}
	}

	// For writes and creates, debounce and check for real changes
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
		// Cancel any pending delete for this task - the file is back
		// This handles rename scenarios (Remove + Create) and atomic saves
		if fileType == FileTypeTask {
			w.debouncer.CancelDelete(taskID)
			w.logger.Debug("cancelled pending delete (file recreated)", "taskID", taskID)
		}
		w.debouncer.Trigger(taskID, fileType, path)
	}
}

// handleInitiativeFSEvent processes an initiative-related fsnotify event.
func (w *Watcher) handleInitiativeFSEvent(event fsnotify.Event, path string) {
	// Handle directory creation first
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			w.logger.Debug("new initiative directory detected, adding watch", "path", path)
			if err := w.fsWatcher.Add(path); err != nil {
				w.logger.Debug("failed to watch new directory", "path", path, "error", err)
			}
			return // Directories themselves don't trigger initiative events
		}
	}

	// Extract initiative ID from path
	initID := w.extractInitiativeID(path)
	if initID == "" {
		return
	}

	// Only care about initiative.yaml files
	if filepath.Base(path) != "initiative.yaml" {
		return
	}

	w.logger.Debug("initiative fs event",
		"op", event.Op.String(),
		"path", path,
		"initiativeID", initID,
	)

	// Handle file removal
	if event.Has(fsnotify.Remove) {
		w.removeHash(path)
		// Schedule verification after a short delay to handle rename scenarios
		w.initiativeDebouncer.TriggerDelete(initID, path)
		return
	}

	// For writes and creates, debounce and check for real changes
	if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
		w.initiativeDebouncer.CancelDelete(initID)
		w.logger.Debug("cancelled pending initiative delete (file recreated)", "initiativeID", initID)
		w.initiativeDebouncer.Trigger(initID, FileTypeInitiative, path)
	}
}

// handleDebouncedEvent processes a debounced event after the quiet period.
func (w *Watcher) handleDebouncedEvent(taskID string, fileType FileType, path string) {
	// Check if content actually changed
	changed, err := w.hasContentChanged(path)
	if err != nil {
		w.logger.Debug("failed to check content change", "path", path, "error", err)
		return
	}
	if !changed {
		w.logger.Debug("content unchanged, skipping event", "path", path)
		return
	}

	w.logger.Debug("publishing event", "taskID", taskID, "fileType", fileType)

	switch fileType {
	case FileTypeTask:
		w.publishTaskEvent(taskID)
	case FileTypeState:
		w.publishStateEvent(taskID)
	case FileTypePlan, FileTypeSpec:
		// For plan/spec, just refresh the task
		w.publishTaskEvent(taskID)
	}
}

// publishTaskEvent publishes a task created or updated event.
// It also detects weight changes and triggers plan regeneration.
func (w *Watcher) publishTaskEvent(taskID string) {
	// Load the task to get current data
	t, err := task.LoadFrom(w.workDir, taskID)
	if err != nil {
		w.logger.Debug("failed to load task for event", "taskID", taskID, "error", err)
		return
	}

	// Check if this is a new task (wasn't in our hash map before)
	taskPath := filepath.Join(w.tasksDir, taskID, "task.yaml")
	isNew := !w.hasHash(taskPath)

	var eventType events.EventType
	if isNew {
		eventType = events.EventTaskCreated
		// Record initial weight for new tasks
		w.setWeight(taskID, t.Weight)
	} else {
		eventType = events.EventTaskUpdated

		// Check for weight change and regenerate plan if needed
		if oldWeight, hasOldWeight := w.getWeight(taskID); hasOldWeight {
			if oldWeight != t.Weight {
				w.logger.Info("task weight changed",
					"taskID", taskID,
					"oldWeight", oldWeight,
					"newWeight", t.Weight,
				)

				// Only regenerate if task is not running
				if t.Status != task.StatusRunning {
					// Check if plan already matches new weight (API/CLI already regenerated)
					existingPlan, err := plan.LoadFrom(w.workDir, taskID)
					if err == nil && existingPlan.Weight == t.Weight {
						w.logger.Debug("plan already matches new weight, skipping regeneration",
							"taskID", taskID,
						)
					} else {
						// Plan doesn't exist or has wrong weight - regenerate
						result, err := plan.RegeneratePlanForTask(w.workDir, t)
						if err != nil {
							w.logger.Error("failed to regenerate plan for weight change",
								"taskID", taskID,
								"error", err,
							)
						} else {
							w.logger.Info("plan regenerated for weight change",
								"taskID", taskID,
								"preservedPhases", result.PreservedPhases,
								"resetPhases", result.ResetPhases,
							)
						}
					}
				} else {
					w.logger.Warn("skipping plan regeneration for running task",
						"taskID", taskID,
					)
				}

				// Update tracked weight
				w.setWeight(taskID, t.Weight)
			}
		} else {
			// First time seeing this task, record weight
			w.setWeight(taskID, t.Weight)
		}
	}

	w.publisher.Publish(events.NewEvent(eventType, taskID, map[string]any{
		"task": t,
	}))
}

// getWeight returns the tracked weight for a task.
func (w *Watcher) getWeight(taskID string) (task.Weight, bool) {
	w.weightsMu.RLock()
	defer w.weightsMu.RUnlock()
	weight, ok := w.weights[taskID]
	return weight, ok
}

// setWeight updates the tracked weight for a task.
func (w *Watcher) setWeight(taskID string, weight task.Weight) {
	w.weightsMu.Lock()
	defer w.weightsMu.Unlock()
	w.weights[taskID] = weight
}

// removeWeight removes the tracked weight for a task.
func (w *Watcher) removeWeight(taskID string) {
	w.weightsMu.Lock()
	defer w.weightsMu.Unlock()
	delete(w.weights, taskID)
}

// publishStateEvent publishes a state update event.
func (w *Watcher) publishStateEvent(taskID string) {
	// Load state to get current data
	statePath := filepath.Join(w.tasksDir, taskID, "state.yaml")
	data, err := os.ReadFile(statePath)
	if err != nil {
		w.logger.Debug("failed to read state for event", "taskID", taskID, "error", err)
		return
	}

	// Publish as a state event (reuse existing event type)
	w.publisher.Publish(events.NewEvent(events.EventState, taskID, map[string]any{
		"raw": string(data),
	}))
}

// publishTaskDeleted publishes a task deleted event.
func (w *Watcher) publishTaskDeleted(taskID string) {
	// Clean up weight tracking for deleted task
	w.removeWeight(taskID)

	w.publisher.Publish(events.NewEvent(events.EventTaskDeleted, taskID, map[string]any{
		"task_id": taskID,
	}))
}

// handleDebouncedInitiativeEvent processes a debounced initiative event.
func (w *Watcher) handleDebouncedInitiativeEvent(initID string, fileType FileType, path string) {
	// Check if content actually changed
	changed, err := w.hasContentChanged(path)
	if err != nil {
		w.logger.Debug("failed to check initiative content change", "path", path, "error", err)
		return
	}
	if !changed {
		w.logger.Debug("initiative content unchanged, skipping event", "path", path)
		return
	}

	w.logger.Debug("publishing initiative event", "initiativeID", initID)
	w.publishInitiativeEvent(initID)
}

// publishInitiativeEvent publishes an initiative created or updated event.
func (w *Watcher) publishInitiativeEvent(initID string) {
	// Load the initiative to get current data
	init, err := initiative.LoadFrom(filepath.Join(w.workDir, ".orc", "initiatives"), initID)
	if err != nil {
		w.logger.Debug("failed to load initiative for event", "initiativeID", initID, "error", err)
		return
	}

	// Check if this is a new initiative (wasn't in our hash map before)
	initPath := filepath.Join(w.initiativesDir, initID, "initiative.yaml")
	isNew := !w.hasHash(initPath)

	// Sync to database for external edits (file modified outside of CLI)
	if err := initiative.SyncToDB(w.workDir, init, w.logger); err != nil {
		w.logger.Warn("failed to sync initiative to database", "initiativeID", initID, "error", err)
	}

	var eventType events.EventType
	if isNew {
		eventType = events.EventInitiativeCreated
	} else {
		eventType = events.EventInitiativeUpdated
	}

	w.publisher.Publish(events.NewEvent(eventType, initID, map[string]any{
		"initiative": init,
	}))
}

// publishInitiativeDeleted publishes an initiative deleted event.
func (w *Watcher) publishInitiativeDeleted(initID string) {
	// Sync deletion to database for external deletions (file deleted outside of CLI)
	if err := initiative.DeleteFromDB(w.workDir, initID, w.logger); err != nil {
		w.logger.Warn("failed to delete initiative from database", "initiativeID", initID, "error", err)
	}

	w.publisher.Publish(events.NewEvent(events.EventInitiativeDeleted, initID, map[string]any{
		"initiative_id": initID,
	}))
}

// extractInitiativeID extracts the initiative ID from a file path.
// Returns empty string if the path is not an initiative file or has an invalid ID.
func (w *Watcher) extractInitiativeID(path string) string {
	// Path should be like: .orc/initiatives/INIT-001/initiative.yaml
	rel, err := filepath.Rel(w.initiativesDir, path)
	if err != nil {
		return ""
	}

	// First component is the initiative ID
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}

	initID := parts[0]
	// Validate the initiative ID format strictly to prevent path traversal
	if err := initiative.ValidateID(initID); err != nil {
		return ""
	}

	return initID
}

// extractTaskID extracts the task ID from a file path.
// Returns empty string if the path is not a task file.
func (w *Watcher) extractTaskID(path string) string {
	// Path should be like: .orc/tasks/TASK-001/task.yaml
	rel, err := filepath.Rel(w.tasksDir, path)
	if err != nil {
		return ""
	}

	// First component is the task ID
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}

	taskID := parts[0]
	// Validate it looks like a task ID
	if !strings.HasPrefix(taskID, "TASK-") {
		return ""
	}

	return taskID
}

// classifyFile determines the type of file from its path.
func (w *Watcher) classifyFile(path string) FileType {
	base := filepath.Base(path)

	switch base {
	case "task.yaml":
		return FileTypeTask
	case "state.yaml":
		return FileTypeState
	case "plan.yaml":
		return FileTypePlan
	case "spec.md":
		return FileTypeSpec
	default:
		return FileTypeUnknown
	}
}

// hasContentChanged checks if the file content has changed since last check.
// Updates the stored hash if changed.
func (w *Watcher) hasContentChanged(path string) (bool, error) {
	newHash, err := w.hashFile(path)
	if err != nil {
		return false, err
	}

	w.hashesMu.Lock()
	defer w.hashesMu.Unlock()

	oldHash, exists := w.hashes[path]
	if exists && oldHash == newHash {
		return false, nil
	}

	w.hashes[path] = newHash
	return true, nil
}

// hasHash checks if we have a hash for the given path.
func (w *Watcher) hasHash(path string) bool {
	w.hashesMu.RLock()
	defer w.hashesMu.RUnlock()
	_, exists := w.hashes[path]
	return exists
}

// removeHash removes the hash for a path.
func (w *Watcher) removeHash(path string) {
	w.hashesMu.Lock()
	defer w.hashesMu.Unlock()
	delete(w.hashes, path)
}

// hashFile computes the SHA256 hash of a file's contents.
func (w *Watcher) hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
