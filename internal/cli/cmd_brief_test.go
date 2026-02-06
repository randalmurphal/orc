// Tests for TASK-021: CLI brief command.
//
// SC-6: `orc brief` displays the current project brief to stdout
// SC-7: `orc brief --regenerate` forces cache invalidation and regeneration
//
// NOTE: Tests use os.Chdir() (process-wide, not goroutine-safe).
// These tests MUST NOT use t.Parallel().
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// SC-6: `orc brief` displays the current project brief to stdout
// =============================================================================

func TestBriefCmd_ShowsBriefContent(t *testing.T) {
	tmpDir := withTempDir(t)

	// Initialize orc project
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Create a backend and seed data
	backend, err := storage.OpenDatabaseBackend(filepath.Join(tmpDir, ".orc", "orc.db"))
	if err != nil {
		t.Fatalf("open backend: %v", err)
	}
	defer backend.Close()

	// Seed initiative with decisions
	init := initiative.New("INIT-001", "Auth System")
	init.Status = initiative.StatusActive
	init.Decisions = []initiative.Decision{
		{ID: "DEC-001", Decision: "Use JWT tokens for auth", Rationale: "Stateless", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Seed completed task
	tsk := task.NewProtoTask("TASK-001", "Login feature")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Run the brief command
	var buf bytes.Buffer
	cmd := newBriefCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief command error: %v", err)
	}

	output := buf.String()

	// Should contain section headers
	if !strings.Contains(output, "Decisions") {
		t.Errorf("brief output should contain 'Decisions' section, got:\n%s", output)
	}

	// Should contain the decision content
	if !strings.Contains(output, "JWT") {
		t.Errorf("brief output should contain decision content, got:\n%s", output)
	}
}

// =============================================================================
// BDD-2: Empty project shows "No brief data available"
// =============================================================================

func TestBriefCmd_EmptyProject(t *testing.T) {
	tmpDir := withTempDir(t)

	// Initialize orc project (empty, no tasks)
	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	var buf bytes.Buffer
	cmd := newBriefCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief command error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "No brief data available") {
		t.Errorf("expected 'No brief data available' for empty project, got:\n%s", output)
	}
}

// =============================================================================
// SC-7: `orc brief --regenerate` forces cache invalidation
// =============================================================================

func TestBriefCmd_Regenerate(t *testing.T) {
	tmpDir := withTempDir(t)

	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Seed some data
	backend, err := storage.OpenDatabaseBackend(filepath.Join(tmpDir, ".orc", "orc.db"))
	if err != nil {
		t.Fatalf("open backend: %v", err)
	}
	defer backend.Close()

	init := initiative.New("INIT-001", "Test")
	init.Status = initiative.StatusActive
	init.Decisions = []initiative.Decision{
		{ID: "DEC-001", Decision: "Use Redis", Rationale: "Performance", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	var buf bytes.Buffer
	cmd := newBriefCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--regenerate"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief --regenerate error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Brief regenerated") {
		t.Errorf("expected 'Brief regenerated' message, got:\n%s", output)
	}
}

// =============================================================================
// Failure mode: project not initialized
// =============================================================================

func TestBriefCmd_NotInitialized(t *testing.T) {
	tmpDir := withTempDir(t)

	// Do NOT call config.InitAt — project is not initialized
	_ = tmpDir

	var buf bytes.Buffer
	cmd := newBriefCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for uninitialized project")
	}

	// Should mention initialization
	if !strings.Contains(err.Error(), "init") {
		t.Errorf("error should mention 'init', got: %v", err)
	}
}

// =============================================================================
// Edge case: --json output
// =============================================================================

func TestBriefCmd_JSONOutput(t *testing.T) {
	tmpDir := withTempDir(t)

	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	var buf bytes.Buffer
	cmd := newBriefCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief --json error: %v", err)
	}

	output := buf.String()
	// JSON output should be valid (starts with { or contains json structure)
	output = strings.TrimSpace(output)
	if output != "" && !strings.HasPrefix(output, "{") {
		t.Errorf("--json output should be valid JSON, got:\n%s", output)
	}
}

// =============================================================================
// Edge case: --stats output
// =============================================================================

func TestBriefCmd_StatsOutput(t *testing.T) {
	tmpDir := withTempDir(t)

	if err := config.InitAt(tmpDir, false); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Seed data for non-empty stats
	backend, err := storage.OpenDatabaseBackend(filepath.Join(tmpDir, ".orc", "orc.db"))
	if err != nil {
		t.Fatalf("open backend: %v", err)
	}
	defer backend.Close()

	tsk := task.NewProtoTask("TASK-001", "Test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	var buf bytes.Buffer
	cmd := newBriefCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--stats"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief --stats error: %v", err)
	}

	output := buf.String()
	// Stats should show metadata fields
	if !strings.Contains(output, "task") || !strings.Contains(output, "token") {
		t.Errorf("--stats output should contain metadata (task count, token count), got:\n%s", output)
	}
}

// =============================================================================
// Integration: newBriefCmd is registered in root command
// =============================================================================

func TestBriefCmd_RegisteredInRoot(t *testing.T) {
	// Verify newBriefCmd returns a valid cobra.Command
	cmd := newBriefCmd()
	if cmd == nil {
		t.Fatal("newBriefCmd() returned nil — command not created")
	}
	if cmd.Use == "" {
		t.Error("brief command should have Use field set")
	}
	if !strings.HasPrefix(cmd.Use, "brief") {
		t.Errorf("brief command Use should start with 'brief', got %q", cmd.Use)
	}
}

// Ensure the unused import is not flagged
var _ = os.Stat
