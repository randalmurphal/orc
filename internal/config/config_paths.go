package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/orc/internal/project"
)

// ExpandPath expands ~ to the user's home directory.
// Returns the original path unchanged if expansion fails or not needed.
func ExpandPath(path string) string {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// ResolveWorktreeDir returns the absolute worktree directory path.
// If configDir is non-empty, it's treated as a user override (relative to projectDir if not absolute).
// If empty, resolves to ~/.orc/worktrees/<project-id>/ via the project registry.
// Falls back to <projectDir>/.orc/worktrees if project is not registered.
func ResolveWorktreeDir(configDir, projectDir string) string {
	if configDir != "" {
		if filepath.IsAbs(configDir) {
			return configDir
		}
		return filepath.Join(projectDir, configDir)
	}
	projectID, err := project.ResolveProjectID(projectDir)
	if err != nil {
		return filepath.Join(projectDir, ".orc", "worktrees")
	}
	wtDir, err := project.ProjectWorktreeDir(projectID)
	if err != nil {
		return filepath.Join(projectDir, ".orc", "worktrees")
	}
	return wtDir
}

// InitAt initializes the orc directory structure at the specified base path.
func InitAt(basePath string, force bool) error {
	orcDir := filepath.Join(basePath, OrcDir)
	if !force {
		if _, err := os.Stat(orcDir); err == nil {
			return fmt.Errorf("orc already initialized (use --force to overwrite)")
		}
	}

	dirs := []string{
		orcDir,
		filepath.Join(orcDir, "tasks"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	cfg := Default()
	if err := cfg.SaveTo(filepath.Join(orcDir, ConfigFileName)); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// IsInitializedAt returns true if orc is initialized at the specified base path.
func IsInitializedAt(basePath string) bool {
	_, err := os.Stat(filepath.Join(basePath, OrcDir))
	return err == nil
}

// RequireInit returns an error if orc is not initialized in the current directory.
func RequireInit() error {
	if envRoot := os.Getenv("ORC_PROJECT_ROOT"); envRoot != "" {
		return RequireInitAt(envRoot)
	}
	return RequireInitAt(".")
}

// RequireInitAt returns an error if orc is not initialized at the specified base path.
func RequireInitAt(basePath string) error {
	if !IsInitializedAt(basePath) {
		return fmt.Errorf("not an orc project (no %s directory). Run 'orc init' first", OrcDir)
	}
	return nil
}

// FindProjectRoot finds the main project root directory that contains .orc/config.yaml.
// This handles git worktrees where config is stored in the main repo, not the worktree.
func FindProjectRoot() (string, error) {
	if envRoot := os.Getenv("ORC_PROJECT_ROOT"); envRoot != "" {
		return envRoot, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	if hasConfigFile(cwd) {
		return cwd, nil
	}

	mainRepo, err := findMainGitRepo()
	if err == nil && mainRepo != "" && mainRepo != cwd {
		if hasConfigFile(mainRepo) {
			return mainRepo, nil
		}
	}

	tempRoot, tempRootErr := filepath.Abs(os.TempDir())
	dir := cwd
	for {
		absDir, absErr := filepath.Abs(dir)
		if absErr == nil && tempRootErr == nil && absDir == tempRoot {
			break
		}
		if hasConfigFile(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	if mainRoot := extractMainRepoFromWorktreePath(cwd); mainRoot != "" {
		return mainRoot, nil
	}

	if hasConfigFile(cwd) {
		return cwd, nil
	}

	return "", fmt.Errorf("not in an orc project (no %s directory found)", OrcDir)
}

func hasConfigFile(dir string) bool {
	cfgPath := filepath.Join(dir, OrcDir, "config.yaml")
	info, err := os.Stat(cfgPath)
	return err == nil && !info.IsDir()
}

func extractMainRepoFromWorktreePath(path string) string {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalWorktrees := filepath.Join(homeDir, ".orc", "worktrees")
		if strings.HasPrefix(path, globalWorktrees) {
			rel, relErr := filepath.Rel(globalWorktrees, path)
			if relErr == nil {
				parts := strings.SplitN(rel, string(filepath.Separator), 2)
				if len(parts) >= 1 {
					projectID := parts[0]
					reg, regErr := project.LoadRegistry()
					if regErr == nil {
						for _, p := range reg.Projects {
							if p.ID == projectID && hasConfigFile(p.Path) {
								return p.Path
							}
						}
					}
				}
			}
		}
	}

	worktreeMarker := filepath.Join(OrcDir, "worktrees")
	idx := strings.Index(path, worktreeMarker)
	if idx == -1 {
		return ""
	}

	mainRoot := strings.TrimSuffix(path[:idx], string(filepath.Separator))
	if mainRoot == "" {
		return ""
	}

	if hasConfigFile(mainRoot) {
		return mainRoot
	}

	return ""
}

func findMainGitRepo() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	gitCommonDir := strings.TrimSpace(string(output))
	if gitCommonDir == "" || gitCommonDir == ".git" {
		return "", nil
	}

	if filepath.Base(gitCommonDir) == ".git" {
		return filepath.Dir(gitCommonDir), nil
	}
	return filepath.Dir(gitCommonDir), nil
}
