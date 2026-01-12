package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/config"
)

// handleGetConfig returns orc configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	configPath := filepath.Join(s.workDir, ".orc", "config.yaml")
	cfg, err := config.LoadFrom(configPath)
	if err != nil {
		cfg = config.Default()
	}

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
	})
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
			if d, err := time.ParseDuration(req.Execution.Timeout); err == nil {
				cfg.Timeout = d
			}
		}
	}

	// Apply git settings
	if req.Git != nil {
		if req.Git.BranchPrefix != "" {
			cfg.BranchPrefix = req.Git.BranchPrefix
		}
		if req.Git.CommitPrefix != "" {
			cfg.CommitPrefix = req.Git.CommitPrefix
		}
	}

	// Save config to workDir
	if err := cfg.SaveTo(configPath); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

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
	})
}
