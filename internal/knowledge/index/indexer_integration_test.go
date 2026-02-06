package index

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// --- Integration tests: Verify indexer wires to each code sub-component ---
//
// These tests complement the unit tests in code/ — they verify that the
// indexer (the production CALLER of the code package) actually calls each
// sub-component. The litmus test: if you remove the call to the secret
// detector from the indexer, does this test fail? YES.
//
// Reuses mockStores and helpers from indexer_test.go (same package).

// Integration: Indexer calls secret detector during pipeline.
// Proves wiring: indexer.go imports and calls code.SecretDetector.
// Fails if: secret detection call is removed from the indexer pipeline.
func TestIndexerIntegration_SecretRedactionInPipeline(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "config.go", `package config

// AWSKey is an access key.
const AWSKey = "AKIAIOSFODNN7EXAMPLE"

func GetKey() string {
	return AWSKey
}
`)
	writeTestFile(t, root, "main.go", `package main

func main() {}
`)

	mock := &mockStores{}
	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Chunks stored in vector store must have secrets redacted.
	// If the secret detector isn't wired, the raw key appears in payloads.
	for _, v := range mock.upsertedVectors {
		content, ok := v.Payload["content"].(string)
		if !ok {
			continue
		}
		if strings.Contains(content, "AKIAIOSFODNN7EXAMPLE") {
			t.Error("vector payload contains unredacted AWS key — secret detector not wired into pipeline")
		}
	}
}

// Integration: Indexer calls pattern detector during pipeline.
// Proves wiring: indexer.go imports and calls code.PatternDetector.
// Fails if: pattern detection call is removed from the indexer pipeline.
func TestIndexerIntegration_PatternDetectionInPipeline(t *testing.T) {
	root := t.TempDir()
	// Create 6 structurally similar Go files to trigger pattern detection.
	// Each has a struct + constructor + 2 methods — identical AST shape.
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("handler_%d.go", i)
		content := fmt.Sprintf(`package handlers

// Handler%d processes requests of type %d.
type Handler%d struct {
	name string
}

// NewHandler%d creates a new Handler%d.
func NewHandler%d(name string) *Handler%d {
	return &Handler%d{name: name}
}

// Process handles the request.
func (h *Handler%d) Process() error {
	return nil
}

// Validate checks the request.
func (h *Handler%d) Validate() error {
	return nil
}
`, i, i, i, i, i, i, i, i, i, i)
		writeTestFile(t, root, name, content)
	}

	mock := &mockStores{}
	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.PatternsFound == 0 {
		t.Error("PatternsFound = 0 — pattern detector not wired into pipeline")
	}

	// Graph must contain Pattern nodes.
	hasPattern := false
	for _, n := range mock.createdNodes {
		for _, l := range n.Labels {
			if l == "Pattern" {
				hasPattern = true
				break
			}
		}
		if hasPattern {
			break
		}
	}
	if !hasPattern {
		t.Error("graph has no Pattern nodes — pattern detector not wired into pipeline")
	}
}

// Integration: Indexer calls relationship extractor during pipeline.
// Proves wiring: indexer.go imports and calls code.ExtractRelationships.
// Fails if: relationship extraction call is removed from the indexer pipeline.
func TestIndexerIntegration_RelationshipExtractionInPipeline(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

	mock := &mockStores{}
	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Graph must have IMPORTS relationships (main.go importing "fmt").
	hasImport := false
	for _, r := range mock.createdRels {
		if r.relType == "IMPORTS" {
			hasImport = true
			break
		}
	}
	if !hasImport {
		t.Error("graph has no IMPORTS relationships — relationship extractor not wired into pipeline")
	}
}

// Integration: Indexer wires tree-sitter parser for multi-language support.
// Proves wiring: indexer.go routes Python/JS/TS files to TreeSitterParser.
// Fails if: tree-sitter parser is not wired for non-Go files.
func TestIndexerIntegration_MultiLanguageParsing(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "main.go", `package main

func main() {}
`)
	writeTestFile(t, root, "app.py", `def greet(name):
    """Say hello."""
    print(f"Hello, {name}")
`)
	writeTestFile(t, root, "util.js", `function add(a, b) {
    return a + b;
}
`)

	mock := &mockStores{}
	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.FilesProcessed < 3 {
		t.Errorf("FilesProcessed = %d, want >= 3 — multi-language walker not wired", result.FilesProcessed)
	}

	// Graph should have Symbol nodes from each language.
	languages := make(map[string]bool)
	for _, n := range mock.createdNodes {
		for _, l := range n.Labels {
			if l == "Symbol" {
				if lang, ok := n.Properties["language"].(string); ok {
					languages[lang] = true
				}
			}
		}
	}
	for _, lang := range []string{"go", "python", "javascript"} {
		if !languages[lang] {
			t.Errorf("no Symbol nodes for %s — parser not wired for this language", lang)
		}
	}
}

// Integration: Indexer stores FOLLOWS_PATTERN relationships linking files to patterns.
// Proves wiring: indexer.go writes pattern detection results as graph relationships.
// Fails if: pattern results aren't persisted to graph after detection.
func TestIndexerIntegration_PatternRelationshipsInGraph(t *testing.T) {
	root := t.TempDir()
	// Create 5+ structurally similar Go files to trigger pattern detection.
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("worker_%d.go", i)
		content := fmt.Sprintf(`package workers

type Worker%d struct{}

func NewWorker%d() *Worker%d { return &Worker%d{} }

func (w *Worker%d) Run() error { return nil }

func (w *Worker%d) Stop() error { return nil }
`, i, i, i, i, i, i)
		writeTestFile(t, root, name, content)
	}

	mock := &mockStores{}
	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Graph should have FOLLOWS_PATTERN relationships.
	hasFollows := false
	for _, r := range mock.createdRels {
		if r.relType == "FOLLOWS_PATTERN" {
			hasFollows = true
			break
		}
	}
	if !hasFollows {
		t.Error("graph has no FOLLOWS_PATTERN relationships — pattern-to-graph wiring missing")
	}
}
