package bench

import (
	"context"
	"testing"

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

func TestBuildBaseVars(t *testing.T) {
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
	}

	vars := runner.buildBaseVars(project, task, "run-id-1234", "/tmp/bench/runs/abc")

	if vars["TASK_ID"] != "bbolt-001" {
		t.Errorf("expected bbolt-001, got %s", vars["TASK_ID"])
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
}

func TestResolvePhaseConfig(t *testing.T) {
	runner := &Runner{}

	tests := []struct {
		name            string
		phaseID         string
		variant         *Variant
		wantProvider    string
		wantModel       string
		wantReasoning   string
		wantThinking    bool
	}{
		{
			name:    "baseline defaults",
			phaseID: "implement",
			variant: &Variant{
				ID:           "baseline",
				IsBaseline:   true,
				BaseWorkflow: "medium",
			},
			wantProvider: "claude",
			wantModel:    "opus",
			wantThinking: true,
		},
		{
			name:    "codex override",
			phaseID: "implement",
			variant: &Variant{
				ID:           "codex53-high-implement",
				BaseWorkflow: "medium",
				PhaseOverrides: map[string]PhaseOverride{
					"implement": {
						Provider:        "codex",
						Model:           "gpt-5.3-codex",
						ReasoningEffort: "high",
					},
				},
			},
			wantProvider:  "codex",
			wantModel:     "gpt-5.3-codex",
			wantReasoning: "high",
			wantThinking:  true, // Default, not overridden
		},
		{
			name:    "sonnet thinking spec",
			phaseID: "spec",
			variant: &Variant{
				ID:           "sonnet-spec",
				BaseWorkflow: "medium",
				PhaseOverrides: map[string]PhaseOverride{
					"spec": {
						Provider: "claude",
						Model:    "sonnet",
						Thinking: boolPtr(true),
					},
				},
			},
			wantProvider: "claude",
			wantModel:    "sonnet",
			wantThinking: true,
		},
		{
			name:    "override on different phase - uses defaults",
			phaseID: "spec",
			variant: &Variant{
				ID:           "codex-implement",
				BaseWorkflow: "medium",
				PhaseOverrides: map[string]PhaseOverride{
					"implement": {Provider: "codex", Model: "gpt-5.3-codex"},
				},
			},
			wantProvider: "claude",
			wantModel:    "opus",
			wantThinking: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, model, reasoning, thinking := runner.resolvePhaseConfig(tt.phaseID, tt.variant, nil)

			if provider != tt.wantProvider {
				t.Errorf("provider: got %s, want %s", provider, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("model: got %s, want %s", model, tt.wantModel)
			}
			if reasoning != tt.wantReasoning {
				t.Errorf("reasoning: got %s, want %s", reasoning, tt.wantReasoning)
			}
			if thinking != tt.wantThinking {
				t.Errorf("thinking: got %v, want %v", thinking, tt.wantThinking)
			}
		})
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

func boolPtr(b bool) *bool { return &b }
