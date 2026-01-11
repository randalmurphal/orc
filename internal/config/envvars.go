package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvVarMapping defines the mapping between environment variables and config paths.
var EnvVarMapping = map[string]string{
	"ORC_PROFILE":           "profile",
	"ORC_MODEL":             "model",
	"ORC_MAX_ITERATIONS":    "max_iterations",
	"ORC_TIMEOUT":           "timeout",
	"ORC_CLAUDE_PATH":       "claude_path",
	"ORC_RETRY_ENABLED":     "retry.enabled",
	"ORC_RETRY_MAX_RETRIES": "retry.max_retries",
	"ORC_GATES_DEFAULT":     "gates.default_type",
	"ORC_GATES_MAX_RETRIES": "gates.max_retries",
	"ORC_WORKTREE_ENABLED":  "worktree.enabled",
	"ORC_WORKTREE_DIR":      "worktree.dir",
	"ORC_COMPLETION_ACTION": "completion.action",
	"ORC_BRANCH_PREFIX":     "branch_prefix",
	"ORC_COMMIT_PREFIX":     "commit_prefix",
	"ORC_POOL_ENABLED":      "pool.enabled",
	"ORC_HOST":              "server.host",
	"ORC_PORT":              "server.port",
	"ORC_AUTH_ENABLED":      "server.auth.enabled",
	"ORC_AUTH_TYPE":         "server.auth.type",
	"ORC_TEAM_ENABLED":      "team.enabled",
	"ORC_TEAM_SERVER":       "team.server_url",
}

// ApplyEnvVars applies environment variable overrides to a TrackedConfig.
// Returns a list of paths that were overridden.
func ApplyEnvVars(tc *TrackedConfig) []string {
	var overridden []string

	for envVar, configPath := range EnvVarMapping {
		value := os.Getenv(envVar)
		if value == "" {
			continue
		}

		if applyEnvVar(tc.Config, configPath, value) {
			tc.SetSource(configPath, SourceEnv)
			overridden = append(overridden, configPath)
		}
	}

	return overridden
}

// applyEnvVar applies a single environment variable to the config.
// Returns true if the value was applied.
func applyEnvVar(cfg *Config, path string, value string) bool {
	switch path {
	case "profile":
		cfg.Profile = AutomationProfile(value)
	case "model":
		cfg.Model = value
	case "max_iterations":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.MaxIterations = v
		}
	case "timeout":
		if d, err := time.ParseDuration(value); err == nil {
			cfg.Timeout = d
		}
	case "claude_path":
		cfg.ClaudePath = value
	case "retry.enabled":
		cfg.Retry.Enabled = parseBool(value)
	case "retry.max_retries":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Retry.MaxRetries = v
		}
	case "gates.default_type":
		cfg.Gates.DefaultType = value
	case "gates.max_retries":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Gates.MaxRetries = v
		}
	case "worktree.enabled":
		cfg.Worktree.Enabled = parseBool(value)
	case "worktree.dir":
		cfg.Worktree.Dir = value
	case "completion.action":
		cfg.Completion.Action = value
	case "branch_prefix":
		cfg.BranchPrefix = value
	case "commit_prefix":
		cfg.CommitPrefix = value
	case "pool.enabled":
		cfg.Pool.Enabled = parseBool(value)
	case "server.host":
		cfg.Server.Host = value
	case "server.port":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Server.Port = v
		}
	case "server.auth.enabled":
		cfg.Server.Auth.Enabled = parseBool(value)
	case "server.auth.type":
		cfg.Server.Auth.Type = value
	case "team.enabled":
		cfg.Team.Enabled = parseBool(value)
	case "team.server_url":
		cfg.Team.ServerURL = value
	default:
		return false
	}
	return true
}

// parseBool parses a boolean string (case-insensitive).
func parseBool(s string) bool {
	s = strings.ToLower(s)
	return s == "true" || s == "1" || s == "yes" || s == "on"
}
