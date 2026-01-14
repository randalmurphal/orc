// Package initiative provides initiative/feature grouping for related tasks.
package initiative

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/randalmurphal/orc/internal/db"
)

// Store provides hybrid storage for initiatives (YAML + DB cache).
// It handles auto-commit, DB sync, and recovery.
type Store struct {
	projectRoot string
	db          *db.ProjectDB
	logger      *slog.Logger
	mu          sync.RWMutex
	shared      bool

	// Git configuration
	commitPrefix string
	autoCommit   bool
}

// StoreConfig configures the initiative store.
type StoreConfig struct {
	ProjectRoot  string
	Shared       bool
	CommitPrefix string
	AutoCommit   bool
	Logger       *slog.Logger
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		CommitPrefix: "[orc]",
		AutoCommit:   true,
	}
}

// NewStore creates a new initiative store.
func NewStore(cfg StoreConfig) (*Store, error) {
	if cfg.ProjectRoot == "" {
		return nil, fmt.Errorf("project root is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Open project database
	pdb, err := db.OpenProject(cfg.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("open project database: %w", err)
	}

	if cfg.CommitPrefix == "" {
		cfg.CommitPrefix = "[orc]"
	}

	return &Store{
		projectRoot:  cfg.ProjectRoot,
		db:           pdb,
		logger:       logger,
		shared:       cfg.Shared,
		commitPrefix: cfg.CommitPrefix,
		autoCommit:   cfg.AutoCommit,
	}, nil
}

// Close releases database resources.
func (s *Store) Close() error {
	return s.db.Close()
}

// Save persists an initiative to YAML, syncs to DB, and optionally commits to git.
func (s *Store) Save(init *Initiative) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Save to YAML (source of truth)
	var err error
	if s.shared {
		err = init.SaveShared()
	} else {
		err = init.Save()
	}
	if err != nil {
		return fmt.Errorf("save initiative to YAML: %w", err)
	}

	// 2. Sync to database cache
	if syncErr := s.syncToDB(init); syncErr != nil {
		// Log but don't fail - YAML is source of truth
		s.logger.Warn("failed to sync initiative to database", "id", init.ID, "error", syncErr)
	}

	// 3. Auto-commit if enabled
	if s.autoCommit {
		if commitErr := s.commitInitiative(init, "save"); commitErr != nil {
			// Log but don't fail - file is saved
			s.logger.Warn("failed to commit initiative", "id", init.ID, "error", commitErr)
		}
	}

	return nil
}

// Load loads an initiative from YAML.
func (s *Store) Load(id string) (*Initiative, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.shared {
		return LoadShared(id)
	}
	return Load(id)
}

// List lists all initiatives from YAML.
func (s *Store) List() ([]*Initiative, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return List(s.shared)
}

// Delete removes an initiative from YAML, DB, and commits the deletion.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get initiative path before deletion for commit
	baseDir := GetInitiativesDir(s.shared)
	initDir := filepath.Join(baseDir, id)

	// 1. Delete from YAML
	if err := Delete(id, s.shared); err != nil {
		return fmt.Errorf("delete initiative: %w", err)
	}

	// 2. Delete from database cache
	if err := s.db.DeleteInitiative(id); err != nil {
		s.logger.Warn("failed to delete initiative from database", "id", id, "error", err)
	}

	// 3. Auto-commit deletion if enabled
	if s.autoCommit {
		if err := s.commitDeletion(id, initDir); err != nil {
			s.logger.Warn("failed to commit initiative deletion", "id", id, "error", err)
		}
	}

	return nil
}

// syncToDB syncs an initiative to the database cache.
func (s *Store) syncToDB(init *Initiative) error {
	// Convert to DB model
	dbInit := &db.Initiative{
		ID:               init.ID,
		Title:            init.Title,
		Status:           string(init.Status),
		OwnerInitials:    init.Owner.Initials,
		OwnerDisplayName: init.Owner.DisplayName,
		OwnerEmail:       init.Owner.Email,
		Vision:           init.Vision,
		CreatedAt:        init.CreatedAt,
		UpdatedAt:        init.UpdatedAt,
	}

	if err := s.db.SaveInitiative(dbInit); err != nil {
		return fmt.Errorf("save initiative: %w", err)
	}

	// Sync decisions
	for _, dec := range init.Decisions {
		dbDec := &db.InitiativeDecision{
			ID:           dec.ID,
			InitiativeID: init.ID,
			Decision:     dec.Decision,
			Rationale:    dec.Rationale,
			DecidedBy:    dec.By,
			DecidedAt:    dec.Date,
		}
		if err := s.db.AddInitiativeDecision(dbDec); err != nil {
			s.logger.Debug("failed to sync decision", "id", dec.ID, "error", err)
		}
	}

	// Sync tasks
	for seq, taskRef := range init.Tasks {
		if err := s.db.AddTaskToInitiative(init.ID, taskRef.ID, seq); err != nil {
			s.logger.Debug("failed to sync task link", "task_id", taskRef.ID, "error", err)
		}
	}

	// Sync blocked_by dependencies
	if err := s.syncBlockedByToDB(init); err != nil {
		s.logger.Debug("failed to sync blocked_by", "id", init.ID, "error", err)
	}

	return nil
}

// syncBlockedByToDB syncs the blocked_by field to the database.
func (s *Store) syncBlockedByToDB(init *Initiative) error {
	// Clear existing dependencies
	if err := s.db.ClearInitiativeDependencies(init.ID); err != nil {
		return fmt.Errorf("clear dependencies: %w", err)
	}

	// Add new dependencies
	for _, blockerID := range init.BlockedBy {
		if err := s.db.AddInitiativeDependency(init.ID, blockerID); err != nil {
			return fmt.Errorf("add dependency %s: %w", blockerID, err)
		}
	}

	return nil
}

// commitInitiative commits an initiative file to git.
func (s *Store) commitInitiative(init *Initiative, action string) error {
	baseDir := GetInitiativesDir(s.shared)
	initPath := filepath.Join(baseDir, init.ID, "initiative.yaml")

	// Stage the file
	if err := s.gitAdd(initPath); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Commit
	msg := fmt.Sprintf("%s initiative %s: %s - %s", s.commitPrefix, init.ID, action, init.Title)
	if err := s.gitCommit(msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// commitDeletion commits an initiative deletion to git.
func (s *Store) commitDeletion(id, path string) error {
	// Stage the deletion (the directory was already removed)
	if err := s.gitAdd(path); err != nil {
		// If the path doesn't exist, try staging all changes
		if err := s.gitAddAll(); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
	}

	// Commit
	msg := fmt.Sprintf("%s initiative %s: deleted", s.commitPrefix, id)
	if err := s.gitCommit(msg); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return nil
}

// gitAdd stages a file.
func (s *Store) gitAdd(path string) error {
	cmd := exec.Command("git", "-C", s.projectRoot, "add", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add %s: %s: %w", path, string(output), err)
	}
	return nil
}

// gitAddAll stages all changes.
func (s *Store) gitAddAll() error {
	cmd := exec.Command("git", "-C", s.projectRoot, "add", "-A")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add -A: %s: %w", string(output), err)
	}
	return nil
}

// gitCommit creates a commit.
func (s *Store) gitCommit(msg string) error {
	cmd := exec.Command("git", "-C", s.projectRoot, "commit", "-m", msg, "--allow-empty")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if nothing to commit
		if strings.Contains(string(output), "nothing to commit") ||
			strings.Contains(string(output), "no changes added") {
			return nil
		}
		return fmt.Errorf("git commit: %s: %w", string(output), err)
	}
	return nil
}

// RebuildIndex rebuilds the database index from YAML files.
// Use this when the database is missing or corrupted.
func (s *Store) RebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	initiatives, err := List(s.shared)
	if err != nil {
		return fmt.Errorf("list initiatives: %w", err)
	}

	s.logger.Info("rebuilding initiative index", "count", len(initiatives))

	for _, init := range initiatives {
		if err := s.syncToDB(init); err != nil {
			s.logger.Warn("failed to sync initiative to database", "id", init.ID, "error", err)
		}
	}

	return nil
}

// RecoverFromDB regenerates YAML files from the database.
// Use this when YAML files are missing but database has data.
func (s *Store) RecoverFromDB(id string) (*Initiative, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load from database
	dbInit, err := s.db.GetInitiative(id)
	if err != nil {
		return nil, fmt.Errorf("get initiative from database: %w", err)
	}
	if dbInit == nil {
		return nil, fmt.Errorf("initiative %s not found in database", id)
	}

	// Load decisions
	dbDecisions, err := s.db.GetInitiativeDecisions(id)
	if err != nil {
		return nil, fmt.Errorf("get decisions from database: %w", err)
	}

	// Load task links
	taskIDs, err := s.db.GetInitiativeTasks(id)
	if err != nil {
		return nil, fmt.Errorf("get task links from database: %w", err)
	}

	// Load blocked_by
	blockedBy, err := s.db.GetInitiativeDependencies(id)
	if err != nil {
		s.logger.Warn("failed to get dependencies from database", "id", id, "error", err)
		blockedBy = nil
	}

	// Reconstruct Initiative
	init := &Initiative{
		Version:   1,
		ID:        dbInit.ID,
		Title:     dbInit.Title,
		Status:    Status(dbInit.Status),
		Owner:     Identity{Initials: dbInit.OwnerInitials, DisplayName: dbInit.OwnerDisplayName, Email: dbInit.OwnerEmail},
		Vision:    dbInit.Vision,
		BlockedBy: blockedBy,
		CreatedAt: dbInit.CreatedAt,
		UpdatedAt: dbInit.UpdatedAt,
	}

	// Add decisions
	for _, dbDec := range dbDecisions {
		init.Decisions = append(init.Decisions, Decision{
			ID:        dbDec.ID,
			Date:      dbDec.DecidedAt,
			By:        dbDec.DecidedBy,
			Decision:  dbDec.Decision,
			Rationale: dbDec.Rationale,
		})
	}

	// Add task references (title will need to be fetched from task files)
	for _, taskID := range taskIDs {
		init.Tasks = append(init.Tasks, TaskRef{
			ID:     taskID,
			Status: "pending", // Default, actual status comes from task files
		})
	}

	// Save to YAML
	if s.shared {
		err = init.SaveShared()
	} else {
		err = init.Save()
	}
	if err != nil {
		return nil, fmt.Errorf("save recovered initiative: %w", err)
	}

	s.logger.Info("recovered initiative from database", "id", id)

	return init, nil
}

// RecoverAllFromDB regenerates all YAML files from the database.
func (s *Store) RecoverAllFromDB() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// List all initiatives in database
	dbInits, err := s.db.ListInitiatives(db.ListOpts{})
	if err != nil {
		return 0, fmt.Errorf("list initiatives from database: %w", err)
	}

	recovered := 0
	for _, dbInit := range dbInits {
		// Check if YAML already exists
		if Exists(dbInit.ID, s.shared) {
			continue
		}

		// Recover
		s.mu.Unlock()
		_, err := s.RecoverFromDB(dbInit.ID)
		s.mu.Lock()
		if err != nil {
			s.logger.Warn("failed to recover initiative", "id", dbInit.ID, "error", err)
			continue
		}
		recovered++
	}

	return recovered, nil
}

// EnsureYAMLExists checks if an initiative's YAML file exists, and recovers from DB if not.
// Returns true if recovery was needed.
func (s *Store) EnsureYAMLExists(id string) (bool, error) {
	if Exists(id, s.shared) {
		return false, nil
	}

	// Try to recover from database
	_, err := s.RecoverFromDB(id)
	if err != nil {
		return false, fmt.Errorf("recover initiative %s: %w", id, err)
	}

	return true, nil
}

// EnsureDBExists checks if an initiative is indexed in DB, and indexes from YAML if not.
// Returns true if indexing was needed.
func (s *Store) EnsureDBExists(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check DB
	dbInit, err := s.db.GetInitiative(id)
	if err != nil {
		return false, fmt.Errorf("check database: %w", err)
	}
	if dbInit != nil {
		return false, nil
	}

	// Load from YAML and sync to DB
	var init *Initiative
	if s.shared {
		init, err = LoadShared(id)
	} else {
		init, err = Load(id)
	}
	if err != nil {
		return false, fmt.Errorf("load initiative: %w", err)
	}

	if err := s.syncToDB(init); err != nil {
		return false, fmt.Errorf("sync to database: %w", err)
	}

	return true, nil
}

// SyncFromYAML syncs a single initiative from YAML to DB.
// Use this when YAML was modified externally.
func (s *Store) SyncFromYAML(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var init *Initiative
	var err error
	if s.shared {
		init, err = LoadShared(id)
	} else {
		init, err = Load(id)
	}
	if err != nil {
		return fmt.Errorf("load initiative: %w", err)
	}

	if err := s.syncToDB(init); err != nil {
		return fmt.Errorf("sync to database: %w", err)
	}

	s.logger.Debug("synced initiative from YAML", "id", id)
	return nil
}

// GetInitiativesPath returns the full path to the initiatives directory.
func (s *Store) GetInitiativesPath() string {
	baseDir := GetInitiativesDir(s.shared)
	return filepath.Join(s.projectRoot, baseDir)
}

// IsAutoCommitEnabled returns whether auto-commit is enabled.
func (s *Store) IsAutoCommitEnabled() bool {
	return s.autoCommit
}

// SetAutoCommit enables or disables auto-commit.
func (s *Store) SetAutoCommit(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.autoCommit = enabled
}

// CommitAll commits all initiative files that have unstaged changes.
func (s *Store) CommitAll(message string) error {
	baseDir := GetInitiativesDir(s.shared)
	fullPath := filepath.Join(s.projectRoot, baseDir)

	// Check if directory exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil // Nothing to commit
	}

	// Stage all initiative files
	if err := s.gitAdd(fullPath); err != nil {
		return fmt.Errorf("stage initiative files: %w", err)
	}

	// Commit
	msg := fmt.Sprintf("%s %s", s.commitPrefix, message)
	if err := s.gitCommit(msg); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
