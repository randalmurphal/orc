// Package index provides the code indexing pipeline for the knowledge layer.
package index

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/knowledge/index/code"
	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// GraphStorer abstracts graph store operations needed by the indexer.
type GraphStorer interface {
	CreateNode(ctx context.Context, node store.Node) (string, error)
	QueryNodes(ctx context.Context, label string, props map[string]interface{}) ([]store.Node, error)
	CreateRelationship(ctx context.Context, fromID, toID, relType string, props map[string]interface{}) error
	ExecuteCypher(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error)
	DeleteNodesByProperty(ctx context.Context, label, prop, value string) error
}

// VectorStorer abstracts vector store operations needed by the indexer.
type VectorStorer interface {
	Upsert(ctx context.Context, collection string, vectors []store.Vector) error
	CreateCollection(ctx context.Context, name string, dimension int) error
}

// Embedder abstracts embedding operations needed by the indexer.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Type() string
}

// IndexOptions configures an indexing run.
type IndexOptions struct {
	Incremental bool
}

// IndexResult reports what the indexer did.
type IndexResult struct {
	FilesProcessed    int
	ChunksStored      int
	PatternsFound     int
	ErrorsEncountered []error
}

// Indexer orchestrates the code indexing pipeline.
type Indexer struct {
	graph  GraphStorer
	vector VectorStorer
	embed  Embedder
	mu     sync.Mutex
	busy   bool
}

// IndexerOption configures an Indexer.
type IndexerOption func(*Indexer)

// NewIndexer creates a new indexer.
func NewIndexer(opts ...IndexerOption) *Indexer {
	idx := &Indexer{}
	for _, opt := range opts {
		opt(idx)
	}
	return idx
}

// WithGraphStore sets the graph store.
func WithGraphStore(store GraphStorer) IndexerOption {
	return func(idx *Indexer) {
		idx.graph = store
	}
}

// WithVectorStore sets the vector store.
func WithVectorStore(store VectorStorer) IndexerOption {
	return func(idx *Indexer) {
		idx.vector = store
	}
}

// WithEmbedder sets the embedder.
func WithEmbedder(emb Embedder) IndexerOption {
	return func(idx *Indexer) {
		idx.embed = emb
	}
}

// Index runs the full indexing pipeline on the given project root.
func (idx *Indexer) Index(ctx context.Context, root string, opts IndexOptions) (*IndexResult, error) {
	// Concurrency guard
	idx.mu.Lock()
	if idx.busy {
		idx.mu.Unlock()
		return nil, fmt.Errorf("indexing already in progress")
	}
	idx.busy = true
	idx.mu.Unlock()
	defer func() {
		idx.mu.Lock()
		idx.busy = false
		idx.mu.Unlock()
	}()

	result := &IndexResult{}

	// Step 1: Walk files
	walker := code.NewWalker()
	files, err := walker.Walk(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("walk files: %w", err)
	}

	if len(files) == 0 {
		return result, nil
	}

	// Step 2: Handle incremental mode
	var filesToProcess []code.FileInfo
	if opts.Incremental {
		filesToProcess, err = idx.filterChangedFiles(ctx, files, root)
		if err != nil {
			// Fallback to full index on hash query failure
			filesToProcess = files
		}
	} else {
		filesToProcess = files
	}

	if len(filesToProcess) == 0 {
		return result, nil
	}

	result.FilesProcessed = len(filesToProcess)

	// Step 3: Parse, chunk, detect secrets for each file
	allSymbols := make(map[string][]code.Symbol)
	fileContents := make(map[string]string)
	var allChunks []code.Chunk

	goParser := code.NewGoParser()
	tsParser := code.NewTreeSitterParser()
	chunker := code.NewChunker()
	secretDetector := code.NewSecretDetector()

	for _, f := range filesToProcess {
		src, readErr := os.ReadFile(f.Path)
		if readErr != nil {
			result.ErrorsEncountered = append(result.ErrorsEncountered, readErr)
			continue
		}

		content := string(src)
		fileContents[f.Path] = content

		// Parse
		var symbols []code.Symbol
		var parseErr error
		switch f.Language {
		case "go":
			symbols, parseErr = goParser.Parse(ctx, f.Path, src)
		case "python", "javascript", "typescript":
			symbols, parseErr = tsParser.Parse(ctx, f.Path, src)
		}
		if parseErr != nil {
			result.ErrorsEncountered = append(result.ErrorsEncountered, parseErr)
		}

		allSymbols[f.Path] = symbols

		// Chunk
		singleFileContent := map[string]string{f.Path: content}
		chunks, chunkErr := chunker.Chunk(symbols, singleFileContent)
		if chunkErr != nil {
			result.ErrorsEncountered = append(result.ErrorsEncountered, chunkErr)
			continue
		}

		// Detect and redact secrets
		findings := secretDetector.Detect(content)
		if secretDetector.HasSecrets(findings) {
			for i := range chunks {
				chunks[i].Content = secretDetector.Redact(chunks[i].Content, findings)
			}
		}

		allChunks = append(allChunks, chunks...)
	}

	// Step 4: Extract relationships
	flatSymbols := flattenSymbols(allSymbols)
	relationships, relErr := code.ExtractRelationships(flatSymbols, fileContents)
	if relErr != nil {
		result.ErrorsEncountered = append(result.ErrorsEncountered, relErr)
	}

	// Step 5: Detect patterns
	patternDetector := code.NewPatternDetector()
	patterns, patErr := patternDetector.Detect(allSymbols)
	if patErr != nil {
		result.ErrorsEncountered = append(result.ErrorsEncountered, patErr)
	}
	result.PatternsFound = len(patterns)

	// Step 6: Store in graph — File nodes, Symbol nodes, relationships
	fileNodeIDs := make(map[string]string)  // filePath -> nodeID
	symbolNodeIDs := make(map[string]string) // filePath:symbolName -> nodeID

	for _, f := range filesToProcess {
		src, _ := os.ReadFile(f.Path)
		hash := fmt.Sprintf("%x", sha256.Sum256(src))

		fileID, createErr := idx.graph.CreateNode(ctx, store.Node{
			Labels: []string{"File"},
			Properties: map[string]interface{}{
				"path":         f.Path,
				"hash":         hash,
				"last_indexed": time.Now().UTC().Format(time.RFC3339),
			},
		})
		if createErr != nil {
			return nil, fmt.Errorf("create File node for %s: %w", f.Path, createErr)
		}
		fileNodeIDs[f.Path] = fileID
	}

	for filePath, symbols := range allSymbols {
		for _, sym := range symbols {
			symID, createErr := idx.graph.CreateNode(ctx, store.Node{
				Labels: []string{"Symbol"},
				Properties: map[string]interface{}{
					"name":       sym.Name,
					"kind":       string(sym.Kind),
					"file_path":  sym.FilePath,
					"start_line": sym.StartLine,
					"end_line":   sym.EndLine,
					"language":   sym.Language,
					"signature":  sym.Signature,
					"docstring":  sym.Docstring,
				},
			})
			if createErr != nil {
				return nil, fmt.Errorf("create Symbol node %s: %w", sym.Name, createErr)
			}
			key := filePath + ":" + sym.Name
			symbolNodeIDs[key] = symID

			// CONTAINS relationship (File -> Symbol)
			if fileID, ok := fileNodeIDs[filePath]; ok {
				if relCreateErr := idx.graph.CreateRelationship(ctx, fileID, symID, "CONTAINS", nil); relCreateErr != nil {
					return nil, fmt.Errorf("create CONTAINS relationship: %w", relCreateErr)
				}
			}
		}
	}

	// Store code relationships (IMPORTS, CALLS, etc.)
	for _, rel := range relationships {
		relType := "IMPORTS"
		switch rel.Kind {
		case code.RelCall:
			relType = "CALLS"
		case code.RelExtends:
			relType = "EXTENDS"
		case code.RelImplements:
			relType = "IMPLEMENTS"
		}
		// Find source and target node IDs
		sourceKey := rel.SourceFile + ":" + rel.SourceName
		sourceID := symbolNodeIDs[sourceKey]
		if sourceID == "" {
			sourceID = fileNodeIDs[rel.SourceFile]
		}
		targetKey := rel.TargetFile + ":" + rel.TargetName
		targetID := symbolNodeIDs[targetKey]
		if targetID == "" {
			targetID = fileNodeIDs[rel.TargetFile]
		}

		if sourceID != "" && targetID != "" {
			if relCreateErr := idx.graph.CreateRelationship(ctx, sourceID, targetID, relType, nil); relCreateErr != nil {
				return nil, fmt.Errorf("create %s relationship: %w", relType, relCreateErr)
			}
		} else if sourceID != "" {
			// Create relationship with target as external reference
			extID, extErr := idx.graph.CreateNode(ctx, store.Node{
				Labels: []string{"Module"},
				Properties: map[string]interface{}{
					"path": rel.TargetName,
				},
			})
			if extErr != nil {
				result.ErrorsEncountered = append(result.ErrorsEncountered,
					fmt.Errorf("create Module node for %s: %w", rel.TargetName, extErr))
			} else if extID != "" {
				if relErr := idx.graph.CreateRelationship(ctx, sourceID, extID, relType, nil); relErr != nil {
					result.ErrorsEncountered = append(result.ErrorsEncountered,
						fmt.Errorf("create %s relationship to external %s: %w", relType, rel.TargetName, relErr))
				}
			}
		}
	}

	// Store pattern nodes and FOLLOWS_PATTERN relationships
	for _, pat := range patterns {
		patID, patCreateErr := idx.graph.CreateNode(ctx, store.Node{
			Labels: []string{"Pattern"},
			Properties: map[string]interface{}{
				"name":           pat.Name,
				"canonical_file": pat.CanonicalFile,
				"member_count":   pat.MemberCount,
			},
		})
		if patCreateErr != nil {
			return nil, fmt.Errorf("create Pattern node: %w", patCreateErr)
		}

		for _, member := range pat.Members {
			if fileID, ok := fileNodeIDs[member]; ok {
				if relErr := idx.graph.CreateRelationship(ctx, fileID, patID, "FOLLOWS_PATTERN", nil); relErr != nil {
					result.ErrorsEncountered = append(result.ErrorsEncountered,
						fmt.Errorf("create FOLLOWS_PATTERN relationship for %s: %w", member, relErr))
				}
			}
		}
	}

	// Step 7: Embed chunks (batched)
	if len(allChunks) > 0 {
		texts := make([]string, len(allChunks))
		for i, ch := range allChunks {
			texts[i] = ch.Content
		}

		embeddings, embedErr := idx.embed.Embed(ctx, texts)
		if embedErr != nil {
			return nil, fmt.Errorf("embed chunks: %w", embedErr)
		}

		// Step 8: Upsert vectors (batched)
		vectors := make([]store.Vector, len(allChunks))
		for i, ch := range allChunks {
			vectors[i] = store.Vector{
				ID:     fmt.Sprintf("%s:%s:%d", ch.FilePath, ch.Symbol, ch.StartLine),
				Values: embeddings[i],
				Payload: map[string]interface{}{
					"content":    ch.Content,
					"file_path":  ch.FilePath,
					"symbol":     ch.Symbol,
					"kind":       string(ch.Kind),
					"start_line": ch.StartLine,
					"end_line":   ch.EndLine,
				},
			}
		}

		if upsertErr := idx.vector.Upsert(ctx, "code_chunks", vectors); upsertErr != nil {
			return nil, fmt.Errorf("upsert vectors: %w", upsertErr)
		}

		result.ChunksStored = len(vectors)
	}

	return result, nil
}

// filterChangedFiles returns only files whose SHA-256 hash differs from stored.
func (idx *Indexer) filterChangedFiles(ctx context.Context, files []code.FileInfo, root string) ([]code.FileInfo, error) {
	// Query stored hashes from graph
	results, err := idx.graph.ExecuteCypher(ctx,
		"MATCH (f:File) RETURN f.path AS path, f.hash AS hash", nil)
	if err != nil {
		return nil, fmt.Errorf("query file hashes: %w", err)
	}

	storedHashes := make(map[string]string)
	for _, row := range results {
		path, _ := row["path"].(string)
		hash, _ := row["hash"].(string)
		if path != "" {
			storedHashes[path] = hash
		}
	}

	// Check for deleted files (in stored but not on disk)
	currentPaths := make(map[string]bool)
	for _, f := range files {
		currentPaths[f.Path] = true
	}
	for storedPath := range storedHashes {
		if !currentPaths[storedPath] {
			// File was deleted — remove its nodes using relative path
			delPath := storedPath
			if rel, relErr := filepath.Rel(root, storedPath); relErr == nil {
				delPath = rel
			}
			// Best-effort cleanup: deleted file nodes are non-critical and will
			// be overwritten on the next full index if this fails.
			_ = idx.graph.DeleteNodesByProperty(ctx, "File", "path", delPath)
		}
	}

	// Filter to only changed files
	var changed []code.FileInfo
	for _, f := range files {
		src, readErr := os.ReadFile(f.Path)
		if readErr != nil {
			changed = append(changed, f) // can't read? process anyway
			continue
		}
		currentHash := fmt.Sprintf("%x", sha256.Sum256(src))
		if storedHash, exists := storedHashes[f.Path]; exists && storedHash == currentHash {
			continue // unchanged
		}
		changed = append(changed, f)
	}

	return changed, nil
}

func flattenSymbols(allSymbols map[string][]code.Symbol) []code.Symbol {
	var flat []code.Symbol
	for _, syms := range allSymbols {
		flat = append(flat, syms...)
	}
	return flat
}
