// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/hosting"
	_ "github.com/randalmurphal/orc/internal/hosting/github"
	_ "github.com/randalmurphal/orc/internal/hosting/gitlab"
)

func newStagingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "staging",
		Short: "Manage developer staging branch",
		Long: `Manage developer staging branch for accumulating work before merging to main.

A staging branch is a personal branch where tasks merge to by default,
allowing you to batch multiple task changes before syncing to main.

Resolution hierarchy (highest to lowest priority):
  1. Task.TargetBranch (explicit override per task)
  2. Initiative.BranchBase (inherited from initiative)
  3. Developer.StagingBranch (personal staging area) ← THIS
  4. Config.Completion.TargetBranch (project default)
  5. "main" (hardcoded fallback)

Commands:
  status    Show staging branch status and health
  sync      Create PR from staging branch to main
  enable    Enable staging branch (staging_enabled: true)
  disable   Disable staging branch (staging_enabled: false)
  set       Set the staging branch name`,
	}

	cmd.AddCommand(newStagingStatusCmd())
	cmd.AddCommand(newStagingSyncCmd())
	cmd.AddCommand(newStagingEnableCmd())
	cmd.AddCommand(newStagingDisableCmd())
	cmd.AddCommand(newStagingSetCmd())

	return cmd
}

func newStagingStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show staging branch status",
		Long: `Show staging branch configuration and health.

Displays:
  - Current staging branch name
  - Whether staging is enabled
  - Commits ahead/behind main
  - Tasks merged to staging (if any)

Example:
  orc staging status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			out := cmd.OutOrStdout()

			// Show configuration
			_, _ = fmt.Fprintln(out, "Staging Branch Configuration")
			_, _ = fmt.Fprintln(out, strings.Repeat("-", 40))

			if cfg.Developer.StagingBranch == "" {
				_, _ = fmt.Fprintln(out, "Branch:  (not configured)")
				_, _ = fmt.Fprintln(out, "Enabled: false")
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "To configure a staging branch:")
				_, _ = fmt.Fprintln(out, "  orc staging set dev/yourname")
				_, _ = fmt.Fprintln(out, "  orc staging enable")
				return nil
			}

			_, _ = fmt.Fprintf(out, "Branch:  %s\n", cfg.Developer.StagingBranch)
			_, _ = fmt.Fprintf(out, "Enabled: %t\n", cfg.Developer.StagingEnabled)
			if cfg.Developer.AutoSyncAfter > 0 {
				_, _ = fmt.Fprintf(out, "Auto-sync: after %d tasks\n", cfg.Developer.AutoSyncAfter)
			}

			// Determine target branch for sync
			targetBranch := cfg.Completion.TargetBranch
			if targetBranch == "" {
				targetBranch = executor.DefaultTargetBranch
			}
			_, _ = fmt.Fprintf(out, "Sync target: %s\n", targetBranch)

			// Try to get git status
			projectRoot, err := ResolveProjectPath()
			if err != nil {
				_, _ = fmt.Fprintln(out)
				_, _ = fmt.Fprintln(out, "(Not in a git repository)")
				return nil
			}

			gitOps, err := NewGitOpsFromConfig(projectRoot, cfg)
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Check if staging branch exists
			exists, err := gitOps.BranchExists(cfg.Developer.StagingBranch)
			if err != nil {
				return fmt.Errorf("check branch exists: %w", err)
			}

			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "Git Status")
			_, _ = fmt.Fprintln(out, strings.Repeat("-", 40))

			if !exists {
				_, _ = fmt.Fprintf(out, "Branch %s does not exist yet.\n", cfg.Developer.StagingBranch)
				_, _ = fmt.Fprintln(out, "It will be created when the first task runs.")
				return nil
			}

			_, _ = fmt.Fprintf(out, "Branch %s exists.\n", cfg.Developer.StagingBranch)
			_, _ = fmt.Fprintln(out)
			_, _ = fmt.Fprintln(out, "To sync changes to main:")
			_, _ = fmt.Fprintln(out, "  orc staging sync")

			return nil
		},
	}
}

func newStagingSyncCmd() *cobra.Command {
	var force bool
	var draft bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Create PR from staging branch to main",
		Long: `Create a pull request to sync staging branch to main.

This creates a PR from your staging branch to the configured target branch
(usually main). The PR includes all task commits accumulated in staging.

Example:
  orc staging sync          # Create PR staging→main
  orc staging sync --draft  # Create as draft PR`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if cfg.Developer.StagingBranch == "" {
				return fmt.Errorf("staging branch not configured; run: orc staging set <branch>")
			}

			if !cfg.Developer.StagingEnabled && !force {
				return fmt.Errorf("staging is disabled; run: orc staging enable (or use --force)")
			}

			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			gitOps, err := NewGitOpsFromConfig(projectRoot, cfg)
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}
			hostingCfg := hosting.Config{
				Provider:    cfg.Hosting.Provider,
				BaseURL:     cfg.Hosting.BaseURL,
				TokenEnvVar: cfg.Hosting.TokenEnvVar,
			}
			provider, err := hosting.NewProvider(projectRoot, hostingCfg)
			if err != nil {
				return fmt.Errorf("init hosting provider: %w", err)
			}

			// Check staging branch exists
			exists, err := gitOps.BranchExists(cfg.Developer.StagingBranch)
			if err != nil {
				return fmt.Errorf("check branch exists: %w", err)
			}
			if !exists {
				return fmt.Errorf("staging branch %s does not exist", cfg.Developer.StagingBranch)
			}

			// Determine target branch
			targetBranch := cfg.Completion.TargetBranch
			if targetBranch == "" {
				targetBranch = executor.DefaultTargetBranch
			}

			// Push staging branch
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pushing %s to origin...\n", cfg.Developer.StagingBranch)
			if err := gitOps.Push("origin", cfg.Developer.StagingBranch, true); err != nil {
				return fmt.Errorf("push staging branch: %w", err)
			}

			// Create PR
			opts := hosting.PRCreateOptions{
				Title: fmt.Sprintf("[staging] Sync %s → %s", cfg.Developer.StagingBranch, targetBranch),
				Body: fmt.Sprintf(`Syncing staging branch to %s.

Generated by `+"`orc staging sync`", targetBranch),
				Head:   cfg.Developer.StagingBranch,
				Base:   targetBranch,
				Draft:  draft,
				Labels: cfg.Completion.PR.Labels,
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Creating PR: %s → %s...\n", cfg.Developer.StagingBranch, targetBranch)
			pr, err := provider.CreatePR(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("create PR: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nPR created: %s\n", pr.HTMLURL)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Sync even if staging is disabled")
	cmd.Flags().BoolVar(&draft, "draft", false, "Create as draft PR")

	return cmd
}

func newStagingEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable staging branch",
		Long: `Enable the staging branch so tasks merge to it by default.

The staging branch must be configured first with 'orc staging set'.

Example:
  orc staging enable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return setStagingConfig(cmd, "staging_enabled", "true")
		},
	}
}

func newStagingDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable staging branch",
		Long: `Disable the staging branch so tasks merge to their normal targets.

This does not remove the configuration, just disables it.
Use 'orc staging enable' to re-enable.

Example:
  orc staging disable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return setStagingConfig(cmd, "staging_enabled", "false")
		},
	}
}

func newStagingSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <branch-name>",
		Short: "Set the staging branch name",
		Long: `Set the staging branch name for accumulating work.

Common patterns:
  dev/yourname     Personal development branch
  staging/yourname Personal staging area
  feature/batch    Temporary batch for related work

After setting, enable with 'orc staging enable'.

Example:
  orc staging set dev/randy
  orc staging enable`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			// Validate branch name for security and git compatibility
			if err := git.ValidateBranchName(branchName); err != nil {
				return fmt.Errorf("invalid staging branch: %w", err)
			}
			if branchName == "main" || branchName == "master" {
				return fmt.Errorf("cannot use %s as staging branch", branchName)
			}

			return setStagingConfig(cmd, "staging_branch", branchName)
		},
	}
}

// setStagingConfig sets a developer config value in the user's personal config.
// Developer settings should live in personal config (~/.orc/config.yaml) not project config.
func setStagingConfig(cmd *cobra.Command, key, value string) error {
	// Get user home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	targetPath := filepath.Join(home, ".orc", config.ConfigFileName)

	// Load existing config or create new
	cfg, err := config.LoadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load config from %s: %w", targetPath, err)
	}
	if cfg == nil {
		cfg = config.Default()
	}

	// Set the value using the full key path
	fullKey := "developer." + key
	if err := cfg.SetValue(fullKey, value); err != nil {
		return fmt.Errorf("set %s: %w", fullKey, err)
	}

	// Ensure directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", targetDir, err)
	}

	// Save
	if err := cfg.SaveTo(targetPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set developer.%s = %s in ~/.orc/config.yaml\n", key, value)
	return nil
}
