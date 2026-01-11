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
			cfg, err := config.LoadFrom(targetPath)
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
