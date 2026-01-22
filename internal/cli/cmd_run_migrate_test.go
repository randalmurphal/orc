package cli

// Integration tests for auto-migration of stale plans during `orc run`.
// SC-1: When running a task with stale plan, plan is auto-regenerated from current templates.
// SC-4: Plan migration is logged with clear message showing what changed.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// NOTE: These tests verify that `orc run` auto-migrates stale plans before execution.
// The tests call functions that don't exist yet and will fail until implemented:
// - checkAndMigrateStalePlan() or similar function in cmd_run.go

// createRunMigrateTestBackend creates a backend for testing run with migration.
func createRunMigrateTestBackend(t *testing.T) (storage.Backend, string) {
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

func TestCheckAndMigrateStalePlan_DetectsStale(t *testing.T) {
	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task with stale plan
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	tk.Status = task.StatusPlanned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan (old phase sequence with inline prompts)
	stalePlan := &plan.Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Do implementation"},
			{ID: "test", Name: "test", Status: plan.PhasePending, Prompt: "Write tests"},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Call checkAndMigrateStalePlan (this function will need to be implemented)
	migrated, reason, err := checkAndMigrateStalePlan(backend, tk)
	if err != nil {
		t.Fatalf("checkAndMigrateStalePlan failed: %v", err)
	}

	if !migrated {
		t.Error("checkAndMigrateStalePlan should return true for stale plan")
	}
	if reason == "" {
		t.Error("checkAndMigrateStalePlan should return reason for migration")
	}

	// Verify plan was migrated in database
	p, err := backend.LoadPlan("TASK-001")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}

	// Should have current template phases
	if len(p.Phases) != 3 {
		t.Errorf("migrated plan phases = %d, want 3", len(p.Phases))
	}

	// Inline prompts should be cleared
	for _, phase := range p.Phases {
		if phase.Prompt != "" {
			t.Errorf("phase %s has inline Prompt=%q", phase.ID, phase.Prompt)
		}
	}
}

func TestCheckAndMigrateStalePlan_SkipsCurrent(t *testing.T) {
	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	tk.Status = task.StatusPlanned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create current (non-stale) plan
	currentPlan, err := plan.CreateFromTemplate(tk)
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if err := backend.SavePlan(currentPlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// checkAndMigrateStalePlan should return false for current plan
	migrated, _, err := checkAndMigrateStalePlan(backend, tk)
	if err != nil {
		t.Fatalf("checkAndMigrateStalePlan failed: %v", err)
	}

	if migrated {
		t.Error("checkAndMigrateStalePlan should return false for current plan")
	}
}

func TestCheckAndMigrateStalePlan_PreservesCompleted(t *testing.T) {
	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	tk.Status = task.StatusRunning
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan with completed phase
	stalePlan := &plan.Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: plan.PhaseCompleted, CommitSHA: "abc123", Prompt: "Old"},
			{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Old"},
			{ID: "review", Name: "review", Status: plan.PhasePending, Prompt: "Old"},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Migrate
	_, _, err = checkAndMigrateStalePlan(backend, tk)
	if err != nil {
		t.Fatalf("checkAndMigrateStalePlan failed: %v", err)
	}

	// Verify completed phase preserved
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
	// But prompt should be cleared
	if tinySpec.Prompt != "" {
		t.Errorf("tiny_spec Prompt = %q, want empty", tinySpec.Prompt)
	}
}

func TestCheckAndMigrateStalePlan_LogsMigration(t *testing.T) {
	// SC-4: Plan migration is logged with clear message showing what changed
	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan
	stalePlan := &plan.Plan{
		Version: 0, // Old version
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// The reason should explain what changed
	_, reason, err := checkAndMigrateStalePlan(backend, tk)
	if err != nil {
		t.Fatalf("checkAndMigrateStalePlan failed: %v", err)
	}

	// Reason should indicate why migration was needed
	if reason == "" {
		t.Error("reason should explain why migration was performed")
	}
}

func TestCheckAndMigrateStalePlan_NoPlan(t *testing.T) {
	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task without a plan
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// checkAndMigrateStalePlan with no plan - behavior depends on implementation
	// Could either: create a new plan, or return error
	// The run command would typically create a plan if none exists
	migrated, _, err := checkAndMigrateStalePlan(backend, tk)
	// Either outcome is acceptable, but should not panic
	if err != nil {
		// If error, it should be descriptive
		t.Logf("checkAndMigrateStalePlan with no plan returned error: %v", err)
	} else if !migrated {
		t.Logf("checkAndMigrateStalePlan with no plan returned false (no migration)")
	} else {
		// If migrated, a plan should now exist
		_, planErr := backend.LoadPlan("TASK-001")
		if planErr != nil {
			t.Errorf("plan should exist after migration, got error: %v", planErr)
		}
	}
}

func TestRunCmd_MigratesStalePlan(t *testing.T) {
	// Integration test: `orc run` should migrate stale plan before execution
	// This test sets up a stale plan and verifies the run command would migrate it
	// Note: Full run command requires executor setup, so we test the migration path

	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	tk.Status = task.StatusPlanned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan
	stalePlan := &plan.Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhasePending, Prompt: "Legacy prompt"},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Test that IsPlanStale detects the staleness
	stale, reason := plan.IsPlanStale(stalePlan, tk)
	if !stale {
		t.Error("IsPlanStale should detect stale plan")
	}
	t.Logf("Staleness reason: %s", reason)

	// Simulate what run would do: check staleness and migrate
	if stale {
		result, err := plan.MigratePlan(tk, stalePlan)
		if err != nil {
			t.Fatalf("MigratePlan failed: %v", err)
		}

		// Verify migration result
		if result.NewPlan == nil {
			t.Fatal("MigratePlan returned nil NewPlan")
		}
		if len(result.NewPlan.Phases) != 3 {
			t.Errorf("migrated plan phases = %d, want 3", len(result.NewPlan.Phases))
		}

		// Verify prompts cleared
		for _, phase := range result.NewPlan.Phases {
			if phase.Prompt != "" {
				t.Errorf("phase %s Prompt = %q, want empty", phase.ID, phase.Prompt)
			}
		}
	}
}

func TestRunCmd_OutputShowsMigration(t *testing.T) {
	// SC-4: When migration happens, output should show what changed
	backend, tmpDir := createRunMigrateTestBackend(t)

	// Set up working directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create task
	tk := task.New("TASK-001", "Test task")
	tk.Weight = task.WeightSmall
	tk.Status = task.StatusPlanned
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create stale plan with old phases
	stalePlan := &plan.Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []plan.Phase{
			{ID: "implement", Name: "implement", Status: plan.PhaseCompleted, CommitSHA: "abc"},
			{ID: "test", Name: "test", Status: plan.PhasePending},
		},
	}
	if err := backend.SavePlan(stalePlan, "TASK-001"); err != nil {
		t.Fatalf("save plan: %v", err)
	}

	// Call logMigration or similar function to output migration info
	// This tests that the output formatting is correct
	var buf bytes.Buffer

	result, err := plan.MigratePlan(tk, stalePlan)
	if err != nil {
		t.Fatalf("MigratePlan failed: %v", err)
	}

	// logMigrationResult should format the migration info for user output
	logMigrationResult(&buf, "TASK-001", result)

	output := buf.String()

	// Output should show old phases
	if !hasSubstring(output, "implement") {
		t.Error("migration output should mention 'implement' phase")
	}

	// Output should show preserved count
	if !hasSubstring(output, "preserved") || !hasSubstring(output, "Preserved") {
		t.Error("migration output should mention preserved phases")
	}
}

// NOTE: The following functions are referenced but don't exist yet:
// - checkAndMigrateStalePlan() - should be in cmd_run.go
// - logMigrationResult() - should be in cmd_run.go or a helper
// - plan.IsPlanStale() - should be in internal/plan/migrate.go
// - plan.MigratePlan() - should be in internal/plan/migrate.go
// - plan.MigrationResult - should be in internal/plan/migrate.go
//
// Tests will fail to compile until these are implemented.
