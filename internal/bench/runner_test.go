package bench

import (
	"context"
	"log/slog"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestBuildVarsFromFrozen(t *testing.T) {
	vars := variable.VariableSet{
		"TASK_ID": "test-001",
	}

	frozen := FrozenOutputMap{
		"spec": &FrozenOutput{
			OutputVarName: "SPEC_CONTENT",
			OutputContent: "The specification content",
		},
		"tdd_write": &FrozenOutput{
			OutputVarName: "TDD_TESTS_CONTENT",
			OutputContent: "Test content here",
		},
	}

	BuildVarsFromFrozen(vars, frozen)

	if vars["SPEC_CONTENT"] != "The specification content" {
		t.Errorf("expected spec content, got %q", vars["SPEC_CONTENT"])
	}
	if vars["TDD_TESTS_CONTENT"] != "Test content here" {
		t.Errorf("expected tdd content, got %q", vars["TDD_TESTS_CONTENT"])
	}
	if vars["TASK_ID"] != "test-001" {
		t.Error("original vars should be preserved")
	}
}

func TestBuildTaskVariables(t *testing.T) {
	runner := &Runner{}

	project := &Project{
		ID:       "bbolt",
		Language: "go",
		TestCmd:  "go test ./...",
		BuildCmd: "go build ./...",
		LintCmd:  "golangci-lint run",
	}

	task := &Task{
		ID:          "bbolt-001",
		Title:       "Fix page split",
		Description: "The page splitting algorithm fails on large keys",
		Category:    "bug",
		Tier:        TierMedium,
	}

	vars := runner.buildTaskVariables(task, project, "run-id-1234abcd", "/tmp/bench/runs/abc")

	if vars["TASK_ID"] != "bbolt-001" {
		t.Errorf("expected bbolt-001, got %s", vars["TASK_ID"])
	}
	if vars["TASK_TITLE"] != "Fix page split" {
		t.Errorf("expected 'Fix page split', got %s", vars["TASK_TITLE"])
	}
	if vars["TASK_DESCRIPTION"] != "The page splitting algorithm fails on large keys" {
		t.Errorf("expected description, got %s", vars["TASK_DESCRIPTION"])
	}
	if vars["CATEGORY"] != "bug" {
		t.Errorf("expected bug, got %s", vars["CATEGORY"])
	}
	if vars["WEIGHT"] != "medium" {
		t.Errorf("expected medium, got %s", vars["WEIGHT"])
	}
	if vars["LANGUAGE"] != "go" {
		t.Errorf("expected go, got %s", vars["LANGUAGE"])
	}
	if vars["TEST_COMMAND"] != "go test ./..." {
		t.Errorf("expected go test ./..., got %s", vars["TEST_COMMAND"])
	}
	if vars["WORKTREE_PATH"] != "/tmp/bench/runs/abc" {
		t.Errorf("expected workdir path, got %s", vars["WORKTREE_PATH"])
	}
	if vars["TASK_BRANCH"] != "bench/bbolt-001/run-id-1" {
		t.Errorf("expected bench branch, got %s", vars["TASK_BRANCH"])
	}
}

func TestBuildPhaseOverrides(t *testing.T) {
	// Helper phases matching a medium workflow
	phases := []*db.WorkflowPhase{
		{PhaseTemplateID: "spec"},
		{PhaseTemplateID: "tdd_write"},
		{PhaseTemplateID: "implement"},
		{PhaseTemplateID: "review"},
	}

	tests := []struct {
		name            string
		runner          *Runner
		variant         *Variant
		phaseID         string
		wantProvider    string
		wantModel       string
		wantReasoning   string
		wantThinking    *bool
	}{
		{
			name:    "baseline defaults to opus + thinking",
			runner:  &Runner{},
			variant: &Variant{ID: "baseline", IsBaseline: true},
			phaseID: "implement",
			wantProvider: "claude",
			wantModel:    "opus",
			wantThinking: boolPtr(true),
		},
		{
			name:   "codex override on implement",
			runner: &Runner{},
			variant: &Variant{
				ID: "codex53-high-implement",
				PhaseOverrides: map[string]PhaseOverride{
					"implement": {
						Provider:        "codex",
						Model:           "gpt-5.3-codex",
						ReasoningEffort: "high",
					},
				},
			},
			phaseID:       "implement",
			wantProvider:  "codex",
			wantModel:     "gpt-5.3-codex",
			wantReasoning: "high",
			wantThinking:  boolPtr(true), // Default, not overridden
		},
		{
			name:   "sonnet with explicit thinking on spec",
			runner: &Runner{},
			variant: &Variant{
				ID: "sonnet-spec",
				PhaseOverrides: map[string]PhaseOverride{
					"spec": {
						Provider: "claude",
						Model:    "sonnet",
						Thinking: boolPtr(true),
					},
				},
			},
			phaseID:      "spec",
			wantProvider: "claude",
			wantModel:    "sonnet",
			wantThinking: boolPtr(true),
		},
		{
			name:   "override on different phase uses defaults for spec",
			runner: &Runner{},
			variant: &Variant{
				ID: "codex-implement",
				PhaseOverrides: map[string]PhaseOverride{
					"implement": {Provider: "codex", Model: "gpt-5.3-codex"},
				},
			},
			phaseID:      "spec",
			wantProvider: "claude",
			wantModel:    "opus",
			wantThinking: boolPtr(true),
		},
		{
			name: "global model override takes precedence",
			runner: &Runner{
				overrideProvider:        "claude",
				overrideModel:           "claude-haiku-4-5-20251001",
				overrideReasoningEffort: "",
			},
			variant: &Variant{
				ID: "codex-implement",
				PhaseOverrides: map[string]PhaseOverride{
					"implement": {Provider: "codex", Model: "gpt-5.3-codex"},
				},
			},
			phaseID:      "implement",
			wantProvider: "claude",
			wantModel:    "claude-haiku-4-5-20251001",
			wantThinking: nil, // Global override doesn't force thinking
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overrides := tt.runner.buildPhaseOverrides(tt.variant, phases)

			override, ok := overrides[tt.phaseID]
			if !ok {
				t.Fatalf("no override for phase %s", tt.phaseID)
			}

			if override.Provider != tt.wantProvider {
				t.Errorf("provider: got %s, want %s", override.Provider, tt.wantProvider)
			}
			if override.Model != tt.wantModel {
				t.Errorf("model: got %s, want %s", override.Model, tt.wantModel)
			}
			if override.ReasoningEffort != tt.wantReasoning {
				t.Errorf("reasoning: got %s, want %s", override.ReasoningEffort, tt.wantReasoning)
			}
			if tt.wantThinking == nil {
				if override.Thinking != nil {
					t.Errorf("thinking: got %v, want nil", *override.Thinking)
				}
			} else {
				if override.Thinking == nil {
					t.Errorf("thinking: got nil, want %v", *tt.wantThinking)
				} else if *override.Thinking != *tt.wantThinking {
					t.Errorf("thinking: got %v, want %v", *override.Thinking, *tt.wantThinking)
				}
			}
		})
	}
}

func TestBuildPhaseOverridesAllPhasesPopulated(t *testing.T) {
	// Verify that buildPhaseOverrides produces an entry for every phase,
	// not just the ones with variant overrides.
	phases := []*db.WorkflowPhase{
		{PhaseTemplateID: "spec"},
		{PhaseTemplateID: "tdd_write"},
		{PhaseTemplateID: "implement"},
		{PhaseTemplateID: "review"},
		{PhaseTemplateID: "docs"},
	}

	runner := &Runner{}
	variant := &Variant{
		ID: "codex-implement",
		PhaseOverrides: map[string]PhaseOverride{
			"implement": {Provider: "codex", Model: "gpt-5.3-codex"},
		},
	}

	overrides := runner.buildPhaseOverrides(variant, phases)

	if len(overrides) != len(phases) {
		t.Errorf("expected %d overrides, got %d", len(phases), len(overrides))
	}

	for _, phase := range phases {
		if _, ok := overrides[phase.PhaseTemplateID]; !ok {
			t.Errorf("missing override for phase %s", phase.PhaseTemplateID)
		}
	}
}

func TestSavePhaseResults(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	runner := &Runner{store: store, logger: defaultTestLogger()}

	// Create prerequisite records for FK constraints
	if err := store.SaveProject(ctx, &Project{ID: "proj-001", Language: "go"}); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := store.SaveTask(ctx, &Task{ID: "task-001", ProjectID: "proj-001", Tier: TierMedium}); err != nil {
		t.Fatalf("save task: %v", err)
	}
	if err := store.SaveVariant(ctx, &Variant{ID: "baseline", IsBaseline: true, BaseWorkflow: "medium"}); err != nil {
		t.Fatalf("save variant: %v", err)
	}
	if err := store.SaveRun(ctx, &Run{ID: "run-001", VariantID: "baseline", TaskID: "task-001", TrialNumber: 1, Status: RunStatusRunning}); err != nil {
		t.Fatalf("save run: %v", err)
	}

	result := &executor.WorkflowRunResult{
		PhaseResults: []executor.PhaseResult{
			{
				PhaseID:         "spec",
				Content:         "spec content",
				OutputVarName:   "SPEC_CONTENT",
				WasPrePopulated: true,
			},
			{
				PhaseID:       "implement",
				Content:       "implement content",
				Provider:      "claude",
				Model:         "opus",
				OutputVarName: "OUTPUT_IMPLEMENT",
				InputTokens:   1000,
				OutputTokens:  500,
				CostUSD:       0.05,
				DurationMS:    30000,
			},
		},
	}

	frozenOutputs := FrozenOutputMap{
		"spec": &FrozenOutput{
			ID:            "frozen-spec-id",
			OutputVarName: "SPEC_CONTENT",
			OutputContent: "spec content",
		},
	}

	runner.savePhaseResults(ctx, "run-001", result, &Variant{ID: "baseline"}, frozenOutputs, 1, "task-001")

	// Verify frozen phase result was saved
	prs, err := store.GetPhaseResults(ctx, "run-001")
	if err != nil {
		t.Fatalf("get phase results: %v", err)
	}

	if len(prs) != 2 {
		t.Fatalf("expected 2 phase results, got %d", len(prs))
	}

	// First: frozen spec
	if !prs[0].WasFrozen {
		t.Error("expected spec to be frozen")
	}
	if prs[0].FrozenOutputID != "frozen-spec-id" {
		t.Errorf("expected frozen-spec-id, got %s", prs[0].FrozenOutputID)
	}

	// Second: live implement
	if prs[1].WasFrozen {
		t.Error("expected implement to be live")
	}
	if prs[1].Provider != "claude" {
		t.Errorf("expected claude, got %s", prs[1].Provider)
	}
	if prs[1].InputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", prs[1].InputTokens)
	}
}

func TestSaveFrozenFromResult(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	err = SaveFrozenFromResult(ctx, store, "task-001", "spec", "baseline", "SPEC_CONTENT", "spec output", 1)
	if err != nil {
		t.Fatalf("save frozen: %v", err)
	}

	fo, err := store.GetFrozenOutput(ctx, "task-001", "spec", "baseline", 1)
	if err != nil {
		t.Fatalf("get frozen: %v", err)
	}
	if fo.OutputContent != "spec output" {
		t.Errorf("expected 'spec output', got %q", fo.OutputContent)
	}
	if fo.OutputVarName != "SPEC_CONTENT" {
		t.Errorf("expected SPEC_CONTENT, got %s", fo.OutputVarName)
	}
}

func TestCascadeFreezeDecision(t *testing.T) {
	// Simulates the freeze decision logic from RunSingle.
	// Medium workflow: spec → tdd_write → tdd_integrate → implement → review → docs
	mediumPhases := []string{"spec", "tdd_write", "tdd_integrate", "implement", "review", "docs"}

	// Build frozen outputs for all phases (as if baseline ran)
	allFrozen := make(FrozenOutputMap)
	for _, p := range mediumPhases {
		allFrozen[p] = &FrozenOutput{PhaseID: p, OutputContent: p + " output"}
	}

	tests := []struct {
		name       string
		overrides  map[string]PhaseOverride
		isBaseline bool
		wantFrozen map[string]bool // true = frozen, false = live
	}{
		{
			name:       "baseline runs everything live",
			isBaseline: true,
			overrides:  nil,
			wantFrozen: map[string]bool{
				"spec": false, "tdd_write": false, "tdd_integrate": false,
				"implement": false, "review": false, "docs": false,
			},
		},
		{
			name: "implement override: freeze upstream data phases, live from implement",
			overrides: map[string]PhaseOverride{
				"implement": {Provider: "codex", Model: "gpt-5.3-codex"},
			},
			wantFrozen: map[string]bool{
				"spec": true, "tdd_write": true, "tdd_integrate": true,
				"implement": false, "review": false, "docs": false,
			},
		},
		{
			name: "spec override: cascade from spec onwards, nothing frozen",
			overrides: map[string]PhaseOverride{
				"spec": {Provider: "claude", Model: "sonnet"},
			},
			wantFrozen: map[string]bool{
				"spec": false, "tdd_write": false, "tdd_integrate": false,
				"implement": false, "review": false, "docs": false,
			},
		},
		{
			name: "tdd_write override: freeze spec, cascade from tdd onwards",
			overrides: map[string]PhaseOverride{
				"tdd_write": {Provider: "claude", Model: "sonnet"},
			},
			wantFrozen: map[string]bool{
				"spec": true, "tdd_write": false, "tdd_integrate": false,
				"implement": false, "review": false, "docs": false,
			},
		},
		{
			name: "review override: freeze upstream data phases, implement still live",
			overrides: map[string]PhaseOverride{
				"review": {Provider: "claude", Model: "sonnet"},
			},
			wantFrozen: map[string]bool{
				"spec": true, "tdd_write": true, "tdd_integrate": true,
				"implement": false, // NOT in phasesAllowFreezing — always live
				"review": false, "docs": false,
			},
		},
		{
			name: "implement never frozen even without overrides",
			overrides: map[string]PhaseOverride{
				"docs": {Provider: "claude", Model: "sonnet"},
			},
			wantFrozen: map[string]bool{
				"spec": true, "tdd_write": true, "tdd_integrate": true,
				"implement": false, // phasesAllowFreezing blocks it
				"review": false,    // phasesAllowFreezing blocks it
				"docs": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variant := &Variant{
				IsBaseline:     tt.isBaseline,
				PhaseOverrides: tt.overrides,
			}
			if variant.PhaseOverrides == nil {
				variant.PhaseOverrides = make(map[string]PhaseOverride)
			}

			// Find first override index (mirrors RunSingle logic)
			firstOverrideIdx := len(mediumPhases)
			if !variant.IsBaseline {
				for i, p := range mediumPhases {
					if _, ok := variant.PhaseOverrides[p]; ok {
						firstOverrideIdx = i
						break
					}
				}
			}

			for i, phaseID := range mediumPhases {
				_, hasOverride := variant.PhaseOverrides[phaseID]
				hasFrozen := allFrozen[phaseID] != nil
				shouldFreeze := !variant.IsBaseline && !hasOverride && hasFrozen &&
					phasesAllowFreezing[phaseID] && i < firstOverrideIdx

				want := tt.wantFrozen[phaseID]
				if shouldFreeze != want {
					t.Errorf("phase %s: frozen=%v, want=%v", phaseID, shouldFreeze, want)
				}
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func defaultTestLogger() *slog.Logger {
	return slog.Default()
}
