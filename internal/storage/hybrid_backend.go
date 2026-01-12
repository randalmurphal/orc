package storage

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// HybridBackend uses YAML files as the source of truth with SQLite cache
// for FTS search and queries. This is the default storage mode.
// All file operations are protected by a mutex for concurrent access safety.
type HybridBackend struct {
	projectPath string
	db          *db.ProjectDB
	cfg         *config.StorageConfig
	mu          sync.RWMutex
	logger      *log.Logger
}

// NewHybridBackend creates a new hybrid storage backend.
func NewHybridBackend(projectPath string, cfg *config.StorageConfig) (*HybridBackend, error) {
	pdb, err := db.OpenProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("open project database: %w", err)
	}

	// Create a logger that discards output by default
	// Callers can replace this with a proper logger if needed
	logger := log.New(io.Discard, "", 0)

	return &HybridBackend{
		projectPath: projectPath,
		db:          pdb,
		cfg:         cfg,
		logger:      logger,
	}, nil
}

// SetLogger sets the logger for warnings and debug messages.
func (h *HybridBackend) SetLogger(l *log.Logger) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.logger = l
}

// SaveTask saves a task to both files and database.
func (h *HybridBackend) SaveTask(t *task.Task) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Primary: save to file
	taskDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir, t.ID)
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return fmt.Errorf("create task directory: %w", err)
	}

	if err := t.SaveTo(taskDir); err != nil {
		return fmt.Errorf("save task to file: %w", err)
	}

	// Secondary: sync to database cache (convert types)
	dbTask := &db.Task{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Weight:       string(t.Weight),
		Status:       string(t.Status),
		CurrentPhase: t.CurrentPhase,
		Branch:       t.Branch,
		CreatedAt:    t.CreatedAt,
		StartedAt:    t.StartedAt,
		CompletedAt:  t.CompletedAt,
	}
	if err := h.db.SaveTask(dbTask); err != nil {
		// Log but don't fail - files are source of truth
		h.logger.Printf("warning: failed to sync task to database: %v", err)
	}

	return nil
}

// LoadTask loads a task from files.
func (h *HybridBackend) LoadTask(id string) (*task.Task, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	taskDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir, id)
	taskPath := filepath.Join(taskDir, "task.yaml")

	// Load from file (source of truth)
	if _, err := os.Stat(taskPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return nil, fmt.Errorf("check task %s: %w", id, err)
	}

	return task.Load(id)
}

// LoadAllTasks loads all tasks from files.
func (h *HybridBackend) LoadAllTasks() ([]*task.Task, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	tasksDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir)
	return task.LoadAllFrom(tasksDir)
}

// DeleteTask removes a task from both files and database.
func (h *HybridBackend) DeleteTask(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Delete from files
	taskDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir, id)
	if err := os.RemoveAll(taskDir); err != nil {
		return fmt.Errorf("delete task files: %w", err)
	}

	// Delete from database cache
	if err := h.db.DeleteTask(id); err != nil {
		h.logger.Printf("warning: failed to delete task from database: %v", err)
	}

	return nil
}

// SaveState saves state to files.
func (h *HybridBackend) SaveState(s *state.State) error {
	stateDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir, s.TaskID)
	return s.SaveTo(stateDir)
}

// LoadState loads state from files.
func (h *HybridBackend) LoadState(taskID string) (*state.State, error) {
	return state.Load(taskID)
}

// SavePlan saves plan to files.
func (h *HybridBackend) SavePlan(p *plan.Plan, taskID string) error {
	planDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir, taskID)
	return p.SaveTo(planDir)
}

// LoadPlan loads plan from files.
func (h *HybridBackend) LoadPlan(taskID string) (*plan.Plan, error) {
	return plan.Load(taskID)
}

// AddTranscript adds a transcript to database (for FTS).
func (h *HybridBackend) AddTranscript(t *Transcript) error {
	if !h.cfg.Database.CacheTranscripts {
		return nil
	}

	dbTranscript := &db.Transcript{
		TaskID:  t.TaskID,
		Phase:   t.Phase,
		Content: t.Content,
	}
	if err := h.db.AddTranscript(dbTranscript); err != nil {
		return fmt.Errorf("add transcript to database: %w", err)
	}
	t.ID = dbTranscript.ID

	return nil
}

// GetTranscripts retrieves transcripts for a task.
func (h *HybridBackend) GetTranscripts(taskID string) ([]Transcript, error) {
	dbTranscripts, err := h.db.GetTranscripts(taskID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts: %w", err)
	}

	result := make([]Transcript, len(dbTranscripts))
	for i, t := range dbTranscripts {
		result[i] = Transcript{
			ID:        t.ID,
			TaskID:    t.TaskID,
			Phase:     t.Phase,
			Content:   t.Content,
			Timestamp: t.Timestamp.Unix(),
		}
	}
	return result, nil
}

// SearchTranscripts performs FTS search across transcripts.
func (h *HybridBackend) SearchTranscripts(query string) ([]TranscriptMatch, error) {
	dbMatches, err := h.db.SearchTranscripts(query)
	if err != nil {
		return nil, fmt.Errorf("search transcripts: %w", err)
	}

	result := make([]TranscriptMatch, len(dbMatches))
	for i, m := range dbMatches {
		result[i] = TranscriptMatch{
			TaskID:  m.TaskID,
			Phase:   m.Phase,
			Snippet: m.Snippet,
			Rank:    m.Rank,
		}
	}
	return result, nil
}

// MaterializeContext is not needed for hybrid mode - files are always present.
func (h *HybridBackend) MaterializeContext(taskID, outputPath string) error {
	// In hybrid mode, files are already in place
	return nil
}

// NeedsMaterialization returns false for hybrid mode.
func (h *HybridBackend) NeedsMaterialization() bool {
	return false
}

// Sync flushes any pending operations.
func (h *HybridBackend) Sync() error {
	// Files are written synchronously, nothing to sync
	return nil
}

// Cleanup removes old data based on retention policy.
func (h *HybridBackend) Cleanup() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Clean up completed tasks if configured
	if h.cfg.Files.CleanupOnComplete {
		tasksDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir)
		tasks, err := task.LoadAllFrom(tasksDir)
		if err != nil {
			return fmt.Errorf("load tasks for cleanup: %w", err)
		}

		for _, t := range tasks {
			if t.Status == task.StatusCompleted {
				// Check if task is old enough to clean up
				if t.CompletedAt != nil {
					age := time.Since(*t.CompletedAt)
					if age > 24*time.Hour { // Only cleanup tasks completed >24h ago
						taskDir := filepath.Join(h.projectPath, task.OrcDir, task.TasksDir, t.ID)
						if err := os.RemoveAll(taskDir); err != nil {
							h.logger.Printf("warning: failed to cleanup task %s: %v", t.ID, err)
						}
					}
				}
			}
		}
	}

	return nil
}

// Close releases database resources.
func (h *HybridBackend) Close() error {
	return h.db.Close()
}

// Ensure HybridBackend implements Backend
var _ Backend = (*HybridBackend)(nil)
