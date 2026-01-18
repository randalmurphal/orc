// Package setup provides Claude-powered project setup for orc.
// This is an optional step after `orc init` that spawns an interactive
// Claude session to configure the project based on detection results.
package setup

import (
	"context"
	"fmt"
	"os"

	"github.com/randalmurphal/orc/internal/db"
)

// Options configures the setup process.
type Options struct {
	// WorkDir is the project directory (default: current directory)
	WorkDir string

	// Model is the Claude model to use (default: opus)
	Model string

	// DryRun prints the prompt instead of running Claude
	DryRun bool

	// SkipValidation skips output validation after Claude exits
	SkipValidation bool
}

// Result contains the results of setup.
type Result struct {
	// Prompt is the generated setup prompt
	Prompt string

	// Validated indicates if output was validated
	Validated bool

	// ValidationErrors contains any validation issues found
	ValidationErrors []string
}

// Run executes the Claude-powered setup.
// This spawns an interactive Claude session with a prompt tailored
// to the detected project type.
func Run(ctx context.Context, opts Options) (*Result, error) {
	// Default to current directory
	if opts.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		opts.WorkDir = wd
	}

	// Default model
	if opts.Model == "" {
		opts.Model = "opus"
	}

	// Load detection from SQLite
	pdb, err := db.OpenProject(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("open project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	detection, err := pdb.LoadDetection()
	if err != nil {
		return nil, fmt.Errorf("load detection: %w", err)
	}

	// Generate setup prompt
	prompt, err := GeneratePrompt(opts.WorkDir, detection)
	if err != nil {
		return nil, fmt.Errorf("generate prompt: %w", err)
	}

	result := &Result{Prompt: prompt}

	// Dry run - just print the prompt
	if opts.DryRun {
		fmt.Println(prompt)
		return result, nil
	}

	// Spawn Claude interactively
	spawner := NewSpawner(SpawnerOptions{
		WorkDir: opts.WorkDir,
		Model:   opts.Model,
	})

	if err := spawner.RunInteractive(ctx, prompt); err != nil {
		return result, fmt.Errorf("run claude: %w", err)
	}

	// Validate output
	if !opts.SkipValidation {
		validator := NewValidator(opts.WorkDir)
		errors := validator.Validate()
		result.ValidationErrors = errors
		result.Validated = len(errors) == 0
	}

	return result, nil
}
