package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/project"
)

func newHostingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosting",
		Short: "Manage hosting accounts and project account selection",
	}

	cmd.AddCommand(newHostingAccountsCmd())
	return cmd
}

func newHostingAccountsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accounts",
		Short: "Manage named hosting accounts stored under ~/.orc",
	}

	cmd.AddCommand(newHostingAccountsListCmd())
	cmd.AddCommand(newHostingAccountsAddCmd())
	cmd.AddCommand(newHostingAccountsRemoveCmd())
	cmd.AddCommand(newHostingAccountsUseCmd())
	cmd.AddCommand(newHostingAccountsCheckCmd())
	return cmd
}

func newHostingAccountsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured hosting accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := hosting.LoadAccounts()
			if err != nil {
				return err
			}

			path, err := hosting.AccountsPath()
			if err != nil {
				return err
			}
			selected := currentProjectHostingAccount()

			if len(registry.Accounts) == 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "No hosting accounts configured in %s\n", path)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Accounts: %s\n", path)
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tPROVIDER\tBASE_URL\tTOKEN_ENV_VAR\tSELECTED")
			for _, name := range registry.Names() {
				account := registry.Accounts[name]
				selectedMark := ""
				if name == selected {
					selectedMark = "*"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					name,
					account.Provider,
					displayOrDash(account.BaseURL),
					effectiveTokenEnvVar(account),
					selectedMark,
				)
			}
			_ = w.Flush()
			if selected != "" {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "* current project account")
			}
			return nil
		},
	}
}

func newHostingAccountsAddCmd() *cobra.Command {
	var provider string
	var baseURL string
	var tokenEnvVar string
	var force bool

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a named hosting account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			registry, err := hosting.LoadAccounts()
			if err != nil {
				return err
			}

			if _, exists := registry.Accounts[name]; exists && !force {
				return fmt.Errorf("hosting account %q already exists; use --force to overwrite", name)
			}

			registry.Accounts[name] = hosting.Account{
				Provider:    provider,
				BaseURL:     baseURL,
				TokenEnvVar: tokenEnvVar,
			}

			path, err := hosting.AccountsPath()
			if err != nil {
				return err
			}
			if err := registry.Save(path); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Saved hosting account %q to %s\n", name, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&provider, "provider", "", "Hosting provider (github or gitlab)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL for GHE or self-hosted GitLab")
	cmd.Flags().StringVar(&tokenEnvVar, "token-env-var", "", "Environment variable containing the API token")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing account")
	_ = cmd.MarkFlagRequired("provider")
	return cmd
}

func newHostingAccountsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a named hosting account",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			registry, err := hosting.LoadAccounts()
			if err != nil {
				return err
			}
			if _, exists := registry.Accounts[name]; !exists {
				return fmt.Errorf("hosting account %q does not exist", name)
			}
			delete(registry.Accounts, name)

			path, err := hosting.AccountsPath()
			if err != nil {
				return err
			}
			if err := registry.Save(path); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed hosting account %q from %s\n", name, path)
			return nil
		},
	}
}

func newHostingAccountsUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Select a hosting account for the current project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			registry, err := hosting.LoadAccounts()
			if err != nil {
				return err
			}
			if _, ok := registry.Accounts[name]; !ok {
				return fmt.Errorf("hosting account %q does not exist", name)
			}

			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return err
			}
			projectID, err := project.ResolveProjectID(projectRoot)
			if err != nil {
				return fmt.Errorf("project must be registered before selecting a hosting account: %w", err)
			}
			configPath, err := writeProjectHostingAccountSelection(projectID, name)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Set hosting.account = %s in %s\n", name, configPath)
			return nil
		},
	}
}

func newHostingAccountsCheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [name]",
		Short: "Check resolved hosting account and token availability",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectRoot, err := ResolveProjectPath()
			if err != nil && len(args) == 0 {
				return err
			}

			var resolved hosting.ResolvedConfig
			if len(args) == 1 {
				registry, loadErr := hosting.LoadAccounts()
				if loadErr != nil {
					return loadErr
				}
				account, ok := registry.Accounts[args[0]]
				if !ok {
					return fmt.Errorf("hosting account %q does not exist", args[0])
				}
				cfg := config.Default()
				cfg.Hosting.Account = args[0]
				cfg.Hosting.Provider = account.Provider
				cfg.Hosting.BaseURL = account.BaseURL
				cfg.Hosting.TokenEnvVar = account.TokenEnvVar
				resolved, err = hosting.ResolveConfig(projectRoot, cfg)
			} else {
				cfg, loadErr := config.LoadFrom(projectRoot)
				if loadErr != nil {
					return fmt.Errorf("load config: %w", loadErr)
				}
				resolved, err = hosting.ResolveConfig(projectRoot, cfg)
			}
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Provider: %s\n", resolved.ProviderType)
			if resolved.AccountName != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Account: %s\n", resolved.AccountName)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Base URL: %s\n", displayOrDash(resolved.BaseURL))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Token env var: %s\n", resolved.TokenEnvVar)

			if token := os.Getenv(resolved.TokenEnvVar); token == "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auth: missing (%s is not set)\n", resolved.TokenEnvVar)
				return fmt.Errorf("%s is not set", resolved.TokenEnvVar)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Auth: available (%s is set)\n", resolved.TokenEnvVar)
			return nil
		},
	}
}

func currentProjectHostingAccount() string {
	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return ""
	}
	cfg, err := config.LoadFrom(projectRoot)
	if err != nil {
		return ""
	}
	return cfg.Hosting.Account
}

func displayOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func effectiveTokenEnvVar(account hosting.Account) string {
	if account.TokenEnvVar != "" {
		return account.TokenEnvVar
	}
	return hosting.DefaultTokenEnvVar(hosting.ProviderType(account.Provider))
}

func writeProjectHostingAccountSelection(projectID string, accountName string) (string, error) {
	if err := project.EnsureProjectDirs(projectID); err != nil {
		return "", err
	}

	configPath, err := project.ProjectLocalConfigPath(projectID)
	if err != nil {
		return "", err
	}
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		return "", err
	}

	cfg.Hosting.Account = accountName
	cfg.Hosting.Provider = ""
	cfg.Hosting.BaseURL = ""
	cfg.Hosting.TokenEnvVar = ""
	if err := cfg.SaveTo(configPath); err != nil {
		return "", err
	}
	return configPath, nil
}
