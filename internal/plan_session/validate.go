package plan_session

import (
	"github.com/randalmurphal/orc/internal/task"
)

// ValidationResult wraps the task package's SpecValidation for plan_session use.
type ValidationResult = task.SpecValidation

// ValidateSpec validates a spec against requirements based on weight.
// This is a convenience wrapper around task.ValidateSpec.
func ValidateSpec(content string, weight task.Weight) *ValidationResult {
	return task.ValidateSpec(content, weight)
}
