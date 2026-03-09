package gitlab

import (
	"github.com/randalmurphal/orc/internal/hosting"
)

// resolveToken gets the GitLab API token from environment.
// Uses cfg.TokenEnvVar if set, otherwise defaults to ORC_GITLAB_TOKEN.
func resolveToken(cfg hosting.Config) (string, error) {
	return hosting.ResolveTokenFromEnv(cfg, hosting.ProviderGitLab)
}
