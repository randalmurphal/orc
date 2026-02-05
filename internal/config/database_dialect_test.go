package config

import (
	"os"
	"strings"
	"testing"
)

// =============================================================================
// SC-1: DatabaseConfig.Dialect field accepts "sqlite" (default) or "postgres"
// =============================================================================

func TestDatabaseConfig_DialectField_SQLite(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		Database: DatabaseConfig{
			Dialect: "sqlite",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() with dialect=sqlite should succeed, got: %v", err)
	}
}

func TestDatabaseConfig_DialectField_Postgres(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("TEST_PG_DSN_1", "postgres://user:pass@localhost/db")

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: "testuser"},
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "TEST_PG_DSN_1",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() with valid postgres config should succeed, got: %v", err)
	}
}

func TestDatabaseConfig_DialectField_Empty_DefaultsToSQLite(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		Database: DatabaseConfig{
			Dialect: "", // Empty should be treated as sqlite (default)
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() with empty dialect should succeed (defaults to sqlite), got: %v", err)
	}
}

func TestDatabaseConfig_DialectField_Invalid(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		Database: DatabaseConfig{
			Dialect: "mysql", // Invalid dialect
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() with invalid dialect should fail")
	}
	if !strings.Contains(err.Error(), "database.dialect") {
		t.Errorf("error should mention 'database.dialect', got: %v", err)
	}
}

// =============================================================================
// SC-2: DatabaseConfig.DSNEnv field stores env var name for PostgreSQL DSN
// =============================================================================

func TestDatabaseConfig_DSNEnvField_Exists(t *testing.T) {
	t.Parallel()

	cfg := DatabaseConfig{
		Dialect: "postgres",
		DSNEnv:  "MY_DB_DSN",
	}

	if cfg.DSNEnv != "MY_DB_DSN" {
		t.Errorf("DSNEnv = %q, want MY_DB_DSN", cfg.DSNEnv)
	}
}

// =============================================================================
// SC-3: Validation - dialect=postgres requires dsn_env to be set
// =============================================================================

func TestDatabaseConfig_Postgres_RequiresDSNEnv(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: "testuser"},
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "", // Missing dsn_env
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() with postgres dialect but no dsn_env should fail")
	}
	if !strings.Contains(err.Error(), "dsn_env") {
		t.Errorf("error should mention 'dsn_env', got: %v", err)
	}
}

func TestDatabaseConfig_Postgres_RequiresDSNEnv_Actionable(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: "testuser"},
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	// Error should be actionable - tell user what to do
	errStr := err.Error()
	if !strings.Contains(errStr, "postgres") {
		t.Errorf("error should mention postgres, got: %s", errStr)
	}
}

// =============================================================================
// SC-4: Validation - dialect=postgres requires user.name to be set
// =============================================================================

func TestDatabaseConfig_Postgres_RequiresUserName(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("TEST_PG_DSN_2", "postgres://user:pass@localhost/db")

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: ""}, // Missing user.name
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "TEST_PG_DSN_2",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() with postgres dialect but no user.name should fail")
	}
	if !strings.Contains(err.Error(), "user.name") {
		t.Errorf("error should mention 'user.name', got: %v", err)
	}
}

func TestDatabaseConfig_Postgres_UserNameSet_Succeeds(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("TEST_PG_DSN_3", "postgres://user:pass@localhost/db")

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: "alice"},
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "TEST_PG_DSN_3",
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() with valid postgres config should succeed, got: %v", err)
	}
}

// =============================================================================
// SC-5: Validation - if dsn_env is set, the env var must exist and be non-empty
// =============================================================================

func TestDatabaseConfig_DSNEnv_MustExist(t *testing.T) {
	// Note: The env var "NONEXISTENT_DSN_VAR_FOR_TEST" is expected not to exist
	// Using a unique name to avoid test pollution

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: "testuser"},
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "NONEXISTENT_DSN_VAR",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() with non-existent dsn_env should fail")
	}
	if !strings.Contains(err.Error(), "NONEXISTENT_DSN_VAR") {
		t.Errorf("error should mention the env var name, got: %v", err)
	}
}

func TestDatabaseConfig_DSNEnv_MustBeNonEmpty(t *testing.T) {
	// Note: Cannot use t.Parallel() with t.Setenv()
	t.Setenv("EMPTY_DSN_VAR", "")

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: "testuser"},
		Database: DatabaseConfig{
			Dialect: "postgres",
			DSNEnv:  "EMPTY_DSN_VAR",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() with empty dsn_env value should fail")
	}
	if !strings.Contains(err.Error(), "empty") || !strings.Contains(err.Error(), "EMPTY_DSN_VAR") {
		t.Errorf("error should mention empty env var, got: %v", err)
	}
}

// =============================================================================
// SC-6: Error messages are clear and actionable
// =============================================================================

func TestDatabaseConfig_ErrorMessages_AreActionable(t *testing.T) {
	tests := []struct {
		name         string
		cfg          *Config
		wantContains []string
	}{
		{
			name: "invalid dialect suggests valid options",
			cfg: &Config{
				Worktree: WorktreeConfig{Enabled: true},
				Database: DatabaseConfig{Dialect: "mysql"},
			},
			wantContains: []string{"sqlite", "postgres"},
		},
		{
			name: "missing dsn_env explains requirement",
			cfg: &Config{
				Worktree: WorktreeConfig{Enabled: true},
				User:     UserConfig{Name: "user"},
				Database: DatabaseConfig{Dialect: "postgres", DSNEnv: ""},
			},
			wantContains: []string{"dsn_env", "postgres"},
		},
		{
			name: "missing user.name explains requirement",
			cfg: &Config{
				Worktree: WorktreeConfig{Enabled: true},
				User:     UserConfig{Name: ""},
				Database: DatabaseConfig{Dialect: "postgres", DSNEnv: "SOME_VAR"},
			},
			wantContains: []string{"user.name", "postgres"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up env var if needed (cannot use t.Parallel() with t.Setenv())
			if tt.cfg.Database.DSNEnv != "" {
				t.Setenv(tt.cfg.Database.DSNEnv, "postgres://localhost/db")
			}

			err := tt.cfg.Validate()
			if err == nil {
				t.Fatal("expected error")
			}

			errStr := err.Error()
			for _, want := range tt.wantContains {
				if !strings.Contains(errStr, want) {
					t.Errorf("error should contain %q, got: %s", want, errStr)
				}
			}
		})
	}
}

// =============================================================================
// SC-7: SQLite dialect does NOT require dsn_env or user.name
// =============================================================================

func TestDatabaseConfig_SQLite_NoExtraRequirements(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Worktree: WorktreeConfig{Enabled: true},
		User:     UserConfig{Name: ""}, // No user.name
		Database: DatabaseConfig{
			Dialect: "sqlite",
			DSNEnv:  "", // No dsn_env
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() with sqlite dialect should not require user.name or dsn_env, got: %v", err)
	}
}
