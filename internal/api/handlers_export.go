package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ExportRequest represents a request to export task artifacts.
type ExportRequest struct {
	// TaskDefinition exports task.yaml and plan.yaml
	TaskDefinition *bool `json:"task_definition,omitempty"`
	// FinalState exports state.yaml
	FinalState *bool `json:"final_state,omitempty"`
	// Transcripts exports full transcript files
	Transcripts *bool `json:"transcripts,omitempty"`
	// ContextSummary exports context.md
	ContextSummary *bool `json:"context_summary,omitempty"`
	// ToBranch optionally commits exports to the current branch
	ToBranch bool `json:"to_branch,omitempty"`
}

// ExportResponse represents the result of an export operation.
type ExportResponse struct {
	Success      bool     `json:"success"`
	TaskID       string   `json:"task_id"`
	ExportedTo   string   `json:"exported_to"`
	Files        []string `json:"files,omitempty"`
	CommittedSHA string   `json:"committed_sha,omitempty"`
}

// handleExportTask exports task artifacts to the export directory or branch.
// POST /api/tasks/:id/export
func (s *Server) handleExportTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	if taskID == "" {
		s.jsonError(w, "task ID required", http.StatusBadRequest)
		return
	}

	// Check if task exists
	if !task.ExistsIn(s.workDir, taskID) {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Parse request body
	var req ExportRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		s.jsonError(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Build export options from request or defaults
	opts := buildExportOptions(&req, &cfg.Storage)

	// Get project path from workDir
	projectPath := s.workDir
	if projectPath == "" {
		var err error
		projectPath, err = os.Getwd()
		if err != nil {
			s.jsonError(w, "failed to get working directory: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Create storage backend
	backend, err := storage.NewBackend(projectPath, &cfg.Storage)
	if err != nil {
		s.jsonError(w, "failed to create storage backend: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = backend.Close() }()

	// Create export service
	exportSvc := storage.NewExportService(backend, &cfg.Storage)

	// Perform export
	if req.ToBranch {
		// Get current branch for the task
		t, err := task.LoadFrom(s.workDir, taskID)
		if err != nil {
			s.jsonError(w, "failed to load task: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := exportSvc.ExportToBranch(taskID, t.Branch, opts); err != nil {
			s.jsonError(w, "failed to export to branch: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, ExportResponse{
			Success:    true,
			TaskID:     taskID,
			ExportedTo: t.Branch,
		})
	} else {
		if err := exportSvc.Export(taskID, opts); err != nil {
			s.jsonError(w, "failed to export: "+err.Error(), http.StatusInternalServerError)
			return
		}

		s.jsonResponse(w, ExportResponse{
			Success:    true,
			TaskID:     taskID,
			ExportedTo: ".orc/exports/" + taskID,
		})
	}
}

// handleGetExportConfig returns the current export configuration.
// GET /api/config/export
func (s *Server) handleGetExportConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		s.jsonError(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return resolved export config (with preset applied)
	resolved := cfg.Storage.ResolveExportConfig()
	s.jsonResponse(w, map[string]any{
		"enabled":         resolved.Enabled,
		"preset":          resolved.Preset,
		"task_definition": resolved.TaskDefinition,
		"final_state":     resolved.FinalState,
		"transcripts":     resolved.Transcripts,
		"context_summary": resolved.ContextSummary,
	})
}

// handleUpdateExportConfig updates the export configuration.
// PUT /api/config/export
func (s *Server) handleUpdateExportConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled        *bool   `json:"enabled,omitempty"`
		Preset         *string `json:"preset,omitempty"`
		TaskDefinition *bool   `json:"task_definition,omitempty"`
		FinalState     *bool   `json:"final_state,omitempty"`
		Transcripts    *bool   `json:"transcripts,omitempty"`
		ContextSummary *bool   `json:"context_summary,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		s.jsonError(w, "failed to load config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Update only provided fields
	if req.Enabled != nil {
		cfg.Storage.Export.Enabled = *req.Enabled
	}
	if req.Preset != nil {
		// Validate preset early for better error message
		if *req.Preset != "" && !contains(config.ValidExportPresets, *req.Preset) {
			s.jsonError(w, "invalid preset: must be one of minimal, standard, full", http.StatusBadRequest)
			return
		}
		cfg.Storage.Export.Preset = config.ExportPreset(*req.Preset)
	}
	if req.TaskDefinition != nil {
		cfg.Storage.Export.TaskDefinition = *req.TaskDefinition
	}
	if req.FinalState != nil {
		cfg.Storage.Export.FinalState = *req.FinalState
	}
	if req.Transcripts != nil {
		cfg.Storage.Export.Transcripts = *req.Transcripts
	}
	if req.ContextSummary != nil {
		cfg.Storage.Export.ContextSummary = *req.ContextSummary
	}

	// Validate and save
	if err := cfg.Validate(); err != nil {
		s.jsonError(w, "invalid configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := cfg.Save(); err != nil {
		s.jsonError(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return updated config
	s.handleGetExportConfig(w, r)
}

// buildExportOptions creates ExportOptions from request and config defaults.
func buildExportOptions(req *ExportRequest, storageCfg *config.StorageConfig) *storage.ExportOptions {
	// Start with config defaults
	resolved := storageCfg.ResolveExportConfig()
	opts := &storage.ExportOptions{
		TaskDefinition: resolved.TaskDefinition,
		FinalState:     resolved.FinalState,
		Transcripts:    resolved.Transcripts,
		ContextSummary: resolved.ContextSummary,
	}

	// Override with request values if provided
	if req.TaskDefinition != nil {
		opts.TaskDefinition = *req.TaskDefinition
	}
	if req.FinalState != nil {
		opts.FinalState = *req.FinalState
	}
	if req.Transcripts != nil {
		opts.Transcripts = *req.Transcripts
	}
	if req.ContextSummary != nil {
		opts.ContextSummary = *req.ContextSummary
	}

	return opts
}

// contains checks if a string slice contains a given value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
