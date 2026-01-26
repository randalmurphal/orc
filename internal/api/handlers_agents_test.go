package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHandleGetAgentStats_Empty(t *testing.T) {
	t.Parallel()
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Create .orc directory
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create server
	cfg := &Config{
		Addr:    ":0",
		WorkDir: tmpDir,
	}
	server := New(cfg)

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/agents/stats", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	server.handleGetAgentStats(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response
	var response AgentStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have empty agents list
	if len(response.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(response.Agents))
	}

	// Summary should be zero
	if response.Summary.TotalAgents != 0 {
		t.Errorf("expected TotalAgents=0, got %d", response.Summary.TotalAgents)
	}
}

func TestHandleGetAgentStats_WithTasks(t *testing.T) {
	t.Parallel()
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

	// Get DB to set session_model and usage_metrics
	pdb := backend.DB()

	// Create completed tasks with session_model
	now := time.Now()
	task1 := task.NewProtoTask("TASK-001", "First task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task1.StartedAt = timestamppb.Now()
	task1.CompletedAt = timestamppb.New(now.Add(3 * time.Minute))
	if err := backend.SaveTaskProto(task1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Update session_model directly in DB (SaveTaskProto doesn't expose this field)
	_, err = pdb.Exec(`UPDATE tasks SET session_model = ? WHERE id = ?`, "claude-sonnet-4-20250514", "TASK-001")
	if err != nil {
		t.Fatalf("failed to update session_model: %v", err)
	}

	task2 := task.NewProtoTask("TASK-002", "Second task")
	task2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	task2.StartedAt = timestamppb.Now()
	task2.CompletedAt = timestamppb.New(now.Add(1 * time.Minute))
	if err := backend.SaveTaskProto(task2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	_, err = pdb.Exec(`UPDATE tasks SET session_model = ? WHERE id = ?`, "claude-sonnet-4-20250514", "TASK-002")
	if err != nil {
		t.Fatalf("failed to update session_model: %v", err)
	}

	task3 := task.NewProtoTask("TASK-003", "Failed task")
	task3.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTaskProto(task3); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	_, err = pdb.Exec(`UPDATE tasks SET session_model = ? WHERE id = ?`, "claude-sonnet-4-20250514", "TASK-003")
	if err != nil {
		t.Fatalf("failed to update session_model: %v", err)
	}

	// Add usage metrics for today
	todayMs := time.Now().UnixMilli()
	_, err = pdb.Exec(`
		INSERT INTO usage_metrics (task_id, phase, model, project_path, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost_usd, duration_ms, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "TASK-001", "implement", "claude-sonnet-4-20250514", tmpDir, 50000, 10000, 0, 0, 0.10, 1000, todayMs)
	if err != nil {
		t.Fatalf("failed to insert usage metric: %v", err)
	}

	_, err = pdb.Exec(`
		INSERT INTO usage_metrics (task_id, phase, model, project_path, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost_usd, duration_ms, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "TASK-002", "implement", "claude-sonnet-4-20250514", tmpDir, 30000, 5000, 0, 0, 0.05, 500, todayMs)
	if err != nil {
		t.Fatalf("failed to insert usage metric: %v", err)
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
	req := httptest.NewRequest(http.MethodGet, "/api/agents/stats", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	server.handleGetAgentStats(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response
	var response AgentStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 1 agent (grouped by model)
	if len(response.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(response.Agents))
	}

	// Check the agent stats
	agent := response.Agents[0]
	if agent.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", agent.Model)
	}

	if agent.Stats.TasksDoneTotal != 2 {
		t.Errorf("expected TasksDoneTotal=2, got %d", agent.Stats.TasksDoneTotal)
	}

	// Success rate: 2 completed / 3 total (2 completed + 1 failed) = 0.666...
	expectedRate := 2.0 / 3.0
	if agent.Stats.SuccessRate < expectedRate-0.01 || agent.Stats.SuccessRate > expectedRate+0.01 {
		t.Errorf("expected SuccessRate ~0.66, got %f", agent.Stats.SuccessRate)
	}

	// Tokens today: 50000+10000 + 30000+5000 = 95000
	if agent.Stats.TokensToday != 95000 {
		t.Errorf("expected TokensToday=95000, got %d", agent.Stats.TokensToday)
	}

	// Summary
	if response.Summary.TotalAgents != 1 {
		t.Errorf("expected TotalAgents=1, got %d", response.Summary.TotalAgents)
	}

	if response.Summary.TotalTokensToday != 95000 {
		t.Errorf("expected TotalTokensToday=95000, got %d", response.Summary.TotalTokensToday)
	}
}

func TestHandleGetAgentStats_ActiveStatus(t *testing.T) {
	t.Parallel()
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

	pdb := backend.DB()

	// Create a running task
	runningTask := task.NewProtoTask("TASK-001", "Running task")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTaskProto(runningTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	_, err = pdb.Exec(`UPDATE tasks SET session_model = ? WHERE id = ?`, "claude-opus-4-20250514", "TASK-001")
	if err != nil {
		t.Fatalf("failed to update session_model: %v", err)
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
	req := httptest.NewRequest(http.MethodGet, "/api/agents/stats", nil)

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	server.handleGetAgentStats(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response
	var response AgentStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have 1 agent
	if len(response.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(response.Agents))
	}

	// Agent should be active
	if response.Agents[0].Status != "active" {
		t.Errorf("expected status 'active', got %q", response.Agents[0].Status)
	}

	// Summary should show 1 active agent
	if response.Summary.ActiveAgents != 1 {
		t.Errorf("expected ActiveAgents=1, got %d", response.Summary.ActiveAgents)
	}
}

func TestDeriveAgentName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		model    string
		expected string
	}{
		{"claude-opus-4-20250514", "Opus Agent"},
		{"claude-sonnet-4-20250514", "Sonnet Agent"},
		{"claude-haiku-3-5-20241022", "Haiku Agent"},
		{"gpt-4", "gpt-4"}, // Unknown model, use as-is
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := deriveAgentName(tc.model)
			if result != tc.expected {
				t.Errorf("deriveAgentName(%q) = %q, expected %q", tc.model, result, tc.expected)
			}
		})
	}
}

func TestStringContainsIgnoreCase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"claude-opus-4", "opus", true},
		{"claude-OPUS-4", "opus", true},
		{"claude-Opus-4", "OPUS", true},
		{"claude-sonnet-4", "opus", false},
		{"opus", "opus", true},
		{"", "opus", false},
		{"opus", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.s+"_"+tc.substr, func(t *testing.T) {
			got := stringContainsIgnoreCase(tc.s, tc.substr)
			if got != tc.want {
				t.Errorf("stringContainsIgnoreCase(%q, %q) = %v, want %v", tc.s, tc.substr, got, tc.want)
			}
		})
	}
}

func TestGetAgentStats_DB(t *testing.T) {
	t.Parallel()
	// Test the database method directly
	tmpDir := t.TempDir()

	// Create .orc directory
	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Create project DB
	pdb, err := db.OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to create project db: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Insert test data
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	started := now.Add(-10 * time.Minute)
	completed := now.Add(-5 * time.Minute)

	// Insert task with session_model
	_, err = pdb.Exec(`
		INSERT INTO tasks (id, title, weight, status, branch, queue, priority, category, created_at, started_at, completed_at, session_model)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "TASK-001", "Test task", "small", "completed", "orc/TASK-001", "active", "normal", "feature",
		now.Format(time.RFC3339), started.Format(time.RFC3339), completed.Format(time.RFC3339), "claude-sonnet-4")
	if err != nil {
		t.Fatalf("failed to insert task: %v", err)
	}

	// Insert usage metric
	_, err = pdb.Exec(`
		INSERT INTO usage_metrics (task_id, phase, model, project_path, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost_usd, duration_ms, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "TASK-001", "implement", "claude-sonnet-4", tmpDir, 10000, 2000, 0, 0, 0.05, 1000, now.UnixMilli())
	if err != nil {
		t.Fatalf("failed to insert usage metric: %v", err)
	}

	// Get stats
	stats, err := pdb.GetAgentStats(today)
	if err != nil {
		t.Fatalf("GetAgentStats failed: %v", err)
	}

	// Check stats
	if len(stats) != 1 {
		t.Errorf("expected 1 model in stats, got %d", len(stats))
	}

	sonnetStats, ok := stats["claude-sonnet-4"]
	if !ok {
		t.Fatal("expected claude-sonnet-4 in stats")
	}

	if sonnetStats.TasksDoneTotal != 1 {
		t.Errorf("expected TasksDoneTotal=1, got %d", sonnetStats.TasksDoneTotal)
	}

	if sonnetStats.SuccessRate != 1.0 {
		t.Errorf("expected SuccessRate=1.0, got %f", sonnetStats.SuccessRate)
	}

	if sonnetStats.TokensToday != 12000 {
		t.Errorf("expected TokensToday=12000, got %d", sonnetStats.TokensToday)
	}

	if sonnetStats.IsActive {
		t.Error("expected IsActive=false for completed task")
	}
}
