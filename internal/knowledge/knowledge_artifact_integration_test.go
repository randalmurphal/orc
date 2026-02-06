package knowledge

import (
	"context"
	"fmt"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/knowledge/index/artifact"
	"github.com/randalmurphal/orc/internal/knowledge/store"
	"github.com/randalmurphal/orc/internal/storage"
)

// --- Integration tests: Service.IndexTaskArtifacts → real artifact indexer ---
//
// These tests verify that Service.IndexTaskArtifacts creates a real
// artifact.Indexer, passes the graph store through, and runs the pipeline.
//
// The litmus test: If Service.IndexTaskArtifacts returned nil without
// importing the artifact package, the recording stores would have no data.
// Deleting the artifact import from knowledge.go makes these tests fail.

// SC-6 integration: Service.IndexTaskArtifacts wires graph store to artifact indexer.
// Verifies: Service imports artifact package, creates Indexer, passes GraphStorer.
// Fails if: Service doesn't create artifact.Indexer or doesn't pass graph store.
func TestServiceIndexTaskArtifacts_RunsArtifactPipeline(t *testing.T) {
	rec := newRecordingComponents()
	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(rec))

	fileStr := "internal/handler.go"
	agentStr := "security-reviewer"
	params := artifact.IndexParams{
		TaskID: "TASK-001",
		Spec:   "## Spec\nModifies internal/foo.go and internal/bar.go",
		Findings: []*orcv1.ReviewRoundFindings{
			{
				TaskId: "TASK-001",
				Round:  1,
				Issues: []*orcv1.ReviewFinding{
					{
						Severity:    "high",
						File:        &fileStr,
						Description: "Missing error handling",
						AgentId:     &agentStr,
					},
				},
			},
		},
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Use bcrypt", Rationale: "Standard"},
		},
		InitiativeID: "INIT-001",
		Retries: []artifact.RetryInfo{
			{Attempt: 1, Reason: "test failure", FromPhase: "implement"},
		},
		ChangedFiles: []string{"internal/foo.go", "internal/bar.go"},
		ScratchpadEntries: []storage.ScratchpadEntry{
			{
				ID:        1,
				TaskID:    "TASK-001",
				PhaseID:   "implement",
				Category:  "observation",
				Content:   "Noticed complexity in internal/foo.go",
				CreatedAt: time.Now(),
			},
		},
	}

	err := svc.IndexTaskArtifacts(context.Background(), params)
	if err != nil {
		t.Fatalf("IndexTaskArtifacts: %v", err)
	}

	// Recording store must have received graph operations from the pipeline.
	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.nodes) == 0 {
		t.Fatal("graph store received no nodes — Service didn't wire artifact indexer")
	}

	// Verify each artifact type produced graph nodes.
	nodeLabels := make(map[string]bool)
	for _, n := range rec.nodes {
		for _, l := range n.Labels {
			nodeLabels[l] = true
		}
	}

	for _, expected := range []string{"Spec", "Finding", "Decision", "Retry", "Observation"} {
		if !nodeLabels[expected] {
			t.Errorf("no %s nodes — %s indexer not wired through Service", expected, expected)
		}
	}

	// Verify relationships were created (proves indexer ran fully).
	if len(rec.rels) == 0 {
		t.Error("no relationships created — artifact indexer pipeline incomplete")
	}
}

// SC-10 integration: Service.IndexTaskArtifacts skips when unavailable.
// Verifies: IsAvailable() guard prevents indexing attempts.
// Fails if: Service attempts graph operations when disabled or unhealthy.
func TestServiceIndexTaskArtifacts_SkipsWhenUnavailable(t *testing.T) {
	t.Run("disabled_service", func(t *testing.T) {
		svc := NewService(ServiceConfig{Enabled: false})
		err := svc.IndexTaskArtifacts(context.Background(), artifact.IndexParams{
			TaskID: "TASK-001",
			Spec:   "should not be indexed",
		})
		if err != nil {
			t.Errorf("disabled service should return nil, got: %v", err)
		}
	})

	t.Run("unhealthy_backends", func(t *testing.T) {
		rec := newRecordingComponents()
		rec.neo4jHealthy = false
		svc := NewService(ServiceConfig{Enabled: true}, WithComponents(rec))

		err := svc.IndexTaskArtifacts(context.Background(), artifact.IndexParams{
			TaskID: "TASK-001",
			Spec:   "should not be indexed",
		})
		if err != nil {
			t.Errorf("unhealthy backends should return nil, got: %v", err)
		}

		rec.mu.Lock()
		defer rec.mu.Unlock()
		if len(rec.nodes) > 0 {
			t.Error("graph operations occurred when service was unavailable")
		}
	})

	t.Run("nil_components", func(t *testing.T) {
		svc := NewService(ServiceConfig{Enabled: true})
		// comps is nil
		err := svc.IndexTaskArtifacts(context.Background(), artifact.IndexParams{
			TaskID: "TASK-001",
			Spec:   "should not be indexed",
		})
		if err != nil {
			t.Errorf("nil components should return nil, got: %v", err)
		}
	})
}

// SC-11 integration: Service.IndexTaskArtifacts propagates indexer errors.
// The executor logs these as warnings — tested separately in executor tests.
// Verifies: Service does not silently swallow errors from the artifact indexer.
// Fails if: Service catches and discards graph store errors.
func TestServiceIndexTaskArtifacts_PropagatesIndexerErrors(t *testing.T) {
	failing := &failingArtifactComponents{
		recordingComponents: newRecordingComponents(),
		nodeErr:             fmt.Errorf("graph connection lost"),
	}
	failing.recordingComponents.neo4jHealthy = true
	failing.recordingComponents.qdrantHealthy = true
	failing.recordingComponents.redisHealthy = true

	svc := NewService(ServiceConfig{Enabled: true}, WithComponents(failing))

	params := artifact.IndexParams{
		TaskID: "TASK-001",
		Spec:   "A spec with content",
	}

	err := svc.IndexTaskArtifacts(context.Background(), params)
	if err == nil {
		t.Error("expected error when graph store fails, got nil")
	}
}

// --- Test doubles for artifact integration tests ---

// failingArtifactComponents wraps recordingComponents but returns errors
// from CreateNode. All other graph operations inherit normal behavior.
type failingArtifactComponents struct {
	*recordingComponents
	nodeErr error
}

func (f *failingArtifactComponents) CreateNode(_ context.Context, _ store.Node) (string, error) {
	return "", f.nodeErr
}
