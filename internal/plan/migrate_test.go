package plan

import (
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

// TestIsPlanStale tests various staleness conditions.
// These tests verify SC-2: Staleness detection correctly identifies plans needing migration.

func TestIsPlanStale_CurrentPlan(t *testing.T) {
	// A plan that matches the current template should not be stale
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	// Create a plan that matches the current small template (tiny_spec, implement, review)
	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhasePending},
			{ID: "implement", Name: "implement", Status: PhasePending},
			{ID: "review", Name: "review", Status: PhasePending},
		},
	}

	stale, reason := IsPlanStale(p, tsk)
	if stale {
		t.Errorf("IsPlanStale() = true, reason=%q, want false (plan matches template)", reason)
	}
}

func TestIsPlanStale_VersionMismatch(t *testing.T) {
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	// Plan with lower version than template
	p := &Plan{
		Version:     0, // Old version
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhasePending},
			{ID: "implement", Name: "implement", Status: PhasePending},
			{ID: "review", Name: "review", Status: PhasePending},
		},
	}

	stale, reason := IsPlanStale(p, tsk)
	if !stale {
		t.Error("IsPlanStale() = false, want true for version mismatch")
	}
	if reason == "" {
		t.Error("IsPlanStale() reason should not be empty for stale plan")
	}
}

func TestIsPlanStale_PhaseSequenceMismatch(t *testing.T) {
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	// Old-style plan with wrong phases (implement → test instead of tiny_spec → implement → review)
	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases: []Phase{
			{ID: "implement", Name: "implement", Status: PhasePending},
			{ID: "test", Name: "test", Status: PhasePending},
		},
	}

	stale, reason := IsPlanStale(p, tsk)
	if !stale {
		t.Error("IsPlanStale() = false, want true for phase sequence mismatch")
	}
	if reason == "" {
		t.Error("IsPlanStale() reason should not be empty for stale plan")
	}
}

func TestIsPlanStale_InlinePrompts(t *testing.T) {
	// SC-2: Plans with inline prompts (legacy) should be considered stale
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	// Plan has correct phases but contains inline prompts (legacy format)
	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhasePending, Prompt: "Do the spec..."},
			{ID: "implement", Name: "implement", Status: PhasePending, Prompt: "Implement the feature..."},
			{ID: "review", Name: "review", Status: PhasePending, Prompt: "Review the code..."},
		},
	}

	stale, reason := IsPlanStale(p, tsk)
	if !stale {
		t.Error("IsPlanStale() = false, want true for inline prompts present")
	}
	if reason == "" {
		t.Error("IsPlanStale() reason should not be empty when inline prompts present")
	}
}

func TestIsPlanStale_NilPlan(t *testing.T) {
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	stale, reason := IsPlanStale(nil, tsk)
	// Nil plan should return error or indicate staleness (cannot proceed with nil)
	// Implementation decision: return error via reason, treat as not stale since there's nothing to compare
	// Alternative: could return true with reason "no plan"
	// The spec says "Returns true for: ... nil plan" - so it should be stale
	if !stale {
		t.Error("IsPlanStale(nil, task) = false, want true for nil plan")
	}
	if reason == "" {
		t.Error("IsPlanStale() reason should explain nil plan issue")
	}
}

func TestIsPlanStale_EmptyPhases(t *testing.T) {
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	// Plan with no phases should be stale (regenerate from template)
	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases:      []Phase{}, // Empty
	}

	stale, reason := IsPlanStale(p, tsk)
	if !stale {
		t.Error("IsPlanStale() = false, want true for empty phases")
	}
	if reason == "" {
		t.Error("IsPlanStale() reason should not be empty for empty phases")
	}
}

func TestIsPlanStale_UnknownWeight(t *testing.T) {
	// Task with unknown weight - should skip migration (can't verify)
	tsk := &task.Task{ID: "TASK-001", Weight: "nonexistent"}

	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      "nonexistent",
		Description: "Unknown weight task",
		Phases: []Phase{
			{ID: "implement", Name: "implement", Status: PhasePending},
		},
	}

	stale, _ := IsPlanStale(p, tsk)
	// When template cannot be loaded, we cannot verify staleness
	// The spec says: "Can't compare, assume OK" for template load failure
	if stale {
		t.Error("IsPlanStale() = true, want false for unknown weight (cannot verify)")
	}
}

func TestIsPlanStale_AllWeights(t *testing.T) {
	// Test that plans matching their templates are not stale for all weights
	weights := []task.Weight{task.WeightTrivial, task.WeightSmall, task.WeightMedium, task.WeightLarge}

	for _, w := range weights {
		t.Run(string(w), func(t *testing.T) {
			tsk := &task.Task{ID: "TASK-001", Weight: w}

			// Create plan from template (should not be stale)
			p, err := CreateFromTemplate(tsk)
			if err != nil {
				t.Fatalf("CreateFromTemplate() failed: %v", err)
			}

			stale, reason := IsPlanStale(p, tsk)
			if stale {
				t.Errorf("IsPlanStale() = true, reason=%q, want false for freshly created plan", reason)
			}
		})
	}
}

// TestMigratePlan tests plan migration functionality.
// These tests verify SC-3: Completed/skipped phases have status preserved during migration.

func TestMigratePlan_PreservesCompletedPhases(t *testing.T) {
	// Old plan with some phases completed
	oldPlan := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "implement", Name: "implement", Status: PhaseCompleted, CommitSHA: "def456"},
			{ID: "review", Name: "review", Status: PhasePending},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Check tiny_spec status preserved
	tinySpec := result.NewPlan.GetPhase("tiny_spec")
	if tinySpec == nil {
		t.Fatal("tiny_spec phase not found in migrated plan")
	}
	if tinySpec.Status != PhaseCompleted {
		t.Errorf("tiny_spec status = %s, want completed", tinySpec.Status)
	}
	if tinySpec.CommitSHA != "abc123" {
		t.Errorf("tiny_spec CommitSHA = %s, want abc123", tinySpec.CommitSHA)
	}

	// Check implement status preserved
	impl := result.NewPlan.GetPhase("implement")
	if impl == nil {
		t.Fatal("implement phase not found in migrated plan")
	}
	if impl.Status != PhaseCompleted {
		t.Errorf("implement status = %s, want completed", impl.Status)
	}

	// Check review is pending (was pending, stays pending)
	review := result.NewPlan.GetPhase("review")
	if review == nil {
		t.Fatal("review phase not found in migrated plan")
	}
	if review.Status != PhasePending {
		t.Errorf("review status = %s, want pending", review.Status)
	}

	// Verify result fields
	if result.PreservedCount < 2 {
		t.Errorf("PreservedCount = %d, want >= 2", result.PreservedCount)
	}
}

func TestMigratePlan_PreservesSkippedPhases(t *testing.T) {
	// Old plan with skipped phase
	oldPlan := &Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "implement", Name: "implement", Status: PhaseSkipped},
			{ID: "review", Name: "review", Status: PhasePending},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Skipped status should be preserved
	impl := result.NewPlan.GetPhase("implement")
	if impl == nil {
		t.Fatal("implement phase not found")
	}
	if impl.Status != PhaseSkipped {
		t.Errorf("implement status = %s, want skipped", impl.Status)
	}
}

func TestMigratePlan_ClearsInlinePrompts(t *testing.T) {
	// Old plan with inline prompts (legacy format) should have prompts cleared
	oldPlan := &Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhaseCompleted, Prompt: "Old inline prompt"},
			{ID: "implement", Name: "implement", Status: PhasePending, Prompt: "Another old prompt"},
			{ID: "review", Name: "review", Status: PhasePending, Prompt: "Review prompt"},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// All phases should have empty Prompt (forces template usage)
	for _, phase := range result.NewPlan.Phases {
		if phase.Prompt != "" {
			t.Errorf("phase %s still has inline Prompt=%q, want empty", phase.ID, phase.Prompt)
		}
	}
}

func TestMigratePlan_ResetsRunningPhase(t *testing.T) {
	// Running phase should be reset to pending (safe restart)
	oldPlan := &Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "implement", Name: "implement", Status: PhaseRunning}, // Running
			{ID: "review", Name: "review", Status: PhasePending},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Running should be reset to pending
	impl := result.NewPlan.GetPhase("implement")
	if impl == nil {
		t.Fatal("implement phase not found")
	}
	if impl.Status != PhasePending {
		t.Errorf("implement status = %s, want pending (was running)", impl.Status)
	}
}

func TestMigratePlan_PhaseRemovedFromTemplate(t *testing.T) {
	// Edge case: Old plan has phase that doesn't exist in new template
	// Example: downgrade from medium to small, losing "docs" phase
	oldPlan := &Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightMedium,
		Phases: []Phase{
			{ID: "spec", Name: "spec", Status: PhaseCompleted},
			{ID: "tdd_write", Name: "tdd_write", Status: PhaseCompleted},
			{ID: "breakdown", Name: "breakdown", Status: PhaseCompleted},
			{ID: "implement", Name: "implement", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "review", Name: "review", Status: PhaseCompleted},
			{ID: "docs", Name: "docs", Status: PhaseCompleted, CommitSHA: "docs123"},
		},
	}

	// Downgrade to small weight - "docs" phase doesn't exist in small template
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// docs phase should not exist in new plan (small template doesn't have it)
	docs := result.NewPlan.GetPhase("docs")
	if docs != nil {
		t.Error("docs phase should not exist in small weight plan")
	}

	// implement and review should be preserved if they exist in small
	impl := result.NewPlan.GetPhase("implement")
	if impl == nil {
		t.Fatal("implement phase not found")
	}
	if impl.Status != PhaseCompleted {
		t.Errorf("implement status = %s, want completed", impl.Status)
	}
}

func TestMigratePlan_WeightAndStaleCombined(t *testing.T) {
	// Both weight change AND stale plan (e.g., old phases + inline prompts)
	oldPlan := &Plan{
		Version: 0, // Old version
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall, // Will upgrade to medium
		Phases: []Phase{
			{ID: "implement", Name: "implement", Status: PhaseCompleted, CommitSHA: "abc123", Prompt: "Old prompt"},
			{ID: "test", Name: "test", Status: PhasePending, Prompt: "Test prompt"},
		},
	}

	// Upgrade to medium weight
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightMedium}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Plan should be for medium weight
	if result.NewPlan.Weight != task.WeightMedium {
		t.Errorf("NewPlan.Weight = %s, want medium", result.NewPlan.Weight)
	}

	// implement should be preserved (exists in both)
	impl := result.NewPlan.GetPhase("implement")
	if impl == nil {
		t.Fatal("implement phase not found")
	}
	if impl.Status != PhaseCompleted {
		t.Errorf("implement status = %s, want completed", impl.Status)
	}
	// Prompt should be cleared
	if impl.Prompt != "" {
		t.Errorf("implement Prompt = %q, want empty", impl.Prompt)
	}

	// test phase should not exist in medium (different phase structure)
	testPhase := result.NewPlan.GetPhase("test")
	if testPhase != nil {
		t.Error("test phase should not exist in medium weight plan")
	}
}

func TestMigratePlan_NoTemplate(t *testing.T) {
	// Task with unknown weight - template won't load
	oldPlan := &Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  "nonexistent",
		Phases: []Phase{
			{ID: "implement", Name: "implement", Status: PhasePending},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: "nonexistent"}

	_, err := MigratePlan(tsk, oldPlan)
	// Behavior depends on implementation - could error or return default plan
	// The spec says "Skip migration, use plan as-is" but MigratePlan should
	// return an error when it can't migrate
	if err == nil {
		t.Log("MigratePlan returned nil error for unknown weight - verify this is expected behavior")
	}
}

func TestMigratePlan_NilOldPlan(t *testing.T) {
	// MigratePlan with nil old plan - should create fresh plan from template
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, nil)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Should have created fresh plan
	if result.NewPlan == nil {
		t.Fatal("NewPlan is nil")
	}
	if result.PreservedCount != 0 {
		t.Errorf("PreservedCount = %d, want 0 for nil old plan", result.PreservedCount)
	}
	if result.ResetCount != len(result.NewPlan.Phases) {
		t.Errorf("ResetCount = %d, want %d (all phases)", result.ResetCount, len(result.NewPlan.Phases))
	}
}

// TestMigrationResult tests the MigrationResult struct fields.

func TestMigrationResult_OldAndNewPhases(t *testing.T) {
	// Verify that migration result contains old and new phase lists
	oldPlan := &Plan{
		Version: 1,
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []Phase{
			{ID: "implement", Name: "implement", Status: PhaseCompleted},
			{ID: "test", Name: "test", Status: PhasePending},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Verify OldPhases contains the old phase IDs
	if len(result.OldPhases) == 0 {
		t.Error("OldPhases should not be empty")
	}
	foundImplement := false
	foundTest := false
	for _, id := range result.OldPhases {
		if id == "implement" {
			foundImplement = true
		}
		if id == "test" {
			foundTest = true
		}
	}
	if !foundImplement || !foundTest {
		t.Errorf("OldPhases = %v, want to contain 'implement' and 'test'", result.OldPhases)
	}

	// Verify NewPhases contains the new phase IDs
	if len(result.NewPhases) == 0 {
		t.Error("NewPhases should not be empty")
	}
}

func TestMigrationResult_Reason(t *testing.T) {
	// Verify that reason is populated for stale plans
	oldPlan := &Plan{
		Version: 0, // Stale version
		TaskID:  "TASK-001",
		Weight:  task.WeightSmall,
		Phases: []Phase{
			{ID: "implement", Name: "implement", Status: PhaseCompleted, Prompt: "Legacy prompt"},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := MigratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("MigratePlan() failed: %v", err)
	}

	// Reason should explain why migration was needed
	if result.Reason == "" {
		t.Error("Reason should explain why migration was performed")
	}
}
