package cli

// Tests for the `orc migrate plans` command.
// SC-5: `orc migrate` command regenerates plans for all tasks or specified tasks.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// NOTE: These tests call newMigratePlansCmd() which does not exist yet.
// Tests will fail to compile until the function is implemented in cmd_migrate.go.

// createMigratePlanTestBackend creates a backend for testing migrate plans operations.
func createMigratePlanTestBackend(t *testing.T) (storage.Backend, string) {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory for project root detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	t.Cleanup(func() {
		_ = backend.Close()
	})
	return backend, tmpDir
}

func TestMigratePlansCmd_Exists(t *testing.T) {
	// Verify the migrate command exists and has the plans subcommand
	cmd := newMigrateCmd()

	// migrate should have plans subcommand
	var plansCmd *bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "plans" {
			plansCmd = new(bool)
			*plansCmd = true
			break
		}
	}

	if plansCmd == nil {
		t.Error("migrate command should have 'plans' subcommand")
	}
}

func TestMigratePlansCmd_Flags(t *testing.T) {
	// Tests the command flags exist
	cmd := newMigratePlansCmd()

	// Verify --all flag exists
	if cmd.Flag("all") == nil {
		t.Error("missing --all flag")
	}

	// Verify --dry-run flag exists
	if cmd.Flag("dry-run") == nil {
		t.Error("missing --dry-run flag")
	}
}

func TestMigratePlansCmd_SingleTask(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create a task with a stale plan (old phase sequence)
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	tk.Status = task.StatusPlanned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan (old format: implement → test instead of tiny_spec → implement → review)
	stalePlan := &plan.Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Old plan",
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Old inline prompt"},
			{ID: "test", Name: "test", Status: plan.PhasePending, Prompt: "Test prompt"},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Run migrate plans for single task
	cmd := newMigratePlansCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"TASK-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate plans failed: %v", err)
	}

	// Verify plan was migrated
	migratedPlan, err := backend.LoadPlan("TASK-001")
	if err != nil {
		t.Fatalf("load migrated plan: %v", err)
	}

	// Should have current small template phases (tiny_spec, implement, review)
	if len(migratedPlan.Phases) != 3 {
		t.Errorf("migrated plan phases = %d, want 3", len(migratedPlan.Phases))
	}

	// Check phase IDs match current template
	phaseIDs := make([]string, len(migratedPlan.Phases))
	for i, p := range migratedPlan.Phases {
		phaseIDs[i] = p.ID
	}
	expectedIDs := []string{"tiny_spec", "implement", "review"}
	for i, expected := range expectedIDs {
		if i >= len(phaseIDs) || phaseIDs[i] != expected {
			t.Errorf("phase %d = %q, want %q", i, phaseIDs[i], expected)
		}
	}

	// Verify inline prompts were cleared
	for _, phase := range migratedPlan.Phases {
		if phase.Prompt != "" {
			t.Errorf("phase %s still has inline Prompt=%q", phase.ID, phase.Prompt)
		}
	}
}

func TestMigratePlansCmd_MultipleTasks(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create multiple tasks with stale plans
	for _, id := range []string{"TASK-001", "TASK-002"} {
		tk := task.New(id, "Test task "+id)
		tk.Weight = task.WeightSmall
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %s: %v", id, err)
		}

		stalePlan := &plan.Plan{
			Version: 1,
			TaskID:  id,
			Weight:  task.WeightSmall,
			Phases: []plan.Phase{
				{ID: "implement", Name: "implement", Status: plan.PhasePending},
				{ID: "test", Name: "test", Status: plan.PhasePending},
			},
		}
		if err := backend.SavePlan(stalePlan, id); err != nil {
			t.Fatalf("save plan %s: %v", id, err)
		}
	}

	// Run migrate for specific tasks
	cmd := newMigratePlansCmd()
	cmd.SetArgs([]string{"TASK-001", "TASK-002"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate plans failed: %v", err)
	}

	// Both plans should be migrated
	for _, id := range []string{"TASK-001", "TASK-002"} {
		p, err := backend.LoadPlan(id)
		if err != nil {
			t.Fatalf("load plan %s: %v", id, err)
		}
		if len(p.Phases) != 3 {
			t.Errorf("plan %s phases = %d, want 3", id, len(p.Phases))
		}
	}
}

func TestMigratePlansCmd_AllFlag(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create several tasks
	taskIDs := []string{"TASK-001", "TASK-002", "TASK-003", "TASK-004", "TASK-005"}
	for i, id := range taskIDs {
		tk := task.New(id, "Test task")
		tk.Weight = task.WeightSmall
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %s: %v", id, err)
		}

		// Create stale plans for first 3
		if i < 3 {
			stalePlan := &plan.Plan{
				Version: 1,
				TaskID:  id,
				Weight:  task.WeightSmall,
				Phases: []plan.Phase{
					{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Old prompt"},
				},
			}
			if err := backend.SavePlan(stalePlan, id); err != nil {
				t.Fatalf("save plan %s: %v", id, err)
			}
		}
	}

	// Run migrate --all
	cmd := newMigratePlansCmd()
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate --all failed: %v", err)
	}

	// Tasks with stale plans should be migrated
	for i := 0; i < 3; i++ {
		id := taskIDs[i]
		p, err := backend.LoadPlan(id)
		if err != nil {
			t.Fatalf("load plan %s: %v", id, err)
		}
		// Should have current template phases
		if len(p.Phases) != 3 {
			t.Errorf("plan %s phases = %d, want 3 (current template)", id, len(p.Phases))
		}
	}
}

func TestMigratePlansCmd_DryRun(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task with stale plan
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	stalePlan := &plan.Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending},
			{ID: "test", Name: "test", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Run migrate with --dry-run
	cmd := newMigratePlansCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"TASK-001", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate --dry-run failed: %v", err)
	}

	// Plan should NOT be changed
	p, err := backend.LoadPlan("TASK-001")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}

	// Should still have old phases
	if len(p.Phases) != 2 {
		t.Errorf("plan phases = %d, want 2 (unchanged by dry run)", len(p.Phases))
	}

	// Output should indicate what would be migrated
	output := buf.String()
	if !hasSubstring(output, "TASK-001") {
		t.Error("dry-run output should mention the task ID")
	}
}

func TestMigratePlansCmd_SkipsCurrentPlans(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task with current (non-stale) plan
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create plan matching current template
	currentPlan, err := plan.CreateFromTemplate(tk)
	if err != nil {
		t.Fatalf("create plan from template: %v", err)
	}
	if err := backend.SavePlan(currentPlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Run migrate
	cmd := newMigratePlansCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"TASK-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Output should indicate plan was already current (or skipped)
	// The command should succeed but not modify the plan
	p, err := backend.LoadPlan("TASK-001")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}

	// Should still be current
	if len(p.Phases) != 3 {
		t.Errorf("plan phases = %d, want 3 (unchanged)", len(p.Phases))
	}
}

func TestMigratePlansCmd_PreservesCompletedPhases(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan with some completed phases
	stalePlan := &plan.Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: plan.PhaseCompleted, CommitSHA: "abc123"},
			{ID: "implement", Name: "implement", Status: plan.PhaseCompleted, CommitSHA: "def456"},
			{ID: "review", Name: "review", Status: plan.PhasePending, Prompt: "Old prompt"}, // Has inline prompt
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Run migrate
	cmd := newMigratePlansCmd()
	cmd.SetArgs([]string{"TASK-001"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Verify completed phases preserved
	p, err := backend.LoadPlan("TASK-001")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}

	tinySpec := p.GetPhase("tiny_spec")
	if tinySpec == nil {
		t.Fatal("tiny_spec phase not found")
	}
	if tinySpec.Status != plan.PhaseCompleted {
		t.Errorf("tiny_spec status = %s, want completed", tinySpec.Status)
	}
	if tinySpec.CommitSHA != "abc123" {
		t.Errorf("tiny_spec CommitSHA = %s, want abc123", tinySpec.CommitSHA)
	}

	impl := p.GetPhase("implement")
	if impl == nil {
		t.Fatal("implement phase not found")
	}
	if impl.Status != plan.PhaseCompleted {
		t.Errorf("implement status = %s, want completed", impl.Status)
	}

	// Verify inline prompt was cleared
	review := p.GetPhase("review")
	if review == nil {
		t.Fatal("review phase not found")
	}
	if review.Prompt != "" {
		t.Errorf("review Prompt = %q, want empty", review.Prompt)
	}
}

func TestMigratePlansCmd_TaskNotFound(t *testing.T) {
	_, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Run migrate for non-existent task
	cmd := newMigratePlansCmd()
	cmd.SetArgs([]string{"TASK-999"})
	err := cmd.Execute()

	// Should return error for not found
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestMigratePlansCmd_NoArgs(t *testing.T) {
	_, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Run migrate without args or --all
	cmd := newMigratePlansCmd()
	err := cmd.Execute()

	// Should require either task IDs or --all flag
	if err == nil {
		t.Error("expected error when no task IDs and no --all flag")
	}
}

func TestMigratePlansCmd_OutputShowsSummary(t *testing.T) {
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create several tasks with varying staleness
	taskIDs := []string{"TASK-001", "TASK-002", "TASK-003"}
	staleFlags := []bool{true, true, false}
	for i, stale := range staleFlags {
		id := taskIDs[i]
		tk := task.New(id, "Test task")
		tk.Weight = task.WeightSmall
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %s: %v", id, err)
		}

		var p *plan.Plan
		if stale {
			p = &plan.Plan{
				Version: 1,
				TaskID:  id,
				Weight:  task.WeightSmall,
				Phases: []plan.Phase{
					{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Old"},
				},
			}
		} else {
			var err error
			p, err = plan.CreateFromTemplate(tk)
			if err != nil {
				t.Fatalf("create plan: %v", err)
			}
		}
		if err := backend.SavePlan(p, id); err != nil {
			t.Fatalf("save plan %s: %v", id, err)
		}
	}

	// Run migrate --all
	cmd := newMigratePlansCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	// Output should show summary counts
	output := buf.String()

	// Should show number migrated and skipped
	if !hasSubstring(output, "migrated") && !hasSubstring(output, "Migrated") {
		t.Error("output should mention migrated count")
	}
}

func TestMigratePlansCmd_LargeTaskSet(t *testing.T) {
	// SC-5: orc migrate --all with 1000 tasks - process all, report summary
	// For test speed, we'll use 50 tasks
	backend, tmpDir := createMigratePlanTestBackend(t)

	// Set up working directory
	origDir := setupTestWorkDir(t, tmpDir)
	defer restoreWorkDir(t, origDir)

	// Create many tasks with stale plans
	taskCount := 50
	taskIDs := make([]string, taskCount)
	for i := 0; i < taskCount; i++ {
		taskIDs[i] = fmt.Sprintf("TASK-%03d", i+1)
	}

	for _, id := range taskIDs {
		tk := task.New(id, "Test task")
		tk.Weight = task.WeightSmall
		if err := backend.SaveTask(tk); err != nil {
			t.Fatalf("save task %s: %v", id, err)
		}

		stalePlan := &plan.Plan{
			Version: 1,
			TaskID:  id,
			Weight:  task.WeightSmall,
			Phases: []plan.Phase{
				{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Old"},
			},
		}
		if err := backend.SavePlan(stalePlan, id); err != nil {
			t.Fatalf("save plan %s: %v", id, err)
		}
	}

	// Run migrate --all
	cmd := newMigratePlansCmd()
	cmd.SetArgs([]string{"--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate --all failed: %v", err)
	}

	// Verify all were migrated
	migratedCount := 0
	for _, id := range taskIDs {
		p, err := backend.LoadPlan(id)
		if err != nil {
			t.Errorf("load plan %s: %v", id, err)
			continue
		}
		if len(p.Phases) == 3 {
			migratedCount++
		}
	}

	if migratedCount != taskCount {
		t.Errorf("migrated %d/%d tasks", migratedCount, taskCount)
	}
}

