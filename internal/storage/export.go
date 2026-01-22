package storage

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
	"gopkg.in/yaml.v3"
)

// ExportService handles exporting task artifacts to branches.
type ExportService struct {
	backend Backend
	cfg     *config.StorageConfig
}

// NewExportService creates a new export service.
func NewExportService(backend Backend, cfg *config.StorageConfig) *ExportService {
	return &ExportService{
		backend: backend,
		cfg:     cfg,
	}
}

// Export exports task artifacts based on the provided options.
// If opts is nil, uses the configuration defaults.
func (e *ExportService) Export(taskID string, opts *ExportOptions) error {
	if opts == nil {
		// Use config defaults
		resolved := e.cfg.ResolveExportConfig()
		opts = &ExportOptions{
			TaskDefinition: resolved.TaskDefinition,
			FinalState:     resolved.FinalState,
			Transcripts:    resolved.Transcripts,
			ContextSummary: resolved.ContextSummary,
		}
	}

	// Create export directory
	exportDir := filepath.Join(task.OrcDir, "exports", taskID)
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return fmt.Errorf("create export directory: %w", err)
	}

	// Export task definition (task.yaml + plan.yaml)
	if opts.TaskDefinition {
		if err := e.exportTaskDefinition(taskID, exportDir); err != nil {
			return fmt.Errorf("export task definition: %w", err)
		}
	}

	// Export final state (state.yaml)
	if opts.FinalState {
		if err := e.exportState(taskID, exportDir); err != nil {
			return fmt.Errorf("export state: %w", err)
		}
	}

	// Export context summary (context.md)
	if opts.ContextSummary {
		if err := e.exportContextSummary(taskID, exportDir); err != nil {
			return fmt.Errorf("export context summary: %w", err)
		}
	}

	// Export transcripts (usually large, optional)
	if opts.Transcripts {
		if err := e.exportTranscripts(taskID, exportDir); err != nil {
			return fmt.Errorf("export transcripts: %w", err)
		}
	}

	return nil
}

// ExportToBranch exports task artifacts and commits them to the specified branch.
func (e *ExportService) ExportToBranch(taskID, branch string, opts *ExportOptions) error {
	// First export to temp directory
	if err := e.Export(taskID, opts); err != nil {
		return err
	}

	exportDir := filepath.Join(task.OrcDir, "exports", taskID)

	// Check if we're on the correct branch
	currentBranch, err := getCurrentBranch()
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}

	if currentBranch != branch {
		return fmt.Errorf("current branch %s does not match target branch %s", currentBranch, branch)
	}

	// Stage the exported files
	cmd := exec.Command("git", "add", exportDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stage export files: %w: %s", err, string(output))
	}

	// Check if there are changes to commit
	cmd = exec.Command("git", "diff", "--cached", "--quiet", exportDir)
	if err := cmd.Run(); err == nil {
		// No changes - nothing to commit
		return nil
	}

	// Commit the changes
	commitMsg := fmt.Sprintf("[orc] Export artifacts for %s", taskID)
	cmd = exec.Command("git", "commit", "-m", commitMsg, "--", exportDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("commit export files: %w: %s", err, string(output))
	}

	return nil
}

// exportTaskDefinition exports task.yaml.
func (e *ExportService) exportTaskDefinition(taskID, exportDir string) error {
	t, err := e.backend.LoadTask(taskID)
	if err != nil {
		return err
	}

	// Write task.yaml
	taskData, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}
	if err := os.WriteFile(filepath.Join(exportDir, "task.yaml"), taskData, 0644); err != nil {
		return fmt.Errorf("write task.yaml: %w", err)
	}

	return nil
}

// exportState exports state.yaml.
func (e *ExportService) exportState(taskID, exportDir string) error {
	s, err := e.backend.LoadState(taskID)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(exportDir, "state.yaml"), data, 0644); err != nil {
		return fmt.Errorf("write state.yaml: %w", err)
	}

	return nil
}

// exportContextSummary generates and exports context.md.
func (e *ExportService) exportContextSummary(taskID, exportDir string) error {
	t, err := e.backend.LoadTask(taskID)
	if err != nil {
		return err
	}

	s, err := e.backend.LoadState(taskID)
	if err != nil {
		return err
	}

	// Generate context.md content
	content, err := generateContextSummary(t, s)
	if err != nil {
		return fmt.Errorf("generate context summary: %w", err)
	}

	if err := os.WriteFile(filepath.Join(exportDir, "context.md"), []byte(content), 0644); err != nil {
		return fmt.Errorf("write context.md: %w", err)
	}

	return nil
}

// exportTranscripts exports transcript files.
func (e *ExportService) exportTranscripts(taskID, exportDir string) error {
	transcripts, err := e.backend.GetTranscripts(taskID)
	if err != nil {
		return err
	}

	transcriptsDir := filepath.Join(exportDir, "transcripts")
	if err := os.MkdirAll(transcriptsDir, 0755); err != nil {
		return fmt.Errorf("create transcripts directory: %w", err)
	}

	for i, t := range transcripts {
		filename := fmt.Sprintf("%03d_%s.txt", i+1, t.Phase)
		if err := os.WriteFile(filepath.Join(transcriptsDir, filename), []byte(t.Content), 0644); err != nil {
			return fmt.Errorf("write transcript %s: %w", filename, err)
		}
	}

	return nil
}

// generateContextSummary creates a markdown summary of the task context.
func generateContextSummary(t *task.Task, s *state.State) (string, error) {
	tmpl := `# Task Context: {{.Task.ID}}

## Overview

| Field | Value |
|-------|-------|
| Title | {{.Task.Title}} |
| Weight | {{.Task.Weight}} |
| Status | {{.Task.Status}} |
| Branch | {{.Task.Branch}} |
| Created | {{.Task.CreatedAt.Format "2006-01-02 15:04:05"}} |
{{- if .Task.CompletedAt}}
| Completed | {{.Task.CompletedAt.Format "2006-01-02 15:04:05"}} |
{{- end}}

{{if .Task.Description}}
## Description

{{.Task.Description}}
{{end}}

## Phases

| Phase | Status | Tokens |
|-------|--------|--------|
{{- range $id, $phase := .State.Phases}}
| {{$id}} | {{$phase.Status}} | {{$phase.Tokens.TotalTokens}} |
{{- end}}

## Token Usage

- **Input Tokens:** {{.State.Tokens.InputTokens}}
- **Output Tokens:** {{.State.Tokens.OutputTokens}}
- **Total Tokens:** {{.State.Tokens.TotalTokens}}
- **Total Cost:** ${{printf "%.4f" .State.Cost.TotalCostUSD}}

{{if .State.Gates}}
## Gate Decisions

| Phase | Type | Approved | Reason |
|-------|------|----------|--------|
{{- range .State.Gates}}
| {{.Phase}} | {{.GateType}} | {{.Approved}} | {{.Reason}} |
{{- end}}
{{end}}

---
*Generated by orc on {{.GeneratedAt.Format "2006-01-02 15:04:05"}}*
`

	t2 := template.Must(template.New("context").Parse(tmpl))

	data := struct {
		Task        *task.Task
		State       *state.State
		GeneratedAt time.Time
	}{
		Task:        t,
		State:       s,
		GeneratedAt: time.Now(),
	}

	var buf bytes.Buffer
	if err := t2.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// getCurrentBranch returns the current git branch name.
func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Ensure ExportService implements Exporter
var _ Exporter = (*ExportService)(nil)
