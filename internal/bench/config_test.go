package bench

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSuiteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "suite.yaml")

	yaml := `
projects:
  - id: bbolt
    repo_url: https://github.com/etcd-io/bbolt
    commit_hash: abc123
    language: go
    test_cmd: "go test ./..."
    build_cmd: "go build ./..."

tasks:
  - id: bbolt-001
    project_id: bbolt
    tier: medium
    title: Fix page split
    description: Page splitting fails on large keys
    pre_fix_commit: aaa111

variants:
  - id: baseline
    name: All Opus (Thinking)
    base_workflow: medium
    is_baseline: true

  - id: codex53-high-implement
    name: Codex 5.3 High Implement
    base_workflow: medium
    phase_overrides:
      implement:
        provider: codex
        model: gpt-5.3-codex
        reasoning_effort: high

throttle:
  max_parallel_claude: 2
  max_parallel_codex: 6
  delay_between_claude_ms: 3000
`
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadSuiteConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(cfg.Projects))
	}
	if cfg.Projects[0].ID != "bbolt" {
		t.Errorf("expected bbolt, got %s", cfg.Projects[0].ID)
	}

	if len(cfg.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(cfg.Tasks))
	}
	if cfg.Tasks[0].Tier != TierMedium {
		t.Errorf("expected medium tier, got %s", cfg.Tasks[0].Tier)
	}

	if len(cfg.Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(cfg.Variants))
	}

	if cfg.Throttle.MaxParallelClaude != 2 {
		t.Errorf("expected max_parallel_claude 2, got %d", cfg.Throttle.MaxParallelClaude)
	}

	// Variant overrides
	v := cfg.Variants[1]
	if v.PhaseOverrides == nil {
		t.Fatal("expected phase overrides")
	}
	impl, ok := v.PhaseOverrides["implement"]
	if !ok {
		t.Fatal("expected implement override")
	}
	if impl.Provider != "codex" || impl.Model != "gpt-5.3-codex" || impl.ReasoningEffort != "high" {
		t.Errorf("implement override mismatch: %+v", impl)
	}
}

func TestValidateNoBaseline(t *testing.T) {
	cfg := &SuiteConfig{
		Variants: []Variant{
			{ID: "v1", Name: "v1", BaseWorkflow: "small"},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for missing baseline")
	}
}

func TestValidateMultipleBaselines(t *testing.T) {
	cfg := &SuiteConfig{
		Variants: []Variant{
			{ID: "v1", Name: "v1", BaseWorkflow: "small", IsBaseline: true},
			{ID: "v2", Name: "v2", BaseWorkflow: "small", IsBaseline: true},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for multiple baselines")
	}
}

func TestValidateDuplicateIDs(t *testing.T) {
	cfg := &SuiteConfig{
		Projects: []Project{
			{ID: "p1", RepoURL: "http://a", CommitHash: "abc", Language: "go", TestCmd: "test"},
			{ID: "p1", RepoURL: "http://b", CommitHash: "def", Language: "go", TestCmd: "test"},
		},
		Variants: []Variant{
			{ID: "v1", Name: "v1", BaseWorkflow: "small", IsBaseline: true},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("expected validation error for duplicate project IDs")
	}
}

func TestImportToStore(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	cfg := &SuiteConfig{
		Projects: []Project{
			{ID: "p1", RepoURL: "http://test", CommitHash: "abc", Language: "go", TestCmd: "test"},
		},
		Tasks: []Task{
			{ID: "t1", ProjectID: "p1", Tier: TierSmall, Title: "t", Description: "d", PreFixCommit: "abc"},
		},
		Variants: []Variant{
			{ID: "v1", Name: "v1", BaseWorkflow: "small", IsBaseline: true},
		},
	}

	ctx := context.Background()
	if err := cfg.ImportToStore(ctx, store, t.TempDir()); err != nil {
		t.Fatalf("import: %v", err)
	}

	projects, _ := store.ListProjects(ctx)
	if len(projects) != 1 {
		t.Errorf("expected 1 project, got %d", len(projects))
	}

	tasks, _ := store.ListTasks(ctx, "", "")
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	variants, _ := store.ListVariants(ctx)
	if len(variants) != 1 {
		t.Errorf("expected 1 variant, got %d", len(variants))
	}
}
