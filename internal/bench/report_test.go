package bench

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFormatReport(t *testing.T) {
	report := &FullReport{
		Summary: ReportSummary{
			TotalRuns:        10,
			TotalVariants:    3,
			TotalTasks:       5,
			TotalCostUSD:     12.50,
			BaselinePassRate: 80.0,
		},
		Leaderboards: []PhaseLeaderboard{
			{
				PhaseID: "implement",
				Winner:  "codex53-high",
				Entries: []LeaderboardEntry{
					{
						VariantID:      "codex53-high",
						Provider:       "codex",
						Model:          "gpt-5.3-codex",
						PassRate:       90,
						CostPerSuccess: 0.50,
						SampleSize:     8,
					},
					{
						VariantID:      "baseline",
						Provider:       "claude",
						Model:          "opus",
						PassRate:       80,
						CostPerSuccess: 0.75,
						SampleSize:     8,
					},
				},
			},
		},
		Recommendations: []OptimalConfig{
			{
				PhaseID:        "implement",
				Provider:       "codex",
				Model:          "gpt-5.3-codex",
				PassRate:       90,
				CostPerSuccess: 0.50,
				Confidence:     "high",
				Rationale:      "90% pass rate at $0.50/success",
			},
		},
	}

	output := FormatReport(report)

	// Check key sections exist
	checks := []string{
		"Benchmark Report",
		"Runs: 10",
		"Variants: 3",
		"$12.50",
		"80%",
		"implement",
		"codex53-high",
		"<-- best",
		"Recommended Configuration",
		"confidence: high",
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Errorf("formatted report missing: %q", check)
		}
	}
}

func TestReportGeneratorSummary(t *testing.T) {
	store, err := OpenInMemory()
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Setup: project, tasks, baseline variant, some runs
	if err := store.SaveProject(ctx, &Project{
		ID: "test", RepoURL: "https://example.com", CommitHash: "abc",
		Language: "go", TestCmd: "go test",
	}); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := store.SaveTask(ctx, &Task{
		ID: "task-1", ProjectID: "test", Tier: TierMedium, Title: "Task 1",
		Description: "Test task 1", PreFixCommit: "abc123",
	}); err != nil {
		t.Fatalf("save task-1: %v", err)
	}
	if err := store.SaveTask(ctx, &Task{
		ID: "task-2", ProjectID: "test", Tier: TierMedium, Title: "Task 2",
		Description: "Test task 2", PreFixCommit: "def456",
	}); err != nil {
		t.Fatalf("save task-2: %v", err)
	}
	if err := store.SaveVariant(ctx, &Variant{
		ID: "baseline", Name: "Baseline", BaseWorkflow: "medium", IsBaseline: true,
	}); err != nil {
		t.Fatalf("save variant: %v", err)
	}
	if err := store.SaveRun(ctx, &Run{
		ID: "run-1", VariantID: "baseline", TaskID: "task-1", TrialNumber: 1,
		Status: RunStatusPass, StartedAt: time.Now(), CompletedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save run-1: %v", err)
	}
	if err := store.SaveRun(ctx, &Run{
		ID: "run-2", VariantID: "baseline", TaskID: "task-2", TrialNumber: 1,
		Status: RunStatusFail, StartedAt: time.Now(), CompletedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save run-2: %v", err)
	}

	rg := NewReportGenerator(store)
	summary, err := rg.buildSummary(ctx)
	if err != nil {
		t.Fatalf("buildSummary: %v", err)
	}

	if summary.TotalRuns != 2 {
		t.Errorf("expected 2 runs, got %d", summary.TotalRuns)
	}
	if summary.TotalVariants != 1 {
		t.Errorf("expected 1 variant, got %d", summary.TotalVariants)
	}
	if summary.BaselinePassRate != 50 {
		t.Errorf("expected 50%% baseline pass rate, got %.0f%%", summary.BaselinePassRate)
	}
}

func TestBuildRecommendations(t *testing.T) {
	rg := &ReportGenerator{}

	leaderboards := []PhaseLeaderboard{
		{
			PhaseID: "implement",
			Entries: []LeaderboardEntry{
				{
					VariantID:      "codex53",
					Provider:       "codex",
					Model:          "gpt-5.3-codex",
					PassRate:       90,
					CostPerSuccess: 0.30,
					SampleSize:     10,
				},
				{
					VariantID:      "baseline",
					Provider:       "claude",
					Model:          "opus",
					PassRate:       80,
					CostPerSuccess: 0.60,
					SampleSize:     10,
				},
			},
		},
	}

	recs := rg.buildRecommendations(leaderboards)

	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}

	rec := recs[0]
	if rec.PhaseID != "implement" {
		t.Errorf("expected implement phase, got %s", rec.PhaseID)
	}
	if rec.Provider != "codex" {
		t.Errorf("expected codex provider, got %s", rec.Provider)
	}
	if rec.Confidence != "high" {
		t.Errorf("expected high confidence (10 samples, 90%% pass rate), got %s", rec.Confidence)
	}
	// Should mention cost improvement
	if !strings.Contains(rec.Rationale, "cheaper") {
		t.Errorf("expected rationale to mention cheaper, got: %s", rec.Rationale)
	}
}
