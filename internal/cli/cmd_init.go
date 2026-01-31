// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/detect"
	"github.com/randalmurphal/orc/internal/storage"
)

// defaultErrorPatterns maps primary language to error handling idioms.
var defaultErrorPatterns = map[string]string{
	"go":         "Always check error returns with `if err != nil`. Wrap errors with context: `fmt.Errorf(\"context: %w\", err)`. Never discard errors with `_` in production. Use `errors.Is`/`errors.As` for comparison.",
	"python":     "Use specific exception types, never bare `except`. Log with `logger.exception()` for stack traces. Use `contextlib.suppress` only for documented expected cases.",
	"typescript": "Avoid broad `catch(e)` — catch specific error types. Never swallow errors in empty catch blocks. Use typed error responses at API boundaries.",
	"rust":       "Use `?` operator for propagation. Use `thiserror` for library errors, `anyhow` for application errors. Never `.unwrap()` in production code.",
	"java":       "Catch specific exceptions, never bare `Exception`. Always log with context. Use try-with-resources for closeable resources.",
}

// newInitCmd creates the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize orc in current project",
		Long: `Initialize orc in the current directory.

This wizard guides you through project setup:
  • Detects project languages and frameworks
  • Configures automation profile and target branch
  • Sets up MCP tools (Playwright for frontend projects)
  • Installs Claude Code hooks
  • Optionally sets project constitution

Examples:
  orc init                    # Interactive wizard
  orc init --yes              # Non-interactive with defaults
  orc init --force            # Reinitialize existing project
  orc init --profile strict   # Set specific profile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			nonInteractive, _ := cmd.Flags().GetBool("yes")
			profile, _ := cmd.Flags().GetString("profile")

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			// Check if already initialized
			if !force && config.IsInitializedAt(cwd) {
				return fmt.Errorf("orc already initialized. Use --force to reinitialize")
			}

			// Non-interactive mode: use instant bootstrap
			if nonInteractive {
				return runInstantInit(force, profile)
			}

			// Interactive mode: run wizard
			return runWizardInit(cwd, force)
		},
	}

	cmd.Flags().Bool("force", false, "Overwrite existing configuration")
	cmd.Flags().BoolP("yes", "y", false, "Non-interactive mode with defaults")
	cmd.Flags().String("profile", "", "Set automation profile (auto, fast, safe, strict)")

	return cmd
}

// runInstantInit runs the original instant bootstrap (for --yes flag or CI)
func runInstantInit(force bool, profile string) error {
	opts := bootstrap.Options{
		Force: force,
	}

	if profile != "" {
		opts.Profile = config.AutomationProfile(profile)
	}

	result, err := bootstrap.Run(opts)
	if err != nil {
		return err
	}

	bootstrap.PrintResult(result)
	return nil
}

// runWizardInit runs the interactive wizard-based init
func runWizardInit(projectPath string, force bool) error {
	// Build and run the wizard
	w, state := buildInitWizard(projectPath)

	fmt.Println()
	fmt.Println("  ╭─────────────────────────────────────╮")
	fmt.Println("  │       orc project setup             │")
	fmt.Println("  ╰─────────────────────────────────────╯")
	fmt.Println()

	if err := w.Run(); err != nil {
		return fmt.Errorf("wizard cancelled: %w", err)
	}

	// Extract results from wizard state
	extractWizardResults(w.State(), state)

	// Now run the bootstrap with wizard configuration
	fmt.Println("\nInitializing project...")

	opts := bootstrap.Options{
		Force:            force,
		SkipClaudeMD:     true, // Don't auto-inject CLAUDE.md
		SkipHooks:        !state.InstallHooks,
		SkipGitignore:    !state.UpdateGitignore,
		SkipConstitution: true, // We handle this separately
	}

	if state.Profile != "" {
		opts.Profile = config.AutomationProfile(state.Profile)
	}

	result, err := bootstrap.Run(opts)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Apply additional configuration from wizard
	if err := applyPostBootstrapConfig(projectPath, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: some configuration could not be applied: %v\n", err)
	}

	// Save detected languages to database
	if err := saveDetectedLanguages(projectPath, state); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save language detection: %v\n", err)
	}

	// Handle constitution
	if state.SetConstitution && state.ConstitutionPath != "" {
		if err := setConstitution(projectPath, state.ConstitutionPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not set constitution: %v\n", err)
		}
	}

	// Generate MCP config if enabled
	if state.EnablePlaywright {
		if err := generateMCPConfig(projectPath, state); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not generate MCP config: %v\n", err)
		}
	}

	// Print success
	printWizardResult(result, state)

	return nil
}

// applyPostBootstrapConfig applies wizard configuration to the project config
func applyPostBootstrapConfig(projectPath string, state *InitWizardState) error {
	configPath := filepath.Join(projectPath, ".orc", "config.yaml")

	// Load existing config
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		// If config doesn't exist, create minimal one
		cfg = config.Default()
	}

	// Apply wizard settings
	if state.Profile != "" {
		cfg.Profile = config.AutomationProfile(state.Profile)
	}
	if state.TargetBranch != "" {
		cfg.Completion.TargetBranch = state.TargetBranch
	}

	// Apply MCP settings (runtime config for Playwright)
	if state.EnablePlaywright {
		cfg.MCP.Playwright.Enabled = true
		cfg.MCP.Playwright.Headless = state.PlaywrightConfig.Headless
		cfg.MCP.Playwright.Browser = state.PlaywrightConfig.Browser
	}

	// Set default error patterns based on detected language
	if state.PrimaryLanguage != "" && cfg.ErrorPatterns == "" {
		if patterns, ok := defaultErrorPatterns[state.PrimaryLanguage]; ok {
			cfg.ErrorPatterns = patterns
		}
	}

	// Save back
	return cfg.SaveTo(configPath)
}

// saveDetectedLanguages saves the detected languages to the project database
func saveDetectedLanguages(projectPath string, state *InitWizardState) error {
	pdb, err := db.OpenProject(projectPath)
	if err != nil {
		return fmt.Errorf("open project db: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	// Clear existing languages
	if err := pdb.DeleteAllProjectLanguages(); err != nil {
		return fmt.Errorf("clear languages: %w", err)
	}

	// Save confirmed languages
	confirmedSet := make(map[string]bool)
	for _, lang := range state.ConfirmedLangs {
		confirmedSet[lang] = true
	}

	for i, lang := range state.Languages {
		scope := lang.GetScope()
		if !confirmedSet[scope] {
			continue // Skip unconfirmed languages
		}

		dbLang := &db.ProjectLanguage{
			Language:     string(lang.Language),
			RootPath:     lang.RootPath,
			IsPrimary:    i == 0, // First language is primary
			BuildTool:    string(lang.BuildTool),
			TestCommand:  lang.TestCommand,
			LintCommand:  lang.LintCommand,
			BuildCommand: lang.BuildCommand,
		}

		// Convert frameworks to strings
		if len(lang.Frameworks) > 0 {
			fws := make([]string, len(lang.Frameworks))
			for j, fw := range lang.Frameworks {
				fws[j] = string(fw)
			}
			dbLang.Frameworks = fws
		}

		if err := pdb.SaveProjectLanguage(dbLang); err != nil {
			return fmt.Errorf("save language %s: %w", lang.Language, err)
		}

		// Save scoped commands for this language
		if err := saveLanguageCommands(pdb, lang); err != nil {
			return fmt.Errorf("save commands for %s: %w", lang.Language, err)
		}
	}

	return nil
}

// saveLanguageCommands saves scoped commands for a language
func saveLanguageCommands(pdb *db.ProjectDB, lang detect.LanguageInfo) error {
	scope := lang.GetScope()

	commands := []struct {
		name    string
		command string
	}{
		{"tests", lang.TestCommand},
		{"lint", lang.LintCommand},
		{"build", lang.BuildCommand},
	}

	for _, cmd := range commands {
		if cmd.command == "" {
			continue
		}

		dbCmd := &db.ProjectCommand{
			Name:    cmd.name,
			Scope:   scope,
			Domain:  "code",
			Command: cmd.command,
			Enabled: true,
		}

		if err := pdb.SaveProjectCommand(dbCmd); err != nil {
			return fmt.Errorf("save command %s:%s: %w", cmd.name, scope, err)
		}
	}

	return nil
}

// setConstitution sets the project constitution from a file
func setConstitution(projectPath, constitutionPath string) error {
	content, err := os.ReadFile(constitutionPath)
	if err != nil {
		return fmt.Errorf("read constitution: %w", err)
	}

	backend, err := storage.NewBackend(projectPath, &config.StorageConfig{})
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer func() { _ = backend.Close() }()

	return backend.SaveConstitution(string(content))
}

// generateMCPConfig generates the .mcp.json file for the project.
// This creates a minimal config that defines WHICH MCP servers to use.
// Runtime settings (headless, browser, GPU flags) are stored in config.yaml
// and applied by the executor at runtime via MergeMCPConfigSettings().
func generateMCPConfig(projectPath string, state *InitWizardState) error {
	// Define minimal MCP server configuration
	// The executor will merge in runtime settings from config.yaml
	mcpConfig := map[string]any{
		"mcpServers": map[string]any{
			"playwright": map[string]any{
				"command": "npx",
				"args": []string{
					"@anthropic/mcp-playwright",
				},
			},
		},
	}

	configPath := filepath.Join(projectPath, ".mcp.json")

	// Check if file exists - don't overwrite user customizations
	if _, err := os.Stat(configPath); err == nil {
		return nil
	}

	// Marshal and write
	data, err := marshalJSON(mcpConfig)
	if err != nil {
		return fmt.Errorf("marshal mcp config: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// marshalJSON is a simple JSON marshaler
func marshalJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// printWizardResult prints the result of wizard-based initialization
func printWizardResult(result *bootstrap.Result, state *InitWizardState) {
	fmt.Println()
	fmt.Println("  ✓ Project initialized successfully")
	fmt.Println()

	fmt.Printf("  Project ID:    %s\n", result.ProjectID)
	fmt.Printf("  Profile:       %s\n", state.Profile)
	fmt.Printf("  Target Branch: %s\n", state.TargetBranch)

	if len(state.ConfirmedLangs) > 0 {
		fmt.Printf("  Languages:     %s\n", formatLanguages(state.ConfirmedLangs))
	}

	if state.EnablePlaywright {
		fmt.Println("  MCP:           Playwright enabled")
	}

	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    orc new \"task description\"  # Create a task")
	fmt.Println("    orc serve                    # Start web UI")
	fmt.Println()
}

// formatLanguages formats language list for display
func formatLanguages(langs []string) string {
	if len(langs) == 0 {
		return "none detected"
	}
	if len(langs) <= 3 {
		return strings.Join(langs, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(langs[:3], ", "), len(langs)-3)
}
