package bench

import (
	"context"
	"testing"
)

func TestStoreProjectCRUD(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create
	p := &Project{
		ID:         "bbolt",
		RepoURL:    "https://github.com/etcd-io/bbolt",
		CommitHash: "abc123",
		Language:   "go",
		TestCmd:    "go test ./...",
		BuildCmd:   "go build ./...",
		LintCmd:    "golangci-lint run",
	}
	if err := store.SaveProject(ctx, p); err != nil {
		t.Fatalf("save project: %v", err)
	}

	// Read
	got, err := store.GetProject(ctx, "bbolt")
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	if got.ID != "bbolt" || got.Language != "go" || got.TestCmd != "go test ./..." {
		t.Errorf("project mismatch: got %+v", got)
	}

	// Update (upsert)
	p.CommitHash = "def456"
	if err := store.SaveProject(ctx, p); err != nil {
		t.Fatalf("update project: %v", err)
	}
	got, _ = store.GetProject(ctx, "bbolt")
	if got.CommitHash != "def456" {
		t.Errorf("expected updated commit hash def456, got %s", got.CommitHash)
	}

	// List
	projects, err := store.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}

	// Delete
	if err := store.DeleteProject(ctx, "bbolt"); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	projects, _ = store.ListProjects(ctx)
	if len(projects) != 0 {
		t.Errorf("expected 0 projects after delete, got %d", len(projects))
	}
}

func TestStoreTaskCRUD(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Need a project first
	p := &Project{ID: "zod", RepoURL: "https://github.com/colinhacks/zod", CommitHash: "abc", Language: "typescript", TestCmd: "npm test"}
	store.SaveProject(ctx, p)

	task := &Task{
		ID:             "zod-001",
		ProjectID:      "zod",
		Tier:           TierMedium,
		Category:       "bug",
		Title:          "Fix schema parsing",
		Description:    "The schema parser fails on nested objects...",
		PreFixCommit:   "aaa111",
		ReferencePRURL: "https://github.com/colinhacks/zod/pull/42",
		TestPatch:      "diff --git a/test.ts b/test.ts\n+test content",
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}

	got, err := store.GetTask(ctx, "zod-001")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Tier != TierMedium {
		t.Errorf("expected tier medium, got %s", got.Tier)
	}
	if got.Category != "bug" {
		t.Errorf("expected category bug, got %s", got.Category)
	}
	if got.TestPatch == "" {
		t.Error("expected test_patch to be populated")
	}

	// List with filter
	tasks, err := store.ListTasks(ctx, "zod", "")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	tasks, err = store.ListTasks(ctx, "", TierSmall)
	if err != nil {
		t.Fatalf("list tasks by tier: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 small tasks, got %d", len(tasks))
	}
}

func TestStoreVariantCRUD(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	v := &Variant{
		ID:           "codex53-high-implement",
		Name:         "Codex 5.3 High Implement",
		BaseWorkflow: "medium",
		PhaseOverrides: map[string]PhaseOverride{
			"implement": {
				Provider:        "codex",
				Model:           "gpt-5.3-codex",
				ReasoningEffort: "high",
			},
		},
	}
	if err := store.SaveVariant(ctx, v); err != nil {
		t.Fatalf("save variant: %v", err)
	}

	got, err := store.GetVariant(ctx, "codex53-high-implement")
	if err != nil {
		t.Fatalf("get variant: %v", err)
	}
	if got.BaseWorkflow != "medium" {
		t.Errorf("expected workflow medium, got %s", got.BaseWorkflow)
	}
	if override, ok := got.PhaseOverrides["implement"]; !ok {
		t.Error("expected implement phase override")
	} else if override.Model != "gpt-5.3-codex" {
		t.Errorf("expected model gpt-5.3-codex, got %s", override.Model)
	}

	// Baseline
	baseline := &Variant{
		ID:           "baseline",
		Name:         "All Opus",
		BaseWorkflow: "medium",
		IsBaseline:   true,
	}
	store.SaveVariant(ctx, baseline)

	got, err = store.GetBaselineVariant(ctx)
	if err != nil {
		t.Fatalf("get baseline: %v", err)
	}
	if got.ID != "baseline" {
		t.Errorf("expected baseline id, got %s", got.ID)
	}

	// List (baseline should be first)
	variants, err := store.ListVariants(ctx)
	if err != nil {
		t.Fatalf("list variants: %v", err)
	}
	if len(variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(variants))
	}
	if !variants[0].IsBaseline {
		t.Error("expected baseline variant first in list")
	}
}

func TestStoreRunAndPhaseResults(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup
	store.SaveProject(ctx, &Project{ID: "test", RepoURL: "http://test", CommitHash: "abc", Language: "go", TestCmd: "go test"})
	store.SaveTask(ctx, &Task{ID: "test-001", ProjectID: "test", Tier: TierSmall, Title: "t", Description: "d", PreFixCommit: "abc"})
	store.SaveVariant(ctx, &Variant{ID: "baseline", Name: "b", BaseWorkflow: "small", IsBaseline: true})

	run := &Run{
		ID:          "run-001",
		VariantID:   "baseline",
		TaskID:      "test-001",
		TrialNumber: 1,
		Status:      RunStatusRunning,
	}
	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("save run: %v", err)
	}

	// Update status
	run.Status = RunStatusPass
	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("update run: %v", err)
	}

	got, err := store.GetRun(ctx, "run-001")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got.Status != RunStatusPass {
		t.Errorf("expected pass, got %s", got.Status)
	}

	// Phase results
	pr := &PhaseResult{
		RunID:       "run-001",
		PhaseID:     "implement",
		Provider:    "claude",
		Model:       "opus",
		InputTokens: 1000,
		OutputTokens: 500,
		CostUSD:     0.05,
		DurationMs:  30000,
		TestPass:    true,
		TestCount:   5,
	}
	if err := store.SavePhaseResult(ctx, pr); err != nil {
		t.Fatalf("save phase result: %v", err)
	}
	if pr.ID == 0 {
		t.Error("expected auto-increment ID to be set")
	}

	results, err := store.GetPhaseResults(ctx, "run-001")
	if err != nil {
		t.Fatalf("get phase results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 phase result, got %d", len(results))
	}
	if results[0].CostUSD != 0.05 {
		t.Errorf("expected cost 0.05, got %f", results[0].CostUSD)
	}

	// Run counts
	pass, fail, errCount, err := store.CountRunsByStatus(ctx, "baseline")
	if err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if pass != 1 || fail != 0 || errCount != 0 {
		t.Errorf("expected 1/0/0, got %d/%d/%d", pass, fail, errCount)
	}
}

func TestStoreFrozenOutputs(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	fo := &FrozenOutput{
		ID:            "fo-001",
		TaskID:        "test-001",
		PhaseID:       "spec",
		VariantID:     "baseline",
		TrialNumber:   1,
		OutputContent: `{"content": "The specification is..."}`,
		OutputVarName: "SPEC_CONTENT",
	}
	if err := store.SaveFrozenOutput(ctx, fo); err != nil {
		t.Fatalf("save frozen output: %v", err)
	}

	got, err := store.GetFrozenOutput(ctx, "test-001", "spec", "baseline", 1)
	if err != nil {
		t.Fatalf("get frozen output: %v", err)
	}
	if got.OutputVarName != "SPEC_CONTENT" {
		t.Errorf("expected SPEC_CONTENT, got %s", got.OutputVarName)
	}

	// Save another for same task
	fo2 := &FrozenOutput{
		ID:            "fo-002",
		TaskID:        "test-001",
		PhaseID:       "tdd_write",
		VariantID:     "baseline",
		TrialNumber:   1,
		OutputContent: `{"content": "Tests..."}`,
		OutputVarName: "TDD_TESTS_CONTENT",
	}
	store.SaveFrozenOutput(ctx, fo2)

	outputs, err := store.GetFrozenOutputsForTask(ctx, "test-001", "baseline", 1)
	if err != nil {
		t.Fatalf("get frozen outputs for task: %v", err)
	}
	if len(outputs) != 2 {
		t.Errorf("expected 2 frozen outputs, got %d", len(outputs))
	}

	// Upsert (same task+phase+variant+trial replaces)
	fo.OutputContent = "updated"
	if err := store.SaveFrozenOutput(ctx, fo); err != nil {
		t.Fatalf("upsert frozen output: %v", err)
	}
	got, _ = store.GetFrozenOutput(ctx, "test-001", "spec", "baseline", 1)
	if got.OutputContent != "updated" {
		t.Errorf("expected updated content, got %s", got.OutputContent)
	}
}

func TestStoreJudgments(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup (need valid run for FK)
	store.SaveProject(ctx, &Project{ID: "test", RepoURL: "http://test", CommitHash: "abc", Language: "go", TestCmd: "go test"})
	store.SaveTask(ctx, &Task{ID: "t-001", ProjectID: "test", Tier: TierSmall, Title: "t", Description: "d", PreFixCommit: "abc"})
	store.SaveVariant(ctx, &Variant{ID: "v", Name: "v", BaseWorkflow: "small", IsBaseline: true})
	store.SaveRun(ctx, &Run{ID: "run-001", VariantID: "v", TaskID: "t-001", TrialNumber: 1, Status: RunStatusPass})

	j := &Judgment{
		RunID:         "run-001",
		PhaseID:       "implement",
		JudgeModel:    "opus",
		JudgeProvider: "claude",
		Scores:        map[string]int{"quality": 4, "minimality": 5, "readability": 4},
		Reasoning:     "Clean implementation with good error handling.",
	}
	if err := store.SaveJudgment(ctx, j); err != nil {
		t.Fatalf("save judgment: %v", err)
	}
	if j.ID == 0 {
		t.Error("expected auto-increment ID")
	}

	judgments, err := store.GetJudgments(ctx, "run-001")
	if err != nil {
		t.Fatalf("get judgments: %v", err)
	}
	if len(judgments) != 1 {
		t.Fatalf("expected 1 judgment, got %d", len(judgments))
	}
	if judgments[0].Scores["quality"] != 4 {
		t.Errorf("expected quality score 4, got %d", judgments[0].Scores["quality"])
	}
}
