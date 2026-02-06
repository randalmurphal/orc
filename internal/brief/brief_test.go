package brief

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-1: Generator produces Brief with sections from data sources
// ============================================================================

func TestGenerator_ProducesBriefWithSections(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Seed a completed task with an initiative that has decisions
	seedInitiativeWithDecisions(t, backend, "INIT-001", "Test Initiative", []initiative.Decision{
		{ID: "DEC-001", Decision: "Use bcrypt for passwords", Rationale: "Industry standard", Date: time.Now()},
	})
	seedCompletedTask(t, backend, "TASK-001", "Fix login bug", "INIT-001")
	seedReviewFindings(t, backend, "TASK-001", 1, []*orcv1.ReviewFinding{
		{Severity: "high", Description: "SQL injection in login handler", File: strPtr("internal/auth/login.go"), Line: int32Ptr(42)},
	})

	gen := NewGenerator(backend, DefaultConfig())
	brief, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if brief == nil {
		t.Fatal("Generate() returned nil brief")
	}

	if brief.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should be set")
	}

	if len(brief.Sections) == 0 {
		t.Error("expected at least one section, got 0")
	}

	// Verify decisions section is present
	found := false
	for _, s := range brief.Sections {
		if s.Category == CategoryDecisions {
			found = true
			if len(s.Entries) == 0 {
				t.Error("decisions section should have entries")
			}
		}
	}
	if !found {
		t.Error("expected decisions section in brief")
	}
}

func TestGenerator_IncludesReviewFindings(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	seedCompletedTask(t, backend, "TASK-001", "Auth feature", "")
	seedReviewFindings(t, backend, "TASK-001", 1, []*orcv1.ReviewFinding{
		{Severity: "high", Description: "Missing input validation", File: strPtr("internal/api/handler.go"), Line: int32Ptr(15)},
		{Severity: "low", Description: "Consider adding a comment", File: strPtr("internal/api/handler.go"), Line: int32Ptr(20)},
	})

	gen := NewGenerator(backend, DefaultConfig())
	brief, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Should include high-severity finding but not low
	findingsSection := findSection(brief, CategoryRecentFindings)
	if findingsSection == nil {
		t.Fatal("expected recent_findings section")
	}

	hasHigh := false
	for _, e := range findingsSection.Entries {
		if strings.Contains(e.Content, "Missing input validation") {
			hasHigh = true
		}
		if strings.Contains(e.Content, "Consider adding a comment") {
			t.Error("low-severity finding should not be included (only high+ are included)")
		}
	}
	if !hasHigh {
		t.Error("high-severity finding should be included")
	}
}

// ============================================================================
// SC-2: Data source extractors format entries correctly
// ============================================================================

func TestExtractDecisions_FormatsEntries(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	seedInitiativeWithDecisions(t, backend, "INIT-001", "Auth System", []initiative.Decision{
		{ID: "DEC-001", Decision: "Use JWT tokens", Rationale: "Stateless auth", Date: time.Now()},
		{ID: "DEC-002", Decision: "Use bcrypt for passwords", Rationale: "Industry standard", Date: time.Now()},
	})

	entries, err := ExtractDecisions(context.Background(), backend)
	if err != nil {
		t.Fatalf("ExtractDecisions() error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Check that entries contain decision content and source
	for _, e := range entries {
		if e.Content == "" {
			t.Error("entry Content should not be empty")
		}
		if !strings.Contains(e.Source, "INIT-001") {
			t.Errorf("entry Source should reference initiative, got %q", e.Source)
		}
		if e.Impact <= 0 {
			t.Error("entry Impact should be positive")
		}
	}
}

func TestExtractDecisions_SkipsArchivedInitiatives(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	init := initiative.New("INIT-001", "Archived Feature")
	init.Status = initiative.StatusArchived
	init.Decisions = []initiative.Decision{
		{ID: "DEC-001", Decision: "Old decision", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	entries, err := ExtractDecisions(context.Background(), backend)
	if err != nil {
		t.Fatalf("ExtractDecisions() error: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries for archived initiative, got %d", len(entries))
	}
}

func TestExtractFindings_FiltersHighSeverity(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	seedCompletedTask(t, backend, "TASK-001", "Feature A", "")
	seedReviewFindings(t, backend, "TASK-001", 1, []*orcv1.ReviewFinding{
		{Severity: "high", Description: "Critical bug", File: strPtr("main.go"), Line: int32Ptr(10)},
		{Severity: "medium", Description: "Style issue"},
		{Severity: "low", Description: "Nitpick"},
	})

	entries, err := ExtractFindings(context.Background(), backend)
	if err != nil {
		t.Fatalf("ExtractFindings() error: %v", err)
	}

	// Should only include high severity
	for _, e := range entries {
		if strings.Contains(e.Content, "Nitpick") || strings.Contains(e.Content, "Style issue") {
			t.Error("should only include high-severity findings")
		}
	}
	if len(entries) == 0 {
		t.Error("expected at least one high-severity finding")
	}
}

func TestExtractFindings_IncludesFileAndTask(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	seedCompletedTask(t, backend, "TASK-042", "Fix handler", "")
	file := "internal/api/handler.go"
	line := int32(15)
	seedReviewFindings(t, backend, "TASK-042", 1, []*orcv1.ReviewFinding{
		{Severity: "high", Description: "Missing error check", File: &file, Line: &line},
	})

	entries, err := ExtractFindings(context.Background(), backend)
	if err != nil {
		t.Fatalf("ExtractFindings() error: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}

	e := entries[0]
	if !strings.Contains(e.Content, "handler.go") {
		t.Errorf("entry should reference file, got %q", e.Content)
	}
	if !strings.Contains(e.Source, "TASK-042") {
		t.Errorf("entry Source should reference task, got %q", e.Source)
	}
}

func TestExtractFindings_LimitsToMaxEntries(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Create many tasks with many findings
	for i := 0; i < 15; i++ {
		taskID := taskIDForIndex(i)
		seedCompletedTask(t, backend, taskID, "Task "+taskID, "")
		seedReviewFindings(t, backend, taskID, 1, []*orcv1.ReviewFinding{
			{Severity: "high", Description: "Finding for " + taskID, File: strPtr("file.go")},
		})
	}

	entries, err := ExtractFindings(context.Background(), backend)
	if err != nil {
		t.Fatalf("ExtractFindings() error: %v", err)
	}

	// Max 10 entries as per spec
	if len(entries) > 10 {
		t.Errorf("expected max 10 entries, got %d", len(entries))
	}
}

// ============================================================================
// SC-3: Token budget limits output
// ============================================================================

func TestTokenBudget_TruncatesSectionsAtLimit(t *testing.T) {
	t.Parallel()

	section := Section{
		Category: CategoryDecisions,
		Entries: []Entry{
			{Content: strings.Repeat("word ", 200), Source: "INIT-001", Impact: 0.9},
			{Content: strings.Repeat("word ", 200), Source: "INIT-001", Impact: 0.8},
			{Content: strings.Repeat("word ", 200), Source: "INIT-001", Impact: 0.7},
			{Content: strings.Repeat("word ", 200), Source: "INIT-001", Impact: 0.6},
		},
	}

	cfg := DefaultConfig()
	// Set a small budget to force truncation
	cfg.SectionBudgets[CategoryDecisions] = 100

	truncated := ApplyTokenBudget(section, cfg.SectionBudgets[CategoryDecisions])

	totalTokens := 0
	for _, e := range truncated.Entries {
		totalTokens += EstimateTokens(e.Content)
	}

	if totalTokens > cfg.SectionBudgets[CategoryDecisions] {
		t.Errorf("section tokens (%d) exceed budget (%d)", totalTokens, cfg.SectionBudgets[CategoryDecisions])
	}

	// Should have fewer entries than original
	if len(truncated.Entries) >= len(section.Entries) {
		t.Error("expected truncation to remove entries")
	}
}

func TestTokenBudget_PreservesHighImpactEntries(t *testing.T) {
	t.Parallel()

	section := Section{
		Category: CategoryDecisions,
		Entries: []Entry{
			{Content: "Important decision about architecture", Source: "INIT-001", Impact: 0.95},
			{Content: "Minor style preference", Source: "INIT-002", Impact: 0.1},
			{Content: "Critical security requirement", Source: "INIT-003", Impact: 0.99},
		},
	}

	truncated := ApplyTokenBudget(section, 50) // Very small budget

	// Higher-impact entries should be preserved over lower-impact ones
	if len(truncated.Entries) == 0 {
		t.Fatal("expected at least one entry after truncation")
	}

	// First entry should be highest impact
	if truncated.Entries[0].Impact < 0.9 {
		t.Errorf("expected high-impact entry first, got impact %f", truncated.Entries[0].Impact)
	}
}

func TestTokenBudget_TotalBudgetEnforced(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.MaxTokens = 100 // Very small total budget

	sections := []Section{
		{
			Category: CategoryDecisions,
			Entries: []Entry{
				{Content: strings.Repeat("word ", 50), Source: "INIT-001", Impact: 0.9},
			},
		},
		{
			Category: CategoryRecentFindings,
			Entries: []Entry{
				{Content: strings.Repeat("word ", 50), Source: "TASK-001", Impact: 0.8},
			},
		},
	}

	result := ApplyTotalBudget(sections, cfg.MaxTokens)

	totalTokens := 0
	for _, s := range result {
		for _, e := range s.Entries {
			totalTokens += EstimateTokens(e.Content)
		}
	}

	if totalTokens > cfg.MaxTokens {
		t.Errorf("total tokens (%d) exceed budget (%d)", totalTokens, cfg.MaxTokens)
	}
}

func TestEstimateTokens_ApproximatesCorrectly(t *testing.T) {
	t.Parallel()

	// ~4 chars per token is a reasonable approximation
	text := "This is a test string with about forty characters"
	tokens := EstimateTokens(text)

	// Should be roughly len(text)/4, allow ±50%
	expected := len(text) / 4
	if tokens < expected/2 || tokens > expected*2 {
		t.Errorf("EstimateTokens(%q) = %d, expected roughly %d", text, tokens, expected)
	}
}

func TestEstimateTokens_EmptyString(t *testing.T) {
	t.Parallel()

	if EstimateTokens("") != 0 {
		t.Error("empty string should have 0 tokens")
	}
}

// ============================================================================
// SC-4: Cache layer detects staleness and regenerates
// ============================================================================

func TestCache_StoresAndRetrieves(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(filepath.Join(dir, "brief.cache"))

	brief := &Brief{
		GeneratedAt: time.Now(),
		TaskCount:   5,
		Sections: []Section{
			{Category: CategoryDecisions, Entries: []Entry{{Content: "test", Source: "INIT-001", Impact: 0.9}}},
		},
		TokenCount: 100,
	}

	if err := cache.Store(brief); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	if loaded.TaskCount != 5 {
		t.Errorf("TaskCount = %d, want 5", loaded.TaskCount)
	}

	if len(loaded.Sections) != 1 {
		t.Errorf("Sections len = %d, want 1", len(loaded.Sections))
	}
}

func TestCache_DetectsStaleness(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(filepath.Join(dir, "brief.cache"))

	brief := &Brief{
		GeneratedAt: time.Now(),
		TaskCount:   5,
		TokenCount:  100,
	}

	if err := cache.Store(brief); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	// Task count increased by 3 (default stale threshold)
	if !cache.IsStale(8, 3) {
		t.Error("expected cache to be stale when task count increased by stale threshold")
	}

	// Task count only increased by 1 (below threshold)
	if cache.IsStale(6, 3) {
		t.Error("expected cache to NOT be stale when task count below threshold")
	}
}

func TestCache_ReturnsNilForMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cache := NewCache(filepath.Join(dir, "nonexistent.cache"))

	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() should not error for missing file, got: %v", err)
	}

	if loaded != nil {
		t.Error("Load() should return nil for missing file")
	}
}

func TestCache_InvalidateRemovesFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "brief.cache")
	cache := NewCache(cachePath)

	brief := &Brief{
		GeneratedAt: time.Now(),
		TaskCount:   5,
		TokenCount:  50,
	}

	if err := cache.Store(brief); err != nil {
		t.Fatalf("Store() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("cache file should exist after Store()")
	}

	if err := cache.Invalidate(); err != nil {
		t.Fatalf("Invalidate() error: %v", err)
	}

	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() after invalidate: %v", err)
	}
	if loaded != nil {
		t.Error("Load() after Invalidate() should return nil")
	}
}

// ============================================================================
// SC-5: Generator uses cache and regenerates when stale
// ============================================================================

func TestGenerator_ReturnsCachedBrief(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.CachePath = filepath.Join(dir, "brief.cache")
	cfg.StaleThreshold = 3

	gen := NewGenerator(backend, cfg)

	// Generate first time
	brief1, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("first Generate() error: %v", err)
	}

	// Generate again without changing anything — should use cache
	brief2, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("second Generate() error: %v", err)
	}

	if brief2.GeneratedAt != brief1.GeneratedAt {
		t.Error("expected cached brief (same GeneratedAt), got regenerated")
	}
}

func TestGenerator_RegeneratesWhenStale(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	dir := t.TempDir()
	cfg := DefaultConfig()
	cfg.CachePath = filepath.Join(dir, "brief.cache")
	cfg.StaleThreshold = 1 // Regenerate after 1 new task completion

	gen := NewGenerator(backend, cfg)

	// Generate first time
	brief1, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("first Generate() error: %v", err)
	}

	// Complete a task to make cache stale
	seedCompletedTask(t, backend, "TASK-001", "New task", "")

	// Generate again — cache should be stale
	brief2, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("second Generate() error: %v", err)
	}

	if brief2.GeneratedAt.Equal(brief1.GeneratedAt) {
		t.Error("expected regenerated brief (different GeneratedAt), got cached")
	}
}

// ============================================================================
// SC-6: Empty/fresh project produces empty brief
// ============================================================================

func TestGenerator_EmptyProject_ReturnsEmptyBrief(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	gen := NewGenerator(backend, DefaultConfig())
	brief, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if brief == nil {
		t.Fatal("Generate() returned nil for empty project")
	}

	totalEntries := 0
	for _, s := range brief.Sections {
		totalEntries += len(s.Entries)
	}

	if totalEntries != 0 {
		t.Errorf("expected 0 entries for fresh project, got %d", totalEntries)
	}
}

func TestGenerator_EmptyProject_FormatsEmpty(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	gen := NewGenerator(backend, DefaultConfig())
	brief, err := gen.Generate(context.Background())
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	text := FormatBrief(brief)
	if text != "" {
		t.Errorf("FormatBrief() for empty project should return empty string, got %q", text)
	}
}

// ============================================================================
// SC-7: Brief output is structured text, not markdown prose
// ============================================================================

func TestFormatBrief_StructuredText(t *testing.T) {
	t.Parallel()

	brief := &Brief{
		GeneratedAt: time.Now(),
		TaskCount:   10,
		Sections: []Section{
			{
				Category: CategoryDecisions,
				Entries: []Entry{
					{Content: "Use JWT tokens", Source: "INIT-001", Impact: 0.9},
					{Content: "All DB through driver abstraction", Source: "INIT-041", Impact: 0.85},
				},
			},
			{
				Category: CategoryHotFiles,
				Entries: []Entry{
					{Content: "internal/executor/workflow_executor.go — 7 retries across 4 tasks", Source: "aggregate", Impact: 0.8},
				},
			},
		},
		TokenCount: 200,
	}

	text := FormatBrief(brief)

	// Should have section headers
	if !strings.Contains(text, "## Project Brief") {
		t.Error("should contain project brief header")
	}

	if !strings.Contains(text, "### Decisions") {
		t.Error("should contain Decisions section header")
	}

	if !strings.Contains(text, "### Hot Files") {
		t.Error("should contain Hot Files section header")
	}

	// Should contain entries as bullet points
	if !strings.Contains(text, "- Use JWT tokens") {
		t.Error("should contain decision entry as bullet")
	}

	// Should include source references
	if !strings.Contains(text, "[INIT-001]") {
		t.Error("should include source reference in brackets")
	}
}

func TestFormatBrief_OnlyPopulatedSections(t *testing.T) {
	t.Parallel()

	brief := &Brief{
		GeneratedAt: time.Now(),
		TaskCount:   5,
		Sections: []Section{
			{Category: CategoryDecisions, Entries: []Entry{{Content: "Use JWT", Source: "INIT-001", Impact: 0.9}}},
			{Category: CategoryHotFiles, Entries: nil},                   // Empty
			{Category: CategoryRecentFindings, Entries: []Entry{}},       // Empty
			{Category: CategoryPatterns, Entries: nil},                   // Empty
			{Category: CategoryKnownIssues, Entries: []Entry{{Content: "FTS5 issue", Source: "TASK-791", Impact: 0.7}}},
		},
		TokenCount: 100,
	}

	text := FormatBrief(brief)

	// Should include populated sections
	if !strings.Contains(text, "### Decisions") {
		t.Error("should include Decisions section")
	}
	if !strings.Contains(text, "### Known Issues") {
		t.Error("should include Known Issues section")
	}

	// Should NOT include empty sections
	if strings.Contains(text, "### Hot Files") {
		t.Error("should not include empty Hot Files section")
	}
	if strings.Contains(text, "### Recent Findings") {
		t.Error("should not include empty Recent Findings section")
	}
	if strings.Contains(text, "### Patterns") {
		t.Error("should not include empty Patterns section")
	}
}

// ============================================================================
// Error paths / failure modes
// ============================================================================

func TestGenerator_HandlesDatabaseErrors(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)

	// Close backend to simulate database error
	_ = backend.Close()

	gen := NewGenerator(backend, DefaultConfig())
	_, err := gen.Generate(context.Background())
	if err == nil {
		t.Error("expected error when database is closed")
	}
}

func TestCache_HandleCorruptedFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cachePath := filepath.Join(dir, "brief.cache")

	// Write garbage to cache file
	if err := os.WriteFile(cachePath, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("write corrupt cache: %v", err)
	}

	cache := NewCache(cachePath)

	// Load should return nil (treat corrupt cache as missing)
	loaded, err := cache.Load()
	if err != nil {
		t.Fatalf("Load() should not error on corrupt file, got: %v", err)
	}
	if loaded != nil {
		t.Error("Load() should return nil for corrupt file")
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.MaxTokens != 3000 {
		t.Errorf("default MaxTokens = %d, want 3000", cfg.MaxTokens)
	}

	if cfg.SectionBudgets[CategoryDecisions] != 800 {
		t.Errorf("default decisions budget = %d, want 800", cfg.SectionBudgets[CategoryDecisions])
	}

	if cfg.SectionBudgets[CategoryHotFiles] != 600 {
		t.Errorf("default hot_files budget = %d, want 600", cfg.SectionBudgets[CategoryHotFiles])
	}

	if cfg.SectionBudgets[CategoryPatterns] != 500 {
		t.Errorf("default patterns budget = %d, want 500", cfg.SectionBudgets[CategoryPatterns])
	}

	if cfg.SectionBudgets[CategoryKnownIssues] != 500 {
		t.Errorf("default known_issues budget = %d, want 500", cfg.SectionBudgets[CategoryKnownIssues])
	}

	if cfg.SectionBudgets[CategoryRecentFindings] != 600 {
		t.Errorf("default recent_findings budget = %d, want 600", cfg.SectionBudgets[CategoryRecentFindings])
	}

	if cfg.StaleThreshold != 3 {
		t.Errorf("default StaleThreshold = %d, want 3", cfg.StaleThreshold)
	}
}

// ============================================================================
// Test helpers
// ============================================================================

func seedCompletedTask(t *testing.T, backend *storage.DatabaseBackend, id, title, initiativeID string) {
	t.Helper()

	tk := task.NewProtoTask(id, title)
	tk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if initiativeID != "" {
		tk.InitiativeId = &initiativeID
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("save task %s: %v", id, err)
	}
}

func seedInitiativeWithDecisions(t *testing.T, backend *storage.DatabaseBackend, id, title string, decisions []initiative.Decision) {
	t.Helper()

	init := initiative.New(id, title)
	init.Status = initiative.StatusActive
	init.Decisions = decisions
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative %s: %v", id, err)
	}
}

func seedReviewFindings(t *testing.T, backend *storage.DatabaseBackend, taskID string, round int, findings []*orcv1.ReviewFinding) {
	t.Helper()

	rf := &orcv1.ReviewRoundFindings{
		TaskId:  taskID,
		Round:   int32(round),
		Summary: "Review findings for " + taskID,
		Issues:  findings,
	}
	if err := backend.SaveReviewFindings(rf); err != nil {
		t.Fatalf("save review findings for %s: %v", taskID, err)
	}
}

func findSection(brief *Brief, category string) *Section {
	if brief == nil {
		return nil
	}
	for i := range brief.Sections {
		if brief.Sections[i].Category == category {
			return &brief.Sections[i]
		}
	}
	return nil
}

func strPtr(s string) *string  { return &s }
func int32Ptr(i int32) *int32  { return &i }

func taskIDForIndex(i int) string {
	return fmt.Sprintf("TASK-%03d", i+1)
}
