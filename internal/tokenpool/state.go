package tokenpool

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// State tracks runtime state for the token pool.
type State struct {
	// CurrentIndex is the current position in round-robin selection
	CurrentIndex int `yaml:"current_index"`

	// Accounts maps account ID to its runtime state
	Accounts map[string]*AccountState `yaml:"accounts"`

	// path is the file path for persistence (not serialized)
	path string `yaml:"-"`
}

// NewState creates a new empty state.
func NewState(path string) *State {
	return &State{
		CurrentIndex: 0,
		Accounts:     make(map[string]*AccountState),
		path:         path,
	}
}

// LoadState loads state from the specified path.
func LoadState(path string) (*State, error) {
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
			return NewState(path), nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}

	state := NewState(path)
	if err := yaml.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	state.path = path

	return state, nil
}

// Save writes the state to disk.
func (s *State) Save() error {
	if s.path == "" {
		return fmt.Errorf("state path not set")
	}

	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}

	return nil
}

// GetAccountState returns the state for an account, creating if needed.
func (s *State) GetAccountState(accountID string) *AccountState {
	if s.Accounts == nil {
		s.Accounts = make(map[string]*AccountState)
	}

	state, exists := s.Accounts[accountID]
	if !exists {
		state = &AccountState{}
		s.Accounts[accountID] = state
	}
	return state
}

// MarkExhausted marks an account as exhausted due to rate limiting.
func (s *State) MarkExhausted(accountID string, reason string) {
	state := s.GetAccountState(accountID)
	state.Exhausted = true
	now := time.Now()
	state.ExhaustedAt = &now
	state.LastError = reason
}

// ClearExhausted clears the exhausted flag for an account.
func (s *State) ClearExhausted(accountID string) {
	state := s.GetAccountState(accountID)
	state.Exhausted = false
	state.ExhaustedAt = nil
	state.LastError = ""
}

// ResetAllExhausted clears exhausted flags for all accounts.
func (s *State) ResetAllExhausted() {
	for _, state := range s.Accounts {
		state.Exhausted = false
		state.ExhaustedAt = nil
		state.LastError = ""
	}
}

// IsExhausted returns true if the account is marked as exhausted.
func (s *State) IsExhausted(accountID string) bool {
	state := s.GetAccountState(accountID)
	return state.Exhausted
}

// SetCurrentIndex updates the current round-robin index.
func (s *State) SetCurrentIndex(index int) {
	s.CurrentIndex = index
}
