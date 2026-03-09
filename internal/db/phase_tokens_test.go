package db

import "testing"

func TestProjectDB_SavePhase_PersistsExtendedTokenFields(t *testing.T) {
	pdb := setupProjectDB(t)
	if err := pdb.SaveTask(&Task{
		ID:       "TASK-001",
		Title:    "Test task",
		Status:   "running",
		Weight:   "medium",
		Queue:    "active",
		Priority: "normal",
		Category: "feature",
	}); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	phase := &Phase{
		TaskID:              "TASK-001",
		PhaseID:             "implement",
		Status:              "completed",
		InputTokens:         100,
		OutputTokens:        50,
		CacheCreationTokens: 25,
		CacheReadTokens:     10,
		TotalTokens:         185,
	}
	if err := pdb.SavePhase(phase); err != nil {
		t.Fatalf("SavePhase failed: %v", err)
	}

	phases, err := pdb.GetPhases("TASK-001")
	if err != nil {
		t.Fatalf("GetPhases failed: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(phases))
	}

	got := phases[0]
	if got.CacheCreationTokens != 25 {
		t.Fatalf("cache_creation_tokens = %d, want 25", got.CacheCreationTokens)
	}
	if got.CacheReadTokens != 10 {
		t.Fatalf("cache_read_tokens = %d, want 10", got.CacheReadTokens)
	}
	if got.TotalTokens != 185 {
		t.Fatalf("total_tokens = %d, want 185", got.TotalTokens)
	}
}
