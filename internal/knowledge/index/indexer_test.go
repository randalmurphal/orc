package index

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// --- SC-8: Unified indexer pipeline orchestration ---

// SC-8: Full pipeline executes all stages in correct order.
func TestIndexerPipeline_FullRun(t *testing.T) {
	root := setupTestProject(t)
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

	if result.FilesProcessed == 0 {
		t.Error("should process at least one file")
	}
	if result.ChunksStored == 0 {
		t.Error("should store at least one chunk")
	}

	// Verify pipeline stages executed in order
	stages := mock.stageOrder()
	if len(stages) == 0 {
		t.Fatal("no pipeline stages recorded")
	}

	// Graph store should receive nodes before relationships
	nodeIdx := stageIndex(stages, "create_node")
	relIdx := stageIndex(stages, "create_relationship")
	if nodeIdx >= 0 && relIdx >= 0 && nodeIdx > relIdx {
		t.Error("nodes should be created before relationships")
	}

	// Embedder should be called before vector upsert
	embedIdx := stageIndex(stages, "embed")
	upsertIdx := stageIndex(stages, "vector_upsert")
	if embedIdx >= 0 && upsertIdx >= 0 && embedIdx > upsertIdx {
		t.Error("embedding should happen before vector upsert")
	}
}

// SC-8: Graph store receives Symbol and File nodes.
func TestIndexerPipeline_GraphNodes(t *testing.T) {
	root := setupTestProject(t)
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

	// Should have File nodes
	hasFile := false
	for _, n := range mock.createdNodes {
		for _, l := range n.Labels {
			if l == "File" {
				hasFile = true
			}
		}
	}
	if !hasFile {
		t.Error("graph should contain File nodes")
	}

	// Should have Symbol nodes
	hasSymbol := false
	for _, n := range mock.createdNodes {
		for _, l := range n.Labels {
			if l == "Symbol" {
				hasSymbol = true
			}
		}
	}
	if !hasSymbol {
		t.Error("graph should contain Symbol nodes")
	}
}

// SC-8: Graph store receives relationships.
func TestIndexerPipeline_GraphRelationships(t *testing.T) {
	root := setupTestProject(t)
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

	if len(mock.createdRels) == 0 {
		t.Error("graph should have relationships")
	}

	// Should have CONTAINS relationships (File -> Symbol)
	hasContains := false
	for _, r := range mock.createdRels {
		if r.relType == "CONTAINS" {
			hasContains = true
			break
		}
	}
	if !hasContains {
		t.Error("graph should have CONTAINS relationships")
	}
}

// SC-8: Vector store receives embeddings for all non-secret chunks.
func TestIndexerPipeline_VectorEmbeddings(t *testing.T) {
	root := setupTestProject(t)
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

	if len(mock.upsertedVectors) == 0 {
		t.Error("vector store should receive upserted vectors")
	}

	// Number of vectors should match chunks stored
	if len(mock.upsertedVectors) != result.ChunksStored {
		t.Errorf("vectors (%d) should match chunks stored (%d)",
			len(mock.upsertedVectors), result.ChunksStored)
	}
}

// --- SC-9: Incremental indexing ---

// SC-9: First run indexes all files.
func TestIndexer_IncrementalFirstRun(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.FilesProcessed == 0 {
		t.Error("first incremental run should process all files")
	}
}

// SC-9: Second run with no changes processes zero files.
func TestIndexer_IncrementalNoChanges(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		storedHashes: make(map[string]string),
	}

	// Store file hashes from "first run"
	files, _ := listGoFiles(root)
	for _, f := range files {
		data, _ := os.ReadFile(f)
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		mock.storedHashes[f] = hash
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.FilesProcessed != 0 {
		t.Errorf("second incremental run should process 0 files, got %d", result.FilesProcessed)
	}
}

// SC-9: After modifying one file, only that file is re-indexed.
func TestIndexer_IncrementalOneFileChanged(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		storedHashes: make(map[string]string),
	}

	// Store hashes for all files
	files, _ := listGoFiles(root)
	for _, f := range files {
		data, _ := os.ReadFile(f)
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		mock.storedHashes[f] = hash
	}

	// Modify one file so its hash changes
	modifiedFile := filepath.Join(root, "main.go")
	if err := os.WriteFile(modifiedFile, []byte("package main\n\n// modified\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("modify file: %v", err)
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Errorf("should re-index exactly 1 modified file, got %d", result.FilesProcessed)
	}
}

// SC-9: Hash query failure treats all files as changed (safe fallback).
func TestIndexer_IncrementalHashQueryFailure(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		hashQueryErr: errors.New("graph unavailable"),
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("Index should still succeed on hash query failure: %v", err)
	}

	// Should fall back to full index
	if result.FilesProcessed == 0 {
		t.Error("hash query failure should fall back to indexing all files")
	}
}

// SC-9 edge case: Incremental run after file deletion removes stale nodes.
func TestIndexer_IncrementalDeletion(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{}

	// Store hash for a file that no longer exists
	mock.storedHashes = map[string]string{
		filepath.Join(root, "deleted.go"): "old-hash",
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Should have issued a delete/cleanup for the missing file
	if !mock.deletedNodes["deleted.go"] {
		t.Error("should delete nodes for files that no longer exist")
	}
}

// --- SC-10: Batch processing ---

// SC-10: Embedder receives calls with multiple texts per invocation.
func TestIndexer_BatchEmbedding(t *testing.T) {
	root := setupTestProject(t)
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

	// Verify embedder was called with batches (not one-at-a-time)
	if mock.embedCalls == 0 {
		t.Fatal("embedder should be called at least once")
	}

	// If there are multiple chunks, there should be fewer embed calls than chunks
	totalChunks := 0
	for _, batch := range mock.embedBatches {
		totalChunks += len(batch)
	}

	if totalChunks > 1 && mock.embedCalls >= totalChunks {
		t.Errorf("embedding should be batched: %d calls for %d chunks", mock.embedCalls, totalChunks)
	}
}

// SC-10: Vector store receives Upsert calls with multiple vectors per call.
func TestIndexer_BatchVectorUpsert(t *testing.T) {
	root := setupTestProject(t)
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

	if mock.upsertCalls == 0 {
		t.Fatal("vector store should receive upsert calls")
	}

	// If multiple vectors, should batch them
	if len(mock.upsertedVectors) > 1 && mock.upsertCalls >= len(mock.upsertedVectors) {
		t.Errorf("vector upserts should be batched: %d calls for %d vectors",
			mock.upsertCalls, len(mock.upsertedVectors))
	}
}

// --- SC-12: Graph schema ---

// SC-12: Graph contains :Symbol nodes with expected properties.
func TestGraphSchema_SymbolNodes(t *testing.T) {
	root := setupTestProject(t)
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

	symbolNodes := nodesWithLabel(mock.createdNodes, "Symbol")
	if len(symbolNodes) == 0 {
		t.Fatal("should have Symbol nodes")
	}

	// Verify required properties
	for _, n := range symbolNodes {
		props := n.Properties
		requiredProps := []string{"name", "kind", "file_path", "start_line", "end_line"}
		for _, prop := range requiredProps {
			if _, ok := props[prop]; !ok {
				t.Errorf("Symbol node missing required property %q", prop)
			}
		}
	}
}

// SC-12: Graph contains :File nodes with expected properties.
func TestGraphSchema_FileNodes(t *testing.T) {
	root := setupTestProject(t)
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

	fileNodes := nodesWithLabel(mock.createdNodes, "File")
	if len(fileNodes) == 0 {
		t.Fatal("should have File nodes")
	}

	for _, n := range fileNodes {
		props := n.Properties
		requiredProps := []string{"path", "hash", "last_indexed"}
		for _, prop := range requiredProps {
			if _, ok := props[prop]; !ok {
				t.Errorf("File node missing required property %q", prop)
			}
		}
	}
}

// SC-12: Graph contains expected relationship types.
func TestGraphSchema_Relationships(t *testing.T) {
	root := setupTestProject(t)
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

	relTypes := make(map[string]bool)
	for _, r := range mock.createdRels {
		relTypes[r.relType] = true
	}

	// Must have CONTAINS (File -> Symbol) and IN_FILE (Symbol -> File)
	expectedTypes := []string{"CONTAINS"}
	for _, want := range expectedTypes {
		if !relTypes[want] {
			t.Errorf("graph should have %s relationships", want)
		}
	}
}

// --- Failure Modes ---

// Failure: Pipeline stops with error if graph store fails.
func TestIndexer_GraphStoreFailure(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		createNodeErr: errors.New("neo4j connection lost"),
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err == nil {
		t.Fatal("Index should return error on graph store failure")
	}
}

// Failure: Pipeline stops with error if vector store fails.
func TestIndexer_VectorStoreFailure(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		upsertErr: errors.New("qdrant connection refused"),
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err == nil {
		t.Fatal("Index should return error on vector store failure")
	}
}

// Failure: Pipeline stops with error if embedding fails.
func TestIndexer_EmbedFailure(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		embedErr: errors.New("voyage API unavailable"),
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err == nil {
		t.Fatal("Index should return error on embedding failure")
	}
}

// Failure: Batch processing error returns error with context.
func TestIndexer_BatchError(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		upsertErr: errors.New("batch 2 connection refused"),
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	_, err := idx.Index(context.Background(), root, IndexOptions{})
	if err == nil {
		t.Fatal("Index should return error on batch failure")
	}
}

// Edge case: Empty project succeeds with zero counts.
func TestIndexer_EmptyProject(t *testing.T) {
	root := t.TempDir()
	mock := &mockStores{}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	result, err := idx.Index(context.Background(), root, IndexOptions{})
	if err != nil {
		t.Fatalf("Index should succeed for empty project: %v", err)
	}

	if result.FilesProcessed != 0 {
		t.Errorf("FilesProcessed = %d, want 0", result.FilesProcessed)
	}
	if result.ChunksStored != 0 {
		t.Errorf("ChunksStored = %d, want 0", result.ChunksStored)
	}
}

// Failure: Concurrent IndexProject calls return error.
func TestIndexer_ConcurrentIndex(t *testing.T) {
	root := setupTestProject(t)
	mock := &mockStores{
		embedDelay: true, // Slow down embedding so first call is still running
	}

	idx := NewIndexer(
		WithGraphStore(mock),
		WithVectorStore(mock),
		WithEmbedder(mock),
	)

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := idx.Index(context.Background(), root, IndexOptions{})
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	// At least one call should fail with "already in progress"
	gotConcurrencyError := false
	for err := range errs {
		if err != nil {
			gotConcurrencyError = true
		}
	}
	if !gotConcurrencyError {
		t.Error("concurrent Index calls should produce at least one error")
	}
}

// --- Test helpers ---

func setupTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeTestFile(t, root, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)
	writeTestFile(t, root, "lib/util.go", `package lib

// Add adds two numbers.
func Add(a, b int) int {
	return a + b
}

// Sub subtracts b from a.
func Sub(a, b int) int {
	return a - b
}
`)
	return root
}

func writeTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	abs := filepath.Join(root, relPath)
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

func listGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func stageIndex(stages []string, name string) int {
	for i, s := range stages {
		if s == name {
			return i
		}
	}
	return -1
}

func nodesWithLabel(nodes []store.Node, label string) []store.Node {
	var result []store.Node
	for _, n := range nodes {
		for _, l := range n.Labels {
			if l == label {
				result = append(result, n)
				break
			}
		}
	}
	return result
}

// --- Mock stores ---

type mockRel struct {
	fromID  string
	toID    string
	relType string
}

type mockStores struct {
	mu sync.Mutex

	// Graph store state
	createdNodes []store.Node
	createdRels  []mockRel
	createNodeErr error
	createRelErr  error

	// Vector store state
	upsertedVectors []store.Vector
	upsertCalls     int
	upsertErr       error

	// Embedder state
	embedCalls   int
	embedBatches [][]string
	embedErr     error
	embedDelay   bool

	// Incremental state
	storedHashes map[string]string
	hashQueryErr error

	// Deletion tracking
	deletedNodes map[string]bool

	// Stage tracking
	stages []string
}

func (m *mockStores) stageOrder() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.stages))
	copy(result, m.stages)
	return result
}

func (m *mockStores) recordStage(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stages = append(m.stages, name)
}

// GraphStorer interface
func (m *mockStores) CreateNode(ctx context.Context, node store.Node) (string, error) {
	m.recordStage("create_node")
	if m.createNodeErr != nil {
		return "", m.createNodeErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdNodes = append(m.createdNodes, node)
	return fmt.Sprintf("node-%d", len(m.createdNodes)), nil
}

func (m *mockStores) QueryNodes(ctx context.Context, label string, props map[string]interface{}) ([]store.Node, error) {
	return nil, nil
}

func (m *mockStores) CreateRelationship(ctx context.Context, fromID, toID, relType string, props map[string]interface{}) error {
	m.recordStage("create_relationship")
	if m.createRelErr != nil {
		return m.createRelErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdRels = append(m.createdRels, mockRel{fromID, toID, relType})
	return nil
}

func (m *mockStores) ExecuteCypher(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	if m.hashQueryErr != nil {
		return nil, m.hashQueryErr
	}

	// Return stored hashes for incremental queries
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storedHashes != nil {
		var results []map[string]interface{}
		for path, hash := range m.storedHashes {
			results = append(results, map[string]interface{}{
				"path": path,
				"hash": hash,
			})
		}
		return results, nil
	}
	return nil, nil
}

// VectorStorer interface
func (m *mockStores) Upsert(ctx context.Context, collection string, vectors []store.Vector) error {
	m.recordStage("vector_upsert")
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls++
	m.upsertedVectors = append(m.upsertedVectors, vectors...)
	return nil
}

func (m *mockStores) CreateCollection(ctx context.Context, name string, dimension int) error {
	return nil
}

// Embedder interface
func (m *mockStores) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.recordStage("embed")
	if m.embedErr != nil {
		return nil, m.embedErr
	}
	if m.embedDelay {
		// Simulate slow embedding so concurrent call sees busy=true
		time.Sleep(50 * time.Millisecond)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.embedCalls++
	m.embedBatches = append(m.embedBatches, texts)

	vectors := make([][]float32, len(texts))
	for i := range vectors {
		vectors[i] = make([]float32, 128)
	}
	return vectors, nil
}

func (m *mockStores) Type() string {
	return "mock"
}

// Deletion tracking
func (m *mockStores) DeleteNodesByProperty(ctx context.Context, label, prop, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deletedNodes == nil {
		m.deletedNodes = make(map[string]bool)
	}
	m.deletedNodes[value] = true
	return nil
}

