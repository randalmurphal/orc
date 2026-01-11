package tokenpool

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestAccount_IsUsable(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		state   *AccountState
		want    bool
	}{
		{
			name:    "enabled account with nil state",
			account: &Account{Enabled: true},
			state:   nil,
			want:    true,
		},
		{
			name:    "enabled account not exhausted",
			account: &Account{Enabled: true},
			state:   &AccountState{Exhausted: false},
			want:    true,
		},
		{
			name:    "enabled account but exhausted",
			account: &Account{Enabled: true},
			state:   &AccountState{Exhausted: true},
			want:    false,
		},
		{
			name:    "disabled account",
			account: &Account{Enabled: false},
			state:   nil,
			want:    false,
		},
		{
			name:    "disabled account with state",
			account: &Account{Enabled: false},
			state:   &AccountState{Exhausted: false},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.IsUsable(tt.state); got != tt.want {
				t.Errorf("Account.IsUsable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAccount_Token(t *testing.T) {
	account := &Account{
		AccessToken: "sk-ant-oat01-test-token-12345",
	}

	if got := account.Token(); got != account.AccessToken {
		t.Errorf("Account.Token() = %v, want %v", got, account.AccessToken)
	}
}

func TestAccount_Redacted(t *testing.T) {
	account := &Account{
		ID:           "test",
		Name:         "Test Account",
		AccessToken:  "sk-ant-oat01-this-is-a-very-long-token",
		RefreshToken: "sk-ant-ort01-this-is-a-very-long-refresh-token",
		Enabled:      true,
	}

	redacted := account.Redacted()

	// Should preserve non-sensitive fields
	if redacted.ID != account.ID {
		t.Errorf("Redacted().ID = %v, want %v", redacted.ID, account.ID)
	}
	if redacted.Name != account.Name {
		t.Errorf("Redacted().Name = %v, want %v", redacted.Name, account.Name)
	}
	if redacted.Enabled != account.Enabled {
		t.Errorf("Redacted().Enabled = %v, want %v", redacted.Enabled, account.Enabled)
	}

	// Should truncate tokens
	if len(redacted.AccessToken) > 25 {
		t.Errorf("Redacted().AccessToken not truncated: %v", redacted.AccessToken)
	}
	if len(redacted.RefreshToken) > 25 {
		t.Errorf("Redacted().RefreshToken not truncated: %v", redacted.RefreshToken)
	}
}

func TestPoolConfig_AddRemoveAccount(t *testing.T) {
	cfg := DefaultPoolConfig()

	// Add first account
	acc1 := &Account{ID: "acc1", Name: "Account 1", Enabled: true}
	if err := cfg.AddAccount(acc1); err != nil {
		t.Fatalf("AddAccount(acc1) error = %v", err)
	}

	if len(cfg.Accounts) != 1 {
		t.Errorf("Expected 1 account, got %d", len(cfg.Accounts))
	}

	// Add second account
	acc2 := &Account{ID: "acc2", Name: "Account 2", Enabled: true}
	if err := cfg.AddAccount(acc2); err != nil {
		t.Fatalf("AddAccount(acc2) error = %v", err)
	}

	if len(cfg.Accounts) != 2 {
		t.Errorf("Expected 2 accounts, got %d", len(cfg.Accounts))
	}

	// Add duplicate should fail
	accDup := &Account{ID: "acc1", Name: "Duplicate"}
	if err := cfg.AddAccount(accDup); err == nil {
		t.Error("AddAccount(duplicate) should return error")
	}

	// Remove account
	if err := cfg.RemoveAccount("acc1"); err != nil {
		t.Fatalf("RemoveAccount(acc1) error = %v", err)
	}

	if len(cfg.Accounts) != 1 {
		t.Errorf("Expected 1 account after removal, got %d", len(cfg.Accounts))
	}

	// Remove non-existent should fail
	if err := cfg.RemoveAccount("nonexistent"); err == nil {
		t.Error("RemoveAccount(nonexistent) should return error")
	}
}

func TestPoolConfig_EnabledAccounts(t *testing.T) {
	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Name: "Account 1", Enabled: true},
		{ID: "acc2", Name: "Account 2", Enabled: false},
		{ID: "acc3", Name: "Account 3", Enabled: true},
	}

	enabled := cfg.EnabledAccounts()
	if len(enabled) != 2 {
		t.Errorf("Expected 2 enabled accounts, got %d", len(enabled))
	}

	// Verify correct accounts are enabled
	for _, acc := range enabled {
		if !acc.Enabled {
			t.Errorf("Account %s should be enabled", acc.ID)
		}
	}
}

func TestPoolConfig_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	// Create config with accounts
	cfg := DefaultPoolConfig()
	cfg.Strategy = StrategyRoundRobin
	cfg.SwitchOnRateLimit = true
	cfg.Accounts = []*Account{
		{ID: "acc1", Name: "Test Account", AccessToken: "token1", Enabled: true},
	}

	// Save
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file permissions
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("File permissions = %o, want 0600", info.Mode().Perm())
	}

	// Load
	loaded, err := LoadPoolConfig(configPath)
	if err != nil {
		t.Fatalf("LoadPoolConfig() error = %v", err)
	}

	// Verify loaded config
	if loaded.Strategy != cfg.Strategy {
		t.Errorf("Strategy = %v, want %v", loaded.Strategy, cfg.Strategy)
	}
	if len(loaded.Accounts) != 1 {
		t.Errorf("Accounts count = %d, want 1", len(loaded.Accounts))
	}
	if loaded.Accounts[0].ID != "acc1" {
		t.Errorf("Account ID = %v, want acc1", loaded.Accounts[0].ID)
	}
}

func TestState_MarkExhausted(t *testing.T) {
	state := NewState("")

	// Mark exhausted
	state.MarkExhausted("acc1", "rate limit hit")

	if !state.IsExhausted("acc1") {
		t.Error("Account should be marked as exhausted")
	}

	accState := state.GetAccountState("acc1")
	if accState.LastError != "rate limit hit" {
		t.Errorf("LastError = %v, want 'rate limit hit'", accState.LastError)
	}
	if accState.ExhaustedAt == nil {
		t.Error("ExhaustedAt should be set")
	}
}

func TestState_ClearExhausted(t *testing.T) {
	state := NewState("")

	// Mark and then clear
	state.MarkExhausted("acc1", "rate limit")
	state.ClearExhausted("acc1")

	if state.IsExhausted("acc1") {
		t.Error("Account should not be exhausted after clear")
	}

	accState := state.GetAccountState("acc1")
	if accState.ExhaustedAt != nil {
		t.Error("ExhaustedAt should be nil after clear")
	}
	if accState.LastError != "" {
		t.Error("LastError should be empty after clear")
	}
}

func TestState_ResetAllExhausted(t *testing.T) {
	state := NewState("")

	// Mark multiple accounts
	state.MarkExhausted("acc1", "limit 1")
	state.MarkExhausted("acc2", "limit 2")

	// Reset all
	state.ResetAllExhausted()

	if state.IsExhausted("acc1") || state.IsExhausted("acc2") {
		t.Error("All accounts should be reset")
	}
}

func TestState_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.yaml")

	// Create state with data
	state := NewState(statePath)
	state.SetCurrentIndex(2)
	state.MarkExhausted("acc1", "test error")

	// Save
	if err := state.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load
	loaded, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("LoadState() error = %v", err)
	}

	// Verify
	if loaded.CurrentIndex != 2 {
		t.Errorf("CurrentIndex = %d, want 2", loaded.CurrentIndex)
	}
	if !loaded.IsExhausted("acc1") {
		t.Error("acc1 should be exhausted after load")
	}
}

func TestPool_Current(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	// Create pool config with accounts
	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Name: "Account 1", AccessToken: "token1", Enabled: true},
		{ID: "acc2", Name: "Account 2", AccessToken: "token2", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Current should be first account
	current := pool.Current()
	if current == nil {
		t.Fatal("Current() returned nil")
	}
	if current.ID != "acc1" {
		t.Errorf("Current().ID = %v, want acc1", current.ID)
	}
}

func TestPool_Token(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", AccessToken: "test-token-123", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	token := pool.Token()
	if token != "test-token-123" {
		t.Errorf("Token() = %v, want test-token-123", token)
	}
}

func TestPool_Next_RoundRobin(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Strategy = StrategyRoundRobin
	cfg.Accounts = []*Account{
		{ID: "acc1", Name: "Account 1", Enabled: true},
		{ID: "acc2", Name: "Account 2", Enabled: true},
		{ID: "acc3", Name: "Account 3", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// First current should be acc1
	if current := pool.Current(); current.ID != "acc1" {
		t.Errorf("Initial Current() = %v, want acc1", current.ID)
	}

	// Next should be acc2
	next, err := pool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if next.ID != "acc2" {
		t.Errorf("Next() = %v, want acc2", next.ID)
	}

	// Next should be acc3
	next, err = pool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if next.ID != "acc3" {
		t.Errorf("Next() = %v, want acc3", next.ID)
	}

	// Next should wrap around to acc1
	next, err = pool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if next.ID != "acc1" {
		t.Errorf("Next() = %v, want acc1 (wrap)", next.ID)
	}
}

func TestPool_Next_SkipsExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
		{ID: "acc2", Enabled: true},
		{ID: "acc3", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mark acc2 as exhausted
	pool.state.MarkExhausted("acc2", "test")

	// Next from acc1 should skip acc2 and go to acc3
	next, err := pool.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if next.ID != "acc3" {
		t.Errorf("Next() = %v, want acc3 (skipped exhausted acc2)", next.ID)
	}
}

func TestPool_Next_AllExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
		{ID: "acc2", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mark all as exhausted
	pool.state.MarkExhausted("acc1", "test")
	pool.state.MarkExhausted("acc2", "test")

	// Next should return error
	_, err = pool.Next()
	if err != ErrAllExhausted {
		t.Errorf("Next() error = %v, want ErrAllExhausted", err)
	}
}

func TestPool_Next_NoAccounts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	// No accounts
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = pool.Next()
	if err != ErrNoAccounts {
		t.Errorf("Next() error = %v, want ErrNoAccounts", err)
	}
}

func TestPool_MarkExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mark current as exhausted
	pool.MarkExhausted("rate limit hit")

	// Verify state
	if !pool.state.IsExhausted("acc1") {
		t.Error("Current account should be exhausted")
	}
}

func TestPool_ResetExhausted(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
		{ID: "acc2", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mark both exhausted
	pool.state.MarkExhausted("acc1", "test")
	pool.state.MarkExhausted("acc2", "test")

	// Reset
	pool.ResetExhausted()

	// Verify
	if !pool.HasAvailable() {
		t.Error("Should have available accounts after reset")
	}
}

func TestPool_SwitchTo(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
		{ID: "acc2", Enabled: true},
		{ID: "acc3", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Switch to acc3
	if err := pool.SwitchTo("acc3"); err != nil {
		t.Fatalf("SwitchTo(acc3) error = %v", err)
	}

	current := pool.Current()
	if current.ID != "acc3" {
		t.Errorf("Current() = %v, want acc3", current.ID)
	}

	// Switch to non-existent should fail
	if err := pool.SwitchTo("nonexistent"); err == nil {
		t.Error("SwitchTo(nonexistent) should return error")
	}
}

func TestPool_HasAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
		{ID: "acc2", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Initially should have available
	if !pool.HasAvailable() {
		t.Error("Should have available accounts initially")
	}

	// Mark one exhausted, should still have available
	pool.state.MarkExhausted("acc1", "test")
	if !pool.HasAvailable() {
		t.Error("Should still have available accounts with one exhausted")
	}

	// Mark all exhausted
	pool.state.MarkExhausted("acc2", "test")
	if pool.HasAvailable() {
		t.Error("Should not have available accounts when all exhausted")
	}
}

func TestPool_Status(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Name: "Account 1", Enabled: true},
		{ID: "acc2", Name: "Account 2", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Mark one exhausted
	pool.state.MarkExhausted("acc2", "rate limit")

	statuses := pool.Status()
	if len(statuses) != 2 {
		t.Fatalf("Status() returned %d items, want 2", len(statuses))
	}

	// First should be current
	if !statuses[0].IsCurrent {
		t.Error("First status should be current")
	}

	// Second should be exhausted
	if statuses[1].State == nil || !statuses[1].State.Exhausted {
		t.Error("Second account should be exhausted")
	}
}

func TestPool_AddRemoveAccount(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Add account
	newAcc := &Account{
		ID:          "new-acc",
		Name:        "New Account",
		AccessToken: "new-token",
		Enabled:     true,
	}
	if err := pool.AddAccount(newAcc); err != nil {
		t.Fatalf("AddAccount() error = %v", err)
	}

	// Verify added
	accounts := pool.Accounts()
	if len(accounts) != 1 {
		t.Errorf("Expected 1 account, got %d", len(accounts))
	}

	// Verify persisted
	reloaded, err := LoadPoolConfig(configPath)
	if err != nil {
		t.Fatalf("LoadPoolConfig() error = %v", err)
	}
	if len(reloaded.Accounts) != 1 {
		t.Errorf("Reloaded config has %d accounts, want 1", len(reloaded.Accounts))
	}

	// Remove account
	if err := pool.RemoveAccount("new-acc"); err != nil {
		t.Fatalf("RemoveAccount() error = %v", err)
	}

	// Verify removed
	accounts = pool.Accounts()
	if len(accounts) != 0 {
		t.Errorf("Expected 0 accounts after removal, got %d", len(accounts))
	}
}

func TestLoadPoolConfig_NonExistent(t *testing.T) {
	cfg, err := LoadPoolConfig("/nonexistent/path/pool.yaml")
	if err != nil {
		t.Fatalf("LoadPoolConfig() should return default for non-existent, got error = %v", err)
	}

	if cfg == nil {
		t.Fatal("LoadPoolConfig() returned nil config")
	}

	// Should return defaults
	if cfg.Strategy != StrategyRoundRobin {
		t.Errorf("Default strategy = %v, want %v", cfg.Strategy, StrategyRoundRobin)
	}
}

func TestLoadState_NonExistent(t *testing.T) {
	state, err := LoadState("/nonexistent/path/state.yaml")
	if err != nil {
		t.Fatalf("LoadState() should return empty state for non-existent, got error = %v", err)
	}

	if state == nil {
		t.Fatal("LoadState() returned nil state")
	}

	if state.CurrentIndex != 0 {
		t.Errorf("Default CurrentIndex = %d, want 0", state.CurrentIndex)
	}
}

func TestPoolConfig_GetAccount(t *testing.T) {
	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Name: "Account 1"},
		{ID: "acc2", Name: "Account 2"},
	}

	// Found case
	acc, err := cfg.GetAccount("acc1")
	if err != nil {
		t.Fatalf("GetAccount(acc1) error = %v", err)
	}
	if acc.Name != "Account 1" {
		t.Errorf("GetAccount(acc1).Name = %v, want 'Account 1'", acc.Name)
	}

	// Not found case
	_, err = cfg.GetAccount("nonexistent")
	if err == nil {
		t.Error("GetAccount(nonexistent) should return error")
	}
}

func TestPool_Strategy(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Strategy = StrategyFailover
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if pool.Strategy() != StrategyFailover {
		t.Errorf("Strategy() = %v, want %v", pool.Strategy(), StrategyFailover)
	}
}

func TestPool_SwitchOnRateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.SwitchOnRateLimit = false
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if pool.SwitchOnRateLimit() {
		t.Error("SwitchOnRateLimit() = true, want false")
	}
}

func TestPool_Current_EmptyPool(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig() // No accounts
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if current := pool.Current(); current != nil {
		t.Errorf("Current() = %v, want nil for empty pool", current)
	}
}

func TestPool_Token_EmptyPool(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig() // No accounts
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	pool, err := New(configPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if token := pool.Token(); token != "" {
		t.Errorf("Token() = %q, want empty string for empty pool", token)
	}
}

func TestState_Save_EmptyPath(t *testing.T) {
	state := NewState("") // Empty path
	if err := state.Save(); err == nil {
		t.Error("Save() should error with empty path")
	}
}

func TestLoadPoolConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pool.yaml")
	os.WriteFile(path, []byte("invalid: yaml: content: [[["), 0600)

	_, err := LoadPoolConfig(path)
	if err == nil {
		t.Error("LoadPoolConfig should error on invalid YAML")
	}
}

func TestLoadState_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "state.yaml")
	os.WriteFile(path, []byte("invalid: yaml: content: [[["), 0600)

	_, err := LoadState(path)
	if err == nil {
		t.Error("LoadState should error on invalid YAML")
	}
}

func TestPool_WithLogger(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pool.yaml")

	cfg := DefaultPoolConfig()
	cfg.Accounts = []*Account{
		{ID: "acc1", Enabled: true},
	}
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Save config error = %v", err)
	}

	// Create pool with custom logger - should not panic
	logger := slog.Default()
	pool, err := New(configPath, WithLogger(logger))
	if err != nil {
		t.Fatalf("New() with logger error = %v", err)
	}

	// Verify pool works
	if pool.Current() == nil {
		t.Error("Pool with logger should have current account")
	}
}
