package hosting

import (
	"fmt"
	"os/exec"
	"strings"
)

// Config holds hosting provider configuration.
type Config struct {
	// Provider type: "github", "gitlab", or "auto" (default).
	// When "auto", the provider is detected from the git remote URL.
	Provider string `yaml:"provider" json:"provider"`

	// BaseURL for self-hosted instances (e.g., "https://gitlab.company.com").
	// Leave empty for github.com / gitlab.com.
	BaseURL string `yaml:"base_url" json:"base_url,omitempty"`

	// TokenEnvVar overrides the default token environment variable name.
	// Default: GITHUB_TOKEN for GitHub, GITLAB_TOKEN for GitLab.
	TokenEnvVar string `yaml:"token_env_var" json:"token_env_var,omitempty"`
}

// NewProviderFunc is a constructor function for creating a hosting provider.
// This is used by the factory to avoid import cycles — the actual GitHub/GitLab
// constructors are registered at init time by the provider packages.
type NewProviderFunc func(workDir string, cfg Config) (Provider, error)

// Provider constructors registered by provider packages.
var providerConstructors = map[ProviderType]NewProviderFunc{}

// RegisterProvider registers a provider constructor.
// Called from init() in provider packages (github/, gitlab/).
func RegisterProvider(providerType ProviderType, constructor NewProviderFunc) {
	providerConstructors[providerType] = constructor
}

// NewProvider creates a hosting provider for the given working directory.
// If cfg.Provider is "auto" or empty, the provider is detected from the git remote URL.
func NewProvider(workDir string, cfg Config) (Provider, error) {
	providerType, err := resolveProviderType(workDir, cfg)
	if err != nil {
		return nil, err
	}

	constructor, ok := providerConstructors[providerType]
	if !ok {
		return nil, fmt.Errorf("no provider registered for %q (registered: %v)", providerType, registeredProviders())
	}

	return constructor(workDir, cfg)
}

// resolveProviderType determines which provider to use.
func resolveProviderType(workDir string, cfg Config) (ProviderType, error) {
	if cfg.Provider != "" && cfg.Provider != "auto" {
		pt := ProviderType(cfg.Provider)
		if pt != ProviderGitHub && pt != ProviderGitLab {
			return "", fmt.Errorf("unknown provider %q (supported: github, gitlab)", cfg.Provider)
		}
		return pt, nil
	}

	// Auto-detect from git remote
	remoteURL, err := getRemoteURL(workDir)
	if err != nil {
		return "", fmt.Errorf("detect provider: %w", err)
	}

	detected := DetectProvider(remoteURL)
	if detected == ProviderUnknown {
		return "", fmt.Errorf("cannot detect hosting provider from remote URL %q (set provider explicitly in config)", remoteURL)
	}

	return detected, nil
}

// getRemoteURL gets the origin remote URL for the repo at workDir.
func getRemoteURL(workDir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func registeredProviders() []ProviderType {
	var providers []ProviderType
	for pt := range providerConstructors {
		providers = append(providers, pt)
	}
	return providers
}
