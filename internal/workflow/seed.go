package workflow

import (
	"log/slog"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// DefaultCodeQualityChecks is the JSON for standard code quality checks.
// Applied to the implement phase to run tests, lint, build, and typecheck after code changes.
const DefaultCodeQualityChecks = `[{"type":"code","name":"tests","enabled":true,"on_failure":"block"},{"type":"code","name":"lint","enabled":true,"on_failure":"block"},{"type":"code","name":"build","enabled":true,"on_failure":"block"},{"type":"code","name":"typecheck","enabled":true,"on_failure":"block"}]`

// SeedBuiltins populates the database with built-in phase templates and workflows.
// This uses YAML files as the source of truth (embedded in the binary).
// Returns the number of items seeded (templates + workflows).
func SeedBuiltins(gdb *db.GlobalDB) (int, error) {
	cache := NewCacheServiceFromOrcDir("", gdb)
	result, err := cache.SyncAll()
	if err != nil {
		return 0, err
	}

	total := result.WorkflowsAdded + result.WorkflowsUpdated + result.PhasesAdded + result.PhasesUpdated

	if len(result.Errors) > 0 {
		slog.Warn("seed completed with errors",
			"total", total,
			"errors", result.Errors)
	}

	return total, nil
}

// SeedBuiltinsFromDir populates the database with built-in phase templates and workflows
// using a specific orc directory. This is useful for testing.
func SeedBuiltinsFromDir(gdb *db.GlobalDB, orcDir string) (int, error) {
	cache := NewCacheServiceFromOrcDir(orcDir, gdb)
	result, err := cache.SyncAll()
	if err != nil {
		return 0, err
	}

	total := result.WorkflowsAdded + result.WorkflowsUpdated + result.PhasesAdded + result.PhasesUpdated
	return total, nil
}

// EnsureBuiltinsSynced ensures the database is up to date with YAML files.
// This is a more lightweight check than SeedBuiltins - it only syncs if stale.
// Returns true if sync was performed.
func EnsureBuiltinsSynced(gdb *db.GlobalDB) (bool, error) {
	cache := NewCacheServiceFromOrcDir("", gdb)
	return cache.EnsureSynced()
}

// ListBuiltinWorkflowIDs returns all built-in workflow IDs.
// This reads from embedded YAML files.
func ListBuiltinWorkflowIDs() []string {
	resolver := NewResolver(WithEmbedded(true))
	workflows, err := resolver.ListWorkflows()
	if err != nil {
		slog.Warn("failed to list workflows", "error", err)
		return nil
	}

	ids := make([]string, 0, len(workflows))
	for _, rw := range workflows {
		if rw.Source == SourceEmbedded {
			ids = append(ids, rw.Workflow.ID)
		}
	}
	return ids
}

// ListBuiltinPhaseIDs returns all built-in phase template IDs.
// This reads from embedded YAML files.
func ListBuiltinPhaseIDs() []string {
	resolver := NewResolver(WithEmbedded(true))
	phases, err := resolver.ListPhases()
	if err != nil {
		slog.Warn("failed to list phases", "error", err)
		return nil
	}

	ids := make([]string, 0, len(phases))
	for _, rp := range phases {
		if rp.Source == SourceEmbedded {
			ids = append(ids, rp.Phase.ID)
		}
	}
	return ids
}

// WeightToWorkflowID returns the default workflow ID for a task weight.
// Returns empty string for unspecified or invalid weight.
// This uses hardcoded defaults. For config-based resolution, use
// config.WeightsConfig.GetWorkflowID(weight).
func WeightToWorkflowID(weight orcv1.TaskWeight) string {
	return WeightToWorkflowIDString(weight.String())
}

// WeightToWorkflowIDString returns the default workflow ID for a weight string.
// This is the string-based version that uses hardcoded defaults.
// For config-based resolution, use config.WeightsConfig.GetWorkflowID(weight).
func WeightToWorkflowIDString(weight string) string {
	switch weight {
	case "TASK_WEIGHT_TRIVIAL", "trivial":
		return "implement-trivial"
	case "TASK_WEIGHT_SMALL", "small":
		return "implement-small"
	case "TASK_WEIGHT_MEDIUM", "medium":
		return "implement-medium"
	case "TASK_WEIGHT_LARGE", "large":
		return "implement-large"
	default:
		return ""
	}
}

// MigratePhaseTemplateModels is now a no-op since YAML files are the source of truth.
// The CacheService handles updates automatically during SeedBuiltins/EnsureBuiltinsSynced.
// This function is kept for backwards compatibility.
// Returns 0 (no updates needed - handled by cache sync).
func MigratePhaseTemplateModels(_ *db.ProjectDB) (int, error) {
	// No-op: YAML files are authoritative, cache sync handles updates
	return 0, nil
}
