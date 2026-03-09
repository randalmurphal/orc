package github

import (
	"github.com/randalmurphal/orc/internal/hosting"
)

// resolveToken gets the GitHub API token from environment.
// Uses cfg.TokenEnvVar if set, otherwise defaults to ORC_GITHUB_TOKEN.
func resolveToken(cfg hosting.Config) (string, error) {
	return hosting.ResolveTokenFromEnv(cfg, hosting.ProviderGitHub)
}
