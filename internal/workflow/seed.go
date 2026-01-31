package workflow

import (
	"fmt"
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

	// Seed hook scripts to GlobalDB so they're available for phase settings
	if _, err := SeedHookScripts(gdb); err != nil {
		return 0, fmt.Errorf("seed hook scripts: %w", err)
	}

	total := result.WorkflowsAdded + result.WorkflowsUpdated + result.PhasesAdded + result.PhasesUpdated

	if len(result.Errors) > 0 {
		slog.Warn("seed completed with errors",
			"total", total,
			"errors", result.Errors)
	}

	return total, nil
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

// IsWeightBasedWorkflow returns true if the workflow ID is one of the
// standard weight-based workflows (implement-trivial/small/medium/large).
func IsWeightBasedWorkflow(workflowID string) bool {
	switch workflowID {
	case "implement-trivial", "implement-small", "implement-medium", "implement-large":
		return true
	default:
		return false
	}
}

