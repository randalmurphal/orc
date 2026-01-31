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

	// Skip options allow the wizard to handle these separately
	SkipClaudeMD  bool // Don't auto-inject orc section into CLAUDE.md
	SkipHooks     bool // Don't install Claude Code hooks
	SkipGitignore    bool // Don't update .gitignore
	SkipConstitution bool // Don't check for constitution files
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

	// FoundConstitution indicates if a constitution file was found
	FoundConstitution bool

	// ConstitutionPath is the path to the found constitution file
	ConstitutionPath string
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

	// 1. Create .orc/ directory structure (config-only, all git-tracked)
	dirs := []string{
		orcDir,
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

	// 3. Register in global registry FIRST (need project ID for DB path resolution)
	proj, err := project.RegisterProject(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("register project: %w", err)
	}

	// 3b. Create runtime directories at ~/.orc/projects/<id>/ and ~/.orc/worktrees/<id>/
	if err := project.EnsureProjectDirs(proj.ID); err != nil {
		return nil, fmt.Errorf("create project runtime dirs: %w", err)
	}

	// 4. Create project SQLite database and run migrations
	// DB is now at ~/.orc/projects/<id>/orc.db (resolved via registry)
	pdb, err := db.OpenProject(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("create project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	// 5. Run detection and store in SQLite
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

	// 5b. Seed project commands for quality checks
	if err := seedProjectCommands(pdb, detection); err != nil {
		// Non-fatal - just warn
		fmt.Fprintf(os.Stderr, "Warning: could not seed project commands: %v\n", err)
	}

	// 6. Sync to global SQLite database
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

	// 6. Update .gitignore (unless skipped)
	if !opts.SkipGitignore {
		if err := updateGitignore(opts.WorkDir); err != nil {
			// Non-fatal - just warn
			fmt.Fprintf(os.Stderr, "Warning: could not update .gitignore: %v\n", err)
		}
	}

	// 7. Install orc stop hook for ralph-style loops (unless skipped)
	if !opts.SkipHooks {
		if err := InstallHooks(opts.WorkDir); err != nil {
			// Non-fatal - just warn
			fmt.Fprintf(os.Stderr, "Warning: could not install hooks: %v\n", err)
		} else {
			fmt.Printf("Installed: .claude/hooks/orc-stop.sh (ralph-style loop hook)\n")
		}
	}

	// 8. Plugin installation is manual - user runs commands in Claude Code
	// (extraKnownMarketplaces in settings.json doesn't work reliably)

	// 9. Inject orc section into CLAUDE.md (unless skipped)
	if !opts.SkipClaudeMD {
		if err := InjectOrcSection(opts.WorkDir); err != nil {
			// Non-fatal - just warn
			fmt.Fprintf(os.Stderr, "Warning: could not update CLAUDE.md: %v\n", err)
		} else {
			fmt.Printf("Updated: CLAUDE.md (orc workflow documentation)\n")
		}
	}

	// 10. Check for constitution file to offer as constitution (unless skipped)
	var foundConstitution bool
	var constitutionPath string

	if !opts.SkipConstitution {
		// Check common locations for constitution files
		constitutionPaths := []string{
			filepath.Join(opts.WorkDir, "CONSTITUTION.md"),
			filepath.Join(opts.WorkDir, "constitution.md"),
			filepath.Join(opts.WorkDir, "INVARIANTS.md"),
			filepath.Join(opts.WorkDir, "docs", "CONSTITUTION.md"),
			filepath.Join(opts.WorkDir, "docs", "INVARIANTS.md"),
		}
		for _, path := range constitutionPaths {
			if _, err := os.Stat(path); err == nil {
				foundConstitution = true
				constitutionPath = path
				break
			}
		}
	}

	return &Result{
		Duration:          time.Since(start),
		ProjectID:         proj.ID,
		Detection:         detection,
		ConfigPath:        configPath,
		DatabasePath:      pdb.Path(),
		FoundConstitution: foundConstitution,
		ConstitutionPath:  constitutionPath,
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

// seedProjectCommands creates default project commands based on detection results.
// These commands are used by quality checks during phase execution.
func seedProjectCommands(pdb *db.ProjectDB, detection *detect.Detection) error {
	if detection == nil {
		return nil
	}

	commands := []*db.ProjectCommand{}

	// Test command
	if detection.TestCommand != "" {
		cmd := &db.ProjectCommand{
			Name:        "tests",
			Domain:      "code",
			Command:     detection.TestCommand,
			Enabled:     true,
			Description: "Run project tests",
		}
		// Add short command variant for Go
		if detection.Language == detect.ProjectTypeGo {
			cmd.ShortCommand = "go test -short ./..."
		}
		commands = append(commands, cmd)
	}

	// Lint command
	if detection.LintCommand != "" {
		commands = append(commands, &db.ProjectCommand{
			Name:        "lint",
			Domain:      "code",
			Command:     detection.LintCommand,
			Enabled:     true,
			Description: "Run linter",
		})
	}

	// Build command
	if detection.BuildCommand != "" {
		commands = append(commands, &db.ProjectCommand{
			Name:        "build",
			Domain:      "code",
			Command:     detection.BuildCommand,
			Enabled:     true,
			Description: "Build project",
		})
	}

	// Typecheck command (inferred by language)
	typecheckCmd := inferTypecheckCommand(detection)
	if typecheckCmd != "" {
		commands = append(commands, &db.ProjectCommand{
			Name:        "typecheck",
			Domain:      "code",
			Command:     typecheckCmd,
			Enabled:     true,
			Description: "Run type checker",
		})
	}

	// Save all commands
	for _, cmd := range commands {
		if err := pdb.SaveProjectCommand(cmd); err != nil {
			return fmt.Errorf("save project command %s: %w", cmd.Name, err)
		}
	}

	return nil
}

// inferTypecheckCommand returns the typecheck command based on language.
func inferTypecheckCommand(d *detect.Detection) string {
	switch d.Language {
	case detect.ProjectTypeGo:
		return "go build -o /dev/null ./..."
	case detect.ProjectTypeTypeScript:
		// Check for common package managers
		for _, tool := range d.BuildTools {
			switch tool {
			case detect.BuildToolPnpm:
				return "pnpm exec tsc --noEmit"
			case detect.BuildToolYarn:
				return "yarn tsc --noEmit"
			case detect.BuildToolBun:
				return "bun tsc --noEmit"
			}
		}
		return "npx tsc --noEmit"
	case detect.ProjectTypePython:
		return "pyright"
	case detect.ProjectTypeRust:
		return "cargo check"
	default:
		return ""
	}
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
