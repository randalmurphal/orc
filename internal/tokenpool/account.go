// Package tokenpool provides OAuth token pool management for automatic
// account switching when rate limits are hit.
package tokenpool

import (
	"time"
)

// Account represents a single OAuth credential set for Claude Code.
type Account struct {
	// ID is the unique identifier for this account
	ID string `yaml:"id"`

	// Name is a human-readable name for the account
	Name string `yaml:"name"`

	// AccessToken is the OAuth access token (sk-ant-oat01-...)
	AccessToken string `yaml:"access_token"`

	// RefreshToken is used to refresh expired access tokens (sk-ant-ort01-...)
	RefreshToken string `yaml:"refresh_token"`

	// Enabled controls whether this account is used in the pool
	Enabled bool `yaml:"enabled"`
}

// AccountState tracks runtime state for an account.
type AccountState struct {
	// Exhausted indicates the account hit rate limits
	Exhausted bool `yaml:"exhausted"`

	// ExhaustedAt is when the account was marked exhausted
	ExhaustedAt *time.Time `yaml:"exhausted_at,omitempty"`

	// LastError is the last error message from this account
	LastError string `yaml:"last_error,omitempty"`
}

// IsUsable returns true if the account can be used (enabled and not exhausted).
func (a *Account) IsUsable(state *AccountState) bool {
	if !a.Enabled {
		return false
	}
	if state != nil && state.Exhausted {
		return false
	}
	return true
}

// Token returns the access token for use with CLAUDE_CODE_OAUTH_TOKEN.
func (a *Account) Token() string {
	return a.AccessToken
}

// Redacted returns a copy of the account with tokens redacted for logging.
func (a *Account) Redacted() Account {
	redacted := *a
	if len(a.AccessToken) > 20 {
		redacted.AccessToken = a.AccessToken[:20] + "..."
	}
	if len(a.RefreshToken) > 20 {
		redacted.RefreshToken = a.RefreshToken[:20] + "..."
	}
	return redacted
}
