// Package executor implements phase condition evaluation for conditional
// phase execution in workflows.
package executor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// ConditionContext provides the evaluation context for phase conditions.
type ConditionContext struct {
	Task *orcv1.Task
	Vars variable.VariableSet
	RCtx *variable.ResolutionContext
}

// EvaluateCondition evaluates a JSON condition string against the given context.
// Returns (true, nil) if the condition is met or empty (no condition = always run).
// Returns (false, nil) if the condition is not met.
// Returns (false, error) for invalid JSON or unknown operators.
func EvaluateCondition(conditionJSON string, ctx *ConditionContext) (bool, error) {
	// Empty or null condition → always run
	trimmed := strings.TrimSpace(conditionJSON)
	if trimmed == "" || trimmed == "null" {
		return true, nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(conditionJSON), &raw); err != nil {
		return false, fmt.Errorf("parse condition JSON: %w", err)
	}

	return evaluateConditionMap(raw, ctx)
}

func evaluateConditionMap(raw map[string]json.RawMessage, ctx *ConditionContext) (bool, error) {
	_, hasField := raw["field"]
	_, hasOp := raw["op"]
	_, hasAll := raw["all"]
	_, hasAny := raw["any"]

	isSimple := hasField || hasOp
	isCompound := hasAll || hasAny

	if isSimple && isCompound {
		return false, fmt.Errorf("ambiguous condition: has both simple (field/op) and compound (all/any) fields")
	}

	if isCompound {
		return evaluateCompound(raw, ctx)
	}

	return evaluateSimple(raw, ctx)
}

func evaluateCompound(raw map[string]json.RawMessage, ctx *ConditionContext) (bool, error) {
	if allRaw, ok := raw["all"]; ok {
		var conditions []json.RawMessage
		if err := json.Unmarshal(allRaw, &conditions); err != nil {
			return false, fmt.Errorf("parse 'all' array: %w", err)
		}
		// Vacuous truth: empty all → true
		for _, condRaw := range conditions {
			var sub map[string]json.RawMessage
			if err := json.Unmarshal(condRaw, &sub); err != nil {
				return false, fmt.Errorf("parse sub-condition: %w", err)
			}
			result, err := evaluateConditionMap(sub, ctx)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil
	}

	if anyRaw, ok := raw["any"]; ok {
		var conditions []json.RawMessage
		if err := json.Unmarshal(anyRaw, &conditions); err != nil {
			return false, fmt.Errorf("parse 'any' array: %w", err)
		}
		// Empty any → false
		for _, condRaw := range conditions {
			var sub map[string]json.RawMessage
			if err := json.Unmarshal(condRaw, &sub); err != nil {
				return false, fmt.Errorf("parse sub-condition: %w", err)
			}
			result, err := evaluateConditionMap(sub, ctx)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil
	}

	return false, fmt.Errorf("compound condition has neither 'all' nor 'any'")
}

func evaluateSimple(raw map[string]json.RawMessage, ctx *ConditionContext) (bool, error) {
	var field, op string
	if err := json.Unmarshal(raw["field"], &field); err != nil {
		return false, fmt.Errorf("parse condition field: %w", err)
	}
	if err := json.Unmarshal(raw["op"], &op); err != nil {
		return false, fmt.Errorf("parse condition op: %w", err)
	}

	fieldValue := resolveField(field, ctx)

	switch op {
	case "exists":
		return fieldValue != "", nil
	case "eq":
		var value string
		if err := json.Unmarshal(raw["value"], &value); err != nil {
			return false, fmt.Errorf("parse condition value: %w", err)
		}
		return fieldValue == normalizeValue(value), nil
	case "neq":
		var value string
		if err := json.Unmarshal(raw["value"], &value); err != nil {
			return false, fmt.Errorf("parse condition value: %w", err)
		}
		return fieldValue != normalizeValue(value), nil
	case "in":
		var values []string
		if err := json.Unmarshal(raw["value"], &values); err != nil {
			return false, fmt.Errorf("'in' operator requires array value: %w", err)
		}
		for _, v := range values {
			if fieldValue == normalizeValue(v) {
				return true, nil
			}
		}
		return false, nil
	case "contains":
		var value string
		if err := json.Unmarshal(raw["value"], &value); err != nil {
			return false, fmt.Errorf("parse condition value: %w", err)
		}
		return strings.Contains(fieldValue, value), nil
	case "gt":
		var value string
		if err := json.Unmarshal(raw["value"], &value); err != nil {
			return false, fmt.Errorf("parse condition value: %w", err)
		}
		return compareGtLt(fieldValue, value, true), nil
	case "lt":
		var value string
		if err := json.Unmarshal(raw["value"], &value); err != nil {
			return false, fmt.Errorf("parse condition value: %w", err)
		}
		return compareGtLt(fieldValue, value, false), nil
	default:
		return false, fmt.Errorf("unknown operator: %q", op)
	}
}

// resolveField resolves a field reference to its string value.
// Returns empty string for unknown prefixes, nil task, or missing values.
func resolveField(field string, ctx *ConditionContext) string {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) < 2 {
		return ""
	}

	prefix := parts[0]
	name := parts[1]

	switch prefix {
	case "task":
		return resolveTaskField(name, ctx.Task)
	case "var":
		if ctx.Vars == nil {
			return ""
		}
		return ctx.Vars[name]
	case "env":
		if ctx.RCtx == nil || ctx.RCtx.Environment == nil {
			return ""
		}
		return ctx.RCtx.Environment[name]
	case "phase_output":
		return resolvePhaseOutputField(name, ctx)
	default:
		return ""
	}
}

// resolveTaskField resolves task.FIELD to its lowercase short form.
func resolveTaskField(name string, t *orcv1.Task) string {
	if t == nil {
		return ""
	}

	switch name {
	case "weight":
		return weightToShort(t.Weight)
	case "category":
		return categoryToShort(t.Category)
	case "priority":
		return priorityToShort(t.Priority)
	default:
		return ""
	}
}

// resolvePhaseOutputField resolves phase_output.PHASE.FIELD by parsing the
// prior output as JSON and extracting the nested field.
func resolvePhaseOutputField(nameAndField string, ctx *ConditionContext) string {
	if ctx.RCtx == nil || ctx.RCtx.PriorOutputs == nil {
		return ""
	}

	parts := strings.SplitN(nameAndField, ".", 2)
	if len(parts) < 2 {
		return ""
	}

	phase := parts[0]
	jsonField := parts[1]

	output, ok := ctx.RCtx.PriorOutputs[phase]
	if !ok {
		return ""
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return ""
	}

	val, ok := parsed[jsonField]
	if !ok {
		return ""
	}

	return fmt.Sprintf("%v", val)
}

// normalizeValue normalizes a comparison value. If it looks like a proto enum
// (e.g., "TASK_WEIGHT_MEDIUM"), convert to the short lowercase form.
func normalizeValue(value string) string {
	if strings.HasPrefix(value, "TASK_WEIGHT_") {
		return weightToShort(orcv1.TaskWeight(orcv1.TaskWeight_value[value]))
	}
	if strings.HasPrefix(value, "TASK_CATEGORY_") {
		return categoryToShort(orcv1.TaskCategory(orcv1.TaskCategory_value[value]))
	}
	if strings.HasPrefix(value, "TASK_PRIORITY_") {
		return priorityToShort(orcv1.TaskPriority(orcv1.TaskPriority_value[value]))
	}
	return value
}

func weightToShort(w orcv1.TaskWeight) string {
	switch w {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		return "trivial"
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		return "small"
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		return "medium"
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return "large"
	default:
		return ""
	}
}

func categoryToShort(c orcv1.TaskCategory) string {
	// Strip prefix "TASK_CATEGORY_" and lowercase
	s := c.String()
	if strings.HasPrefix(s, "TASK_CATEGORY_") {
		return strings.ToLower(strings.TrimPrefix(s, "TASK_CATEGORY_"))
	}
	return strings.ToLower(s)
}

func priorityToShort(p orcv1.TaskPriority) string {
	s := p.String()
	if strings.HasPrefix(s, "TASK_PRIORITY_") {
		return strings.ToLower(strings.TrimPrefix(s, "TASK_PRIORITY_"))
	}
	return strings.ToLower(s)
}

// compareGtLt compares two values. If both are numeric, compare numerically.
// Otherwise fall back to string comparison. gt=true means "greater than".
func compareGtLt(fieldValue, compareValue string, gt bool) bool {
	fNum, fErr := strconv.ParseFloat(fieldValue, 64)
	cNum, cErr := strconv.ParseFloat(compareValue, 64)

	if fErr == nil && cErr == nil {
		if gt {
			return fNum > cNum
		}
		return fNum < cNum
	}

	// String fallback
	if gt {
		return fieldValue > compareValue
	}
	return fieldValue < compareValue
}

// IsPhaseTerminalForResume returns true if a phase status indicates it should
// not be re-executed during resume. Both COMPLETED and SKIPPED phases are terminal.
func IsPhaseTerminalForResume(status orcv1.PhaseStatus) bool {
	return status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED ||
		status == orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
}

// SkipPhaseForCondition handles skipping a phase when its condition evaluates to false.
// It updates the task proto, the workflow run phase record, saves to the backend,
// and publishes a PhaseSkipped event.
func (we *WorkflowExecutor) SkipPhaseForCondition(
	t *orcv1.Task,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	phase *db.WorkflowPhase,
) error {
	phaseID := phase.PhaseTemplateID

	// Update task proto phase state
	task.SkipPhaseProto(t.Execution, phaseID, phase.Condition)

	// Update workflow run phase record
	now := time.Now()
	runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String()
	runPhase.CompletedAt = &now
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		return fmt.Errorf("save skipped run phase %s: %w", phaseID, err)
	}

	// Save task state
	if err := we.backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task after skip %s: %w", phaseID, err)
	}

	// Publish skip event
	we.publisher.PhaseSkipped(t.Id, phaseID)

	we.logger.Info("phase skipped by condition", "phase", phaseID, "condition", phase.Condition)

	return nil
}
