package retrieve

import (
	"fmt"
	"time"
)

// Preset stage limits.
const (
	standardSemanticLimit = 20
	standardGraphDepth    = 2
	standardRerankTopK    = 10

	fastSemanticLimit = 10

	deepSemanticLimit = 40
	deepGraphDepth    = 3
	deepRerankTopK    = 20

	graphFirstSemanticLimit = 20
	graphFirstGraphDepth    = 3

	recencySemanticLimit = 20
)

// Standard signal weights.
var standardWeights = map[string]float64{
	"semantic": 0.3,
	"graph":    0.2,
	"temporal": 0.2,
	"pagerank": 0.1,
	"rerank":   0.2,
}

var fastWeights = map[string]float64{
	"semantic": 1.0,
}

var graphFirstWeights = map[string]float64{
	"semantic": 0.3,
	"graph":    0.4,
	"pagerank": 0.3,
}

var recencyWeights = map[string]float64{
	"semantic": 0.5,
	"temporal": 0.5,
}

func validateCoreDeps(deps PresetDeps) error {
	if deps.VectorStore == nil {
		return fmt.Errorf("VectorStore is required")
	}
	if deps.Embedder == nil {
		return fmt.Errorf("Embedder is required")
	}
	return nil
}

func validateGraphDeps(deps PresetDeps) error {
	if deps.GraphStore == nil {
		return fmt.Errorf("GraphStore is required")
	}
	return nil
}

// StandardPreset builds the full pipeline: semantic → hydrate → graph → temporal → pagerank → rerank.
func StandardPreset(deps PresetDeps) (*Pipeline, error) {
	if err := validateCoreDeps(deps); err != nil {
		return nil, fmt.Errorf("standard preset: %w", err)
	}
	if err := validateGraphDeps(deps); err != nil {
		return nil, fmt.Errorf("standard preset: %w", err)
	}

	b := NewPipelineBuilder().
		AddStage(NewSemanticStage(deps.Embedder, deps.VectorStore, deps.Collection, standardSemanticLimit)).
		AddStage(NewHydrateStage(deps.GraphStore)).
		AddStage(NewGraphExpansionStage(deps.GraphStore, standardGraphDepth)).
		AddStage(NewTemporalDecayStage(7*24*time.Hour, time.Now())).
		AddStage(NewPageRankStage(deps.GraphStore))

	if deps.Reranker != nil {
		b.AddStage(NewRerankStage(deps.Reranker, standardRerankTopK))
	}

	return b.WithScorer(NewWeightedScorer(standardWeights)).Build()
}

// FastPreset builds a lightweight pipeline: semantic → hydrate.
func FastPreset(deps PresetDeps) (*Pipeline, error) {
	if err := validateCoreDeps(deps); err != nil {
		return nil, fmt.Errorf("fast preset: %w", err)
	}

	return NewPipelineBuilder().
		AddStage(NewSemanticStage(deps.Embedder, deps.VectorStore, deps.Collection, fastSemanticLimit)).
		AddStage(NewHydrateStage(deps.GraphStore)).
		WithScorer(NewWeightedScorer(fastWeights)).
		Build()
}

// DeepPreset builds a full pipeline with higher limits.
func DeepPreset(deps PresetDeps) (*Pipeline, error) {
	if err := validateCoreDeps(deps); err != nil {
		return nil, fmt.Errorf("deep preset: %w", err)
	}
	if err := validateGraphDeps(deps); err != nil {
		return nil, fmt.Errorf("deep preset: %w", err)
	}

	b := NewPipelineBuilder().
		AddStage(NewSemanticStage(deps.Embedder, deps.VectorStore, deps.Collection, deepSemanticLimit)).
		AddStage(NewHydrateStage(deps.GraphStore)).
		AddStage(NewGraphExpansionStage(deps.GraphStore, deepGraphDepth)).
		AddStage(NewTemporalDecayStage(7*24*time.Hour, time.Now())).
		AddStage(NewPageRankStage(deps.GraphStore))

	if deps.Reranker != nil {
		b.AddStage(NewRerankStage(deps.Reranker, deepRerankTopK))
	}

	return b.WithScorer(NewWeightedScorer(standardWeights)).Build()
}

// GraphFirstPreset builds a graph-heavy pipeline: semantic → hydrate → graph → pagerank.
func GraphFirstPreset(deps PresetDeps) (*Pipeline, error) {
	if err := validateCoreDeps(deps); err != nil {
		return nil, fmt.Errorf("graph_first preset: %w", err)
	}
	if err := validateGraphDeps(deps); err != nil {
		return nil, fmt.Errorf("graph_first preset: %w", err)
	}

	return NewPipelineBuilder().
		AddStage(NewSemanticStage(deps.Embedder, deps.VectorStore, deps.Collection, graphFirstSemanticLimit)).
		AddStage(NewHydrateStage(deps.GraphStore)).
		AddStage(NewGraphExpansionStage(deps.GraphStore, graphFirstGraphDepth)).
		AddStage(NewPageRankStage(deps.GraphStore)).
		WithScorer(NewWeightedScorer(graphFirstWeights)).
		Build()
}

// RecencyPreset builds a recency-focused pipeline: semantic → hydrate → temporal.
func RecencyPreset(deps PresetDeps) (*Pipeline, error) {
	if err := validateCoreDeps(deps); err != nil {
		return nil, fmt.Errorf("recency preset: %w", err)
	}

	return NewPipelineBuilder().
		AddStage(NewSemanticStage(deps.Embedder, deps.VectorStore, deps.Collection, recencySemanticLimit)).
		AddStage(NewHydrateStage(deps.GraphStore)).
		AddStage(NewTemporalDecayStage(7*24*time.Hour, time.Now())).
		WithScorer(NewWeightedScorer(recencyWeights)).
		Build()
}
