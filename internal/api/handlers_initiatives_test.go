package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// setupInitiativesTestServer creates a test server with backend for initiative tests.
func setupInitiativesTestServer(t *testing.T) (*Server, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "orc-initiatives-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	pub := events.NewMemoryPublisher()

	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}

	srv := New(cfg)
	srv.backend = backend
	srv.publisher = pub

	cleanup := func() {
		_ = backend.Close()
		_ = os.RemoveAll(tmpDir)
	}

	return srv, cleanup
}

// TestHandleListInitiatives_AutoCompletes tests that the API list endpoint
// triggers auto-completion for eligible initiatives.
// Covers SC-4: API initiative list endpoint triggers auto-completion check.
func TestHandleListInitiatives_AutoCompletes(t *testing.T) {
	srv, cleanup := setupInitiativesTestServer(t)
	defer cleanup()

	// Create initiative WITHOUT BranchBase, in active status
	init := initiative.New("INIT-001", "Should Auto-Complete via API")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Completed task", nil)
	if err := srv.backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Completed task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	initID := "INIT-001"
	tk.InitiativeId = &initID
	if err := srv.backend.SaveTaskProto(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Make request to list initiatives
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/initiatives", nil)
	srv.handleListInitiatives(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var initiatives []*initiative.Initiative
	if err := json.NewDecoder(w.Body).Decode(&initiatives); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Find our initiative in the response
	var found *initiative.Initiative
	for _, init := range initiatives {
		if init.ID == "INIT-001" {
			found = init
			break
		}
	}

	if found == nil {
		t.Fatal("INIT-001 not found in response")
	}

	// After the fix, the initiative should be completed in the response
	if found.Status != initiative.StatusCompleted {
		t.Errorf("initiative Status = %q, want %q (should auto-complete on API list)",
			found.Status, initiative.StatusCompleted)
	}

	// Verify the change was persisted to database
	reloaded, _ := srv.backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("persisted Status = %q, want %q", reloaded.Status, initiative.StatusCompleted)
	}
}

// TestHandleListInitiatives_DoesNotAutoCompleteWithBranchBase tests that
// initiatives with BranchBase are skipped during API auto-completion.
func TestHandleListInitiatives_DoesNotAutoCompleteWithBranchBase(t *testing.T) {
	srv, cleanup := setupInitiativesTestServer(t)
	defer cleanup()

	// Create initiative WITH BranchBase
	init := initiative.New("INIT-001", "Feature Branch Initiative")
	init.Status = initiative.StatusActive
	init.BranchBase = "feature/auth" // Has branch base - should use merge flow
	init.AddTask("TASK-001", "Task", nil)
	if err := srv.backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	initID := "INIT-001"
	tk.InitiativeId = &initID
	if err := srv.backend.SaveTaskProto(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Make request
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/initiatives", nil)
	srv.handleListInitiatives(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify initiative remains active (BranchBase uses merge flow)
	reloaded, _ := srv.backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusActive {
		t.Errorf("Status = %q, want %q (BranchBase initiatives should use merge flow)",
			reloaded.Status, initiative.StatusActive)
	}
}

// TestHandleGetInitiative_AutoCompletes tests that the API get endpoint
// triggers auto-completion for a specific initiative.
// Related to SC-4/SC-5: API endpoints trigger auto-completion.
func TestHandleGetInitiative_AutoCompletes(t *testing.T) {
	srv, cleanup := setupInitiativesTestServer(t)
	defer cleanup()

	// Create initiative WITHOUT BranchBase
	init := initiative.New("INIT-001", "Should Auto-Complete on Get")
	init.Status = initiative.StatusActive
	init.AddTask("TASK-001", "Done task", nil)
	if err := srv.backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create completed task
	tk := task.NewProtoTask("TASK-001", "Done task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	initID := "INIT-001"
	tk.InitiativeId = &initID
	if err := srv.backend.SaveTaskProto(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Make request to get specific initiative
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/initiatives/INIT-001", nil)
	r.SetPathValue("id", "INIT-001")
	srv.handleGetInitiative(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var result initiative.Initiative
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// After the fix, the initiative should be completed in the response
	if result.Status != initiative.StatusCompleted {
		t.Errorf("response Status = %q, want %q (should auto-complete on API get)",
			result.Status, initiative.StatusCompleted)
	}

	// Verify persisted
	reloaded, _ := srv.backend.LoadInitiative("INIT-001")
	if reloaded.Status != initiative.StatusCompleted {
		t.Errorf("persisted Status = %q, want %q", reloaded.Status, initiative.StatusCompleted)
	}
}

// TestHandleListInitiatives_CompletedInitiativesInResponse tests that
// completed initiatives are returned correctly without BLOCKED status.
// Related to SC-3: Completed initiatives don't show BLOCKED.
func TestHandleListInitiatives_CompletedInitiativesInResponse(t *testing.T) {
	srv, cleanup := setupInitiativesTestServer(t)
	defer cleanup()

	// Create "blocker" initiative that is NOT completed
	blocker := initiative.New("INIT-001", "Blocker Initiative")
	blocker.Status = initiative.StatusActive
	if err := srv.backend.SaveInitiative(blocker); err != nil {
		t.Fatalf("save blocker: %v", err)
	}

	// Create completed initiative with BlockedBy dependency
	completed := initiative.New("INIT-002", "Completed With Blocker")
	completed.Status = initiative.StatusCompleted
	completed.BlockedBy = []string{"INIT-001"} // Has unmet blocker
	if err := srv.backend.SaveInitiative(completed); err != nil {
		t.Fatalf("save completed: %v", err)
	}

	// Make request
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/initiatives", nil)
	srv.handleListInitiatives(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var initiatives []*initiative.Initiative
	if err := json.NewDecoder(w.Body).Decode(&initiatives); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Find completed initiative
	var found *initiative.Initiative
	for _, init := range initiatives {
		if init.ID == "INIT-002" {
			found = init
			break
		}
	}

	if found == nil {
		t.Fatal("INIT-002 not found in response")
	}

	// Verify it's still completed (not changed)
	if found.Status != initiative.StatusCompleted {
		t.Errorf("completed initiative Status = %q, want %q", found.Status, initiative.StatusCompleted)
	}

	// Note: The API returns raw initiative data; the frontend or CLI
	// is responsible for not displaying "[BLOCKED]" for completed initiatives.
	// The fix is in the display logic, not in the API response format.
}

// TestHandleListInitiatives_AutoCompleteErrorHandling tests that errors
// during auto-completion don't break the list response.
func TestHandleListInitiatives_AutoCompleteErrorHandling(t *testing.T) {
	srv, cleanup := setupInitiativesTestServer(t)
	defer cleanup()

	// Create initiative with missing task (will fail auto-completion check)
	init1 := initiative.New("INIT-001", "Has Missing Task")
	init1.Status = initiative.StatusActive
	init1.AddTask("TASK-MISSING", "Task doesn't exist", nil)
	if err := srv.backend.SaveInitiative(init1); err != nil {
		t.Fatalf("save init1: %v", err)
	}

	// Create initiative that should auto-complete
	init2 := initiative.New("INIT-002", "Should Complete")
	init2.Status = initiative.StatusActive
	init2.AddTask("TASK-001", "Done task", nil)
	if err := srv.backend.SaveInitiative(init2); err != nil {
		t.Fatalf("save init2: %v", err)
	}

	tk := task.NewProtoTask("TASK-001", "Done task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	initID := "INIT-002"
	tk.InitiativeId = &initID
	if err := srv.backend.SaveTaskProto(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Request should succeed despite one initiative having issues
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/initiatives", nil)
	srv.handleListInitiatives(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse and verify we got both initiatives
	var initiatives []*initiative.Initiative
	if err := json.NewDecoder(w.Body).Decode(&initiatives); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(initiatives) != 2 {
		t.Errorf("expected 2 initiatives, got %d", len(initiatives))
	}

	// Verify INIT-002 was still auto-completed despite INIT-001 issues
	reloaded2, _ := srv.backend.LoadInitiative("INIT-002")
	if reloaded2.Status != initiative.StatusCompleted {
		t.Errorf("INIT-002 Status = %q, want %q", reloaded2.Status, initiative.StatusCompleted)
	}

	// INIT-001 should remain active
	reloaded1, _ := srv.backend.LoadInitiative("INIT-001")
	if reloaded1.Status != initiative.StatusActive {
		t.Errorf("INIT-001 Status = %q, want %q", reloaded1.Status, initiative.StatusActive)
	}
}
