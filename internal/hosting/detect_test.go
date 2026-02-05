package hosting

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectProvider(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected ProviderType
	}{
		// GitHub
		{"github ssh", "git@github.com:owner/repo.git", ProviderGitHub},
		{"github https", "https://github.com/owner/repo.git", ProviderGitHub},
		{"github enterprise ssh", "git@github.company.com:org/repo.git", ProviderGitHub},
		{"github enterprise https", "https://github.acme.com/org/repo.git", ProviderGitHub},
		// GitLab
		{"gitlab ssh", "git@gitlab.com:owner/repo.git", ProviderGitLab},
		{"gitlab https", "https://gitlab.com/owner/repo.git", ProviderGitLab},
		{"gitlab self-hosted ssh", "git@gitlab.company.com:org/repo.git", ProviderGitLab},
		{"gitlab self-hosted https", "https://gitlab.acme.com/group/subgroup/repo.git", ProviderGitLab},
		// Unknown
		{"bitbucket", "git@bitbucket.org:owner/repo.git", ProviderUnknown},
		{"random", "git@myserver.com:owner/repo.git", ProviderUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectProvider(tt.url)
			if got != tt.expected {
				t.Errorf("DetectProvider(%q) = %q, want %q", tt.url, got, tt.expected)
			}
		})
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
	}{
		{"github ssh", "git@github.com:owner/repo.git", "owner", "repo"},
		{"github https", "https://github.com/owner/repo.git", "owner", "repo"},
		{"ssh with port", "ssh://git@github.com:22/owner/repo.git", "owner", "repo"},
		{"gitlab subgroup", "git@gitlab.com:group/subgroup/repo.git", "group/subgroup", "repo"},
		{"no .git suffix", "https://github.com/owner/repo", "owner", "repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := ParseOwnerRepo(tt.url)
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("ParseOwnerRepo(%q) = (%q, %q), want (%q, %q)",
					tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

// TestGetTokenEnvVar tests SC-3: Token existence check via env var name.
// Tests the new GetTokenEnvVar function that returns the expected token env var
// based on provider type and config.
func TestGetTokenEnvVar(t *testing.T) {
	tests := []struct {
		name       string
		provider   ProviderType
		cfg        Config
		wantEnvVar string
	}{
		{
			name:       "github default",
			provider:   ProviderGitHub,
			cfg:        Config{},
			wantEnvVar: "GITHUB_TOKEN",
		},
		{
			name:       "gitlab default",
			provider:   ProviderGitLab,
			cfg:        Config{},
			wantEnvVar: "GITLAB_TOKEN",
		},
		{
			name:     "github custom token env var",
			provider: ProviderGitHub,
			cfg: Config{
				TokenEnvVar: "MY_GITHUB_TOKEN",
			},
			wantEnvVar: "MY_GITHUB_TOKEN",
		},
		{
			name:     "gitlab custom token env var",
			provider: ProviderGitLab,
			cfg: Config{
				TokenEnvVar: "MY_GITLAB_TOKEN",
			},
			wantEnvVar: "MY_GITLAB_TOKEN",
		},
		{
			name:       "unknown provider returns empty",
			provider:   ProviderUnknown,
			cfg:        Config{},
			wantEnvVar: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTokenEnvVar(tt.provider, tt.cfg)
			if got != tt.wantEnvVar {
				t.Errorf("GetTokenEnvVar(%q, cfg) = %q, want %q", tt.provider, got, tt.wantEnvVar)
			}
		})
	}
}

// TestValidateToken_Success tests SC-4: Token validation when token is valid.
func TestValidateToken_Success(t *testing.T) {
	// Create a test server that returns a valid user response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header is present
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// GitHub-style user response
		_, _ = w.Write([]byte(`{"login": "testuser", "id": 12345}`))
	}))
	defer server.Close()

	result, err := ValidateToken(context.Background(), ProviderGitHub, "test-token", server.URL)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v, want nil", err)
	}
	if result.Username == "" {
		t.Error("ValidateToken() username should not be empty on success")
	}
	if !result.Valid {
		t.Error("ValidateToken() Valid should be true on success")
	}
}

// TestValidateToken_InvalidToken tests SC-4: Token validation when token is invalid.
func TestValidateToken_InvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message": "Bad credentials"}`))
	}))
	defer server.Close()

	result, err := ValidateToken(context.Background(), ProviderGitHub, "invalid-token", server.URL)
	if err == nil {
		t.Fatal("ValidateToken() should return error for invalid token")
	}
	if result.Valid {
		t.Error("ValidateToken() Valid should be false for invalid token")
	}
}

// TestValidateToken_NetworkError tests failure mode: Network error during validation.
func TestValidateToken_NetworkError(t *testing.T) {
	// Use a URL that will fail to connect
	result, err := ValidateToken(context.Background(), ProviderGitHub, "test-token", "http://localhost:1")
	if err == nil {
		t.Fatal("ValidateToken() should return error for network failure")
	}
	if result.Valid {
		t.Error("ValidateToken() Valid should be false on network error")
	}
}

// TestValidateToken_GitLabProvider tests SC-4: Token validation for GitLab.
func TestValidateToken_GitLabProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Private-Token") == "" && r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// GitLab-style user response
		_, _ = w.Write([]byte(`{"username": "gitlabuser", "id": 67890}`))
	}))
	defer server.Close()

	result, err := ValidateToken(context.Background(), ProviderGitLab, "test-token", server.URL)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v, want nil", err)
	}
	if result.Username == "" {
		t.Error("ValidateToken() username should not be empty on success")
	}
}

// TestValidateToken_UnknownProvider tests failure mode: Unknown provider returns error.
func TestValidateToken_UnknownProvider(t *testing.T) {
	_, err := ValidateToken(context.Background(), ProviderUnknown, "test-token", "")
	if err == nil {
		t.Fatal("ValidateToken() should return error for unknown provider")
	}
}

// TestExtractBaseURL tests SC-5: Self-hosted URL base extraction.
func TestExtractBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		remoteURL   string
		provider    ProviderType
		wantBaseURL string
		wantIsSelfHosted bool
	}{
		{
			name:        "github.com returns empty (not self-hosted)",
			remoteURL:   "git@github.com:owner/repo.git",
			provider:    ProviderGitHub,
			wantBaseURL: "",
			wantIsSelfHosted: false,
		},
		{
			name:        "gitlab.com returns empty (not self-hosted)",
			remoteURL:   "git@gitlab.com:owner/repo.git",
			provider:    ProviderGitLab,
			wantBaseURL: "",
			wantIsSelfHosted: false,
		},
		{
			name:        "github enterprise SSH returns base URL",
			remoteURL:   "git@github.company.com:org/repo.git",
			provider:    ProviderGitHub,
			wantBaseURL: "https://github.company.com",
			wantIsSelfHosted: true,
		},
		{
			name:        "gitlab self-hosted SSH returns base URL",
			remoteURL:   "git@gitlab.company.com:org/repo.git",
			provider:    ProviderGitLab,
			wantBaseURL: "https://gitlab.company.com",
			wantIsSelfHosted: true,
		},
		{
			name:        "github enterprise HTTPS returns base URL",
			remoteURL:   "https://github.acme.com/org/repo.git",
			provider:    ProviderGitHub,
			wantBaseURL: "https://github.acme.com",
			wantIsSelfHosted: true,
		},
		{
			name:        "gitlab self-hosted HTTPS returns base URL",
			remoteURL:   "https://gitlab.acme.com/group/repo.git",
			provider:    ProviderGitLab,
			wantBaseURL: "https://gitlab.acme.com",
			wantIsSelfHosted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, isSelfHosted := ExtractBaseURL(tt.remoteURL, tt.provider)
			if baseURL != tt.wantBaseURL {
				t.Errorf("ExtractBaseURL() baseURL = %q, want %q", baseURL, tt.wantBaseURL)
			}
			if isSelfHosted != tt.wantIsSelfHosted {
				t.Errorf("ExtractBaseURL() isSelfHosted = %v, want %v", isSelfHosted, tt.wantIsSelfHosted)
			}
		})
	}
}

// TestIsSelfHosted tests edge cases for detecting self-hosted instances.
func TestIsSelfHosted(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		provider ProviderType
		want     bool
	}{
		{"github.com not self-hosted", "https://github.com/o/r", ProviderGitHub, false},
		{"gitlab.com not self-hosted", "https://gitlab.com/o/r", ProviderGitLab, false},
		{"github enterprise is self-hosted", "https://github.acme.com/o/r", ProviderGitHub, true},
		{"gitlab self-hosted is self-hosted", "https://gitlab.acme.com/o/r", ProviderGitLab, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSelfHosted(tt.url, tt.provider)
			if got != tt.want {
				t.Errorf("IsSelfHosted(%q, %q) = %v, want %v", tt.url, tt.provider, got, tt.want)
			}
		})
	}
}

// TestTokenValidationResult tests the structure of validation results.
func TestTokenValidationResult(t *testing.T) {
	// Ensure TokenValidationResult has expected fields
	result := TokenValidationResult{
		Valid:    true,
		Username: "testuser",
		Error:    nil,
	}

	if !result.Valid {
		t.Error("result.Valid should be true")
	}
	if result.Username != "testuser" {
		t.Errorf("result.Username = %q, want %q", result.Username, "testuser")
	}

	// Test with error
	result2 := TokenValidationResult{
		Valid:    false,
		Username: "",
		Error:    errors.New("auth failed"),
	}

	if result2.Valid {
		t.Error("result2.Valid should be false when there's an error")
	}
	if result2.Error == nil {
		t.Error("result2.Error should not be nil")
	}
}
