package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectDataDir returns the runtime data directory for a project.
// Path: ~/.orc/projects/<project-id>/
func ProjectDataDir(projectID string) (string, error) {
	globalDir, err := GlobalPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalDir, "projects", projectID), nil
}

// ProjectDBPath returns the database path for a project.
// Path: ~/.orc/projects/<project-id>/orc.db
func ProjectDBPath(projectID string) (string, error) {
	dataDir, err := ProjectDataDir(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "orc.db"), nil
}

// ProjectWorktreeDir returns the worktree directory for a project.
// Path: ~/.orc/worktrees/<project-id>/
func ProjectWorktreeDir(projectID string) (string, error) {
	globalDir, err := GlobalPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalDir, "worktrees", projectID), nil
}

// ProjectExportDir returns the export directory for a project.
// Path: ~/.orc/projects/<project-id>/exports/
func ProjectExportDir(projectID string) (string, error) {
	dataDir, err := ProjectDataDir(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "exports"), nil
}

// ProjectLocalConfigPath returns the personal config path for a project.
// Path: ~/.orc/projects/<project-id>/config.yaml
func ProjectLocalConfigPath(projectID string) (string, error) {
	dataDir, err := ProjectDataDir(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "config.yaml"), nil
}

// ProjectLocalPromptsDir returns the personal prompts directory for a project.
// Path: ~/.orc/projects/<project-id>/prompts/
func ProjectLocalPromptsDir(projectID string) (string, error) {
	dataDir, err := ProjectDataDir(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "prompts"), nil
}

// ProjectSequencesPath returns the task ID sequences file path for a project.
// Path: ~/.orc/projects/<project-id>/sequences.yaml
func ProjectSequencesPath(projectID string) (string, error) {
	dataDir, err := ProjectDataDir(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "sequences.yaml"), nil
}

// ResolveProjectID looks up a project ID from its filesystem path via the registry.
// Returns the project ID or an error if the project is not registered.
func ResolveProjectID(projectPath string) (string, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	reg, err := LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	proj, err := reg.Get(absPath)
	if err != nil {
		return "", fmt.Errorf("project not registered: %w", err)
	}

	return proj.ID, nil
}

// EnsureProjectDirs creates the runtime directories for a project:
//   - ~/.orc/projects/<project-id>/
//   - ~/.orc/worktrees/<project-id>/
func EnsureProjectDirs(projectID string) error {
	dataDir, err := ProjectDataDir(projectID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create project data dir: %w", err)
	}

	wtDir, err := ProjectWorktreeDir(projectID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		return fmt.Errorf("create worktree dir: %w", err)
	}

	return nil
}
