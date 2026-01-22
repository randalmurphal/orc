package plan

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/task"
)

// MigrationResult contains the result of a plan migration.
type MigrationResult struct {
	// NewPlan is the migrated plan
	NewPlan *Plan
	// OldPhases lists the IDs of phases in the old plan
	OldPhases []string
	// NewPhases lists the IDs of phases in the new plan
	NewPhases []string
	// PreservedCount is the number of phases whose status was preserved
	PreservedCount int
	// ResetCount is the number of phases that were reset to pending
	ResetCount int
	// Reason explains why the migration was performed
	Reason string
}

// IsPlanStale checks if a plan needs migration.
// Returns true and a reason if the plan is stale.
// Returns false if the plan is current or cannot be verified.
func IsPlanStale(p *Plan, t *task.Task) (bool, string) {
	// Check 1: Nil plan
	if p == nil {
		return true, "plan is nil"
	}

	// Check 2: Empty phases
	if len(p.Phases) == 0 {
		return true, "plan has no phases"
	}

	// Load current template for task's weight
	tmpl, err := LoadTemplate(t.Weight)
	if err != nil {
		// Can't compare, assume OK (unknown weight or template not found)
		return false, ""
	}

	// Check 3: Version mismatch
	if p.Version < tmpl.Version {
		return true, fmt.Sprintf("version %d < template %d", p.Version, tmpl.Version)
	}

	// Check 4: Phase sequence mismatch
	if !phaseSequenceMatches(p.Phases, tmpl.Phases) {
		return true, "phase sequence differs from template"
	}

	// Check 5: Any inline prompts (legacy)
	for _, phase := range p.Phases {
		if phase.Prompt != "" {
			return true, "has inline prompts (legacy format)"
		}
	}

	return false, ""
}

// phaseSequenceMatches checks if two phase lists have the same phase IDs in the same order.
func phaseSequenceMatches(planPhases, tmplPhases []Phase) bool {
	if len(planPhases) != len(tmplPhases) {
		return false
	}

	for i := range planPhases {
		if planPhases[i].ID != tmplPhases[i].ID {
			return false
		}
	}

	return true
}

// MigratePlan creates a new plan for a task from its current template,
// preserving completed/skipped statuses for phases that exist in both old and new plans.
// All inline prompts are cleared to force template usage.
func MigratePlan(t *task.Task, oldPlan *Plan) (*MigrationResult, error) {
	// Detect staleness reason for result
	reason := ""
	if oldPlan != nil {
		_, reason = IsPlanStale(oldPlan, t)
	} else {
		reason = "no existing plan"
	}

	// Collect old phase IDs
	oldPhases := make([]string, 0)
	if oldPlan != nil {
		for _, phase := range oldPlan.Phases {
			oldPhases = append(oldPhases, phase.ID)
		}
	}

	// Reuse RegeneratePlan for the heavy lifting
	regenerateResult, err := RegeneratePlan(t, oldPlan)
	if err != nil {
		return nil, fmt.Errorf("regenerate plan for %s: %w", t.ID, err)
	}

	// Clear all inline prompts (they should load from templates)
	for i := range regenerateResult.NewPlan.Phases {
		regenerateResult.NewPlan.Phases[i].Prompt = ""
	}

	// Collect new phase IDs
	newPhases := make([]string, len(regenerateResult.NewPlan.Phases))
	for i, phase := range regenerateResult.NewPlan.Phases {
		newPhases[i] = phase.ID
	}

	return &MigrationResult{
		NewPlan:        regenerateResult.NewPlan,
		OldPhases:      oldPhases,
		NewPhases:      newPhases,
		PreservedCount: len(regenerateResult.PreservedPhases),
		ResetCount:     len(regenerateResult.ResetPhases),
		Reason:         reason,
	}, nil
}
