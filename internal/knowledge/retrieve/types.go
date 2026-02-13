// Package retrieve implements a multi-signal search pipeline with configurable
// stages, weighted scoring, and preset configurations.
package retrieve

import (
	"context"
	"time"
)

// Document represents a knowledge document with content and metadata.
type Document struct {
	ID        string
	Content   string
	Summary   string
	FilePath  string
	Tokens    int
	UpdatedAt time.Time
	Metadata  map[string]interface{}
}

// ScoredDocument wraps a Document with signal scores from pipeline stages.
type ScoredDocument struct {
	Document
	Signals    map[string]float64
	FinalScore float64
}

// VectorSearchResult holds a single result from vector similarity search.
type VectorSearchResult struct {
	ID      string
	Score   float32
	Payload map[string]interface{}
}

// RelatedDoc represents a graph-connected document with traversal depth.
type RelatedDoc struct {
	Doc   Document
	Depth int
}

// QueryOpts configures a retrieval query.
type QueryOpts struct {
	Preset      string
	MaxTokens   int
	MinScore    float64
	SummaryOnly bool
	Limit       int
}

// PipelineResult holds the output of a pipeline execution.
type PipelineResult struct {
	Documents  []ScoredDocument
	TokensUsed int
}

// Stage processes candidates in a pipeline step.
type Stage interface {
	Name() string
	Execute(ctx context.Context, query string, candidates []ScoredDocument) ([]ScoredDocument, error)
}

// Scorer computes a final score from signal values.
type Scorer interface {
	Score(signals map[string]float64) float64
}

// Embedder converts text to vector embeddings.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Type() string
}

// VectorSearcher searches a vector store by embedding similarity.
type VectorSearcher interface {
	Search(ctx context.Context, collection string, vector []float32, limit int) ([]VectorSearchResult, error)
}

// GraphQuerier provides graph-based document operations.
type GraphQuerier interface {
	FetchDocument(ctx context.Context, id string) (*Document, error)
	FindRelated(ctx context.Context, id string, maxDepth int) ([]RelatedDoc, error)
	GetCentrality(ctx context.Context, ids []string) (map[string]float64, error)
}

// Reranker reorders documents by relevance to a query.
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []ScoredDocument) ([]ScoredDocument, error)
}

// PresetDeps provides dependencies for preset pipeline builders.
type PresetDeps struct {
	Embedder    Embedder
	VectorStore VectorSearcher
	GraphStore  GraphQuerier
	Reranker    Reranker
	Collection  string
}

// EnrichmentProvider exposes graph-derived data for brief enrichment.
type EnrichmentProvider interface {
	GetPatterns(ctx context.Context) ([]Document, error)
	GetHotFiles(ctx context.Context) ([]Document, error)
	GetKnownIssues(ctx context.Context) ([]Document, error)
}
