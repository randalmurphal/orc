package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
)

// ConfigSourceInfo represents source information for a config value.
type ConfigSourceInfo struct {
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`
}

// handleGetConfig returns orc configuration with optional source tracking.
// Query params:
//   - with_sources=true: Include source metadata for each config value
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	withSources := r.URL.Query().Get("with_sources") == "true"

	// Load config with source tracking
	tc, err := config.LoadWithSourcesFrom(s.workDir)
	if err != nil {
		// Fall back to defaults
		tc = config.NewTrackedConfig()
	}
	cfg := tc.Config

	response := map[string]any{
		"version": "1.0.0",
		"profile": cfg.Profile,
		"automation": map[string]any{
			"profile":       cfg.Profile,
			"gates_default": cfg.Gates.DefaultType,
			"retry_enabled": cfg.Retry.Enabled,
			"retry_max":     cfg.Retry.MaxRetries,
		},
		"execution": map[string]any{
			"model":          cfg.Model,
			"max_iterations": cfg.MaxIterations,
			"timeout":        cfg.Timeout.String(),
		},
		"git": map[string]any{
			"branch_prefix": cfg.BranchPrefix,
			"commit_prefix": cfg.CommitPrefix,
		},
		"worktree": map[string]any{
			"enabled":             cfg.Worktree.Enabled,
			"dir":                 cfg.Worktree.Dir,
			"cleanup_on_complete": cfg.Worktree.CleanupOnComplete,
			"cleanup_on_fail":     cfg.Worktree.CleanupOnFail,
		},
		"completion": map[string]any{
			"action":        cfg.Completion.Action,
			"target_branch": cfg.Completion.TargetBranch,
			"delete_branch": cfg.Completion.DeleteBranch,
		},
		"timeouts": map[string]any{
			"phase_max":          cfg.Timeouts.PhaseMax.String(),
			"turn_max":           cfg.Timeouts.TurnMax.String(),
			"idle_warning":       cfg.Timeouts.IdleWarning.String(),
			"heartbeat_interval": cfg.Timeouts.HeartbeatInterval.String(),
			"idle_timeout":       cfg.Timeouts.IdleTimeout.String(),
		},
	}

	// Include source metadata if requested
	if withSources {
		sources := make(map[string]ConfigSourceInfo)

		// Map config paths to their sources
		sourceKeys := []string{
			"profile",
			"model",
			"max_iterations",
			"timeout",
			"gates.default_type",
			"retry.enabled",
			"retry.max_retries",
			"branch_prefix",
			"commit_prefix",
		}

		for _, key := range sourceKeys {
			ts := tc.GetTrackedSource(key)
			sources[key] = ConfigSourceInfo{
				Source: string(ts.Source),
				Path:   ts.Path,
			}
		}

		response["sources"] = sources
	}

	s.jsonResponse(w, response)
}

// ConfigUpdateRequest represents a config update request.
type ConfigUpdateRequest struct {
	Profile    string `json:"profile,omitempty"`
	Automation *struct {
		GatesDefault string `json:"gates_default,omitempty"`
		RetryEnabled *bool  `json:"retry_enabled,omitempty"`
		RetryMax     *int   `json:"retry_max,omitempty"`
	} `json:"automation,omitempty"`
	Execution *struct {
		Model         string `json:"model,omitempty"`
		MaxIterations *int   `json:"max_iterations,omitempty"`
		Timeout       string `json:"timeout,omitempty"`
	} `json:"execution,omitempty"`
	Git *struct {
		BranchPrefix string `json:"branch_prefix,omitempty"`
		CommitPrefix string `json:"commit_prefix,omitempty"`
	} `json:"git,omitempty"`
	Worktree *struct {
		Enabled           *bool  `json:"enabled,omitempty"`
		Dir               string `json:"dir,omitempty"`
		CleanupOnComplete *bool  `json:"cleanup_on_complete,omitempty"`
		CleanupOnFail     *bool  `json:"cleanup_on_fail,omitempty"`
	} `json:"worktree,omitempty"`
	Completion *struct {
		Action       string `json:"action,omitempty"`
		TargetBranch string `json:"target_branch,omitempty"`
		DeleteBranch *bool  `json:"delete_branch,omitempty"`
	} `json:"completion,omitempty"`
	Timeouts *struct {
		PhaseMax          string `json:"phase_max,omitempty"`
		TurnMax           string `json:"turn_max,omitempty"`
		IdleWarning       string `json:"idle_warning,omitempty"`
		HeartbeatInterval string `json:"heartbeat_interval,omitempty"`
		IdleTimeout       string `json:"idle_timeout,omitempty"`
	} `json:"timeouts,omitempty"`
}

// handleUpdateConfig updates orc configuration.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Load existing config from workDir
	configPath := filepath.Join(s.workDir, ".orc", "config.yaml")
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		cfg = config.Default()
	}

	// Apply profile if specified
	if req.Profile != "" {
		profile := config.AutomationProfile(req.Profile)
		cfg.ApplyProfile(profile)
	}

	// Apply automation settings
	if req.Automation != nil {
		if req.Automation.GatesDefault != "" {
			cfg.Gates.DefaultType = req.Automation.GatesDefault
		}
		if req.Automation.RetryEnabled != nil {
			cfg.Retry.Enabled = *req.Automation.RetryEnabled
		}
		if req.Automation.RetryMax != nil {
			cfg.Retry.MaxRetries = *req.Automation.RetryMax
		}
	}

	// Apply execution settings
	if req.Execution != nil {
		if req.Execution.Model != "" {
			cfg.Model = req.Execution.Model
		}
		if req.Execution.MaxIterations != nil {
			cfg.MaxIterations = *req.Execution.MaxIterations
		}
		if req.Execution.Timeout != "" {
			d, err := time.ParseDuration(req.Execution.Timeout)
			if err != nil {
				s.jsonError(w, fmt.Sprintf("invalid timeout format: %v", err), http.StatusBadRequest)
				return
			}
			cfg.Timeout = d
		}
	}

	// Apply git settings
	if req.Git != nil {
		if req.Git.BranchPrefix != "" {
			// Validate branch prefix by testing with a sample task ID
			testBranch := req.Git.BranchPrefix + "TASK-001"
			if err := git.ValidateBranchName(testBranch); err != nil {
				s.jsonError(w, "invalid branch_prefix", http.StatusBadRequest)
				return
			}
			cfg.BranchPrefix = req.Git.BranchPrefix
		}
		if req.Git.CommitPrefix != "" {
			cfg.CommitPrefix = req.Git.CommitPrefix
		}
	}

	// Apply worktree settings
	if req.Worktree != nil {
		if req.Worktree.Enabled != nil {
			cfg.Worktree.Enabled = *req.Worktree.Enabled
		}
		if req.Worktree.Dir != "" {
			cfg.Worktree.Dir = req.Worktree.Dir
		}
		if req.Worktree.CleanupOnComplete != nil {
			cfg.Worktree.CleanupOnComplete = *req.Worktree.CleanupOnComplete
		}
		if req.Worktree.CleanupOnFail != nil {
			cfg.Worktree.CleanupOnFail = *req.Worktree.CleanupOnFail
		}
	}

	// Apply completion settings
	if req.Completion != nil {
		if req.Completion.Action != "" {
			cfg.Completion.Action = req.Completion.Action
		}
		if req.Completion.TargetBranch != "" {
			// Validate target branch name for security
			if err := git.ValidateBranchName(req.Completion.TargetBranch); err != nil {
				s.jsonError(w, "invalid target_branch", http.StatusBadRequest)
				return
			}
			cfg.Completion.TargetBranch = req.Completion.TargetBranch
		}
		if req.Completion.DeleteBranch != nil {
			cfg.Completion.DeleteBranch = *req.Completion.DeleteBranch
		}
	}

	// Apply timeout settings
	if req.Timeouts != nil {
		if req.Timeouts.PhaseMax != "" {
			d, err := time.ParseDuration(req.Timeouts.PhaseMax)
			if err != nil {
				s.jsonError(w, fmt.Sprintf("invalid phase_max format: %v", err), http.StatusBadRequest)
				return
			}
			cfg.Timeouts.PhaseMax = d
		}
		if req.Timeouts.TurnMax != "" {
			d, err := time.ParseDuration(req.Timeouts.TurnMax)
			if err != nil {
				s.jsonError(w, fmt.Sprintf("invalid turn_max format: %v", err), http.StatusBadRequest)
				return
			}
			cfg.Timeouts.TurnMax = d
		}
		if req.Timeouts.IdleWarning != "" {
			d, err := time.ParseDuration(req.Timeouts.IdleWarning)
			if err != nil {
				s.jsonError(w, fmt.Sprintf("invalid idle_warning format: %v", err), http.StatusBadRequest)
				return
			}
			cfg.Timeouts.IdleWarning = d
		}
		if req.Timeouts.HeartbeatInterval != "" {
			d, err := time.ParseDuration(req.Timeouts.HeartbeatInterval)
			if err != nil {
				s.jsonError(w, fmt.Sprintf("invalid heartbeat_interval format: %v", err), http.StatusBadRequest)
				return
			}
			cfg.Timeouts.HeartbeatInterval = d
		}
		if req.Timeouts.IdleTimeout != "" {
			d, err := time.ParseDuration(req.Timeouts.IdleTimeout)
			if err != nil {
				s.jsonError(w, fmt.Sprintf("invalid idle_timeout format: %v", err), http.StatusBadRequest)
				return
			}
			cfg.Timeouts.IdleTimeout = d
		}
	}

	// Save config to workDir
	if err := cfg.SaveTo(configPath); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	// Auto-commit config change
	s.autoCommitConfig("automation settings updated")

	// Return updated config
	s.jsonResponse(w, map[string]any{
		"version": "1.0.0",
		"profile": cfg.Profile,
		"automation": map[string]any{
			"profile":       cfg.Profile,
			"gates_default": cfg.Gates.DefaultType,
			"retry_enabled": cfg.Retry.Enabled,
			"retry_max":     cfg.Retry.MaxRetries,
		},
		"execution": map[string]any{
			"model":          cfg.Model,
			"max_iterations": cfg.MaxIterations,
			"timeout":        cfg.Timeout.String(),
		},
		"git": map[string]any{
			"branch_prefix": cfg.BranchPrefix,
			"commit_prefix": cfg.CommitPrefix,
		},
		"worktree": map[string]any{
			"enabled":             cfg.Worktree.Enabled,
			"dir":                 cfg.Worktree.Dir,
			"cleanup_on_complete": cfg.Worktree.CleanupOnComplete,
			"cleanup_on_fail":     cfg.Worktree.CleanupOnFail,
		},
		"completion": map[string]any{
			"action":        cfg.Completion.Action,
			"target_branch": cfg.Completion.TargetBranch,
			"delete_branch": cfg.Completion.DeleteBranch,
		},
		"timeouts": map[string]any{
			"phase_max":          cfg.Timeouts.PhaseMax.String(),
			"turn_max":           cfg.Timeouts.TurnMax.String(),
			"idle_warning":       cfg.Timeouts.IdleWarning.String(),
			"heartbeat_interval": cfg.Timeouts.HeartbeatInterval.String(),
			"idle_timeout":       cfg.Timeouts.IdleTimeout.String(),
		},
	})
}

// autoCommitConfig commits config changes to git if auto-commit is enabled.
func (s *Server) autoCommitConfig(description string) {
	if s.orcConfig == nil || s.orcConfig.Tasks.DisableAutoCommit {
		return
	}

	configPath := filepath.Join(s.workDir, ".orc", "config.yaml")

	// Git add the config file
	addCmd := exec.Command("git", "add", configPath)
	addCmd.Dir = s.workDir
	if err := addCmd.Run(); err != nil {
		s.logger.Debug("skip config auto-commit: git add failed", "error", err)
		return
	}

	// Check if there are staged changes
	diffCmd := exec.Command("git", "diff", "--cached", "--quiet")
	diffCmd.Dir = s.workDir
	if err := diffCmd.Run(); err == nil {
		// No changes to commit
		return
	}

	// Commit the changes
	prefix := s.orcConfig.CommitPrefix
	if prefix == "" {
		prefix = "[orc]"
	}
	commitMsg := fmt.Sprintf("%s config: %s", prefix, description)
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitCmd.Dir = s.workDir
	if err := commitCmd.Run(); err != nil {
		s.logger.Warn("failed to auto-commit config change", "error", err)
	}
}
