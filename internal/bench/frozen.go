package bench

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/randalmurphal/orc/internal/variable"
)

// FrozenOutputMap maps phase ID → output content for injection into variables.
type FrozenOutputMap map[string]*FrozenOutput

// BuildVarsFromFrozen injects frozen phase outputs into a variable set.
// This is the core of phase-isolation testing: phases that aren't being tested
// get their outputs replayed from the baseline, keeping everything else constant.
func BuildVarsFromFrozen(vars variable.VariableSet, frozen FrozenOutputMap) {
	for _, fo := range frozen {
		if fo.OutputVarName != "" && fo.OutputContent != "" {
			vars[fo.OutputVarName] = fo.OutputContent
		}
	}
}

// LoadFrozenOutputs loads all frozen outputs for a task from the baseline variant.
// Returns a map of phase_id → FrozenOutput.
func LoadFrozenOutputs(ctx context.Context, store *Store, taskID, baselineVariantID string, trial int) (FrozenOutputMap, error) {
	outputs, err := store.GetFrozenOutputsForTask(ctx, taskID, baselineVariantID, trial)
	if err != nil {
		return nil, fmt.Errorf("load frozen outputs for task %s: %w", taskID, err)
	}

	result := make(FrozenOutputMap, len(outputs))
	for _, fo := range outputs {
		result[fo.PhaseID] = fo
	}
	return result, nil
}

// SaveFrozenFromResult saves a phase's output as a frozen output for future replay.
func SaveFrozenFromResult(ctx context.Context, store *Store, taskID, phaseID, variantID, outputVarName, content string, trial int) error {
	fo := &FrozenOutput{
		ID:            uuid.New().String(),
		TaskID:        taskID,
		PhaseID:       phaseID,
		VariantID:     variantID,
		TrialNumber:   trial,
		OutputContent: content,
		OutputVarName: outputVarName,
	}
	return store.SaveFrozenOutput(ctx, fo)
}
