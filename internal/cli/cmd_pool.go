// Package cli implements the orc command-line interface.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/tokenpool"
)

func newPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage OAuth token pool for automatic account switching",
		Long: `Manage a pool of OAuth tokens for automatic account switching when rate limits are hit.

The token pool allows you to configure multiple Claude accounts and automatically
switch between them when one account hits its rate limit.

Commands:
  init     Initialize the token pool directory
  add      Add a new account to the pool
  list     List all accounts in the pool
  status   Show detailed status of all accounts
  switch   Manually switch to a specific account
  remove   Remove an account from the pool
  reset    Reset all exhausted flags`,
	}

	cmd.AddCommand(newPoolInitCmd())
	cmd.AddCommand(newPoolAddCmd())
	cmd.AddCommand(newPoolListCmd())
	cmd.AddCommand(newPoolStatusCmd())
	cmd.AddCommand(newPoolSwitchCmd())
	cmd.AddCommand(newPoolRemoveCmd())
	cmd.AddCommand(newPoolResetCmd())

	return cmd
}

func newPoolInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the token pool directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			poolPath := expandPath(cfg.Pool.ConfigPath)
			poolDir := filepath.Dir(poolPath)

			// Create directory
			if err := os.MkdirAll(poolDir, 0700); err != nil {
				return fmt.Errorf("create pool directory: %w", err)
			}

			// Create default pool config if not exists
			if _, err := os.Stat(poolPath); os.IsNotExist(err) {
				poolCfg := tokenpool.DefaultPoolConfig()
				if err := poolCfg.Save(poolPath); err != nil {
					return fmt.Errorf("create pool config: %w", err)
				}
				fmt.Printf("Initialized token pool at %s\n", poolDir)
			} else {
				fmt.Printf("Token pool already initialized at %s\n", poolDir)
			}

			return nil
		},
	}
}

func newPoolAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Add a new account to the pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			// Create temp directory for auth
			tempDir, err := os.MkdirTemp("", "orc-pool-"+name+"-")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer func() { _ = os.RemoveAll(tempDir) }()

			claudeDir := filepath.Join(tempDir, ".claude")
			if err := os.MkdirAll(claudeDir, 0700); err != nil {
				return fmt.Errorf("create claude dir: %w", err)
			}

			fmt.Printf("Opening browser for authentication...\n")
			fmt.Printf("Please log in with the account you want to add as '%s'\n\n", name)

			// Run claude login with custom config dir
			claudeCmd := exec.Command("claude", "login")
			claudeCmd.Env = append(os.Environ(), "CLAUDE_CONFIG_DIR="+tempDir)
			claudeCmd.Stdin = os.Stdin
			claudeCmd.Stdout = os.Stdout
			claudeCmd.Stderr = os.Stderr

			if err := claudeCmd.Run(); err != nil {
				return fmt.Errorf("claude login failed: %w", err)
			}

			// Read credentials from temp dir
			credsPath := filepath.Join(claudeDir, ".credentials.json")
			credsData, err := os.ReadFile(credsPath)
			if err != nil {
				return fmt.Errorf("read credentials: %w", err)
			}

			var creds struct {
				ClaudeAiOauth struct {
					AccessToken  string `json:"accessToken"`
					RefreshToken string `json:"refreshToken"`
				} `json:"claudeAiOauth"`
			}
			if err := json.Unmarshal(credsData, &creds); err != nil {
				return fmt.Errorf("parse credentials: %w", err)
			}

			if creds.ClaudeAiOauth.AccessToken == "" {
				return fmt.Errorf("no OAuth token found - login may have failed")
			}

			// Load or create pool
			pool, err := tokenpool.New(cfg.Pool.ConfigPath)
			if err != nil {
				// Create new pool if doesn't exist
				poolPath := expandPath(cfg.Pool.ConfigPath)
				poolDir := filepath.Dir(poolPath)
				if err := os.MkdirAll(poolDir, 0700); err != nil {
					return fmt.Errorf("create pool dir: %w", err)
				}
				pool, err = tokenpool.New(cfg.Pool.ConfigPath)
				if err != nil {
					return fmt.Errorf("create pool: %w", err)
				}
			}

			// Add account
			account := &tokenpool.Account{
				ID:           name,
				Name:         name,
				AccessToken:  creds.ClaudeAiOauth.AccessToken,
				RefreshToken: creds.ClaudeAiOauth.RefreshToken,
				Enabled:      true,
			}

			if err := pool.AddAccount(account); err != nil {
				return fmt.Errorf("add account: %w", err)
			}

			fmt.Printf("\nSuccessfully added account '%s' to the pool\n", name)

			return nil
		},
	}
}

func newPoolListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all accounts in the pool",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			pool, err := tokenpool.New(cfg.Pool.ConfigPath)
			if err != nil {
				return fmt.Errorf("load pool: %w", err)
			}

			accounts := pool.Accounts()
			if len(accounts) == 0 {
				fmt.Println("No accounts configured. Use 'orc pool add <name>' to add an account.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tNAME\tENABLED")
			_, _ = fmt.Fprintln(w, "--\t----\t-------")

			current := pool.Current()
			for _, acc := range accounts {
				marker := ""
				if current != nil && acc.ID == current.ID {
					marker = "*"
				}
				_, _ = fmt.Fprintf(w, "%s%s\t%s\t%v\n", marker, acc.ID, acc.Name, acc.Enabled)
			}
			_ = w.Flush()

			if current != nil {
				fmt.Printf("\n* = current account\n")
			}

			return nil
		},
	}
}

func newPoolStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show detailed status of all accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			pool, err := tokenpool.New(cfg.Pool.ConfigPath)
			if err != nil {
				return fmt.Errorf("load pool: %w", err)
			}

			statuses := pool.Status()
			if len(statuses) == 0 {
				fmt.Println("No accounts configured.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSTATUS\tEXHAUSTED\tLAST ERROR")
			_, _ = fmt.Fprintln(w, "--\t------\t---------\t----------")

			for _, s := range statuses {
				status := "ready"
				if s.IsCurrent {
					status = "active"
				}
				if !s.Account.Enabled {
					status = "disabled"
				}

				exhausted := "-"
				if s.State != nil && s.State.Exhausted {
					exhausted = "yes"
					if s.State.ExhaustedAt != nil {
						exhausted = s.State.ExhaustedAt.Format("15:04:05")
					}
				}

				lastError := "-"
				if s.State != nil && s.State.LastError != "" {
					lastError = truncate(s.State.LastError, 40)
				}

				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Account.ID, status, exhausted, lastError)
			}
			_ = w.Flush()

			fmt.Printf("\nStrategy: %s\n", pool.Strategy())
			fmt.Printf("Auto-switch on rate limit: %v\n", pool.SwitchOnRateLimit())

			return nil
		},
	}
}

func newPoolSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <account-id>",
		Short: "Manually switch to a specific account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			accountID := args[0]

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			pool, err := tokenpool.New(cfg.Pool.ConfigPath)
			if err != nil {
				return fmt.Errorf("load pool: %w", err)
			}

			if err := pool.SwitchTo(accountID); err != nil {
				return fmt.Errorf("switch account: %w", err)
			}

			fmt.Printf("Switched to account '%s'\n", accountID)
			return nil
		},
	}
}

func newPoolRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <account-id>",
		Short: "Remove an account from the pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			accountID := args[0]

			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			pool, err := tokenpool.New(cfg.Pool.ConfigPath)
			if err != nil {
				return fmt.Errorf("load pool: %w", err)
			}

			if err := pool.RemoveAccount(accountID); err != nil {
				return fmt.Errorf("remove account: %w", err)
			}

			fmt.Printf("Removed account '%s' from the pool\n", accountID)
			return nil
		},
	}
}

func newPoolResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset all exhausted flags",
		Long:  "Clear the exhausted flag for all accounts, allowing them to be used again.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}

			pool, err := tokenpool.New(cfg.Pool.ConfigPath)
			if err != nil {
				return fmt.Errorf("load pool: %w", err)
			}

			pool.ResetExhausted()
			fmt.Println("Reset all account exhaustion flags")
			return nil
		},
	}
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
