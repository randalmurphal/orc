package plan_session

import (
	"github.com/randalmurphal/orc/internal/task"
)

// ValidateSpec validates a spec against requirements based on weight.
// This is a convenience wrapper around task.ValidateSpec.
func ValidateSpec(content string, weight task.Weight) *task.SpecValidation {
	return task.ValidateSpec(content, weight)
}
