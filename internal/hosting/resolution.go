package hosting

import (
	"fmt"
	"os"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
)

// ResolvedConfig is the fully resolved hosting configuration for a project.
type ResolvedConfig struct {
	Config
	AccountName  string
	ProviderType ProviderType
}

// DefaultTokenEnvVar returns the default token environment variable for a provider.
func DefaultTokenEnvVar(provider ProviderType) string {
	if provider == ProviderGitLab {
		return "ORC_GITLAB_TOKEN"
	}
	return "ORC_GITHUB_TOKEN"
}

// ResolveTokenFromEnv resolves the configured token environment variable.
func ResolveTokenFromEnv(cfg Config, provider ProviderType) (string, error) {
	envVar := strings.TrimSpace(cfg.TokenEnvVar)
	if envVar == "" {
		envVar = DefaultTokenEnvVar(provider)
	}

	token := strings.TrimSpace(os.Getenv(envVar))
	if token == "" {
		return "", fmt.Errorf("%s environment variable is not set (required for %s API access)", envVar, provider)
	}
	return token, nil
}

// ResolveConfig builds the effective hosting configuration for a project.
func ResolveConfig(workDir string, appCfg *config.Config) (ResolvedConfig, error) {
	cfg := Config{}
	accountName := ""
	if appCfg != nil {
		cfg.Provider = strings.TrimSpace(appCfg.Hosting.Provider)
		cfg.BaseURL = strings.TrimSpace(appCfg.Hosting.BaseURL)
		cfg.TokenEnvVar = strings.TrimSpace(appCfg.Hosting.TokenEnvVar)
		accountName = strings.TrimSpace(appCfg.Hosting.Account)
	}

	if accountName != "" {
		registry, err := LoadAccounts()
		if err != nil {
			return ResolvedConfig{}, fmt.Errorf("load hosting accounts: %w", err)
		}
		account, ok := registry.Accounts[accountName]
		if !ok {
			path, pathErr := AccountsPath()
			if pathErr != nil {
				return ResolvedConfig{}, fmt.Errorf("resolve hosting account %q: %w", accountName, pathErr)
			}
			return ResolvedConfig{}, fmt.Errorf("hosting.account %q not found in %s", accountName, path)
		}
		if err := mergeAccountConfig(&cfg, accountName, account); err != nil {
			return ResolvedConfig{}, err
		}
	}

	providerType, err := resolveProviderType(workDir, cfg)
	if err != nil {
		return ResolvedConfig{}, err
	}
	cfg.Provider = string(providerType)
	if strings.TrimSpace(cfg.TokenEnvVar) == "" {
		cfg.TokenEnvVar = DefaultTokenEnvVar(providerType)
	}

	return ResolvedConfig{
		Config:       cfg,
		AccountName:  accountName,
		ProviderType: providerType,
	}, nil
}

// NewProviderFromAppConfig resolves the effective hosting config and creates a provider.
func NewProviderFromAppConfig(workDir string, appCfg *config.Config) (Provider, error) {
	resolved, err := ResolveConfig(workDir, appCfg)
	if err != nil {
		return nil, err
	}
	return NewProvider(workDir, resolved.Config)
}

func mergeAccountConfig(cfg *Config, accountName string, account Account) error {
	accountProvider := normalizeProviderSetting(account.Provider)
	currentProvider := normalizeProviderSetting(cfg.Provider)
	if currentProvider != "" && accountProvider != "" && currentProvider != accountProvider {
		return fmt.Errorf("hosting.account %q provider %q conflicts with hosting.provider %q", accountName, account.Provider, cfg.Provider)
	}
	if currentProvider == "" {
		cfg.Provider = accountProvider
	}

	if strings.TrimSpace(cfg.BaseURL) != "" && strings.TrimSpace(account.BaseURL) != "" && !sameBaseURL(cfg.BaseURL, account.BaseURL) {
		return fmt.Errorf("hosting.account %q base_url %q conflicts with hosting.base_url %q", accountName, account.BaseURL, cfg.BaseURL)
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = strings.TrimSpace(account.BaseURL)
	}

	if strings.TrimSpace(cfg.TokenEnvVar) != "" && strings.TrimSpace(account.TokenEnvVar) != "" && strings.TrimSpace(cfg.TokenEnvVar) != strings.TrimSpace(account.TokenEnvVar) {
		return fmt.Errorf("hosting.account %q token_env_var %q conflicts with hosting.token_env_var %q", accountName, account.TokenEnvVar, cfg.TokenEnvVar)
	}
	if strings.TrimSpace(cfg.TokenEnvVar) == "" {
		cfg.TokenEnvVar = strings.TrimSpace(account.TokenEnvVar)
	}

	return nil
}

func normalizeProviderSetting(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" || trimmed == "auto" {
		return ""
	}
	return trimmed
}

func sameBaseURL(left string, right string) bool {
	normalize := func(value string) string {
		return strings.TrimRight(strings.TrimSpace(value), "/")
	}
	return normalize(left) == normalize(right)
}
