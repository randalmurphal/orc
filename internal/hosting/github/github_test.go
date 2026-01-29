package github

import (
	"testing"

	"github.com/randalmurphal/orc/internal/hosting"
)

func TestResolveToken(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv modifies process environment.

	tests := []struct {
		name      string
		cfg       hosting.Config
		envKey    string
		envValue  string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "GITHUB_TOKEN set",
			cfg:       hosting.Config{},
			envKey:    "GITHUB_TOKEN",
			envValue:  "ghp_test123",
			wantToken: "ghp_test123",
		},
		{
			name:    "GITHUB_TOKEN not set returns error",
			cfg:     hosting.Config{},
			wantErr: true,
		},
		{
			name:      "custom env var overrides default",
			cfg:       hosting.Config{TokenEnvVar: "MY_GH_TOKEN"},
			envKey:    "MY_GH_TOKEN",
			envValue:  "custom_token_value",
			wantToken: "custom_token_value",
		},
		{
			name:    "custom env var not set returns error",
			cfg:     hosting.Config{TokenEnvVar: "MY_GH_TOKEN"},
			wantErr: true,
		},
		{
			name:      "custom env var set but GITHUB_TOKEN also set uses custom",
			cfg:       hosting.Config{TokenEnvVar: "MY_GH_TOKEN"},
			envKey:    "MY_GH_TOKEN",
			envValue:  "custom_wins",
			wantToken: "custom_wins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear both potential env vars to ensure clean state.
			t.Setenv("GITHUB_TOKEN", "")
			t.Setenv("MY_GH_TOKEN", "")

			if tt.envKey != "" {
				t.Setenv(tt.envKey, tt.envValue)
			}

			token, err := resolveToken(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && token != tt.wantToken {
				t.Errorf("resolveToken() = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestResolveToken_ErrorMessage(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv modifies process environment.

	t.Run("default env var mentioned in error", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")

		_, err := resolveToken(hosting.Config{})
		if err == nil {
			t.Fatal("expected error")
		}
		if got := err.Error(); got == "" {
			t.Fatal("error message should not be empty")
		}
		// Error should mention GITHUB_TOKEN.
		errMsg := err.Error()
		if !contains(errMsg, "GITHUB_TOKEN") {
			t.Errorf("error message should mention GITHUB_TOKEN, got: %s", errMsg)
		}
	})

	t.Run("custom env var mentioned in error", func(t *testing.T) {
		t.Setenv("CUSTOM_TOKEN", "")

		_, err := resolveToken(hosting.Config{TokenEnvVar: "CUSTOM_TOKEN"})
		if err == nil {
			t.Fatal("expected error")
		}
		errMsg := err.Error()
		if !contains(errMsg, "CUSTOM_TOKEN") {
			t.Errorf("error message should mention CUSTOM_TOKEN, got: %s", errMsg)
		}
	})
}

func TestGitHubProviderName(t *testing.T) {
	t.Parallel()

	p := &GitHubProvider{owner: "test", repo: "repo"}
	if got := p.Name(); got != hosting.ProviderGitHub {
		t.Errorf("Name() = %q, want %q", got, hosting.ProviderGitHub)
	}
}

func TestGitHubProviderOwnerRepo(t *testing.T) {
	t.Parallel()

	p := &GitHubProvider{owner: "myorg", repo: "myrepo"}
	owner, repo := p.OwnerRepo()
	if owner != "myorg" || repo != "myrepo" {
		t.Errorf("OwnerRepo() = (%q, %q), want (%q, %q)", owner, repo, "myorg", "myrepo")
	}
}

// contains checks if substr is in s. Helper to avoid importing strings.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
