// Package spec provides interactive specification sessions for feature planning.
// It spawns Claude to collaboratively create specifications with the user.
package spec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

// Options configures a spec session.
type Options struct {
	// WorkDir is the working directory for the spec session
	WorkDir string

	// Model is the Claude model to use
	Model string

	// InitiativeID links the spec to an existing initiative
	InitiativeID string

	// DryRun shows the prompt without running Claude
	DryRun bool

	// CreateTasks determines if tasks should be created from spec output
	CreateTasks bool

	// Shared indicates if initiative is in the shared directory
	Shared bool

	// Backend is the storage backend for tasks and initiatives
	Backend storage.Backend
}

// Result contains the outcome of a spec session.
type Result struct {
	// SpecPath is the path to the generated spec file
	SpecPath string

	// TaskIDs contains IDs of created tasks (if CreateTasks was true)
	TaskIDs []string

	// Decisions contains decisions captured during the session
	Decisions []string
}

// Run executes an interactive spec session.
func Run(ctx context.Context, title string, opts Options) (*Result, error) {
	if opts.WorkDir == "" {
		var err error
		opts.WorkDir, err = config.FindProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("WorkDir not specified and not in orc project: %w", err)
		}
	}

	// Load detection if available (non-fatal - enhances prompts but not required)
	var detection *db.Detection
	pdb, err := db.OpenProject(opts.WorkDir)
	if err == nil {
		defer func() { _ = pdb.Close() }()
		var loadErr error
		detection, loadErr = pdb.LoadDetection()
		if loadErr != nil {
			slog.Debug("spec: could not load detection (non-fatal)", "error", loadErr)
		}
	}

	// Load initiative if specified
	var init *initiative.Initiative
	if opts.InitiativeID != "" && opts.Backend != nil {
		init, err = opts.Backend.LoadInitiative(opts.InitiativeID)
		if err != nil {
			return nil, fmt.Errorf("load initiative: %w", err)
		}
	}

	// Generate prompt
	prompt, err := GeneratePrompt(PromptData{
		Title:       title,
		WorkDir:     opts.WorkDir,
		Detection:   detection,
		Initiative:  init,
		CreateTasks: opts.CreateTasks,
	})
	if err != nil {
		return nil, fmt.Errorf("generate prompt: %w", err)
	}

	// Dry run - just show the prompt
	if opts.DryRun {
		fmt.Printf("=== Spec Session Prompt ===\n\n%s\n", prompt)
		return &Result{}, nil
	}

	// Spawn Claude
	spawner := NewSpawner(SpawnerOptions{
		WorkDir:                    opts.WorkDir,
		Model:                      opts.Model,
		DangerouslySkipPermissions: true,
	})

	if err := spawner.RunInteractive(ctx, prompt); err != nil {
		return nil, fmt.Errorf("run claude: %w", err)
	}

	// Find the spec file
	result := &Result{}
	specPath := findSpecFile(opts.WorkDir, opts.InitiativeID)
	if specPath != "" {
		result.SpecPath = specPath
	}

	return result, nil
}

// findSpecFile looks for a recently created spec file.
func findSpecFile(workDir, initiativeID string) string {
	// Check initiative-specific location first
	if initiativeID != "" {
		initPath := filepath.Join(workDir, ".orc", "initiatives", initiativeID, "spec.md")
		if _, err := os.Stat(initPath); err == nil {
			return initPath
		}
		// Also check shared location
		sharedPath := filepath.Join(workDir, ".orc", "shared", "initiatives", initiativeID, "spec.md")
		if _, err := os.Stat(sharedPath); err == nil {
			return sharedPath
		}
	}

	// Check default spec location
	defaultPath := filepath.Join(workDir, ".orc", "specs")
	entries, err := os.ReadDir(defaultPath)
	if err != nil {
		return ""
	}

	// Find the most recent .md file
	var latestPath string
	var latestTime int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latestPath = filepath.Join(defaultPath, e.Name())
		}
	}

	return latestPath
}
