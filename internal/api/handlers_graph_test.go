package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestMapTaskStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"completed", "done"},
		{"running", "running"},
		{"finalizing", "running"},
		{"blocked", "blocked"},
		{"paused", "paused"},
		{"failed", "failed"},
		{"created", "ready"},
		{"planned", "ready"},
		{"unknown", "pending"},
		{"", "pending"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := mapTaskStatus(tc.input)
			if result != tc.expected {
				t.Errorf("mapTaskStatus(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestBuildGraphFromTasks(t *testing.T) {
	taskMap := map[string]*task.Task{
		"TASK-001": {
			ID:     "TASK-001",
			Title:  "First task",
			Status: task.StatusCompleted,
		},
		"TASK-002": {
			ID:        "TASK-002",
			Title:     "Second task",
			Status:    task.StatusPlanned,
			BlockedBy: []string{"TASK-001"},
		},
		"TASK-003": {
			ID:        "TASK-003",
			Title:     "Third task",
			Status:    task.StatusCreated,
			BlockedBy: []string{"TASK-001", "TASK-002"},
		},
		"TASK-004": {
			ID:     "TASK-004",
			Title:  "Unrelated task",
			Status: task.StatusRunning,
		},
	}

	// Request only TASK-001, TASK-002, TASK-003
	requestedIDs := map[string]bool{
		"TASK-001": true,
		"TASK-002": true,
		"TASK-003": true,
	}

	graph := buildGraphFromTasks(taskMap, requestedIDs)

	// Check nodes
	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}

	// Nodes should be sorted by ID
	expectedIDs := []string{"TASK-001", "TASK-002", "TASK-003"}
	for i, id := range expectedIDs {
		if graph.Nodes[i].ID != id {
			t.Errorf("node %d: expected ID %q, got %q", i, id, graph.Nodes[i].ID)
		}
	}

	// Check statuses
	statusMap := make(map[string]string)
	for _, n := range graph.Nodes {
		statusMap[n.ID] = n.Status
	}
	if statusMap["TASK-001"] != "done" {
		t.Errorf("TASK-001 status: expected 'done', got %q", statusMap["TASK-001"])
	}
	if statusMap["TASK-002"] != "ready" {
		t.Errorf("TASK-002 status: expected 'ready', got %q", statusMap["TASK-002"])
	}
	if statusMap["TASK-003"] != "ready" {
		t.Errorf("TASK-003 status: expected 'ready', got %q", statusMap["TASK-003"])
	}

	// Check edges
	if len(graph.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(graph.Edges))
	}

	// Edges should include TASK-001 -> TASK-002, TASK-001 -> TASK-003, TASK-002 -> TASK-003
	edgeSet := make(map[string]bool)
	for _, e := range graph.Edges {
		edgeSet[e.From+"->"+e.To] = true
	}

	expectedEdges := []string{
		"TASK-001->TASK-002",
		"TASK-001->TASK-003",
		"TASK-002->TASK-003",
	}
	for _, edge := range expectedEdges {
		if !edgeSet[edge] {
			t.Errorf("missing expected edge: %s", edge)
		}
	}
}

func TestBuildGraphFromTasks_NoExternalDeps(t *testing.T) {
	taskMap := map[string]*task.Task{
		"TASK-001": {
			ID:     "TASK-001",
			Title:  "First task",
			Status: task.StatusCompleted,
		},
		"TASK-002": {
			ID:        "TASK-002",
			Title:     "Second task",
			Status:    task.StatusPlanned,
			BlockedBy: []string{"TASK-001", "TASK-999"}, // TASK-999 not in requested set
		},
	}

	requestedIDs := map[string]bool{
		"TASK-001": true,
		"TASK-002": true,
	}

	graph := buildGraphFromTasks(taskMap, requestedIDs)

	// Should only have 1 edge (TASK-999 excluded)
	if len(graph.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(graph.Edges))
	}

	if graph.Edges[0].From != "TASK-001" || graph.Edges[0].To != "TASK-002" {
		t.Errorf("unexpected edge: %s -> %s", graph.Edges[0].From, graph.Edges[0].To)
	}
}

func TestHandleGetInitiativeDependencyGraph(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create .orc directory
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create and save tasks
	task1 := task.New("TASK-001", "First task")
	task1.Status = task.StatusCompleted
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	task2 := task.New("TASK-002", "Second task")
	task2.Status = task.StatusPlanned
	task2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create and save initiative
	init := initiative.New("INIT-001", "Test Initiative")
	init.Tasks = []initiative.TaskRef{
		{ID: "TASK-001", Title: "First task", Status: "completed"},
		{ID: "TASK-002", Title: "Second task", Status: "planned"},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("failed to save initiative: %v", err)
	}

	// Close backend before creating server
	_ = backend.Close()

	// Create server
	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/initiatives/INIT-001/dependency-graph", nil)
	req.SetPathValue("id", "INIT-001")

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	server.handleGetInitiativeDependencyGraph(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response
	var graph DependencyGraphResponse
	if err := json.NewDecoder(rr.Body).Decode(&graph); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check nodes
	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graph.Nodes))
	}

	// Check edges
	if len(graph.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(graph.Edges))
	}
}

func TestHandleGetTasksDependencyGraph(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create .orc directory
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create and save tasks
	task1 := task.New("TASK-001", "First task")
	task1.Status = task.StatusCompleted
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	task2 := task.New("TASK-002", "Second task")
	task2.Status = task.StatusPlanned
	task2.BlockedBy = []string{"TASK-001"}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before creating server
	_ = backend.Close()

	// Create server
	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/dependency-graph?ids=TASK-001,TASK-002", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	server.handleGetTasksDependencyGraph(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response
	var graph DependencyGraphResponse
	if err := json.NewDecoder(rr.Body).Decode(&graph); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check nodes
	if len(graph.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(graph.Nodes))
	}

	// Check edges
	if len(graph.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(graph.Edges))
	}
}

func TestHandleGetTasksDependencyGraph_MissingIDs(t *testing.T) {
	cfg := &Config{
		Addr:    ":0",
		WorkDir: t.TempDir(),
	}
	server := New(cfg)

	// Request without ids parameter
	req := httptest.NewRequest(http.MethodGet, "/api/tasks/dependency-graph", nil)
	rr := httptest.NewRecorder()

	server.handleGetTasksDependencyGraph(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}
