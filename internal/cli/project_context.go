// project_context.go provides project resolution for multi-project CLI support.
package cli

import (
	"fmt"
	"os"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/project"
)

// projectFlag is set via --project/-P flag
var projectFlag string

// ResolveProjectID returns the project ID based on:
// 1. --project flag
// 2. ORC_PROJECT env var
// 3. Current directory detection
// 4. Error if none found
func ResolveProjectID() (string, error) {
	// 1. Flag takes priority
	if projectFlag != "" {
		return resolveProjectRef(projectFlag)
	}

	// 2. Env var
	if envProject := os.Getenv("ORC_PROJECT"); envProject != "" {
		return resolveProjectRef(envProject)
	}

	// 3. Cwd detection - try to find project root from current dir
	projectRoot, err := config.FindProjectRoot()
	if err == nil {
		reg, err := project.LoadRegistry()
		if err != nil {
			// Registry doesn't exist yet - that's OK in single-project mode
			// Return empty string to indicate "use current directory"
			return "", nil
		}

		// Try to find project by ID first
		proj, err := reg.Get(projectRoot)
		if err == nil {
			return proj.ID, nil
		}

		// Try to find by path
		for _, p := range reg.Projects {
			if p.Path == projectRoot {
				return p.ID, nil
			}
		}

		// Project root exists but not in registry - return empty for backwards compat
		return "", nil
	}

	// 4. Not in a project directory
	return "", fmt.Errorf("not in an orc project; use --project or cd to a project directory")
}

// ResolveProjectPath returns the project path for the resolved project.
// If no project ID is resolved (single-project mode), returns current project root.
func ResolveProjectPath() (string, error) {
	projectID, err := ResolveProjectID()
	if err != nil {
		return "", err
	}

	// Single-project mode - use current directory
	if projectID == "" {
		return config.FindProjectRoot()
	}

	// Multi-project mode - lookup in registry
	reg, err := project.LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	proj, err := reg.Get(projectID)
	if err != nil {
		return "", fmt.Errorf("project not found: %w", err)
	}

	return proj.Path, nil
}

// resolveProjectRef resolves a project reference (ID, name, or path) to an ID.
func resolveProjectRef(ref string) (string, error) {
	reg, err := project.LoadRegistry()
	if err != nil {
		return "", fmt.Errorf("load registry: %w", err)
	}

	// Try as ID first
	if proj, err := reg.Get(ref); err == nil {
		return proj.ID, nil
	}

	// Try as name (must be unique)
	var matches []project.Project
	for _, p := range reg.Projects {
		if p.Name == ref {
			matches = append(matches, p)
		}
	}
	if len(matches) == 1 {
		return matches[0].ID, nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous project name %q matches %d projects; use project ID instead", ref, len(matches))
	}

	// Try as path
	for _, p := range reg.Projects {
		if p.Path == ref {
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("project not found: %s (not a valid ID, name, or path)", ref)
}

// IsMultiProjectMode returns true if we're operating in multi-project mode
// (i.e., a specific project was selected via flag/env).
func IsMultiProjectMode() bool {
	return projectFlag != "" || os.Getenv("ORC_PROJECT") != ""
}
