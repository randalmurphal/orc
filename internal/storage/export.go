package storage

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
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
	t, err := e.backend.LoadTaskProto(taskID)
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

// exportState exports state.yaml using task.Execution.
// Note: This uses the Task-centric approach where execution state is in task.Execution.
func (e *ExportService) exportState(taskID, exportDir string) error {
	t, err := e.backend.LoadTaskProto(taskID)
	if err != nil {
		return err
	}

	// Export the execution state from the task
	data, err := yaml.Marshal(t.Execution)
	if err != nil {
		return fmt.Errorf("marshal execution state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(exportDir, "state.yaml"), data, 0644); err != nil {
		return fmt.Errorf("write state.yaml: %w", err)
	}

	return nil
}

// exportContextSummary generates and exports context.md.
// Note: This uses the Task-centric approach where execution state is in task.Execution.
func (e *ExportService) exportContextSummary(taskID, exportDir string) error {
	t, err := e.backend.LoadTaskProto(taskID)
	if err != nil {
		return err
	}

	// Generate context.md content using task.Execution
	content, err := generateContextSummary(t)
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
// Note: Uses task.Execution for execution state (Task-centric approach).
func generateContextSummary(t *orcv1.Task) (string, error) {
	// Build the summary manually to avoid template complexity with proto types
	var buf bytes.Buffer

	// Header
	buf.WriteString(fmt.Sprintf("# Task Context: %s\n\n", t.Id))

	// Overview
	buf.WriteString("## Overview\n\n")
	buf.WriteString("| Field | Value |\n")
	buf.WriteString("|-------|-------|\n")
	buf.WriteString(fmt.Sprintf("| Title | %s |\n", t.Title))
	buf.WriteString(fmt.Sprintf("| Weight | %s |\n", task.WeightFromProto(t.Weight)))
	buf.WriteString(fmt.Sprintf("| Status | %s |\n", task.StatusFromProto(t.Status)))
	buf.WriteString(fmt.Sprintf("| Branch | %s |\n", t.Branch))
	if t.CreatedAt != nil {
		buf.WriteString(fmt.Sprintf("| Created | %s |\n", t.CreatedAt.AsTime().Format("2006-01-02 15:04:05")))
	}
	if t.CompletedAt != nil {
		buf.WriteString(fmt.Sprintf("| Completed | %s |\n", t.CompletedAt.AsTime().Format("2006-01-02 15:04:05")))
	}
	buf.WriteString("\n")

	// Description
	description := task.GetDescriptionProto(t)
	if description != "" {
		buf.WriteString("## Description\n\n")
		buf.WriteString(description)
		buf.WriteString("\n\n")
	}

	// Phases
	task.EnsureExecutionProto(t)
	if len(t.Execution.Phases) > 0 {
		buf.WriteString("## Phases\n\n")
		buf.WriteString("| Phase | Status | Tokens |\n")
		buf.WriteString("|-------|--------|--------|\n")
		for id, phase := range t.Execution.Phases {
			tokens := int32(0)
			if phase.Tokens != nil {
				tokens = phase.Tokens.TotalTokens
			}
			buf.WriteString(fmt.Sprintf("| %s | %s | %d |\n", id, task.PhaseStatusFromProto(phase.Status), tokens))
		}
		buf.WriteString("\n")
	}

	// Token Usage
	buf.WriteString("## Token Usage\n\n")
	inputTokens := int32(0)
	outputTokens := int32(0)
	totalTokens := int32(0)
	totalCost := float64(0)
	if t.Execution.Tokens != nil {
		inputTokens = t.Execution.Tokens.InputTokens
		outputTokens = t.Execution.Tokens.OutputTokens
		totalTokens = t.Execution.Tokens.TotalTokens
	}
	if t.Execution.Cost != nil {
		totalCost = t.Execution.Cost.TotalCostUsd
	}
	buf.WriteString(fmt.Sprintf("- **Input Tokens:** %d\n", inputTokens))
	buf.WriteString(fmt.Sprintf("- **Output Tokens:** %d\n", outputTokens))
	buf.WriteString(fmt.Sprintf("- **Total Tokens:** %d\n", totalTokens))
	buf.WriteString(fmt.Sprintf("- **Total Cost:** $%.4f\n\n", totalCost))

	// Gate Decisions
	if len(t.Execution.Gates) > 0 {
		buf.WriteString("## Gate Decisions\n\n")
		buf.WriteString("| Phase | Type | Approved | Reason |\n")
		buf.WriteString("|-------|------|----------|--------|\n")
		for _, gate := range t.Execution.Gates {
			reason := ""
			if gate.Reason != nil {
				reason = *gate.Reason
			}
			buf.WriteString(fmt.Sprintf("| %s | %s | %v | %s |\n", gate.Phase, gate.GateType, gate.Approved, reason))
		}
		buf.WriteString("\n")
	}

	// Footer
	buf.WriteString("---\n")
	buf.WriteString(fmt.Sprintf("*Generated by orc on %s*\n", time.Now().Format("2006-01-02 15:04:05")))

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
