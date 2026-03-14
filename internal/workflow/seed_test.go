package workflow

import (
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

// openTestGlobalDB opens a global DB in a temp directory for testing
func openTestGlobalDB(t *testing.T) *db.GlobalDB {
	t.Helper()
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, ".orc"))
	if err != nil {
		t.Fatalf("failed to open global db: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Close() })
	return gdb
}

func TestSeedBuiltins(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	// Seed built-ins
	seeded, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	// Should have seeded at least phase templates and workflows
	if seeded == 0 {
		t.Error("expected to seed at least one item")
	}

	// Verify phase templates exist
	templates, err := gdb.ListPhaseTemplates()
	if err != nil {
		t.Fatalf("ListPhaseTemplates failed: %v", err)
	}
	if len(templates) == 0 {
		t.Error("expected phase templates to be seeded")
	}

	// Check specific built-in phase templates
	expectedPhases := []string{"spec", "tiny_spec", "tdd_write", "breakdown", "implement", "review", "docs"}
	for _, id := range expectedPhases {
		pt, err := gdb.GetPhaseTemplate(id)
		if err != nil {
			t.Errorf("GetPhaseTemplate(%s) failed: %v", id, err)
			continue
		}
		if pt == nil {
			t.Errorf("phase template %s not found", id)
			continue
		}
		if !pt.IsBuiltin {
			t.Errorf("phase template %s should be marked as builtin", id)
		}
	}

	// Verify workflows exist
	workflows, err := gdb.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows failed: %v", err)
	}
	if len(workflows) == 0 {
		t.Error("expected workflows to be seeded")
	}

	// Check specific built-in workflows
	expectedWorkflows := []string{"implement-large", "implement-medium", "implement-small", "implement-trivial", "review", "spec", "docs", "qa"}
	for _, id := range expectedWorkflows {
		wf, err := gdb.GetWorkflow(id)
		if err != nil {
			t.Errorf("GetWorkflow(%s) failed: %v", id, err)
			continue
		}
		if wf == nil {
			t.Errorf("workflow %s not found", id)
			continue
		}
		if !wf.IsBuiltin {
			t.Errorf("workflow %s should be marked as builtin", id)
		}
	}

	// Verify workflow phases exist
	phases, err := gdb.GetWorkflowPhases("implement-medium")
	if err != nil {
		t.Fatalf("GetWorkflowPhases(implement-medium) failed: %v", err)
	}
	if len(phases) == 0 {
		t.Error("expected implement-medium workflow to have phases")
	}
}

func TestSeedBuiltinsIdempotent(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	// Seed twice — both should succeed without error
	seeded1, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("first SeedBuiltins failed: %v", err)
	}

	seeded2, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("second SeedBuiltins failed: %v", err)
	}

	// Both calls should report items (embedded sources always upsert to keep DB in sync)
	if seeded1 == 0 {
		t.Error("expected first SeedBuiltins to seed items")
	}
	if seeded2 == 0 {
		t.Error("expected second SeedBuiltins to update items")
	}

	// Both should report the same count (same source data)
	if seeded1 != seeded2 {
		t.Errorf("expected consistent seed count: first=%d second=%d", seeded1, seeded2)
	}
}

func TestListBuiltinWorkflowIDs(t *testing.T) {
	t.Parallel()

	ids := ListBuiltinWorkflowIDs()
	if len(ids) == 0 {
		t.Error("expected at least one built-in workflow ID")
	}

	// Check expected workflows are in the list
	expected := map[string]bool{
		"implement-large":   false,
		"implement-medium":  false,
		"implement-small":   false,
		"implement-trivial": false,
		"review":            false,
	}

	for _, id := range ids {
		if _, ok := expected[id]; ok {
			expected[id] = true
		}
	}

	for id, found := range expected {
		if !found {
			t.Errorf("expected workflow %s to be in built-in list", id)
		}
	}
}

func TestListBuiltinPhaseIDs(t *testing.T) {
	t.Parallel()

	ids := ListBuiltinPhaseIDs()
	if len(ids) == 0 {
		t.Error("expected at least one built-in phase ID")
	}

	// Check expected phases are in the list
	expected := map[string]bool{
		"spec":      false,
		"tiny_spec": false,
		"implement": false,
		"review":    false,
	}

	for _, id := range ids {
		if _, ok := expected[id]; ok {
			expected[id] = true
		}
	}

	for id, found := range expected {
		if !found {
			t.Errorf("expected phase %s to be in built-in list", id)
		}
	}
}

func TestBuiltinPhaseTemplatesHaveRequiredFields(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	_, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	templates, err := gdb.ListPhaseTemplates()
	if err != nil {
		t.Fatalf("ListPhaseTemplates failed: %v", err)
	}

	for _, pt := range templates {
		if pt.ID == "" {
			t.Error("phase template has empty ID")
		}
		if pt.Name == "" {
			t.Errorf("phase template %s has empty Name", pt.ID)
		}
		if pt.PromptSource == "" {
			t.Errorf("phase template %s has empty PromptSource", pt.ID)
		}
		if pt.PromptSource == "embedded" && pt.PromptPath == "" {
			t.Errorf("phase template %s has embedded source but empty PromptPath", pt.ID)
		}
	}
}

func TestImplementCodexPhaseHasQualityChecks(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	_, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	implement, err := gdb.GetPhaseTemplate("implement")
	if err != nil {
		t.Fatalf("GetPhaseTemplate(implement) failed: %v", err)
	}
	implementCodex, err := gdb.GetPhaseTemplate("implement_codex")
	if err != nil {
		t.Fatalf("GetPhaseTemplate(implement_codex) failed: %v", err)
	}

	if implement == nil || implementCodex == nil {
		t.Fatal("implement and implement_codex phase templates must both exist")
	}

	if implement.QualityChecks == "" {
		t.Fatal("implement phase should define quality checks")
	}
	if implementCodex.QualityChecks == "" {
		t.Fatal("implement_codex phase should define quality checks")
	}
	if implementCodex.QualityChecks != implement.QualityChecks {
		t.Fatalf("implement_codex quality checks = %q, want %q", implementCodex.QualityChecks, implement.QualityChecks)
	}
}

func TestBuiltinWorkflowsHavePhases(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	_, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	workflows, err := gdb.ListWorkflows()
	if err != nil {
		t.Fatalf("ListWorkflows failed: %v", err)
	}

	for _, wf := range workflows {
		phases, err := gdb.GetWorkflowPhases(wf.ID)
		if err != nil {
			t.Errorf("GetWorkflowPhases(%s) failed: %v", wf.ID, err)
			continue
		}
		if len(phases) == 0 {
			t.Errorf("workflow %s has no phases", wf.ID)
		}

		// Verify phases are in sequence order
		for i, phase := range phases {
			if phase.Sequence != i {
				t.Errorf("workflow %s phase %d has wrong sequence: got %d, want %d",
					wf.ID, i, phase.Sequence, i)
			}
		}
	}
}

func TestSeedBuiltins_CrossModelStandardPersistsPhaseOverrides(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	_, err := SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	phases, err := gdb.GetWorkflowPhases("crossmodel-standard")
	if err != nil {
		t.Fatalf("GetWorkflowPhases(crossmodel-standard) failed: %v", err)
	}

	if len(phases) == 0 {
		t.Fatal("expected crossmodel-standard to have phases")
	}

	byID := make(map[string]*db.WorkflowPhase, len(phases))
	for _, phase := range phases {
		byID[phase.PhaseTemplateID] = phase
	}

	implement := byID["implement_codex"]
	if implement == nil {
		t.Fatal("implement_codex phase missing from crossmodel-standard")
	}
	if got := implement.ModelOverride; got != "gpt-5.4" {
		t.Fatalf("implement_codex model_override = %q, want %q", got, "gpt-5.4")
	}
	if got := implement.ProviderOverride; got != "" {
		t.Fatalf("implement_codex provider_override = %q, want empty string", got)
	}

	reviewCross := byID["review_cross"]
	if reviewCross == nil {
		t.Fatal("review_cross phase missing from crossmodel-standard")
	}
	if got := reviewCross.ProviderOverride; got != "codex" {
		t.Fatalf("review_cross provider_override = %q, want %q", got, "codex")
	}
	if got := reviewCross.ModelOverride; got != "gpt-5.4" {
		t.Fatalf("review_cross model_override = %q, want %q", got, "gpt-5.4")
	}

	docs := byID["docs"]
	if docs == nil {
		t.Fatal("docs phase missing from crossmodel-standard")
	}
	if got := docs.ProviderOverride; got != "codex" {
		t.Fatalf("docs provider_override = %q, want %q", got, "codex")
	}
	if got := docs.ModelOverride; got != "gpt-5.4" {
		t.Fatalf("docs model_override = %q, want %q", got, "gpt-5.4")
	}
}
