package hosting

import (
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
