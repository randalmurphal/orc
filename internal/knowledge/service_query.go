package knowledge

import (
	"context"
	"fmt"

	"github.com/randalmurphal/orc/internal/brief"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
)

// QueryComponents extends Components with retrieval capabilities.
type QueryComponents interface {
	Components
	retrieve.Embedder
	retrieve.VectorSearcher
	retrieve.GraphQuerier
	retrieve.Reranker
}

// TaskContext holds categorized search results for task-scoped queries.
type TaskContext struct {
	FileHistory []retrieve.Document
	RelatedWork []retrieve.Document
	Decisions   []retrieve.Document
	Patterns    []retrieve.Document
	Warnings    []retrieve.Document
}

// Query executes a retrieval pipeline with the given preset and options.
func (s *Service) Query(ctx context.Context, query string, opts retrieve.QueryOpts) (*retrieve.PipelineResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	if !s.IsAvailable() {
		return &retrieve.PipelineResult{}, nil
	}

	deps, ok := s.buildPresetDeps()
	if !ok {
		return &retrieve.PipelineResult{}, nil
	}

	preset := opts.Preset
	if preset == "" {
		preset = "standard"
	}

	pipeline, err := buildPipeline(preset, deps)
	if err != nil {
		return nil, fmt.Errorf("build pipeline %s: %w", preset, err)
	}

	// Apply query options
	if opts.MaxTokens != 0 {
		pipeline.SetMaxTokens(opts.MaxTokens)
	}
	if opts.MinScore > 0 {
		pipeline.SetMinScore(opts.MinScore)
	}
	if opts.SummaryOnly {
		pipeline.SetSummaryOnly(true)
	}
	if opts.Limit > 0 {
		pipeline.SetLimit(opts.Limit)
	}

	return pipeline.Execute(ctx, query)
}

// QueryForTask returns structured context for a task description.
func (s *Service) QueryForTask(ctx context.Context, description string) (*TaskContext, error) {
	tc := &TaskContext{}
	if description == "" {
		return tc, nil
	}
	if !s.IsAvailable() {
		return tc, nil
	}

	result, err := s.Query(ctx, description, retrieve.QueryOpts{
		Preset:      "fast",
		SummaryOnly: true,
	})
	if err != nil {
		return tc, nil
	}

	for _, doc := range result.Documents {
		docType := ""
		if doc.Metadata != nil {
			if t, ok := doc.Metadata["type"].(string); ok {
				docType = t
			}
		}
		d := doc.Document
		switch docType {
		case "pattern":
			tc.Patterns = append(tc.Patterns, d)
		case "warning":
			tc.Warnings = append(tc.Warnings, d)
		case "decision":
			tc.Decisions = append(tc.Decisions, d)
		case "file_history":
			tc.FileHistory = append(tc.FileHistory, d)
		default:
			tc.RelatedWork = append(tc.RelatedWork, d)
		}
	}
	return tc, nil
}

// EnrichBrief adds graph-derived sections to a brief using category constants.
func (s *Service) EnrichBrief(ctx context.Context, b *brief.Brief) (*brief.Brief, error) {
	if b == nil {
		return nil, nil
	}
	if !s.IsAvailable() {
		return b, nil
	}

	enricher, ok := s.comps.(retrieve.EnrichmentProvider)
	if !ok {
		return b, nil
	}

	// Copy the brief to avoid mutating the original
	enriched := &brief.Brief{
		GeneratedAt: b.GeneratedAt,
		TaskCount:   b.TaskCount,
		TokenCount:  b.TokenCount,
		Sections:    make([]brief.Section, len(b.Sections)),
	}
	copy(enriched.Sections, b.Sections)

	patterns, err := enricher.GetPatterns(ctx)
	if err != nil {
		return b, nil
	}
	hotFiles, err := enricher.GetHotFiles(ctx)
	if err != nil {
		return b, nil
	}
	knownIssues, err := enricher.GetKnownIssues(ctx)
	if err != nil {
		return b, nil
	}

	if len(patterns) > 0 {
		enriched.Sections = append(enriched.Sections, brief.Section{
			Category: brief.CategoryPatterns,
			Entries:  docsToEntries(patterns),
		})
	}
	if len(hotFiles) > 0 {
		enriched.Sections = append(enriched.Sections, brief.Section{
			Category: brief.CategoryHotFiles,
			Entries:  docsToEntries(hotFiles),
		})
	}
	if len(knownIssues) > 0 {
		enriched.Sections = append(enriched.Sections, brief.Section{
			Category: brief.CategoryKnownIssues,
			Entries:  docsToEntries(knownIssues),
		})
	}

	return enriched, nil
}

func docsToEntries(docs []retrieve.Document) []brief.Entry {
	entries := make([]brief.Entry, 0, len(docs))
	for _, d := range docs {
		source := d.FilePath
		if source == "" {
			source = d.ID
		}
		entries = append(entries, brief.Entry{
			Content: d.Content,
			Source:  source,
		})
	}
	return entries
}

func (s *Service) buildPresetDeps() (retrieve.PresetDeps, bool) {
	qc, ok := s.comps.(QueryComponents)
	if !ok {
		return retrieve.PresetDeps{}, false
	}

	return retrieve.PresetDeps{
		Embedder:    qc,
		VectorStore: qc,
		GraphStore:  qc,
		Reranker:    qc,
		Collection:  "documents",
	}, true
}

func buildPipeline(preset string, deps retrieve.PresetDeps) (*retrieve.Pipeline, error) {
	switch preset {
	case "standard":
		return retrieve.StandardPreset(deps)
	case "fast":
		return retrieve.FastPreset(deps)
	case "deep":
		return retrieve.DeepPreset(deps)
	case "graph_first":
		return retrieve.GraphFirstPreset(deps)
	case "recency":
		return retrieve.RecencyPreset(deps)
	default:
		return retrieve.StandardPreset(deps)
	}
}
