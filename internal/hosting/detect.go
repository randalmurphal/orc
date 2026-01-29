package hosting

import (
	"regexp"
	"strings"
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
