package project

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// MigrateIfNeeded checks if a project still has runtime state in <project>/.orc/
// and moves it to ~/.orc/projects/<id>/ and ~/.orc/worktrees/<id>/.
// Returns true if migration was performed.
func MigrateIfNeeded(projectPath, projectID string) (bool, error) {
	oldDBPath := filepath.Join(projectPath, ".orc", "orc.db")
	if _, err := os.Stat(oldDBPath); os.IsNotExist(err) {
		return false, nil // No old DB, nothing to migrate
	}

	newDBPath, err := ProjectDBPath(projectID)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(newDBPath); err == nil {
		return false, nil // New DB already exists, already migrated
	}

	// Ensure target directories exist
	if err := EnsureProjectDirs(projectID); err != nil {
		return false, fmt.Errorf("create project dirs: %w", err)
	}

	slog.Info("migrating project data to ~/.orc/",
		"project_id", projectID,
		"from", filepath.Join(projectPath, ".orc"),
	)

	// Move database files
	if err := migrateDBFiles(projectPath, projectID); err != nil {
		return false, fmt.Errorf("migrate database: %w", err)
	}

	// Move local config
	migrateLocalConfig(projectPath, projectID)

	// Move local prompts
	migrateLocalPrompts(projectPath, projectID)

	// Move sequences
	migrateSequences(projectPath, projectID)

	// Move exports
	migrateExports(projectPath, projectID)

	// Move worktrees (requires git worktree repair)
	migrateWorktrees(projectPath, projectID)

	// Clean up empty legacy directories
	cleanupLegacyDirs(projectPath)

	return true, nil
}

// migrateDBFiles moves orc.db and its journal/wal/shm files.
func migrateDBFiles(projectPath, projectID string) error {
	newDBPath, err := ProjectDBPath(projectID)
	if err != nil {
		return err
	}

	dbFiles := []string{"orc.db", "orc.db-journal", "orc.db-wal", "orc.db-shm"}
	for _, f := range dbFiles {
		src := filepath.Join(projectPath, ".orc", f)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(filepath.Dir(newDBPath), f)
		if err := moveFile(src, dst); err != nil {
			return fmt.Errorf("move %s: %w", f, err)
		}
	}
	return nil
}

// migrateLocalConfig moves .orc/local/config.yaml to ~/.orc/projects/<id>/config.yaml.
func migrateLocalConfig(projectPath, projectID string) {
	src := filepath.Join(projectPath, ".orc", "local", "config.yaml")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	dst, err := ProjectLocalConfigPath(projectID)
	if err != nil {
		slog.Warn("skip local config migration", "error", err)
		return
	}
	if err := moveFile(src, dst); err != nil {
		slog.Warn("failed to migrate local config", "error", err)
	}
}

// migrateLocalPrompts moves .orc/local/prompts/ to ~/.orc/projects/<id>/prompts/.
func migrateLocalPrompts(projectPath, projectID string) {
	src := filepath.Join(projectPath, ".orc", "local", "prompts")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	dst, err := ProjectLocalPromptsDir(projectID)
	if err != nil {
		slog.Warn("skip local prompts migration", "error", err)
		return
	}
	if err := moveDir(src, dst); err != nil {
		slog.Warn("failed to migrate local prompts", "error", err)
	}
}

// migrateSequences moves .orc/local/sequences.yaml to ~/.orc/projects/<id>/sequences.yaml.
func migrateSequences(projectPath, projectID string) {
	src := filepath.Join(projectPath, ".orc", "local", "sequences.yaml")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	dst, err := ProjectSequencesPath(projectID)
	if err != nil {
		slog.Warn("skip sequences migration", "error", err)
		return
	}
	if err := moveFile(src, dst); err != nil {
		slog.Warn("failed to migrate sequences", "error", err)
	}
}

// migrateExports moves .orc/exports/ to ~/.orc/projects/<id>/exports/.
func migrateExports(projectPath, projectID string) {
	src := filepath.Join(projectPath, ".orc", "exports")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}
	dst, err := ProjectExportDir(projectID)
	if err != nil {
		slog.Warn("skip exports migration", "error", err)
		return
	}
	if err := moveDir(src, dst); err != nil {
		slog.Warn("failed to migrate exports", "error", err)
	}
}

// migrateWorktrees moves .orc/worktrees/ to ~/.orc/worktrees/<id>/ and repairs git references.
func migrateWorktrees(projectPath, projectID string) {
	src := filepath.Join(projectPath, ".orc", "worktrees")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return
	}

	entries, err := os.ReadDir(src)
	if err != nil || len(entries) == 0 {
		// Empty or unreadable — just remove it
		_ = os.RemoveAll(src)
		return
	}

	dst, err := ProjectWorktreeDir(projectID)
	if err != nil {
		slog.Warn("skip worktree migration", "error", err)
		return
	}

	if err := moveDir(src, dst); err != nil {
		slog.Warn("failed to migrate worktrees", "error", err)
		return
	}

	// git worktree repair updates git's internal worktree references
	// after we moved them to a new location
	cmd := exec.Command("git", "worktree", "repair")
	cmd.Dir = projectPath
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("git worktree repair failed (worktrees may need manual repair)",
			"error", err,
			"output", string(out),
		)
	}
}

// cleanupLegacyDirs removes empty legacy directories from <project>/.orc/.
func cleanupLegacyDirs(projectPath string) {
	legacyDirs := []string{
		filepath.Join(projectPath, ".orc", "tasks"),
		filepath.Join(projectPath, ".orc", "worktrees"),
		filepath.Join(projectPath, ".orc", "local"),
		filepath.Join(projectPath, ".orc", "exports"),
		filepath.Join(projectPath, ".orc", "shared"),
	}
	for _, dir := range legacyDirs {
		// os.Remove only removes empty directories, safe to call
		_ = os.Remove(dir)
	}
}

// moveFile moves a file from src to dst. Falls back to copy+delete if rename fails
// (which happens when src and dst are on different filesystems).
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Try rename first (atomic, same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + delete (cross-filesystem)
	return copyAndDelete(src, dst)
}

// moveDir moves a directory from src to dst.
func moveDir(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Try rename first
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to recursive copy + delete
	if err := copyDirRecursive(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyAndDelete(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	info, err := in.Stat()
	if err != nil {
		_ = in.Close()
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		_ = in.Close()
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = in.Close()
		_ = out.Close()
		return err
	}

	// Close before deleting
	if err := in.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	return os.Remove(src)
}

func copyDirRecursive(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)
	return err
}
