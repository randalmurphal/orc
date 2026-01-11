// Package bootstrap provides instant project initialization for orc.
// The init command completes in < 500ms with zero prompts.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/detect"
	"github.com/randalmurphal/orc/internal/project"
)

// Options configures the init process.
type Options struct {
	// WorkDir is the directory to initialize (default: current directory)
	WorkDir string

	// Force overwrites existing configuration
	Force bool

	// Profile sets the initial automation profile (default: auto)
	Profile config.AutomationProfile
}

// Result contains the results of initialization.
type Result struct {
	// Duration is how long init took
	Duration time.Duration

	// ProjectID is the assigned project ID
	ProjectID string

	// Detection results
	Detection *detect.Detection

	// ConfigPath is the path to the created config file
	ConfigPath string

	// DatabasePath is the path to the project database
	DatabasePath string
}

// Run performs instant project initialization.
// This function is designed to complete in < 500ms with zero prompts.
func Run(opts Options) (*Result, error) {
	start := time.Now()

	// Default to current directory
	if opts.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		opts.WorkDir = wd
	}

	// Resolve absolute path
	absPath, err := filepath.Abs(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	opts.WorkDir = absPath

	// Check if already initialized
	orcDir := filepath.Join(opts.WorkDir, ".orc")
	if !opts.Force {
		if _, err := os.Stat(orcDir); err == nil {
			return nil, fmt.Errorf("orc already initialized in %s (use --force to reinitialize)", opts.WorkDir)
		}
	}

	// 1. Create .orc/ directory structure
	dirs := []string{
		orcDir,
		filepath.Join(orcDir, "tasks"),
		filepath.Join(orcDir, "prompts"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// 2. Create minimal config.yaml
	cfg := config.Default()
	if opts.Profile != "" {
		cfg.ApplyProfile(opts.Profile)
	}
	configPath := filepath.Join(orcDir, "config.yaml")
	if err := cfg.SaveTo(configPath); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// 3. Create project SQLite database and run migrations
	pdb, err := db.OpenProject(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("create project database: %w", err)
	}
	defer pdb.Close()

	// 4. Run detection and store in SQLite
	detection, err := detect.Detect(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("detect project: %w", err)
	}

	// Convert to db.Detection and store
	dbDetection := &db.Detection{
		Language:    string(detection.Language),
		Frameworks:  frameworksToStrings(detection.Frameworks),
		BuildTools:  buildToolsToStrings(detection.BuildTools),
		HasTests:    detection.HasTests,
		TestCommand: detection.TestCommand,
		LintCommand: detection.LintCommand,
	}
	if err := pdb.StoreDetection(dbDetection); err != nil {
		return nil, fmt.Errorf("store detection: %w", err)
	}

	// 5. Register in global registry (YAML - for backwards compat during migration)
	proj, err := project.RegisterProject(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("register project: %w", err)
	}

	// Also sync to global SQLite database
	gdb, err := db.OpenGlobal()
	if err != nil {
		// Non-fatal - YAML registry is the fallback
		fmt.Fprintf(os.Stderr, "Warning: could not open global database: %v\n", err)
	} else {
		defer gdb.Close()
		if err := gdb.SyncProject(db.Project{
			ID:        proj.ID,
			Name:      proj.Name,
			Path:      proj.Path,
			Language:  string(detection.Language),
			CreatedAt: proj.CreatedAt,
		}); err != nil {
			// Non-fatal - YAML registry is the fallback
			fmt.Fprintf(os.Stderr, "Warning: could not sync to global database: %v\n", err)
		}
	}

	// 6. Update .gitignore
	if err := updateGitignore(opts.WorkDir); err != nil {
		// Non-fatal - just warn
		fmt.Fprintf(os.Stderr, "Warning: could not update .gitignore: %v\n", err)
	}

	return &Result{
		Duration:     time.Since(start),
		ProjectID:    proj.ID,
		Detection:    detection,
		ConfigPath:   configPath,
		DatabasePath: pdb.Path(),
	}, nil
}

// frameworksToStrings converts Framework slice to string slice.
func frameworksToStrings(frameworks []detect.Framework) []string {
	result := make([]string, len(frameworks))
	for i, f := range frameworks {
		result[i] = string(f)
	}
	return result
}

// buildToolsToStrings converts BuildTool slice to string slice.
func buildToolsToStrings(tools []detect.BuildTool) []string {
	result := make([]string, len(tools))
	for i, t := range tools {
		result[i] = string(t)
	}
	return result
}

// PrintResult prints a summary of the initialization.
func PrintResult(r *Result) {
	fmt.Printf("Initialized orc in %v\n", r.Duration.Round(time.Millisecond))
	fmt.Printf("  Project ID: %s\n", r.ProjectID)
	if r.Detection != nil && r.Detection.Language != detect.ProjectTypeUnknown {
		fmt.Printf("  Detected: %s\n", detect.DescribeProject(r.Detection))
	}
	fmt.Printf("  Config: %s\n", r.ConfigPath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  orc new \"task description\"  # Create a new task\n")
	fmt.Printf("  orc setup                    # (Optional) Configure with Claude\n")
}
