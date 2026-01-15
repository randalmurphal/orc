package tokenpool

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Strategy defines how accounts are selected from the pool.
type Strategy string

const (
	// StrategyRoundRobin rotates through accounts evenly
	StrategyRoundRobin Strategy = "round-robin"

	// StrategyFailover uses accounts in priority order
	StrategyFailover Strategy = "failover"

	// StrategyLowestUtilization picks account with most remaining capacity
	// (requires usage API integration - future enhancement)
	StrategyLowestUtilization Strategy = "lowest-utilization"
)

// PoolConfig is the configuration for the token pool stored in pool.yaml.
type PoolConfig struct {
	// Version is the config file version
	Version int `yaml:"version"`

	// Strategy defines how accounts are selected (default: round-robin)
	Strategy Strategy `yaml:"strategy"`

	// SwitchOnRateLimit enables automatic switching when rate limits are hit
	SwitchOnRateLimit bool `yaml:"switch_on_rate_limit"`

	// Accounts is the list of OAuth accounts in the pool
	Accounts []*Account `yaml:"accounts"`
}

// DefaultPoolConfig returns the default pool configuration.
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		Version:           1,
		Strategy:          StrategyRoundRobin,
		SwitchOnRateLimit: true,
		Accounts:          []*Account{},
	}
}

// LoadPoolConfig loads pool configuration from the specified path.
func LoadPoolConfig(path string) (*PoolConfig, error) {
	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultPoolConfig(), nil
		}
		return nil, fmt.Errorf("read pool config: %w", err)
	}

	cfg := DefaultPoolConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	return cfg, nil
}

// Save writes the pool configuration to the specified path.
func (c *PoolConfig) Save(path string) error {
	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create pool config dir: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal pool config: %w", err)
	}

	// Write atomically: temp file then rename (prevents corruption on crash)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("write pool config temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) // Clean up temp file on rename failure
		return fmt.Errorf("rename pool config: %w", err)
	}

	return nil
}

// AddAccount adds an account to the pool configuration.
func (c *PoolConfig) AddAccount(account *Account) error {
	// Check for duplicate ID
	for _, existing := range c.Accounts {
		if existing.ID == account.ID {
			return fmt.Errorf("account with ID %q already exists", account.ID)
		}
	}

	c.Accounts = append(c.Accounts, account)
	return nil
}

// RemoveAccount removes an account from the pool configuration by ID.
func (c *PoolConfig) RemoveAccount(id string) error {
	for i, account := range c.Accounts {
		if account.ID == id {
			c.Accounts = append(c.Accounts[:i], c.Accounts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("account %q not found", id)
}

// GetAccount returns an account by ID.
func (c *PoolConfig) GetAccount(id string) (*Account, error) {
	for _, account := range c.Accounts {
		if account.ID == id {
			return account, nil
		}
	}
	return nil, fmt.Errorf("account %q not found", id)
}

// EnabledAccounts returns only enabled accounts.
func (c *PoolConfig) EnabledAccounts() []*Account {
	var enabled []*Account
	for _, account := range c.Accounts {
		if account.Enabled {
			enabled = append(enabled, account)
		}
	}
	return enabled
}
