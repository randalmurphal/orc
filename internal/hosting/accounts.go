package hosting

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/project"
	"gopkg.in/yaml.v3"
)

const accountsFileName = "hosting_accounts.yaml"

// Account describes a named hosting account stored in ~/.orc/hosting_accounts.yaml.
type Account struct {
	Provider    string `yaml:"provider"`
	BaseURL     string `yaml:"base_url,omitempty"`
	TokenEnvVar string `yaml:"token_env_var,omitempty"`
}

// AccountRegistry stores named hosting accounts.
type AccountRegistry struct {
	Accounts map[string]Account `yaml:"accounts"`
}

// AccountsPath returns the global hosting account registry path.
func AccountsPath() (string, error) {
	globalDir, err := project.GlobalPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalDir, accountsFileName), nil
}

// LoadAccounts loads the global hosting account registry.
func LoadAccounts() (*AccountRegistry, error) {
	path, err := AccountsPath()
	if err != nil {
		return nil, err
	}
	return LoadAccountsFrom(path)
}

// LoadAccountsFrom loads the account registry from a specific path.
func LoadAccountsFrom(path string) (*AccountRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AccountRegistry{Accounts: map[string]Account{}}, nil
		}
		return nil, fmt.Errorf("read hosting accounts %s: %w", path, err)
	}

	registry := &AccountRegistry{}
	if err := yaml.Unmarshal(data, registry); err != nil {
		return nil, fmt.Errorf("parse hosting accounts %s: %w", path, err)
	}
	if registry.Accounts == nil {
		registry.Accounts = map[string]Account{}
	}
	if err := registry.Validate(); err != nil {
		return nil, fmt.Errorf("validate hosting accounts %s: %w", path, err)
	}
	return registry, nil
}

// Save writes the account registry to disk atomically.
func (r *AccountRegistry) Save(path string) error {
	if r == nil {
		return fmt.Errorf("hosting account registry is required")
	}
	if err := r.Validate(); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create hosting account directory: %w", err)
	}

	data, err := yaml.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal hosting accounts: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp hosting accounts: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename hosting accounts: %w", err)
	}
	return nil
}

// Validate checks account names and fields.
func (r *AccountRegistry) Validate() error {
	if r == nil {
		return fmt.Errorf("hosting account registry is required")
	}
	for name, account := range r.Accounts {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("account name cannot be empty")
		}
		if err := validateAccount(name, account); err != nil {
			return err
		}
	}
	return nil
}

// Names returns sorted account names.
func (r *AccountRegistry) Names() []string {
	if r == nil || len(r.Accounts) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.Accounts))
	for name := range r.Accounts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validateAccount(name string, account Account) error {
	provider := strings.TrimSpace(account.Provider)
	switch provider {
	case string(ProviderGitHub), string(ProviderGitLab):
	default:
		return fmt.Errorf("account %q has invalid provider %q (must be github or gitlab)", name, account.Provider)
	}
	if strings.TrimSpace(account.TokenEnvVar) == "" {
		return nil
	}
	return nil
}
