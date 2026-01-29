package github

import (
	"fmt"
	"os"

	"github.com/randalmurphal/orc/internal/hosting"
)

// resolveToken gets the GitHub API token from environment.
// Uses cfg.TokenEnvVar if set, otherwise defaults to GITHUB_TOKEN.
func resolveToken(cfg hosting.Config) (string, error) {
	envVar := "GITHUB_TOKEN"
	if cfg.TokenEnvVar != "" {
		envVar = cfg.TokenEnvVar
	}

	token := os.Getenv(envVar)
	if token == "" {
		return "", fmt.Errorf("%s environment variable is not set (required for GitHub API access)", envVar)
	}

	return token, nil
}
