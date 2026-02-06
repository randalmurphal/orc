package retrieve

import (
	"context"
	"fmt"
	"math"
	"time"
)

// SemanticStage performs vector similarity search.
type SemanticStage struct {
	embedder   Embedder
	searcher   VectorSearcher
	collection string
	limit      int
}

// NewSemanticStage creates a stage that embeds the query and searches the vector store.
func NewSemanticStage(embedder Embedder, searcher VectorSearcher, collection string, limit int) *SemanticStage {
	return &SemanticStage{
		embedder:   embedder,
		searcher:   searcher,
		collection: collection,
		limit:      limit,
	}
}

func (s *SemanticStage) Name() string { return "semantic" }

func (s *SemanticStage) Execute(ctx context.Context, query string, _ []ScoredDocument) ([]ScoredDocument, error) {
	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("embed returned no vectors")
	}

	results, err := s.searcher.Search(ctx, s.collection, vectors[0], s.limit)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	var candidates []ScoredDocument
	for _, r := range results {
		candidates = append(candidates, ScoredDocument{
			Document: Document{
				ID:      r.ID,
				Metadata: r.Payload,
			},
			Signals: map[string]float64{"semantic": float64(r.Score)},
		})
	}
	return candidates, nil
}

// HydrateStage loads full document content from a graph store.
type HydrateStage struct {
	graph GraphQuerier
}

// NewHydrateStage creates a stage that loads full content for candidates.
func NewHydrateStage(graph GraphQuerier) *HydrateStage {
	return &HydrateStage{graph: graph}
}

func (s *HydrateStage) Name() string { return "hydrate" }

func (s *HydrateStage) Execute(ctx context.Context, _ string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	if s.graph == nil {
		return candidates, nil
	}
	var result []ScoredDocument
	for _, c := range candidates {
		if c.Content != "" {
			// Already hydrated
			result = append(result, c)
			continue
		}
		doc, err := s.graph.FetchDocument(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("fetch document %s: %w", c.ID, err)
		}
		if doc == nil {
			// Missing document, skip
			continue
		}
		c.Content = doc.Content
		c.Summary = doc.Summary
		c.FilePath = doc.FilePath
		c.Tokens = doc.Tokens
		c.UpdatedAt = doc.UpdatedAt
		if c.Document.Metadata == nil {
			c.Document.Metadata = doc.Metadata
		}
		result = append(result, c)
	}
	return result, nil
}

// GraphExpansionStage adds graph-connected documents with depth decay.
type GraphExpansionStage struct {
	graph    GraphQuerier
	maxDepth int
}

// NewGraphExpansionStage creates a stage that adds related documents from the graph.
func NewGraphExpansionStage(graph GraphQuerier, maxDepth int) *GraphExpansionStage {
	return &GraphExpansionStage{graph: graph, maxDepth: maxDepth}
}

func (s *GraphExpansionStage) Name() string { return "graph_expansion" }

func (s *GraphExpansionStage) Execute(ctx context.Context, _ string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	seen := make(map[string]int) // id → index in result
	var result []ScoredDocument

	// Add originals with depth-0 graph signal
	for _, c := range candidates {
		if c.Signals == nil {
			c.Signals = make(map[string]float64)
		}
		c.Signals["graph"] = 1.0 // depth 0 → 1/(1+0) = 1.0
		seen[c.ID] = len(result)
		result = append(result, c)
	}

	// Expand each original candidate
	for _, c := range candidates {
		related, err := s.graph.FindRelated(ctx, c.ID, s.maxDepth)
		if err != nil {
			return nil, fmt.Errorf("find related for %s: %w", c.ID, err)
		}
		for _, rel := range related {
			graphSignal := 1.0 / (1.0 + float64(rel.Depth))
			if idx, exists := seen[rel.Doc.ID]; exists {
				// Merge graph signal (keep higher)
				if graphSignal > result[idx].Signals["graph"] {
					result[idx].Signals["graph"] = graphSignal
				}
				continue
			}
			signals := make(map[string]float64)
			signals["graph"] = graphSignal
			seen[rel.Doc.ID] = len(result)
			result = append(result, ScoredDocument{
				Document: rel.Doc,
				Signals:  signals,
			})
		}
	}

	return result, nil
}

// TemporalDecayStage scores documents by recency using exponential decay.
type TemporalDecayStage struct {
	halfLife time.Duration
	now      time.Time
}

const temporalMinFloor = 0.01
const temporalDefaultScore = 0.5

// NewTemporalDecayStage creates a stage that applies temporal decay scoring.
func NewTemporalDecayStage(halfLife time.Duration, now time.Time) *TemporalDecayStage {
	return &TemporalDecayStage{halfLife: halfLife, now: now}
}

func (s *TemporalDecayStage) Name() string { return "temporal_decay" }

func (s *TemporalDecayStage) Execute(_ context.Context, _ string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	for i := range candidates {
		if candidates[i].Signals == nil {
			candidates[i].Signals = make(map[string]float64)
		}

		if candidates[i].UpdatedAt.IsZero() {
			candidates[i].Signals["temporal"] = temporalDefaultScore
			continue
		}

		age := s.now.Sub(candidates[i].UpdatedAt)
		if age < 0 {
			age = 0
		}

		// exp(-ln(2) * age / halfLife)
		score := math.Exp(-math.Ln2 * float64(age) / float64(s.halfLife))
		if score < temporalMinFloor {
			score = temporalMinFloor
		}
		candidates[i].Signals["temporal"] = score
	}
	return candidates, nil
}

// PageRankStage adds graph centrality as a soft signal.
type PageRankStage struct {
	graph GraphQuerier
}

const pagerankDefault = 0.1

// NewPageRankStage creates a stage that assigns PageRank scores from graph centrality.
func NewPageRankStage(graph GraphQuerier) *PageRankStage {
	return &PageRankStage{graph: graph}
}

func (s *PageRankStage) Name() string { return "pagerank" }

func (s *PageRankStage) Execute(ctx context.Context, _ string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}

	centrality, err := s.graph.GetCentrality(ctx, ids)
	if err != nil {
		// Soft signal: graph failure continues with defaults
		centrality = nil
	}

	for i := range candidates {
		if candidates[i].Signals == nil {
			candidates[i].Signals = make(map[string]float64)
		}
		if centrality != nil {
			if score, ok := centrality[candidates[i].ID]; ok {
				candidates[i].Signals["pagerank"] = score
				continue
			}
		}
		candidates[i].Signals["pagerank"] = pagerankDefault
	}
	return candidates, nil
}

// RerankStage reorders top-K candidates using a reranker.
type RerankStage struct {
	reranker Reranker
	topK     int
}

// NewRerankStage creates a stage that reranks top-K documents with content.
func NewRerankStage(reranker Reranker, topK int) *RerankStage {
	return &RerankStage{reranker: reranker, topK: topK}
}

func (s *RerankStage) Name() string { return "rerank" }

func (s *RerankStage) Execute(ctx context.Context, query string, candidates []ScoredDocument) ([]ScoredDocument, error) {
	// Separate docs with content (eligible for reranking) from those without
	var withContent []ScoredDocument
	var withContentIdx []int
	for i, c := range candidates {
		if c.Content != "" {
			withContent = append(withContent, c)
			withContentIdx = append(withContentIdx, i)
		}
	}

	// Only rerank top-K docs with content
	rerankCount := len(withContent)
	if rerankCount > s.topK {
		rerankCount = s.topK
	}

	if rerankCount == 0 {
		return candidates, nil
	}

	toRerank := withContent[:rerankCount]
	reranked, err := s.reranker.Rerank(ctx, query, toRerank)
	if err != nil {
		return nil, fmt.Errorf("rerank: %w", err)
	}

	// Apply reranked results back to candidates
	for j, r := range reranked {
		idx := withContentIdx[j]
		candidates[idx] = r
	}

	return candidates, nil
}
