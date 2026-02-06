package executor

import (
	"context"
	"fmt"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/knowledge/index/artifact"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
	"github.com/randalmurphal/orc/internal/task"
)

// DefaultKnowledgeMaxTokens is the default token budget when none is configured.
const DefaultKnowledgeMaxTokens = 4000

// KnowledgePhaseConfig holds configuration for a knowledge phase.
type KnowledgePhaseConfig struct {
	Query     string `json:"query"`
	Preset    string `json:"preset,omitempty"`
	OutputVar string `json:"output_var"`
	Fallback  string `json:"fallback,omitempty"` // "skip" or "error" (default)
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// KnowledgeQueryService is the interface that KnowledgePhaseExecutor requires.
// Satisfied by *knowledge.Service.
type KnowledgeQueryService interface {
	IsAvailable() bool
	Query(ctx context.Context, query string, opts retrieve.QueryOpts) (*retrieve.PipelineResult, error)
}

// KnowledgeArtifactIndexService extends KnowledgeQueryService with artifact indexing.
// Satisfied by *knowledge.Service when the knowledge layer supports artifact indexing.
type KnowledgeArtifactIndexService interface {
	IsAvailable() bool
	IndexTaskArtifacts(ctx context.Context, params artifact.IndexParams) error
}

// KnowledgePhaseExecutor executes knowledge retrieval phases.
type KnowledgePhaseExecutor struct {
	svc KnowledgeQueryService
}

// NewKnowledgePhaseExecutor creates a new executor wrapping the given service.
func NewKnowledgePhaseExecutor(svc KnowledgeQueryService) *KnowledgePhaseExecutor {
	return &KnowledgePhaseExecutor{svc: svc}
}

// Name returns the executor type name.
func (e *KnowledgePhaseExecutor) Name() string {
	return "knowledge"
}

// ExecutePhase queries the knowledge service and stores results as a workflow variable.
func (e *KnowledgePhaseExecutor) ExecutePhase(ctx context.Context, params PhaseTypeParams) (PhaseResult, error) {
	result := PhaseResult{
		PhaseID: params.PhaseTemplate.ID,
	}

	cfg := params.KnowledgeConfig
	if cfg == nil {
		cfg = &KnowledgePhaseConfig{}
	}

	// Check availability
	if e.svc == nil || !e.svc.IsAvailable() {
		if cfg.Fallback == "skip" {
			result.Status = orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String()
			// Set output variable to empty string so downstream templates don't break
			if cfg.OutputVar != "" && params.Vars != nil {
				params.Vars[cfg.OutputVar] = ""
			}
			return result, nil
		}
		return result, fmt.Errorf("knowledge service unavailable")
	}

	// Resolve query: use task description as fallback
	query := cfg.Query
	if query == "" && params.Task != nil {
		query = task.GetDescriptionProto(params.Task)
		if query == "" {
			query = params.Task.Title
		}
	}

	// Build query options
	opts := retrieve.QueryOpts{
		Preset:    cfg.Preset,
		MaxTokens: cfg.MaxTokens,
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = DefaultKnowledgeMaxTokens
	}

	// Execute query
	pipelineResult, err := e.svc.Query(ctx, query, opts)
	if err != nil {
		if cfg.Fallback == "skip" {
			result.Status = orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String()
			if cfg.OutputVar != "" && params.Vars != nil {
				params.Vars[cfg.OutputVar] = ""
			}
			return result, nil
		}
		return result, fmt.Errorf("knowledge query: %w", err)
	}

	// Render results as markdown content
	content := renderKnowledgeContent(pipelineResult)

	// Store output variable
	outputVar := cfg.OutputVar
	if outputVar == "" {
		outputVar = params.PhaseTemplate.OutputVarName
	}
	if outputVar != "" {
		if params.Vars != nil {
			params.Vars[outputVar] = content
		}
		if params.RCtx != nil && params.RCtx.PhaseOutputVars != nil {
			params.RCtx.PhaseOutputVars[outputVar] = content
		}
	}

	result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
	result.Content = content

	return result, nil
}

// renderKnowledgeContent formats pipeline results as markdown.
func renderKnowledgeContent(pr *retrieve.PipelineResult) string {
	if pr == nil || len(pr.Documents) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Knowledge Context\n\n")

	for i, doc := range pr.Documents {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		if doc.ID != "" {
			fmt.Fprintf(&b, "## %s\n\n", doc.ID)
		}
		b.WriteString(doc.Content)
		b.WriteString("\n")
	}

	return b.String()
}
