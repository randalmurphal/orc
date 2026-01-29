package gitlab

import (
	"fmt"
	"os"

	"github.com/randalmurphal/orc/internal/hosting"
)

// resolveToken gets the GitLab API token from environment.
// Uses cfg.TokenEnvVar if set, otherwise tries GITLAB_TOKEN then GITLAB_PRIVATE_TOKEN.
func resolveToken(cfg hosting.Config) (string, error) {
	if cfg.TokenEnvVar != "" {
		token := os.Getenv(cfg.TokenEnvVar)
		if token == "" {
			return "", fmt.Errorf("%s environment variable is not set", cfg.TokenEnvVar)
		}
		return token, nil
	}

	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		return token, nil
	}
	if token := os.Getenv("GITLAB_PRIVATE_TOKEN"); token != "" {
		return token, nil
	}

	return "", fmt.Errorf("GITLAB_TOKEN or GITLAB_PRIVATE_TOKEN environment variable is not set (required for GitLab API access)")
}
