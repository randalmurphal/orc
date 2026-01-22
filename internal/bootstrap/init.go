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

	// FoundInvariants indicates if INVARIANTS.md was found
	FoundInvariants bool

	// InvariantsPath is the path to the found INVARIANTS.md
	InvariantsPath string
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

	// 2. Create minimal config.yaml (only if it doesn't exist)
	configPath := filepath.Join(orcDir, "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		cfg := config.Default()
		if opts.Profile != "" {
			cfg.ApplyProfile(opts.Profile)
		}
		if err := cfg.SaveTo(configPath); err != nil {
			return nil, fmt.Errorf("write config: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("check config: %w", err)
	}
	// If config exists, preserve user customizations

	// 3. Create project SQLite database and run migrations
	pdb, err := db.OpenProject(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("create project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

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
		defer func() { _ = gdb.Close() }()
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

	// 7. Install orc stop hook for ralph-style loops
	if err := InstallHooks(opts.WorkDir); err != nil {
		// Non-fatal - just warn
		fmt.Fprintf(os.Stderr, "Warning: could not install hooks: %v\n", err)
	} else {
		fmt.Printf("Installed: .claude/hooks/orc-stop.sh (ralph-style loop hook)\n")
	}

	// 8. Plugin installation is manual - user runs commands in Claude Code
	// (extraKnownMarketplaces in settings.json doesn't work reliably)

	// 9. Inject orc section into CLAUDE.md
	if err := InjectOrcSection(opts.WorkDir); err != nil {
		// Non-fatal - just warn
		fmt.Fprintf(os.Stderr, "Warning: could not update CLAUDE.md: %v\n", err)
	} else {
		fmt.Printf("Updated: CLAUDE.md (orc workflow documentation)\n")
	}

	// 10. Inject knowledge section into CLAUDE.md
	if err := InjectKnowledgeSection(opts.WorkDir); err != nil {
		// Non-fatal - just warn
		fmt.Fprintf(os.Stderr, "Warning: could not add knowledge section to CLAUDE.md: %v\n", err)
	} else {
		fmt.Printf("Updated: CLAUDE.md (knowledge capture section)\n")
	}

	// 11. Check for INVARIANTS.md to offer as constitution
	var foundInvariants bool
	var invariantsPath string

	// Check common locations for INVARIANTS.md
	invariantsPaths := []string{
		filepath.Join(opts.WorkDir, "INVARIANTS.md"),
		filepath.Join(opts.WorkDir, "docs", "INVARIANTS.md"),
	}
	for _, path := range invariantsPaths {
		if _, err := os.Stat(path); err == nil {
			foundInvariants = true
			invariantsPath = path
			break
		}
	}

	return &Result{
		Duration:        time.Since(start),
		ProjectID:       proj.ID,
		Detection:       detection,
		ConfigPath:      configPath,
		DatabasePath:    pdb.Path(),
		FoundInvariants: foundInvariants,
		InvariantsPath:  invariantsPath,
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
	fmt.Printf("\nClaude Code plugins (run once in Claude Code):\n")
	fmt.Printf("  /plugin marketplace add randalmurphal/orc-claude-plugin\n")
	fmt.Printf("  /plugin install orc@orc\n")
	if r.Detection != nil && r.Detection.HasFrontend {
		fmt.Printf("  /plugin install playwright@claude-plugins-official  # Frontend detected\n")
	}
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  orc new \"task description\"  # Create a new task\n")
	fmt.Printf("  orc serve                    # Start web UI at localhost:8080\n")
	fmt.Printf("  orc setup                    # (Optional) Configure with Claude\n")
}
