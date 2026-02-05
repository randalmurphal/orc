package hosting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// DetectProvider determines the hosting provider from a git remote URL.
//
// Supported URL formats:
//   - git@github.com:owner/repo.git
//   - https://github.com/owner/repo.git
//   - git@gitlab.com:owner/repo.git
//   - https://gitlab.com/owner/repo.git
//   - git@gitlab.company.com:org/repo.git (self-hosted GitLab)
//   - https://github.company.com/org/repo.git (GitHub Enterprise)
func DetectProvider(remoteURL string) ProviderType {
	url := strings.ToLower(strings.TrimSpace(remoteURL))

	// Check for GitHub patterns
	if isGitHub(url) {
		return ProviderGitHub
	}

	// Check for GitLab patterns
	if isGitLab(url) {
		return ProviderGitLab
	}

	return ProviderUnknown
}

// GitHub URL patterns
var githubPatterns = []*regexp.Regexp{
	regexp.MustCompile(`github\.com[:/]`),
	regexp.MustCompile(`github\.[a-z0-9-]+\.[a-z]+[:/]`), // GitHub Enterprise (github.company.com)
}

func isGitHub(url string) bool {
	for _, p := range githubPatterns {
		if p.MatchString(url) {
			return true
		}
	}
	return false
}

// GitLab URL patterns
var gitlabPatterns = []*regexp.Regexp{
	regexp.MustCompile(`gitlab\.com[:/]`),
	regexp.MustCompile(`gitlab\.[a-z0-9-]+\.[a-z]+[:/]`), // Self-hosted GitLab (gitlab.company.com)
}

func isGitLab(url string) bool {
	for _, p := range gitlabPatterns {
		if p.MatchString(url) {
			return true
		}
	}
	return false
}

// ParseOwnerRepo extracts owner and repo from a git remote URL.
//
// Handles:
//   - git@github.com:owner/repo.git → (owner, repo)
//   - https://github.com/owner/repo.git → (owner, repo)
//   - ssh://git@github.com:22/owner/repo.git → (owner, repo)
//   - git@gitlab.com:group/subgroup/repo.git → (group/subgroup, repo)
func ParseOwnerRepo(remoteURL string) (owner, repo string) {
	raw := strings.TrimSpace(remoteURL)
	raw = strings.TrimSuffix(raw, ".git")

	// SSH format: ssh://git@host:port/owner/repo
	if strings.HasPrefix(raw, "ssh://") {
		raw = strings.TrimPrefix(raw, "ssh://")
		if idx := strings.Index(raw, "/"); idx != -1 {
			raw = raw[idx+1:]
			raw = strings.TrimLeft(raw, "/")
		}
	} else if strings.HasPrefix(raw, "https://") || strings.HasPrefix(raw, "http://") {
		// HTTPS format: https://host/owner/repo
		raw = strings.TrimPrefix(raw, "https://")
		raw = strings.TrimPrefix(raw, "http://")
		// Remove host part (first segment)
		if idx := strings.Index(raw, "/"); idx != -1 {
			raw = raw[idx+1:]
		}
	} else if idx := strings.Index(raw, ":"); idx != -1 {
		// SCP-style SSH: git@host:owner/repo
		raw = raw[idx+1:]
	}

	// Split remaining path into owner and repo
	// For GitLab, owner can be "group/subgroup" so take last segment as repo
	parts := strings.Split(raw, "/")
	if len(parts) < 2 {
		return raw, ""
	}

	repo = parts[len(parts)-1]
	owner = strings.Join(parts[:len(parts)-1], "/")
	return owner, repo
}

// GetTokenEnvVar returns the expected environment variable name for the hosting token.
// Uses custom TokenEnvVar from config if set, otherwise returns the default for the provider.
func GetTokenEnvVar(provider ProviderType, cfg Config) string {
	// Use custom token env var if configured
	if cfg.TokenEnvVar != "" {
		return cfg.TokenEnvVar
	}

	// Return default based on provider
	switch provider {
	case ProviderGitHub:
		return "GITHUB_TOKEN"
	case ProviderGitLab:
		return "GITLAB_TOKEN"
	default:
		return ""
	}
}

// TokenValidationResult contains the result of validating a hosting token.
type TokenValidationResult struct {
	Valid    bool
	Username string
	Error    error
}

// ValidateToken validates a hosting provider token by making an API call.
// For GitHub, it calls /user. For GitLab, it calls /api/v4/user.
// The baseURL parameter allows testing against mock servers or self-hosted instances.
func ValidateToken(ctx context.Context, provider ProviderType, token string, baseURL string) (TokenValidationResult, error) {
	if provider == ProviderUnknown {
		return TokenValidationResult{Valid: false}, fmt.Errorf("unknown provider")
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var url string
	var req *http.Request
	var err error

	switch provider {
	case ProviderGitHub:
		if baseURL != "" {
			url = baseURL + "/user"
		} else {
			url = "https://api.github.com/user"
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return TokenValidationResult{Valid: false}, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

	case ProviderGitLab:
		if baseURL != "" {
			url = baseURL + "/api/v4/user"
		} else {
			url = "https://gitlab.com/api/v4/user"
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return TokenValidationResult{Valid: false}, err
		}
		req.Header.Set("Private-Token", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return TokenValidationResult{Valid: false, Error: err}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return TokenValidationResult{Valid: false, Error: fmt.Errorf("status %d", resp.StatusCode)},
			fmt.Errorf("token validation failed: %s", resp.Status)
	}

	// Parse response to get username
	var userResp struct {
		Login    string `json:"login"`    // GitHub
		Username string `json:"username"` // GitLab
	}
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return TokenValidationResult{Valid: false, Error: err}, err
	}

	username := userResp.Login
	if username == "" {
		username = userResp.Username
	}

	return TokenValidationResult{
		Valid:    true,
		Username: username,
	}, nil
}

// ExtractBaseURL extracts the base URL from a git remote URL.
// Returns the base URL and whether the instance is self-hosted.
// For github.com/gitlab.com, returns empty string and false.
// For self-hosted instances, returns the base URL (e.g., "https://gitlab.company.com") and true.
func ExtractBaseURL(remoteURL string, provider ProviderType) (baseURL string, isSelfHosted bool) {
	if !IsSelfHosted(remoteURL, provider) {
		return "", false
	}

	// Extract host from URL
	url := strings.TrimSpace(remoteURL)

	var host string

	// Handle SSH format: git@host:path
	if strings.HasPrefix(url, "git@") {
		// git@gitlab.company.com:org/repo.git
		url = strings.TrimPrefix(url, "git@")
		if idx := strings.Index(url, ":"); idx != -1 {
			host = url[:idx]
		}
	} else if strings.HasPrefix(url, "ssh://") {
		// ssh://git@host:port/path
		url = strings.TrimPrefix(url, "ssh://")
		// TrimPrefix is a no-op if prefix doesn't exist
		url = strings.TrimPrefix(url, "git@")
		if idx := strings.Index(url, ":"); idx != -1 {
			host = url[:idx]
		} else if idx := strings.Index(url, "/"); idx != -1 {
			host = url[:idx]
		}
	} else if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// https://gitlab.company.com/org/repo.git
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")
		if idx := strings.Index(url, "/"); idx != -1 {
			host = url[:idx]
		}
	}

	if host == "" {
		return "", false
	}

	return "https://" + host, true
}

// IsSelfHosted returns true if the remote URL points to a self-hosted instance.
// github.com and gitlab.com are not self-hosted.
func IsSelfHosted(url string, provider ProviderType) bool {
	lowerURL := strings.ToLower(url)

	switch provider {
	case ProviderGitHub:
		// Check for public github.com host
		// Match github.com exactly (followed by :, /, or end of string)
		if isPublicHost(lowerURL, "github.com") {
			return false
		}
		// Any other GitHub-detected URL is self-hosted (GitHub Enterprise)
		return true

	case ProviderGitLab:
		// Check for public gitlab.com host
		if isPublicHost(lowerURL, "gitlab.com") {
			return false
		}
		// Any other GitLab-detected URL is self-hosted
		return true

	default:
		return false
	}
}

// isPublicHost checks if the URL contains the exact public host (not a subdomain).
// For example, "github.com" should match in "git@github.com:owner/repo.git"
// but NOT in "git@github.company.com:org/repo.git".
func isPublicHost(url, host string) bool {
	// For SSH format: git@host:path
	if strings.Contains(url, "@"+host+":") || strings.Contains(url, "@"+host+"/") {
		return true
	}
	// For HTTPS format: https://host/path
	if strings.Contains(url, "://"+host+"/") || strings.Contains(url, "://"+host+":") {
		return true
	}
	// For SSH with explicit port: ssh://git@host:port/path
	if strings.HasSuffix(url, "@"+host) {
		return true
	}
	return false
}
