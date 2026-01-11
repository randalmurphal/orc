// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
)

// newConfigCmd creates the config command
func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config [key] [value]",
		Short: "Get or set configuration",
		Long: `Get or set orc configuration.

Automation profiles:
  auto   - Fully automated, no human intervention (default)
  fast   - Minimal gates, speed over safety
  safe   - AI reviews, human approval only for merge
  strict - Human gates on spec/review/merge

Example:
  orc config                    # show all
  orc config profile            # show profile
  orc config profile safe       # set profile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(args) == 0 {
				// Show all config
				fmt.Println("Current configuration:")
				fmt.Println()
				fmt.Println("Automation:")
				fmt.Printf("  profile:         %s\n", cfg.Profile)
				fmt.Printf("  gates.default:   %s\n", cfg.Gates.DefaultType)
				fmt.Printf("  retry.enabled:   %v\n", cfg.Retry.Enabled)
				fmt.Printf("  retry.max:       %d\n", cfg.Retry.MaxRetries)
				fmt.Println()
				fmt.Println("Execution:")
				fmt.Printf("  model:           %s\n", cfg.Model)
				fmt.Printf("  max_iterations:  %d\n", cfg.MaxIterations)
				fmt.Printf("  timeout:         %s\n", cfg.Timeout)
				fmt.Println()
				fmt.Println("Git:")
				fmt.Printf("  branch_prefix:   %s\n", cfg.BranchPrefix)
				fmt.Printf("  commit_prefix:   %s\n", cfg.CommitPrefix)
				return nil
			}

			// Set config value
			if len(args) == 2 {
				key, value := args[0], args[1]
				switch key {
				case "profile":
					cfg.ApplyProfile(config.AutomationProfile(value))
					if err := cfg.Save(); err != nil {
						return fmt.Errorf("save config: %w", err)
					}
					fmt.Printf("Set profile to: %s\n", value)
				default:
					return fmt.Errorf("unknown config key: %s", key)
				}
				return nil
			}

			// Show specific key
			key := args[0]
			switch key {
			case "profile":
				fmt.Println(cfg.Profile)
			case "gates":
				fmt.Printf("default: %s\n", cfg.Gates.DefaultType)
				fmt.Printf("auto_approve: %v\n", cfg.Gates.AutoApproveOnSuccess)
				if len(cfg.Gates.PhaseOverrides) > 0 {
					fmt.Println("phase_overrides:")
					for k, v := range cfg.Gates.PhaseOverrides {
						fmt.Printf("  %s: %s\n", k, v)
					}
				}
			case "retry":
				fmt.Printf("enabled: %v\n", cfg.Retry.Enabled)
				fmt.Printf("max_retries: %d\n", cfg.Retry.MaxRetries)
				if len(cfg.Retry.RetryMap) > 0 {
					fmt.Println("retry_map:")
					for k, v := range cfg.Retry.RetryMap {
						fmt.Printf("  %s -> %s\n", k, v)
					}
				}
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}
			return nil
		},
	}
}
