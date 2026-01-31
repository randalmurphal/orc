// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// newConfigCmd creates the config command with subcommands.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and manage configuration",
		Long: `View and manage orc configuration.

Configuration is loaded from multiple sources with this priority:
  1. Runtime: environment variables (ORC_*), CLI flags
  2. Personal: ~/.orc/config.yaml, .orc/local/config.yaml
  3. Shared: .orc/shared/config.yaml, .orc/config.yaml
  4. Defaults: Built-in values

Personal settings always override shared settings.

Subcommands:
  show        Show merged configuration
  get         Get a specific config value
  set         Set a config value
  resolution  Show full resolution chain for a key
  edit        Open config file in $EDITOR

Examples:
  orc config show                  # Show merged config as YAML
  orc config show --source         # Show with source annotations
  orc config get model             # Get model value
  orc config get model --source    # Get model with source info
  orc config set model claude-sonnet-4    # Set in user config
  orc config set --project profile safe   # Set in project config
  orc config resolution model      # Show resolution chain
  orc config edit                  # Open user config in $EDITOR
  orc config edit --project        # Open project config`,
	}

	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigResolutionCmd())
	cmd.AddCommand(newConfigEditCmd())
	cmd.AddCommand(newConfigDocsCmd())
	cmd.AddCommand(newConfigCommandsCmd())

	return cmd
}

// newConfigShowCmd creates the 'config show' subcommand.
func newConfigShowCmd() *cobra.Command {
	var showSource bool

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show merged configuration",
		Long: `Show the merged configuration from all sources.

By default, outputs valid YAML. Use --source to see where each value comes from.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tc, err := config.LoadWithSources()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()
			if showSource {
				return printConfigWithSources(out, tc)
			}

			return printConfigAsYAML(out, tc.Config)
		},
	}

	cmd.Flags().BoolVar(&showSource, "source", false, "Show source for each value")

	return cmd
}

// newConfigGetCmd creates the 'config get' subcommand.
func newConfigGetCmd() *cobra.Command {
	var showSource bool

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a specific config value",
		Long: `Get a specific configuration value by key.

Keys use dot notation for nested values (e.g., "gates.default_type").

Examples:
  orc config get model
  orc config get gates.default_type
  orc config get retry.enabled --source`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]

			tc, err := config.LoadWithSources()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			value, err := tc.Config.GetValue(key)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if showSource {
				source := tc.GetTrackedSource(key)
				_, _ = fmt.Fprintf(out, "%s (from %s)\n", value, source)
			} else {
				_, _ = fmt.Fprintln(out, value)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showSource, "source", false, "Show source of the value")

	return cmd
}

// newConfigSetCmd creates the 'config set' subcommand.
func newConfigSetCmd() *cobra.Command {
	var (
		setProject bool
		setShared  bool
		setUser    bool
	)

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Long: `Set a configuration value.

By default, values are saved to the user config (~/.orc/config.yaml).
Use flags to specify a different target:

  --user     Save to ~/.orc/config.yaml (default)
  --project  Save to .orc/config.yaml
  --shared   Save to .orc/shared/config.yaml

Examples:
  orc config set model claude-sonnet-4
  orc config set --project profile safe
  orc config set --shared gates.default_type ai`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			// Determine target file
			var targetPath string
			var targetName string

			switch {
			case setProject:
				targetPath = filepath.Join(config.OrcDir, config.ConfigFileName)
				targetName = ".orc/config.yaml"
			case setShared:
				targetPath = filepath.Join(config.OrcDir, "shared", config.ConfigFileName)
				targetName = ".orc/shared/config.yaml"
			default:
				// Default to user config (also handles explicit --user flag)
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("get home directory: %w", err)
				}
				targetPath = filepath.Join(home, ".orc", config.ConfigFileName)
				targetName = "~/.orc/config.yaml"
			}

			// Load existing config from target file or create new
			cfg, err := config.LoadFile(targetPath)
			if err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("load config from %s: %w", targetPath, err)
			}
			if cfg == nil {
				cfg = config.Default()
			}

			// Set the value
			if err := cfg.SetValue(key, value); err != nil {
				return fmt.Errorf("set %s: %w", key, err)
			}

			// Ensure target directory exists
			targetDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetDir, err)
			}

			// Save
			if err := cfg.SaveTo(targetPath); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s in %s\n", key, value, targetName)
			return nil
		},
	}

	cmd.Flags().BoolVar(&setProject, "project", false, "Save to project config (.orc/config.yaml)")
	cmd.Flags().BoolVar(&setShared, "shared", false, "Save to shared config (.orc/shared/config.yaml)")
	cmd.Flags().BoolVar(&setUser, "user", false, "Save to user config (~/.orc/config.yaml)")
	cmd.MarkFlagsMutuallyExclusive("project", "shared", "user")

	return cmd
}

// newConfigResolutionCmd creates the 'config resolution' subcommand.
func newConfigResolutionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resolution <key>",
		Short: "Show full resolution chain for a config key",
		Long: `Show the full resolution chain for a configuration key.

This displays values at all configuration levels (defaults, shared, personal, runtime)
and indicates which value "wins" (takes effect).

Example:
  orc config resolution model

Output shows:
  - RUNTIME: env vars and CLI flags
  - PERSONAL: user global and project local configs
  - SHARED: team and project configs
  - DEFAULTS: built-in values

The winning value is marked with "← WINNER".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			out := cmd.OutOrStdout()

			loader := config.NewLoader("")
			chain, err := loader.GetResolutionChain(key)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(out, "Resolution chain for '%s':\n", key)

			// Group entries by level
			byLevel := make(map[config.ConfigLevel][]config.ResolutionEntry)
			for _, e := range chain.Entries {
				byLevel[e.Level] = append(byLevel[e.Level], e)
			}

			// Print in order: runtime (highest) → defaults (lowest)
			levels := []config.ConfigLevel{
				config.LevelRuntime,
				config.LevelPersonal,
				config.LevelShared,
				config.LevelDefaults,
			}

			for _, level := range levels {
				entries := byLevel[level]
				if len(entries) == 0 {
					continue
				}

				levelName := strings.ToUpper(level.String())
				priority := levelPriority(level)
				_, _ = fmt.Fprintf(out, "  %s (%s):\n", levelName, priority)

				for _, e := range entries {
					status := "not set"
					winner := ""
					if e.IsSet {
						status = e.Value
					}
					if e.IsWinning {
						winner = " ← WINNER"
					}

					// Format path based on source type
					formattedPath := formatResolutionPath(e)
					_, _ = fmt.Fprintf(out, "    %s: %s%s\n", formattedPath, status, winner)
				}
			}

			_, _ = fmt.Fprintf(out, "\nFinal value: %s (from %s)\n", chain.FinalValue, chain.WinningFrom)

			return nil
		},
	}
}

// levelPriority returns a human-readable priority label.
func levelPriority(level config.ConfigLevel) string {
	switch level {
	case config.LevelRuntime:
		return "highest priority"
	case config.LevelPersonal:
		return "second priority"
	case config.LevelShared:
		return "third priority"
	case config.LevelDefaults:
		return "lowest priority"
	default:
		return ""
	}
}

// formatResolutionPath formats a resolution entry path according to the spec.
// For runtime entries:
//   - env vars: "env (ORC_MODEL)"
//   - flags: "flags (--model)"
//
// For file-based entries, returns the path as-is.
func formatResolutionPath(e config.ResolutionEntry) string {
	switch e.Source {
	case config.SourceEnv:
		return fmt.Sprintf("env (%s)", e.Path)
	case config.SourceFlag:
		return fmt.Sprintf("flags (%s)", e.Path)
	default:
		return e.Path
	}
}

// printConfigAsYAML outputs the config as valid YAML.
func printConfigAsYAML(out io.Writer, cfg *config.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	_, _ = fmt.Fprint(out, string(data))
	return nil
}

// printConfigWithSources outputs config values with source annotations.
func printConfigWithSources(out io.Writer, tc *config.TrackedConfig) error {
	// Get all config paths and their values
	paths := config.AllConfigPaths()
	sort.Strings(paths)

	for _, path := range paths {
		value, err := tc.Config.GetValue(path)
		if err != nil {
			continue
		}

		source := tc.GetTrackedSource(path)
		_, _ = fmt.Fprintf(out, "%s = %s (%s)\n", path, value, source)
	}

	return nil
}

// newConfigEditCmd creates the 'config edit' subcommand.
func newConfigEditCmd() *cobra.Command {
	var (
		editProject bool
		editShared  bool
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open config file in $EDITOR",
		Long: `Open a configuration file in your default editor.

By default, opens the user config (~/.orc/config.yaml).
Use flags to specify a different target:

  --project  Open .orc/config.yaml
  --shared   Open .orc/shared/config.yaml

The file will be created if it doesn't exist.

Examples:
  orc config edit              # Open user config
  orc config edit --project    # Open project config
  orc config edit --shared     # Open shared config`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Determine target file
			var targetPath string

			switch {
			case editProject:
				targetPath = filepath.Join(config.OrcDir, config.ConfigFileName)
			case editShared:
				targetPath = filepath.Join(config.OrcDir, "shared", config.ConfigFileName)
			default:
				// Default to user config
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("get home directory: %w", err)
				}
				targetPath = filepath.Join(home, ".orc", config.ConfigFileName)
			}

			// Ensure target directory exists
			targetDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetDir, err)
			}

			// Create file if it doesn't exist
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				if err := os.WriteFile(targetPath, []byte("# orc configuration\n"), 0644); err != nil {
					return fmt.Errorf("create config file: %w", err)
				}
			}

			// Get editor from environment
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = os.Getenv("VISUAL")
			}
			if editor == "" {
				return fmt.Errorf("no editor configured: set $EDITOR or $VISUAL environment variable")
			}

			// Open editor
			editorCmd := exec.Command(editor, targetPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("run editor: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&editProject, "project", false, "Edit project config (.orc/config.yaml)")
	cmd.Flags().BoolVar(&editShared, "shared", false, "Edit shared config (.orc/shared/config.yaml)")
	cmd.MarkFlagsMutuallyExclusive("project", "shared")

	return cmd
}

// ConfigDoc describes a configuration option
type ConfigDoc struct {
	Key         string
	Type        string
	Default     string
	EnvVar      string
	Description string
	Category    string
}

// getConfigDocs returns documentation for all config options
func getConfigDocs() []ConfigDoc {
	return []ConfigDoc{
		// Core
		{Key: "profile", Type: "string", Default: "auto", EnvVar: "ORC_PROFILE", Description: "Automation profile (auto, fast, safe, strict)", Category: "Core"},
		{Key: "model", Type: "string", Default: "claude-sonnet-4", EnvVar: "ORC_MODEL", Description: "Claude model to use", Category: "Core"},
		{Key: "fallback_model", Type: "string", Default: "claude-sonnet-4", EnvVar: "ORC_FALLBACK_MODEL", Description: "Fallback model when primary fails", Category: "Core"},
		{Key: "max_iterations", Type: "int", Default: "50", EnvVar: "ORC_MAX_ITERATIONS", Description: "Maximum Claude iterations per phase", Category: "Core"},
		{Key: "timeout", Type: "duration", Default: "30m", EnvVar: "ORC_TIMEOUT", Description: "Maximum time per phase", Category: "Core"},

		// Gates
		{Key: "gates.default_type", Type: "string", Default: "auto", EnvVar: "ORC_GATES_DEFAULT", Description: "Default gate type (auto, ai, human)", Category: "Gates"},
		{Key: "gates.auto_approve_on_success", Type: "bool", Default: "true", EnvVar: "", Description: "Auto-approve gates when phase succeeds", Category: "Gates"},
		{Key: "gates.retry_on_failure", Type: "bool", Default: "true", EnvVar: "", Description: "Retry when AI gate fails", Category: "Gates"},
		{Key: "gates.max_retries", Type: "int", Default: "5", EnvVar: "ORC_GATES_MAX_RETRIES", Description: "Max AI gate retries", Category: "Gates"},

		// Retry
		{Key: "retry.enabled", Type: "bool", Default: "true", EnvVar: "ORC_RETRY_ENABLED", Description: "Enable cross-phase retry", Category: "Retry"},
		{Key: "retry.max_retries", Type: "int", Default: "5", EnvVar: "ORC_RETRY_MAX_RETRIES", Description: "Max retry attempts (deprecated: use executor.max_retries)", Category: "Retry"},

		// Execution
		{Key: "executor.max_retries", Type: "int", Default: "5", EnvVar: "ORC_EXECUTOR_MAX_RETRIES", Description: "Max retry attempts when a phase fails", Category: "Execution"},

		// Worktree
		{Key: "worktree.enabled", Type: "bool", Default: "true", EnvVar: "ORC_WORKTREE_ENABLED", Description: "Enable git worktree isolation", Category: "Worktree"},
		{Key: "worktree.dir", Type: "string", Default: "", EnvVar: "", Description: "Worktree directory (empty = ~/.orc/worktrees/<project-id>/)", Category: "Worktree"},
		{Key: "worktree.cleanup_on_complete", Type: "bool", Default: "true", EnvVar: "", Description: "Remove worktree after success", Category: "Worktree"},
		{Key: "worktree.cleanup_on_fail", Type: "bool", Default: "false", EnvVar: "", Description: "Remove worktree after failure", Category: "Worktree"},

		// Review
		{Key: "review.enabled", Type: "bool", Default: "true", EnvVar: "", Description: "Enable code review phase", Category: "Review"},
		{Key: "review.rounds", Type: "int", Default: "2", EnvVar: "", Description: "Number of review rounds", Category: "Review"},
		{Key: "review.require_pass", Type: "bool", Default: "true", EnvVar: "", Description: "Require passing review to continue", Category: "Review"},

		// QA
		{Key: "qa.enabled", Type: "bool", Default: "true", EnvVar: "", Description: "Enable QA phase", Category: "QA"},
		{Key: "qa.require_e2e", Type: "bool", Default: "false", EnvVar: "", Description: "Require E2E tests to pass", Category: "QA"},
		{Key: "qa.generate_docs", Type: "bool", Default: "true", EnvVar: "", Description: "Auto-generate feature docs", Category: "QA"},

		// Testing
		{Key: "testing.required", Type: "bool", Default: "true", EnvVar: "", Description: "Require tests to pass", Category: "Testing"},
		{Key: "testing.coverage_threshold", Type: "int", Default: "0", EnvVar: "", Description: "Minimum coverage percentage (0 = disabled)", Category: "Testing"},

		// Completion
		{Key: "completion.action", Type: "string", Default: "pr", EnvVar: "", Description: "Action after completion (pr, merge, none)", Category: "Completion"},
		{Key: "completion.target_branch", Type: "string", Default: "main", EnvVar: "", Description: "Branch to merge into", Category: "Completion"},
		{Key: "completion.delete_branch", Type: "bool", Default: "true", EnvVar: "", Description: "Delete task branch after merge", Category: "Completion"},
		{Key: "completion.pr.auto_merge", Type: "bool", Default: "true", EnvVar: "", Description: "Enable auto-merge when PR approved", Category: "Completion"},

		// Team
		{Key: "team.name", Type: "string", Default: "", EnvVar: "ORC_TEAM_NAME", Description: "Organization/team name", Category: "Team"},
		{Key: "team.activity_logging", Type: "bool", Default: "true", EnvVar: "ORC_TEAM_ACTIVITY_LOG", Description: "Log all actions as history", Category: "Team"},
		{Key: "team.task_claiming", Type: "bool", Default: "false", EnvVar: "ORC_TEAM_TASK_CLAIMING", Description: "Enable task assignment", Category: "Team"},
		{Key: "team.mode", Type: "string", Default: "local", EnvVar: "ORC_TEAM_MODE", Description: "Team mode (local, shared_db)", Category: "Team"},

		// Token Pool
		{Key: "pool.enabled", Type: "bool", Default: "true", EnvVar: "", Description: "Enable OAuth token pool", Category: "Token Pool"},
		{Key: "pool.config_path", Type: "string", Default: "~/.orc/token-pool/pool.yaml", EnvVar: "", Description: "Token pool config path", Category: "Token Pool"},

		// Subtasks
		{Key: "subtasks.allow_creation", Type: "bool", Default: "true", EnvVar: "", Description: "Allow agents to propose sub-tasks", Category: "Subtasks"},
		{Key: "subtasks.auto_approve", Type: "bool", Default: "false", EnvVar: "", Description: "Auto-approve sub-tasks", Category: "Subtasks"},
		{Key: "subtasks.max_pending", Type: "int", Default: "10", EnvVar: "", Description: "Max pending sub-tasks per task", Category: "Subtasks"},

		// Server
		{Key: "server.host", Type: "string", Default: "localhost", EnvVar: "", Description: "API server host", Category: "Server"},
		{Key: "server.port", Type: "int", Default: "8080", EnvVar: "", Description: "API server port", Category: "Server"},
	}
}

// newConfigDocsCmd creates the 'config docs' subcommand.
func newConfigDocsCmd() *cobra.Command {
	var (
		category string
		search   string
	)

	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Show all available configuration options",
		Long: `Display documentation for all orc configuration options.

This shows every configurable setting with:
  - Key name (for YAML config files)
  - Type and default value
  - Environment variable override (if available)
  - Description

Examples:
  orc config docs                    # Show all options
  orc config docs --category Gates   # Filter by category
  orc config docs --search retry     # Search for options`,
		RunE: func(cmd *cobra.Command, args []string) error {
			docs := getConfigDocs()

			// Filter by category
			if category != "" {
				filtered := make([]ConfigDoc, 0)
				for _, d := range docs {
					if strings.EqualFold(d.Category, category) {
						filtered = append(filtered, d)
					}
				}
				docs = filtered
			}

			// Filter by search
			if search != "" {
				filtered := make([]ConfigDoc, 0)
				searchLower := strings.ToLower(search)
				for _, d := range docs {
					if strings.Contains(strings.ToLower(d.Key), searchLower) ||
						strings.Contains(strings.ToLower(d.Description), searchLower) {
						filtered = append(filtered, d)
					}
				}
				docs = filtered
			}

			if len(docs) == 0 {
				fmt.Println("No matching configuration options found.")
				return nil
			}

			// Group by category
			byCategory := make(map[string][]ConfigDoc)
			var categories []string
			for _, d := range docs {
				if _, exists := byCategory[d.Category]; !exists {
					categories = append(categories, d.Category)
				}
				byCategory[d.Category] = append(byCategory[d.Category], d)
			}

			// Print
			for i, cat := range categories {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("═══ %s ═══\n\n", cat)

				for _, d := range byCategory[cat] {
					fmt.Printf("  %s\n", d.Key)
					fmt.Printf("    Type:    %s (default: %s)\n", d.Type, d.Default)
					if d.EnvVar != "" {
						fmt.Printf("    Env:     %s\n", d.EnvVar)
					}
					fmt.Printf("    %s\n\n", d.Description)
				}
			}

			// Show quick tips
			fmt.Println("───────────────────────────────────")
			fmt.Println("Quick commands:")
			fmt.Println("  orc config show                  View current config")
			fmt.Println("  orc config set <key> <value>     Set a value")
			fmt.Println("  orc config resolution <key>      See where value comes from")

			return nil
		},
	}

	cmd.Flags().StringVarP(&category, "category", "c", "", "Filter by category")
	cmd.Flags().StringVarP(&search, "search", "s", "", "Search for options")

	return cmd
}

// newConfigCommandsCmd creates the 'config commands' subcommand.
func newConfigCommandsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commands",
		Short: "Manage project commands for quality checks",
		Long: `Manage project commands used by quality checks.

Project commands (tests, lint, build, typecheck) are used by phase-level
quality checks to validate work. Commands are stored in the database and
seeded during 'orc init' based on project type detection.

Subcommands:
  list      List all project commands (default)
  set       Set or update a command
  enable    Enable a disabled command
  disable   Disable a command
  delete    Delete a command

Examples:
  orc config commands                           # List all commands
  orc config commands list                      # Same as above
  orc config commands set tests "npm test"      # Set test command
  orc config commands set lint "golangci-lint run" --domain go
  orc config commands enable typecheck          # Enable a command
  orc config commands disable build             # Disable a command
  orc config commands delete custom_check       # Delete a command`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to list
			return runConfigCommandsList(cmd)
		},
	}

	cmd.AddCommand(newConfigCommandsListCmd())
	cmd.AddCommand(newConfigCommandsSetCmd())
	cmd.AddCommand(newConfigCommandsEnableCmd())
	cmd.AddCommand(newConfigCommandsDisableCmd())
	cmd.AddCommand(newConfigCommandsDeleteCmd())

	return cmd
}

// newConfigCommandsListCmd creates the 'config commands list' subcommand.
func newConfigCommandsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all project commands",
		Long:  `List all project commands with their status and command strings.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigCommandsList(cmd)
		},
	}
}

func runConfigCommandsList(cmd *cobra.Command) error {
	cfg := config.Default()
	backend, err := storage.NewBackend(".", &cfg.Storage)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = backend.Close() }()

	cmds, err := backend.ListProjectCommands()
	if err != nil {
		return fmt.Errorf("list commands: %w", err)
	}

	out := cmd.OutOrStdout()
	if len(cmds) == 0 {
		_, _ = fmt.Fprintln(out, "No project commands configured.")
		_, _ = fmt.Fprintln(out, "Run 'orc init' to detect and configure commands for your project.")
		return nil
	}

	_, _ = fmt.Fprintln(out, "Project Commands:")
	_, _ = fmt.Fprintln(out, "")

	for _, c := range cmds {
		status := "✓"
		if !c.Enabled {
			status = "✗"
		}
		domain := ""
		if c.Domain != "" {
			domain = fmt.Sprintf(" [%s]", c.Domain)
		}
		_, _ = fmt.Fprintf(out, "  %s %-12s%s: %s\n", status, c.Name, domain, c.Command)
	}

	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Use 'orc config commands set NAME \"command\"' to modify.")

	return nil
}

// newConfigCommandsSetCmd creates the 'config commands set' subcommand.
func newConfigCommandsSetCmd() *cobra.Command {
	var domain string

	cmd := &cobra.Command{
		Use:   "set <name> <command>",
		Short: "Set or update a project command",
		Long: `Set or update a project command.

Standard command names: tests, lint, build, typecheck
Custom commands can also be created for use with custom quality checks.

Examples:
  orc config commands set tests "npm test"
  orc config commands set lint "golangci-lint run ./..."
  orc config commands set typecheck "npx tsc --noEmit"
  orc config commands set custom_check "./scripts/validate.sh" --domain scripts`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, command := args[0], args[1]

			cfg := config.Default()
			backend, err := storage.NewBackend(".", &cfg.Storage)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = backend.Close() }()

			projectCmd := &db.ProjectCommand{
				Name:    name,
				Command: command,
				Domain:  domain,
				Enabled: true,
			}

			if err := backend.SaveProjectCommand(projectCmd); err != nil {
				return fmt.Errorf("save command: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set command '%s' = %s\n", name, command)
			return nil
		},
	}

	cmd.Flags().StringVar(&domain, "domain", "", "Optional domain for the command (e.g., go, node, python)")

	return cmd
}

// newConfigCommandsEnableCmd creates the 'config commands enable' subcommand.
func newConfigCommandsEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a project command",
		Long: `Enable a previously disabled project command.

Example:
  orc config commands enable typecheck`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg := config.Default()
			backend, err := storage.NewBackend(".", &cfg.Storage)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = backend.Close() }()

			if err := backend.SetProjectCommandEnabled(name, true); err != nil {
				return fmt.Errorf("enable command: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Enabled command '%s'\n", name)
			return nil
		},
	}
}

// newConfigCommandsDisableCmd creates the 'config commands disable' subcommand.
func newConfigCommandsDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a project command",
		Long: `Disable a project command without deleting it.

Disabled commands are skipped by quality checks.

Example:
  orc config commands disable build`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg := config.Default()
			backend, err := storage.NewBackend(".", &cfg.Storage)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = backend.Close() }()

			if err := backend.SetProjectCommandEnabled(name, false); err != nil {
				return fmt.Errorf("disable command: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Disabled command '%s'\n", name)
			return nil
		},
	}
}

// newConfigCommandsDeleteCmd creates the 'config commands delete' subcommand.
func newConfigCommandsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a project command",
		Long: `Delete a project command from the database.

This removes the command entirely. To temporarily disable, use 'disable' instead.

Example:
  orc config commands delete custom_check`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg := config.Default()
			backend, err := storage.NewBackend(".", &cfg.Storage)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = backend.Close() }()

			if err := backend.DeleteProjectCommand(name); err != nil {
				return fmt.Errorf("delete command: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted command '%s'\n", name)
			return nil
		},
	}
}
