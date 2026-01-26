package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func setupTestServer(t *testing.T) (*Server, func()) {
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .orc directory
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create publisher
	pub := events.NewMemoryPublisher()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}

	srv := New(cfg)
	srv.backend = backend
	srv.publisher = pub

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return srv, cleanup
}

func TestHandlePostDecision_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// POST to non-existent decision
	req := DecisionRequest{Approved: true}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/decisions/gate_TASK-999_review", bytes.NewReader(body))
	r.SetPathValue("id", "gate_TASK-999_review")

	srv.handlePostDecision(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandlePostDecision_Approve(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create task
	tsk := task.NewProtoTask("TASK-001", "Test Task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	phase := "review"
	tsk.CurrentPhase = &phase
	if err := srv.backend.SaveTaskProto(tsk); err != nil {
		t.Fatal(err)
	}

	// Add pending decision
	decision := &gate.PendingDecision{
		DecisionID:  "gate_TASK-001_review",
		TaskID:      "TASK-001",
		TaskTitle:   "Test Task",
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve?",
		RequestedAt: time.Now(),
	}
	srv.pendingDecisions.Add(decision)

	// Subscribe to events for all tasks
	eventChan := srv.publisher.Subscribe("*")

	// POST approval
	req := DecisionRequest{Approved: true, Reason: "LGTM"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/decisions/gate_TASK-001_review", bytes.NewReader(body))
	r.SetPathValue("id", "gate_TASK-001_review")

	srv.handlePostDecision(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check response
	var resp DecisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Approved != true {
		t.Error("expected Approved true")
	}
	if resp.NewStatus != "planned" {
		t.Errorf("expected NewStatus planned, got %s", resp.NewStatus)
	}

	// Verify task status changed
	reloadedTask, err := srv.backend.LoadTaskProto("TASK-001")
	if err != nil {
		t.Fatal(err)
	}
	if reloadedTask.Status != orcv1.TaskStatus_TASK_STATUS_PLANNED {
		t.Errorf("expected task status planned, got %s", reloadedTask.Status)
	}

	// Verify decision removed from store
	_, ok := srv.pendingDecisions.Get("gate_TASK-001_review")
	if ok {
		t.Error("expected decision to be removed from store")
	}

	// Wait for decision_resolved event
	select {
	case evt := <-eventChan:
		if evt.Type != events.EventDecisionResolved {
			t.Errorf("expected EventDecisionResolved, got %s", evt.Type)
		}
		data, ok := evt.Data.(events.DecisionResolvedData)
		if !ok {
			t.Fatal("expected DecisionResolvedData")
		}
		if data.Approved != true {
			t.Error("expected event Approved true")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for decision_resolved event")
	}
}

func TestHandleListDecisions(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Empty list initially
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/decisions", nil)

	srv.handleListDecisions(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp []PendingDecisionItem
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp) != 0 {
		t.Errorf("expected 0 decisions, got %d", len(resp))
	}

	// Add pending decisions
	now := time.Now()
	decision1 := &gate.PendingDecision{
		DecisionID:  "gate_TASK-001_review_123",
		TaskID:      "TASK-001",
		TaskTitle:   "First Task",
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve first?",
		RequestedAt: now,
	}
	srv.pendingDecisions.Add(decision1)

	decision2 := &gate.PendingDecision{
		DecisionID:  "gate_TASK-002_implement_456",
		TaskID:      "TASK-002",
		TaskTitle:   "Second Task",
		Phase:       "implement",
		GateType:    "ai",
		Question:    "Approve second?",
		Context:     "Some context here",
		RequestedAt: now.Add(time.Minute),
	}
	srv.pendingDecisions.Add(decision2)

	// List decisions
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest("GET", "/api/decisions", nil)

	srv.handleListDecisions(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w2.Code)
	}

	var resp2 []PendingDecisionItem
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatal(err)
	}
	if len(resp2) != 2 {
		t.Errorf("expected 2 decisions, got %d", len(resp2))
	}

	// Verify decision fields are populated correctly
	found := make(map[string]bool)
	for _, d := range resp2 {
		found[d.DecisionID] = true
		if d.DecisionID == "gate_TASK-001_review_123" {
			if d.TaskID != "TASK-001" {
				t.Errorf("expected TaskID TASK-001, got %s", d.TaskID)
			}
			if d.TaskTitle != "First Task" {
				t.Errorf("expected TaskTitle 'First Task', got %s", d.TaskTitle)
			}
			if d.Phase != "review" {
				t.Errorf("expected Phase 'review', got %s", d.Phase)
			}
			if d.GateType != "human" {
				t.Errorf("expected GateType 'human', got %s", d.GateType)
			}
			if d.Question != "Approve first?" {
				t.Errorf("expected Question 'Approve first?', got %s", d.Question)
			}
		}
		if d.DecisionID == "gate_TASK-002_implement_456" {
			if d.Context != "Some context here" {
				t.Errorf("expected Context 'Some context here', got %s", d.Context)
			}
		}
	}

	if !found["gate_TASK-001_review_123"] || !found["gate_TASK-002_implement_456"] {
		t.Error("missing expected decision IDs")
	}
}

func TestHandlePostDecision_Reject(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create task
	tsk := task.NewProtoTask("TASK-002", "Test Task 2")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	phase := "implement"
	tsk.CurrentPhase = &phase
	if err := srv.backend.SaveTaskProto(tsk); err != nil {
		t.Fatal(err)
	}

	// Add pending decision
	decision := &gate.PendingDecision{
		DecisionID:  "gate_TASK-002_implement",
		TaskID:      "TASK-002",
		TaskTitle:   "Test Task 2",
		Phase:       "implement",
		GateType:    "human",
		RequestedAt: time.Now(),
	}
	srv.pendingDecisions.Add(decision)

	// POST rejection
	req := DecisionRequest{Approved: false, Reason: "Needs more work"}
	body, _ := json.Marshal(req)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/decisions/gate_TASK-002_implement", bytes.NewReader(body))
	r.SetPathValue("id", "gate_TASK-002_implement")

	srv.handlePostDecision(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check response
	var resp DecisionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Approved != false {
		t.Error("expected Approved false")
	}
	if resp.NewStatus != "failed" {
		t.Errorf("expected NewStatus failed, got %s", resp.NewStatus)
	}

	// Verify task status changed to failed
	reloadedTask, err := srv.backend.LoadTaskProto("TASK-002")
	if err != nil {
		t.Fatal(err)
	}
	if reloadedTask.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("expected task status failed, got %s", reloadedTask.Status)
	}
}
