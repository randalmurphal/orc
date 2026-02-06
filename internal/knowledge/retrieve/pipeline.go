package retrieve

import (
	"context"
	"fmt"
	"sort"
)

const defaultMaxTokens = 8000
const defaultExcerptLen = 200

// Pipeline executes stages sequentially and applies scoring.
type Pipeline struct {
	stages      []Stage
	scorer      Scorer
	limit       int
	minScore    float64
	maxTokens   int
	maxResults  int
	summaryOnly bool
	dedup       bool
}

// SetMaxTokens overrides the token budget.
func (p *Pipeline) SetMaxTokens(n int) { p.maxTokens = n }

// SetMinScore overrides the minimum score threshold.
func (p *Pipeline) SetMinScore(s float64) { p.minScore = s }

// SetSummaryOnly overrides summary-only mode.
func (p *Pipeline) SetSummaryOnly(b bool) { p.summaryOnly = b }

// SetLimit overrides the result limit.
func (p *Pipeline) SetLimit(n int) { p.limit = n }

// StageNames returns the names of all stages in execution order.
func (p *Pipeline) StageNames() []string {
	names := make([]string, len(p.stages))
	for i, s := range p.stages {
		names[i] = s.Name()
	}
	return names
}

// Execute runs all stages, scores documents, applies filters, and returns results.
func (p *Pipeline) Execute(ctx context.Context, query string) (*PipelineResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	var candidates []ScoredDocument
	var accumulated []ScoredDocument
	for _, stage := range p.stages {
		var err error
		candidates, err = stage.Execute(ctx, query, candidates)
		if err != nil {
			return nil, fmt.Errorf("stage %s: %w", stage.Name(), err)
		}
		if p.dedup {
			accumulated = append(accumulated, candidates...)
		}
	}

	if p.dedup {
		candidates = deduplicateCandidates(accumulated)
	}

	// Score all candidates
	for i := range candidates {
		candidates[i].FinalScore = p.scorer.Score(candidates[i].Signals)
	}

	// Sort descending by score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].FinalScore > candidates[j].FinalScore
	})

	// Apply limit
	if p.limit > 0 && len(candidates) > p.limit {
		candidates = candidates[:p.limit]
	}

	// Apply MinScore filter
	if p.minScore > 0 {
		filtered := candidates[:0]
		for _, c := range candidates {
			if c.FinalScore >= p.minScore {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}

	// Apply summary-only mode
	if p.summaryOnly {
		for i := range candidates {
			if candidates[i].Summary != "" {
				candidates[i].Content = candidates[i].Summary
			} else if len(candidates[i].Content) > defaultExcerptLen {
				candidates[i].Content = candidates[i].Content[:defaultExcerptLen] + "..."
			}
		}
	}

	// Apply token budget
	maxTok := p.maxTokens
	if maxTok == 0 {
		maxTok = defaultMaxTokens
	}

	var budgeted []ScoredDocument
	tokensUsed := 0
	for i, c := range candidates {
		docTokens := c.Tokens
		if docTokens == 0 {
			docTokens = estimateTokens(c.Content)
		}
		if p.summaryOnly && c.Summary != "" {
			docTokens = estimateTokens(c.Content)
		}

		if i == 0 {
			// First result always included
			budgeted = append(budgeted, c)
			tokensUsed += docTokens
			continue
		}

		if tokensUsed+docTokens > maxTok {
			break
		}
		budgeted = append(budgeted, c)
		tokensUsed += docTokens
	}

	// Apply maxResults
	if p.maxResults > 0 && len(budgeted) > p.maxResults {
		budgeted = budgeted[:p.maxResults]
	}

	return &PipelineResult{
		Documents:  budgeted,
		TokensUsed: tokensUsed,
	}, nil
}

func estimateTokens(s string) int {
	// ~4 chars per token
	n := len(s) / 4
	if n == 0 && len(s) > 0 {
		n = 1
	}
	return n
}

func deduplicateCandidates(candidates []ScoredDocument) []ScoredDocument {
	seen := make(map[string]int) // id → index in result
	var result []ScoredDocument

	for _, c := range candidates {
		if idx, ok := seen[c.ID]; ok {
			// Merge signals
			for k, v := range c.Signals {
				if _, exists := result[idx].Signals[k]; !exists {
					result[idx].Signals[k] = v
				}
			}
		} else {
			seen[c.ID] = len(result)
			// Copy signals map to avoid mutation
			signals := make(map[string]float64, len(c.Signals))
			for k, v := range c.Signals {
				signals[k] = v
			}
			c.Signals = signals
			result = append(result, c)
		}
	}
	return result
}

// PipelineBuilder constructs pipelines with a fluent API.
type PipelineBuilder struct {
	stages      []Stage
	scorer      Scorer
	limit       int
	minScore    float64
	maxTokens   int
	maxResults  int
	summaryOnly bool
	dedup       bool
}

// NewPipelineBuilder creates a new pipeline builder.
func NewPipelineBuilder() *PipelineBuilder {
	return &PipelineBuilder{}
}

// AddStage appends a stage to the pipeline.
func (b *PipelineBuilder) AddStage(s Stage) *PipelineBuilder {
	b.stages = append(b.stages, s)
	return b
}

// WithScorer sets the scoring function.
func (b *PipelineBuilder) WithScorer(s Scorer) *PipelineBuilder {
	b.scorer = s
	return b
}

// WithLimit sets the maximum number of results after scoring.
func (b *PipelineBuilder) WithLimit(n int) *PipelineBuilder {
	b.limit = n
	return b
}

// WithMinScore sets the minimum score threshold.
func (b *PipelineBuilder) WithMinScore(score float64) *PipelineBuilder {
	b.minScore = score
	return b
}

// WithMaxTokens sets the token budget for results.
func (b *PipelineBuilder) WithMaxTokens(n int) *PipelineBuilder {
	b.maxTokens = n
	return b
}

// WithMaxResults sets a hard cap on result count.
func (b *PipelineBuilder) WithMaxResults(n int) *PipelineBuilder {
	b.maxResults = n
	return b
}

// WithSummaryOnly enables summary-only mode.
func (b *PipelineBuilder) WithSummaryOnly(enabled bool) *PipelineBuilder {
	b.summaryOnly = enabled
	return b
}

// WithDeduplication enables document deduplication with signal merging.
func (b *PipelineBuilder) WithDeduplication(enabled bool) *PipelineBuilder {
	b.dedup = enabled
	return b
}

// Build validates configuration and returns the pipeline.
func (b *PipelineBuilder) Build() (*Pipeline, error) {
	if len(b.stages) == 0 {
		return nil, fmt.Errorf("pipeline requires at least one stage")
	}
	if b.maxTokens < 0 {
		return nil, fmt.Errorf("negative MaxTokens: %d", b.maxTokens)
	}
	scorer := b.scorer
	if scorer == nil {
		scorer = NewWeightedScorer(nil)
	}
	return &Pipeline{
		stages:      b.stages,
		scorer:      scorer,
		limit:       b.limit,
		minScore:    b.minScore,
		maxTokens:   b.maxTokens,
		maxResults:  b.maxResults,
		summaryOnly: b.summaryOnly,
		dedup:       b.dedup,
	}, nil
}

// WeightedScorer computes a normalized weighted sum of signals.
type WeightedScorer struct {
	weights     map[string]float64
	totalWeight float64
}

// NewWeightedScorer creates a scorer with the given signal weights.
func NewWeightedScorer(weights map[string]float64) *WeightedScorer {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	return &WeightedScorer{weights: weights, totalWeight: total}
}

// Score computes the normalized weighted sum. Missing signals contribute 0.
func (s *WeightedScorer) Score(signals map[string]float64) float64 {
	if s.totalWeight == 0 {
		return 0.0
	}
	sum := 0.0
	for name, weight := range s.weights {
		sum += signals[name] * weight
	}
	score := sum / s.totalWeight
	if score < 0 {
		return 0.0
	}
	if score > 1 {
		return 1.0
	}
	return score
}
