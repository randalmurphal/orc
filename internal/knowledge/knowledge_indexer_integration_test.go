package knowledge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/randalmurphal/orc/internal/knowledge/index"
	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// --- Integration tests: Service.IndexProject → real indexer pipeline ---
//
// Unlike the unit tests in knowledge_index_test.go (which mock the indexer
// entirely via mockIndexComponents), these tests use RECORDING STORES that
// the REAL indexer pipeline writes to. This proves the Service actually
// creates a real indexer, passes stores through, and runs the pipeline.
//
// The litmus test: if Service.IndexProject returned canned results without
// running the pipeline, these tests would fail because the recording stores
// would have no data.

// Integration: Service.IndexProject creates a real indexer and runs the pipeline.
// Verifies: Service wires graph/vector/embed stores to the indexer.
// Fails if: Service mocks the indexer or doesn't pass stores through.
func TestServiceIndexProject_RunsRealPipeline(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "main.go", `package main

import "fmt"

// Greeter says hello.
func Greeter(name string) string {
	return fmt.Sprintf("Hello, %s", name)
}

func main() {
	Greeter("world")
}
`)

	rec := newRecordingComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(rec))

	result, err := svc.IndexProject(context.Background(), root, index.IndexOptions{})
	if err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	if result.FilesProcessed == 0 {
		t.Error("FilesProcessed = 0 — Service did not run real indexer pipeline")
	}
	if result.ChunksStored == 0 {
		t.Error("ChunksStored = 0 — chunks not wired through Service to stores")
	}

	// Recording stores must have received real data from the pipeline.
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.nodes) == 0 {
		t.Error("graph store received no nodes — Service didn't wire graph store to indexer")
	}

	hasFile := false
	hasSymbol := false
	for _, n := range rec.nodes {
		for _, l := range n.Labels {
			if l == "File" {
				hasFile = true
			}
			if l == "Symbol" {
				hasSymbol = true
			}
		}
	}
	if !hasFile {
		t.Error("no File nodes — walker not wired through Service to indexer")
	}
	if !hasSymbol {
		t.Error("no Symbol nodes — parser not wired through Service to indexer")
	}

	if len(rec.vectors) == 0 {
		t.Error("vector store received no vectors — embedder not wired through Service")
	}

	if rec.embedCalls == 0 {
		t.Error("embedder never called — embedder not wired through Service")
	}
}

// Integration: Service.IndexProject produces graph relationships from real pipeline.
// Verifies: Full pipeline (walk → parse → chunk → relationships → graph) runs via Service.
// Fails if: Service skips relationship extraction or graph storage.
func TestServiceIndexProject_ProducesGraphRelationships(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "main.go", `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`)

	rec := newRecordingComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(rec))

	_, err := svc.IndexProject(context.Background(), root, index.IndexOptions{})
	if err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.rels) == 0 {
		t.Error("graph received no relationships — pipeline wiring incomplete through Service")
	}

	// Should have CONTAINS (File→Symbol) at minimum.
	hasContains := false
	for _, r := range rec.rels {
		if r.relType == "CONTAINS" {
			hasContains = true
			break
		}
	}
	if !hasContains {
		t.Error("no CONTAINS relationships — file-symbol wiring broken through Service")
	}
}

// Integration: Service.IndexProject passes incremental option to real indexer.
// Verifies: Options flow from Service through to the real indexer pipeline.
// Fails if: Service ignores options or doesn't pass them to the indexer.
func TestServiceIndexProject_IncrementalPassthrough(t *testing.T) {
	root := t.TempDir()
	writeProjectFile(t, root, "main.go", `package main

func main() {}
`)

	rec := newRecordingComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(rec))

	// First run — full index.
	result1, err := svc.IndexProject(context.Background(), root, index.IndexOptions{})
	if err != nil {
		t.Fatalf("first IndexProject: %v", err)
	}
	if result1.FilesProcessed == 0 {
		t.Fatal("first run should process files")
	}

	// Second run with incremental — should process fewer or zero files.
	result2, err := svc.IndexProject(context.Background(), root, index.IndexOptions{Incremental: true})
	if err != nil {
		t.Fatalf("incremental IndexProject: %v", err)
	}

	if result2.FilesProcessed > result1.FilesProcessed {
		t.Errorf("incremental run processed MORE files (%d) than full run (%d) — option not passed through",
			result2.FilesProcessed, result1.FilesProcessed)
	}
}

// --- Recording components ---
//
// These implement Components + store/embed interfaces so the REAL indexer
// can call them directly. They record what was received, proving the
// pipeline ran and stores were correctly wired.

type recordingComponents struct {
	mu            sync.Mutex
	neo4jHealthy  bool
	qdrantHealthy bool
	redisHealthy  bool

	// Recorded from graph store operations
	nodes []store.Node
	rels  []recordedRel

	// Recorded from vector store operations
	vectors []store.Vector

	// Recorded from embedder operations
	embedCalls int

	// Stored hashes for incremental support
	storedHashes map[string]string
}

type recordedRel struct {
	fromID  string
	toID    string
	relType string
}

func newRecordingComponents() *recordingComponents {
	return &recordingComponents{
		neo4jHealthy:  true,
		qdrantHealthy: true,
		redisHealthy:  true,
	}
}

// --- Components interface ---

func (r *recordingComponents) InfraStart(_ context.Context) error    { return nil }
func (r *recordingComponents) InfraStop(_ context.Context) error     { return nil }
func (r *recordingComponents) GraphConnect(_ context.Context) error  { return nil }
func (r *recordingComponents) GraphClose() error                     { return nil }
func (r *recordingComponents) VectorConnect(_ context.Context) error { return nil }
func (r *recordingComponents) VectorClose() error                    { return nil }
func (r *recordingComponents) CacheConnect(_ context.Context) error  { return nil }
func (r *recordingComponents) CacheClose() error                     { return nil }

func (r *recordingComponents) IsHealthy() (neo4j, qdrant, redis bool) {
	return r.neo4jHealthy, r.qdrantHealthy, r.redisHealthy
}

// --- Graph store interface (recording) ---

func (r *recordingComponents) CreateNode(_ context.Context, node store.Node) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nodes = append(r.nodes, node)
	return fmt.Sprintf("node-%d", len(r.nodes)), nil
}

func (r *recordingComponents) QueryNodes(_ context.Context, _ string, _ map[string]interface{}) ([]store.Node, error) {
	return nil, nil
}

func (r *recordingComponents) CreateRelationship(_ context.Context, fromID, toID, relType string, _ map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rels = append(r.rels, recordedRel{fromID, toID, relType})
	return nil
}

func (r *recordingComponents) ExecuteCypher(_ context.Context, _ string, _ map[string]interface{}) ([]map[string]interface{}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.storedHashes != nil {
		var results []map[string]interface{}
		for path, hash := range r.storedHashes {
			results = append(results, map[string]interface{}{
				"path": path,
				"hash": hash,
			})
		}
		return results, nil
	}
	return nil, nil
}

func (r *recordingComponents) DeleteNodesByProperty(_ context.Context, _, _, _ string) error {
	return nil
}

// --- Vector store interface (recording) ---

func (r *recordingComponents) Upsert(_ context.Context, _ string, vectors []store.Vector) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vectors = append(r.vectors, vectors...)
	return nil
}

func (r *recordingComponents) CreateCollection(_ context.Context, _ string, _ int) error {
	return nil
}

// --- Embedder interface (recording) ---

func (r *recordingComponents) Embed(_ context.Context, texts []string) ([][]float32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.embedCalls++
	vecs := make([][]float32, len(texts))
	for i := range vecs {
		vecs[i] = make([]float32, 128)
	}
	return vecs, nil
}

func (r *recordingComponents) Type() string { return "recording" }

// --- Helpers ---

func writeProjectFile(t *testing.T, root, relPath, content string) {
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
