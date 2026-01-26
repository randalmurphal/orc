package plan_session

import (
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// ValidateSpec validates a spec against requirements based on weight.
// This is a convenience wrapper around task.ValidateSpec.
func ValidateSpec(content string, weight orcv1.TaskWeight) *task.SpecValidation {
	return task.ValidateSpec(content, weight)
}
