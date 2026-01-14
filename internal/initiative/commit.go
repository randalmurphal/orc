// Package initiative provides initiative/feature grouping for related tasks.
package initiative

import (
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// CommitConfig configures the auto-commit behavior.
type CommitConfig struct {
	ProjectRoot  string
	CommitPrefix string
	Logger       *slog.Logger
}

// DefaultCommitConfig returns sensible defaults.
func DefaultCommitConfig() CommitConfig {
	return CommitConfig{
		CommitPrefix: "[orc]",
	}
}

// CommitAndSync commits an initiative file to git and syncs to the database.
// This should be called after any initiative modification via CLI.
func CommitAndSync(init *Initiative, action string, cfg CommitConfig) error {
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		var err error
		projectRoot, err = config.FindProjectRoot()
		if err != nil {
			return fmt.Errorf("find project root: %w", err)
		}
	}

	commitPrefix := cfg.CommitPrefix
	if commitPrefix == "" {
		commitPrefix = "[orc]"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// 1. Stage and commit the initiative file
	initPath := filepath.Join(projectRoot, GetInitiativesDir(false), init.ID, "initiative.yaml")
	if err := gitAdd(projectRoot, initPath); err != nil {
		logger.Warn("failed to stage initiative file", "id", init.ID, "error", err)
	} else {
		msg := fmt.Sprintf("%s initiative %s: %s - %s", commitPrefix, init.ID, action, init.Title)
		if err := gitCommit(projectRoot, msg); err != nil {
			logger.Warn("failed to commit initiative", "id", init.ID, "error", err)
		} else {
			logger.Debug("committed initiative", "id", init.ID, "action", action)
		}
	}

	// 2. Sync to database
	if err := SyncToDB(projectRoot, init, logger); err != nil {
		logger.Warn("failed to sync initiative to database", "id", init.ID, "error", err)
	} else {
		logger.Debug("synced initiative to database", "id", init.ID)
	}

	return nil
}

// CommitDeletion commits an initiative deletion to git and removes from DB.
func CommitDeletion(id string, cfg CommitConfig) error {
	projectRoot := cfg.ProjectRoot
	if projectRoot == "" {
		var err error
		projectRoot, err = config.FindProjectRoot()
		if err != nil {
			return fmt.Errorf("find project root: %w", err)
		}
	}

	commitPrefix := cfg.CommitPrefix
	if commitPrefix == "" {
		commitPrefix = "[orc]"
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// 1. Stage and commit the deletion
	initPath := filepath.Join(projectRoot, GetInitiativesDir(false), id)
	if err := gitAdd(projectRoot, initPath); err != nil {
		// Directory already deleted, try staging all changes
		if err := gitAddAll(projectRoot); err != nil {
			logger.Warn("failed to stage initiative deletion", "id", id, "error", err)
		}
	}

	msg := fmt.Sprintf("%s initiative %s: deleted", commitPrefix, id)
	if err := gitCommit(projectRoot, msg); err != nil {
		logger.Warn("failed to commit initiative deletion", "id", id, "error", err)
	} else {
		logger.Debug("committed initiative deletion", "id", id)
	}

	// 2. Remove from database
	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		logger.Warn("failed to open database for deletion sync", "error", err)
		return nil
	}
	defer pdb.Close()

	if err := pdb.DeleteInitiative(id); err != nil {
		logger.Warn("failed to delete initiative from database", "id", id, "error", err)
	}

	return nil
}

// DeleteFromDB removes an initiative from the database cache.
// This is used by the file watcher when external deletions are detected.
func DeleteFromDB(projectRoot, id string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pdb.Close()

	if err := pdb.DeleteInitiative(id); err != nil {
		return fmt.Errorf("delete initiative from database: %w", err)
	}

	logger.Debug("deleted initiative from database", "id", id)
	return nil
}

// SyncToDB syncs an initiative to the database cache.
// This is used by CLI commands via CommitAndSync and by the file watcher
// when external edits are detected.
func SyncToDB(projectRoot string, init *Initiative, logger *slog.Logger) error {
	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer pdb.Close()

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

	if err := pdb.SaveInitiative(dbInit); err != nil {
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
		if err := pdb.AddInitiativeDecision(dbDec); err != nil {
			logger.Debug("failed to sync decision", "id", dec.ID, "error", err)
		}
	}

	// Sync tasks
	for seq, taskRef := range init.Tasks {
		if err := pdb.AddTaskToInitiative(init.ID, taskRef.ID, seq); err != nil {
			logger.Debug("failed to sync task link", "task_id", taskRef.ID, "error", err)
		}
	}

	// Sync blocked_by dependencies
	if err := pdb.ClearInitiativeDependencies(init.ID); err != nil {
		logger.Debug("failed to clear dependencies", "id", init.ID, "error", err)
	}
	for _, blockerID := range init.BlockedBy {
		if err := pdb.AddInitiativeDependency(init.ID, blockerID); err != nil {
			logger.Debug("failed to add dependency", "id", init.ID, "blocker", blockerID, "error", err)
		}
	}

	return nil
}

// gitAdd stages a file.
func gitAdd(projectRoot, path string) error {
	cmd := exec.Command("git", "-C", projectRoot, "add", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add %s: %s: %w", path, string(output), err)
	}
	return nil
}

// gitAddAll stages all changes.
func gitAddAll(projectRoot string) error {
	cmd := exec.Command("git", "-C", projectRoot, "add", "-A")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add -A: %s: %w", string(output), err)
	}
	return nil
}

// gitCommit creates a commit.
func gitCommit(projectRoot, msg string) error {
	cmd := exec.Command("git", "-C", projectRoot, "commit", "-m", msg)
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

// RebuildDBIndex rebuilds the database index from all YAML files.
// Use this when the database is missing or corrupted.
func RebuildDBIndex(projectRoot string, shared bool, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	initiatives, err := List(shared)
	if err != nil {
		return fmt.Errorf("list initiatives: %w", err)
	}

	logger.Info("rebuilding initiative index", "count", len(initiatives))

	for _, init := range initiatives {
		if err := SyncToDB(projectRoot, init, logger); err != nil {
			logger.Warn("failed to sync initiative to database", "id", init.ID, "error", err)
		}
	}

	return nil
}

// RecoverFromDB regenerates a YAML file from the database.
// Use this when YAML file is missing but database has data.
func RecoverFromDB(projectRoot, id string, shared bool, logger *slog.Logger) (*Initiative, error) {
	if logger == nil {
		logger = slog.Default()
	}

	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	defer pdb.Close()

	// Load from database
	dbInit, err := pdb.GetInitiative(id)
	if err != nil {
		return nil, fmt.Errorf("get initiative from database: %w", err)
	}
	if dbInit == nil {
		return nil, fmt.Errorf("initiative %s not found in database", id)
	}

	// Load decisions
	dbDecisions, err := pdb.GetInitiativeDecisions(id)
	if err != nil {
		return nil, fmt.Errorf("get decisions from database: %w", err)
	}

	// Load task links
	taskIDs, err := pdb.GetInitiativeTasks(id)
	if err != nil {
		return nil, fmt.Errorf("get task links from database: %w", err)
	}

	// Load blocked_by
	blockedBy, err := pdb.GetInitiativeDependencies(id)
	if err != nil {
		logger.Warn("failed to get dependencies from database", "id", id, "error", err)
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
	if shared {
		err = init.SaveShared()
	} else {
		err = init.Save()
	}
	if err != nil {
		return nil, fmt.Errorf("save recovered initiative: %w", err)
	}

	logger.Info("recovered initiative from database", "id", id)

	return init, nil
}
