// Package brief generates auto-generated project context briefs from task history.
// The brief summarizes decisions, findings, hot files, patterns, and known issues
// to inject into phase prompts via the {{PROJECT_BRIEF}} variable.
package brief

import (
	"context"
	"fmt"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// Section categories.
const (
	CategoryDecisions        = "decisions"
	CategoryRecentFindings   = "recent_findings"
	CategoryIndexedArtifacts = "indexed_artifacts"
	CategoryHotFiles         = "hot_files"
	CategoryPatterns         = "patterns"
	CategoryKnownIssues      = "known_issues"
)

// Brief is the generated project context summary.
type Brief struct {
	GeneratedAt      time.Time
	TaskCount        int
	TokenCount       int
	LatestArtifactID int64
	Sections         []Section
}

// Section groups entries by category.
type Section struct {
	Category string
	Entries  []Entry
}

// Entry is a single piece of context within a section.
type Entry struct {
	Content string
	Source  string
	Impact  float64
}

// Config controls brief generation behavior.
type Config struct {
	MaxTokens      int
	SectionBudgets map[string]int
	CachePath      string
	StaleThreshold int
}

// DefaultConfig returns the default brief generation config.
func DefaultConfig() Config {
	return Config{
		MaxTokens: 3000,
		SectionBudgets: map[string]int{
			CategoryDecisions:        800,
			CategoryIndexedArtifacts: 600,
			CategoryHotFiles:         600,
			CategoryPatterns:         500,
			CategoryKnownIssues:      500,
			CategoryRecentFindings:   600,
		},
		StaleThreshold: 3,
	}
}

// Generator produces project briefs from task history.
type Generator struct {
	backend *storage.DatabaseBackend
	cfg     Config
	cache   *Cache
}

// NewGenerator creates a new brief generator.
func NewGenerator(backend *storage.DatabaseBackend, cfg Config) *Generator {
	var cache *Cache
	if cfg.CachePath != "" {
		cache = NewCache(cfg.CachePath)
	}
	return &Generator{
		backend: backend,
		cfg:     cfg,
		cache:   cache,
	}
}

// Generate produces a project brief, using cache when available and fresh.
func (g *Generator) Generate(ctx context.Context) (*Brief, error) {
	// Count completed tasks for staleness check
	taskCount, err := g.countCompletedTasks()
	if err != nil {
		return nil, fmt.Errorf("count completed tasks: %w", err)
	}

	latestArtifactID, err := g.latestArtifactID()
	if err != nil {
		return nil, fmt.Errorf("load latest indexed artifact metadata: %w", err)
	}

	// Check cache
	if g.cache != nil {
		cached, err := g.cache.Load()
		if err != nil {
			return nil, fmt.Errorf("load cache: %w", err)
		}
		if cached != nil && !g.cache.IsStale(taskCount, g.cfg.StaleThreshold) && latestArtifactID <= cached.LatestArtifactID {
			return cached, nil
		}
	}

	// Generate fresh brief
	brief, err := g.generate(ctx, taskCount, latestArtifactID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if g.cache != nil {
		if err := g.cache.Store(brief); err != nil {
			return nil, fmt.Errorf("store cache: %w", err)
		}
	}

	return brief, nil
}

func (g *Generator) generate(ctx context.Context, taskCount int, latestArtifactID int64) (*Brief, error) {
	var sections []Section

	// Extract decisions from active initiatives
	decisions, err := ExtractDecisions(ctx, g.backend)
	if err != nil {
		return nil, fmt.Errorf("extract decisions: %w", err)
	}
	if len(decisions) > 0 {
		sec := Section{Category: CategoryDecisions, Entries: decisions}
		sec = ApplyTokenBudget(sec, g.cfg.SectionBudgets[CategoryDecisions])
		sections = append(sections, sec)
	}

	// Extract high-severity review findings
	findings, err := ExtractFindings(ctx, g.backend)
	if err != nil {
		return nil, fmt.Errorf("extract findings: %w", err)
	}
	if len(findings) > 0 {
		sec := Section{Category: CategoryRecentFindings, Entries: findings}
		sec = ApplyTokenBudget(sec, g.cfg.SectionBudgets[CategoryRecentFindings])
		sections = append(sections, sec)
	}

	indexedArtifacts, err := ExtractIndexedArtifacts(ctx, g.backend)
	if err != nil {
		return nil, fmt.Errorf("extract indexed artifacts: %w", err)
	}
	if len(indexedArtifacts) > 0 {
		sec := Section{Category: CategoryIndexedArtifacts, Entries: indexedArtifacts}
		sec = ApplyTokenBudget(sec, g.cfg.SectionBudgets[CategoryIndexedArtifacts])
		sections = append(sections, sec)
	}

	// Apply total budget across all sections
	sections = ApplyTotalBudget(sections, g.cfg.MaxTokens)

	// Calculate total token count
	totalTokens := 0
	for _, s := range sections {
		for _, e := range s.Entries {
			totalTokens += EstimateTokens(e.Content)
		}
	}

	return &Brief{
		GeneratedAt:      time.Now().Round(0), // Strip monotonic reading for JSON round-trip equality
		TaskCount:        taskCount,
		TokenCount:       totalTokens,
		LatestArtifactID: latestArtifactID,
		Sections:         sections,
	}, nil
}

func (g *Generator) latestArtifactID() (int64, error) {
	entries, err := g.backend.GetRecentArtifacts(db.RecentArtifactOpts{Limit: 1})
	if err != nil {
		return 0, err
	}
	if len(entries) == 0 {
		return 0, nil
	}
	return entries[0].ID, nil
}

// Invalidate removes the cached brief, forcing regeneration on the next Generate call.
func (g *Generator) Invalidate() error {
	if g.cache != nil {
		return g.cache.Invalidate()
	}
	return nil
}

func (g *Generator) countCompletedTasks() (int, error) {
	tasks, err := g.backend.LoadAllTasks()
	if err != nil {
		return 0, fmt.Errorf("load tasks: %w", err)
	}

	count := 0
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			count++
		}
	}
	return count, nil
}
