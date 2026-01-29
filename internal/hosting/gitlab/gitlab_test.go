package gitlab

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/hosting"
)

func TestResolveToken(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv modifies process environment.

	tests := []struct {
		name      string
		cfg       hosting.Config
		envVars   map[string]string
		wantToken string
		wantErr   bool
	}{
		{
			name: "GITLAB_TOKEN set",
			cfg:  hosting.Config{},
			envVars: map[string]string{
				"GITLAB_TOKEN": "glpat-test123",
			},
			wantToken: "glpat-test123",
		},
		{
			name: "GITLAB_PRIVATE_TOKEN fallback",
			cfg:  hosting.Config{},
			envVars: map[string]string{
				"GITLAB_PRIVATE_TOKEN": "glpat-private456",
			},
			wantToken: "glpat-private456",
		},
		{
			name: "GITLAB_TOKEN takes priority over GITLAB_PRIVATE_TOKEN",
			cfg:  hosting.Config{},
			envVars: map[string]string{
				"GITLAB_TOKEN":         "primary",
				"GITLAB_PRIVATE_TOKEN": "fallback",
			},
			wantToken: "primary",
		},
		{
			name:    "no token set returns error",
			cfg:     hosting.Config{},
			wantErr: true,
		},
		{
			name: "custom env var overrides default",
			cfg:  hosting.Config{TokenEnvVar: "MY_GL_TOKEN"},
			envVars: map[string]string{
				"MY_GL_TOKEN": "custom_value",
			},
			wantToken: "custom_value",
		},
		{
			name:    "custom env var not set returns error",
			cfg:     hosting.Config{TokenEnvVar: "MY_GL_TOKEN"},
			wantErr: true,
		},
		{
			name: "custom env var ignores GITLAB_TOKEN",
			cfg:  hosting.Config{TokenEnvVar: "MY_GL_TOKEN"},
			envVars: map[string]string{
				"GITLAB_TOKEN": "should_not_use",
				"MY_GL_TOKEN":  "custom_wins",
			},
			wantToken: "custom_wins",
		},
		{
			name: "custom env var not set ignores GITLAB_TOKEN",
			cfg:  hosting.Config{TokenEnvVar: "MY_GL_TOKEN"},
			envVars: map[string]string{
				"GITLAB_TOKEN": "should_not_use",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all potential env vars.
			t.Setenv("GITLAB_TOKEN", "")
			t.Setenv("GITLAB_PRIVATE_TOKEN", "")
			t.Setenv("MY_GL_TOKEN", "")

			for k, v := range tt.envVars {
				t.Setenv(k, v)
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

func TestResolveToken_ErrorMessages(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv modifies process environment.

	t.Run("default error mentions both env vars", func(t *testing.T) {
		t.Setenv("GITLAB_TOKEN", "")
		t.Setenv("GITLAB_PRIVATE_TOKEN", "")

		_, err := resolveToken(hosting.Config{})
		if err == nil {
			t.Fatal("expected error")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "GITLAB_TOKEN") {
			t.Errorf("error should mention GITLAB_TOKEN, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "GITLAB_PRIVATE_TOKEN") {
			t.Errorf("error should mention GITLAB_PRIVATE_TOKEN, got: %s", errMsg)
		}
	})

	t.Run("custom env var error mentions the custom var", func(t *testing.T) {
		t.Setenv("CUSTOM_GL", "")

		_, err := resolveToken(hosting.Config{TokenEnvVar: "CUSTOM_GL"})
		if err == nil {
			t.Fatal("expected error")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "CUSTOM_GL") {
			t.Errorf("error should mention CUSTOM_GL, got: %s", errMsg)
		}
	})
}

func TestMapJobStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		gitlabStatus   string
		wantStatus     string
		wantConclusion string
	}{
		{
			name:           "success",
			gitlabStatus:   "success",
			wantStatus:     "completed",
			wantConclusion: "success",
		},
		{
			name:           "failed",
			gitlabStatus:   "failed",
			wantStatus:     "completed",
			wantConclusion: "failure",
		},
		{
			name:           "canceled",
			gitlabStatus:   "canceled",
			wantStatus:     "completed",
			wantConclusion: "cancelled",
		},
		{
			name:           "skipped",
			gitlabStatus:   "skipped",
			wantStatus:     "completed",
			wantConclusion: "skipped",
		},
		{
			name:           "running",
			gitlabStatus:   "running",
			wantStatus:     "in_progress",
			wantConclusion: "running",
		},
		{
			name:           "pending",
			gitlabStatus:   "pending",
			wantStatus:     "queued",
			wantConclusion: "",
		},
		{
			name:           "created",
			gitlabStatus:   "created",
			wantStatus:     "queued",
			wantConclusion: "",
		},
		{
			name:           "manual",
			gitlabStatus:   "manual",
			wantStatus:     "queued",
			wantConclusion: "",
		},
		{
			name:           "unknown status defaults to queued",
			gitlabStatus:   "some_future_status",
			wantStatus:     "queued",
			wantConclusion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, conclusion := mapJobStatus(tt.gitlabStatus)
			if status != tt.wantStatus {
				t.Errorf("mapJobStatus(%q) status = %q, want %q", tt.gitlabStatus, status, tt.wantStatus)
			}
			if conclusion != tt.wantConclusion {
				t.Errorf("mapJobStatus(%q) conclusion = %q, want %q", tt.gitlabStatus, conclusion, tt.wantConclusion)
			}
		})
	}
}

func TestGitLabProviderName(t *testing.T) {
	t.Parallel()

	p := &GitLabProvider{owner: "test", repo: "repo"}
	if got := p.Name(); got != hosting.ProviderGitLab {
		t.Errorf("Name() = %q, want %q", got, hosting.ProviderGitLab)
	}
}

func TestGitLabProviderOwnerRepo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		owner     string
		repo      string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "simple owner/repo",
			owner:     "myorg",
			repo:      "myrepo",
			wantOwner: "myorg",
			wantRepo:  "myrepo",
		},
		{
			name:      "nested group owner",
			owner:     "group/subgroup",
			repo:      "myrepo",
			wantOwner: "group/subgroup",
			wantRepo:  "myrepo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &GitLabProvider{owner: tt.owner, repo: tt.repo}
			owner, repo := p.OwnerRepo()
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("OwnerRepo() = (%q, %q), want (%q, %q)", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}
