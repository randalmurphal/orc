package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// EnvVarMapping defines the mapping between environment variables and config paths.
var EnvVarMapping = map[string]string{
	"ORC_PROFILE":              "profile",
	"ORC_MODEL":                "model",
	"ORC_MAX_ITERATIONS":       "max_iterations",
	"ORC_TIMEOUT":              "timeout",
	"ORC_CLAUDE_PATH":          "claude_path",
	"ORC_RETRY_ENABLED":        "retry.enabled",
	"ORC_RETRY_MAX_RETRIES":    "retry.max_retries",
	"ORC_EXECUTOR_MAX_RETRIES": "executor.max_retries",
	"ORC_GATES_DEFAULT":        "gates.default_type",
	"ORC_GATES_MAX_RETRIES":    "gates.max_retries",
	"ORC_WORKTREE_ENABLED":   "worktree.enabled",
	"ORC_WORKTREE_DIR":       "worktree.dir",
	"ORC_COMPLETION_ACTION":  "completion.action",
	"ORC_BRANCH_PREFIX":      "branch_prefix",
	"ORC_COMMIT_PREFIX":      "commit_prefix",
	"ORC_POOL_ENABLED":       "pool.enabled",
	"ORC_HOST":               "server.host",
	"ORC_PORT":               "server.port",
	"ORC_AUTH_ENABLED":       "server.auth.enabled",
	"ORC_AUTH_TYPE":          "server.auth.type",
	"ORC_TEAM_NAME":          "team.name",
	"ORC_TEAM_ACTIVITY_LOG":  "team.activity_logging",
	"ORC_TEAM_TASK_CLAIMING": "team.task_claiming",
	"ORC_TEAM_VISIBILITY":    "team.visibility",
	"ORC_TEAM_MODE":          "team.mode",
	"ORC_TEAM_SERVER":        "team.server_url",
	// Timeouts
	"ORC_PHASE_MAX_TIMEOUT": "timeouts.phase_max",
	"ORC_IDLE_WARNING":      "timeouts.idle_warning",
	// QA settings
	"ORC_QA_ENABLED":       "qa.enabled",
	"ORC_QA_REQUIRE_E2E":   "qa.require_e2e",
	"ORC_QA_GENERATE_DOCS": "qa.generate_docs",
	// Review settings
	"ORC_REVIEW_ENABLED":      "review.enabled",
	"ORC_REVIEW_ROUNDS":       "review.rounds",
	"ORC_REVIEW_REQUIRE_PASS": "review.require_pass",
	// Subtasks settings
	"ORC_SUBTASKS_ALLOW":        "subtasks.allow_creation",
	"ORC_SUBTASKS_AUTO_APPROVE": "subtasks.auto_approve",
	"ORC_SUBTASKS_MAX_PENDING":  "subtasks.max_pending",
	// Database settings
	"ORC_DB_DRIVER":   "database.driver",
	"ORC_DB_PASSWORD": "database.postgres.password",
	"ORC_DB_HOST":     "database.postgres.host",
	"ORC_DB_PORT":     "database.postgres.port",
	"ORC_DB_NAME":     "database.postgres.database",
	"ORC_DB_USER":     "database.postgres.user",
	"ORC_DB_SSL_MODE": "database.postgres.ssl_mode",
	// Storage settings
	"ORC_STORAGE_MODE":               "storage.mode",
	"ORC_STORAGE_FILES_CLEANUP":      "storage.files.cleanup_on_complete",
	"ORC_STORAGE_DB_CACHE":           "storage.database.cache_transcripts",
	"ORC_STORAGE_DB_RETENTION_DAYS":  "storage.database.retention_days",
	"ORC_STORAGE_EXPORT_ENABLED":     "storage.export.enabled",
	"ORC_STORAGE_EXPORT_PRESET":      "storage.export.preset",
	"ORC_STORAGE_EXPORT_TASK":        "storage.export.task_definition",
	"ORC_STORAGE_EXPORT_STATE":       "storage.export.final_state",
	"ORC_STORAGE_EXPORT_TRANSCRIPTS": "storage.export.transcripts",
	"ORC_STORAGE_EXPORT_CONTEXT":     "storage.export.context_summary",
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
	case "executor.max_retries":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Execution.MaxRetries = v
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
	case "team.name":
		cfg.Team.Name = value
	case "team.activity_logging":
		cfg.Team.ActivityLogging = parseBool(value)
	case "team.task_claiming":
		cfg.Team.TaskClaiming = parseBool(value)
	case "team.visibility":
		cfg.Team.Visibility = value
	case "team.mode":
		cfg.Team.Mode = value
	case "team.server_url":
		cfg.Team.ServerURL = value
	// Timeouts
	case "timeouts.phase_max":
		if d, err := time.ParseDuration(value); err == nil {
			cfg.Timeouts.PhaseMax = d
		}
	case "timeouts.idle_warning":
		if d, err := time.ParseDuration(value); err == nil {
			cfg.Timeouts.IdleWarning = d
		}
	// QA settings
	case "qa.enabled":
		cfg.QA.Enabled = parseBool(value)
	case "qa.require_e2e":
		cfg.QA.RequireE2E = parseBool(value)
	case "qa.generate_docs":
		cfg.QA.GenerateDocs = parseBool(value)
	// Review settings
	case "review.enabled":
		cfg.Review.Enabled = parseBool(value)
	case "review.rounds":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Review.Rounds = v
		}
	case "review.require_pass":
		cfg.Review.RequirePass = parseBool(value)
	// Subtasks settings
	case "subtasks.allow_creation":
		cfg.Subtasks.AllowCreation = parseBool(value)
	case "subtasks.auto_approve":
		cfg.Subtasks.AutoApprove = parseBool(value)
	case "subtasks.max_pending":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Subtasks.MaxPending = v
		}
	// Database settings
	case "database.driver":
		cfg.Database.Driver = value
	case "database.postgres.password":
		cfg.Database.Postgres.Password = value
	case "database.postgres.host":
		cfg.Database.Postgres.Host = value
	case "database.postgres.port":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Database.Postgres.Port = v
		}
	case "database.postgres.database":
		cfg.Database.Postgres.Database = value
	case "database.postgres.user":
		cfg.Database.Postgres.User = value
	case "database.postgres.ssl_mode":
		cfg.Database.Postgres.SSLMode = value
	// Storage settings
	case "storage.mode":
		cfg.Storage.Mode = StorageMode(value)
	case "storage.files.cleanup_on_complete":
		cfg.Storage.Files.CleanupOnComplete = parseBool(value)
	case "storage.database.cache_transcripts":
		cfg.Storage.Database.CacheTranscripts = parseBool(value)
	case "storage.database.retention_days":
		if v, err := strconv.Atoi(value); err == nil {
			cfg.Storage.Database.RetentionDays = v
		}
	case "storage.export.enabled":
		cfg.Storage.Export.Enabled = parseBool(value)
	case "storage.export.preset":
		cfg.Storage.Export.Preset = ExportPreset(value)
	case "storage.export.task_definition":
		cfg.Storage.Export.TaskDefinition = parseBool(value)
	case "storage.export.final_state":
		cfg.Storage.Export.FinalState = parseBool(value)
	case "storage.export.transcripts":
		cfg.Storage.Export.Transcripts = parseBool(value)
	case "storage.export.context_summary":
		cfg.Storage.Export.ContextSummary = parseBool(value)
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
