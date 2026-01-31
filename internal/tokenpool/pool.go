package tokenpool

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
)

var (
	// ErrNoAccounts is returned when the pool has no accounts configured.
	ErrNoAccounts = errors.New("no accounts configured in pool")

	// ErrAllExhausted is returned when all accounts are exhausted.
	ErrAllExhausted = errors.New("all accounts exhausted")

	// ErrPoolDisabled is returned when the pool is not enabled.
	ErrPoolDisabled = errors.New("token pool is disabled")
)

// Pool manages a collection of OAuth accounts for automatic rotation.
type Pool struct {
	config *PoolConfig
	state  *State
	logger *slog.Logger
	mu     sync.RWMutex

	// configPath and statePath for persistence
	configPath string
	statePath  string
}

// PoolOption configures a Pool.
type PoolOption func(*Pool)

// WithLogger sets the logger for the pool.
func WithLogger(logger *slog.Logger) PoolOption {
	return func(p *Pool) {
		p.logger = logger
	}
}

// New creates a new token pool from the specified config path.
func New(configPath string, opts ...PoolOption) (*Pool, error) {
	// Determine state path (same directory as config)
	dir := filepath.Dir(configPath)
	statePath := filepath.Join(dir, "state.yaml")

	// Load config
	cfg, err := LoadPoolConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("load pool config: %w", err)
	}

	// Load state
	state, err := LoadState(statePath)
	if err != nil {
		return nil, fmt.Errorf("load pool state: %w", err)
	}

	pool := &Pool{
		config:     cfg,
		state:      state,
		logger:     slog.Default(),
		configPath: configPath,
		statePath:  statePath,
	}

	for _, opt := range opts {
		opt(pool)
	}

	return pool, nil
}

// Current returns the currently active account.
// Returns nil if no accounts are configured.
func (p *Pool) Current() *Account {
	p.mu.RLock()
	defer p.mu.RUnlock()

	accounts := p.config.EnabledAccounts()
	if len(accounts) == 0 {
		return nil
	}

	// Ensure index is within bounds
	index := p.state.CurrentIndex % len(accounts)
	return accounts[index]
}

// Token returns the OAuth token for the current account.
// Returns empty string if no account is active.
func (p *Pool) Token() string {
	account := p.Current()
	if account == nil {
		return ""
	}
	return account.Token()
}

// Next advances to the next available account in round-robin order.
// Skips exhausted accounts. Returns error if all accounts are exhausted.
func (p *Pool) Next() (*Account, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	accounts := p.config.EnabledAccounts()
	if len(accounts) == 0 {
		return nil, ErrNoAccounts
	}

	startIndex := p.state.CurrentIndex
	numAccounts := len(accounts)

	// Try each account in round-robin order
	for i := range numAccounts {
		nextIndex := (startIndex + i + 1) % numAccounts
		account := accounts[nextIndex]

		if !p.state.IsExhausted(account.ID) {
			p.state.SetCurrentIndex(nextIndex)
			if err := p.state.Save(); err != nil {
				p.logger.Warn("failed to save pool state", "error", err)
			}

			p.logger.Info("switched to next account",
				"account_id", account.ID,
				"account_name", account.Name,
				"index", nextIndex)

			return account, nil
		}
	}

	return nil, ErrAllExhausted
}

// MarkExhausted marks the current account as exhausted due to rate limiting.
func (p *Pool) MarkExhausted(reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	account := p.currentLocked()
	if account == nil {
		return
	}

	p.state.MarkExhausted(account.ID, reason)
	if err := p.state.Save(); err != nil {
		p.logger.Warn("failed to save pool state", "error", err)
	}

	p.logger.Info("marked account as exhausted",
		"account_id", account.ID,
		"account_name", account.Name,
		"reason", reason)
}

// ResetExhausted clears exhausted flags for all accounts.
func (p *Pool) ResetExhausted() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state.ResetAllExhausted()
	if err := p.state.Save(); err != nil {
		p.logger.Warn("failed to save pool state", "error", err)
	}

	p.logger.Info("reset all account exhaustion flags")
}

// HasAvailable returns true if any non-exhausted accounts are available.
func (p *Pool) HasAvailable() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, account := range p.config.EnabledAccounts() {
		if !p.state.IsExhausted(account.ID) {
			return true
		}
	}
	return false
}

// Accounts returns a copy of all configured accounts (for listing).
// The returned slice and accounts are copies - safe to use after the call.
func (p *Pool) Accounts() []*Account {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy of the slice with copied accounts
	result := make([]*Account, len(p.config.Accounts))
	for i, acc := range p.config.Accounts {
		copied := *acc
		result[i] = &copied
	}
	return result
}

// AccountStatus returns the status of an account.
type AccountStatus struct {
	Account   *Account
	State     *AccountState
	IsCurrent bool
}

// Status returns the status of all accounts.
// Returns copies of account and state data - safe to use after the call.
func (p *Pool) Status() []AccountStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	current := p.currentLocked()
	var statuses []AccountStatus

	for _, account := range p.config.Accounts {
		state := p.state.GetAccountState(account.ID)

		// Copy account and state to avoid race conditions
		accCopy := *account
		var stateCopy *AccountState
		if state != nil {
			sc := *state
			stateCopy = &sc
		}

		statuses = append(statuses, AccountStatus{
			Account:   &accCopy,
			State:     stateCopy,
			IsCurrent: current != nil && current.ID == account.ID,
		})
	}

	return statuses
}

// SwitchTo switches to a specific account by ID.
func (p *Pool) SwitchTo(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	accounts := p.config.EnabledAccounts()
	for i, account := range accounts {
		if account.ID == accountID {
			p.state.SetCurrentIndex(i)
			if err := p.state.Save(); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			p.logger.Info("switched to account",
				"account_id", account.ID,
				"account_name", account.Name)
			return nil
		}
	}

	return fmt.Errorf("account %q not found or not enabled", accountID)
}

// AddAccount adds an account to the pool.
func (p *Pool) AddAccount(account *Account) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.config.AddAccount(account); err != nil {
		return err
	}

	return p.config.Save(p.configPath)
}

// RemoveAccount removes an account from the pool.
func (p *Pool) RemoveAccount(accountID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.config.RemoveAccount(accountID); err != nil {
		return err
	}

	// Clean up state
	delete(p.state.Accounts, accountID)
	if err := p.state.Save(); err != nil {
		p.logger.Warn("failed to save state after account removal", "error", err)
	}

	return p.config.Save(p.configPath)
}

// currentLocked returns the current account (caller must hold lock).
func (p *Pool) currentLocked() *Account {
	accounts := p.config.EnabledAccounts()
	if len(accounts) == 0 {
		return nil
	}

	index := p.state.CurrentIndex % len(accounts)
	return accounts[index]
}

// Strategy returns the configured selection strategy.
func (p *Pool) Strategy() Strategy {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.Strategy
}

// SwitchOnRateLimit returns whether auto-switching is enabled.
func (p *Pool) SwitchOnRateLimit() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.SwitchOnRateLimit
}
