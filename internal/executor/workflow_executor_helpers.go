package executor

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

func (we *WorkflowExecutor) phaseRequiresPassingReview(phaseID string) bool {
	if we.orcConfig == nil || !we.orcConfig.Review.RequirePass {
		return false
	}
	return phaseID == "review" || phaseID == "review_cross"
}

// applyPhaseContentToVars updates variable maps with phase output content.
func applyPhaseContentToVars(vars map[string]string, rctx *variable.ResolutionContext, phaseID, content, outputVarName string) {
	vars["OUTPUT_"+phaseID] = content

	varName := outputVarName
	if varName == "" {
		varName = "OUTPUT_" + strings.ToUpper(strings.ReplaceAll(phaseID, "-", "_"))
	}

	vars[varName] = content

	if rctx.PhaseOutputVars == nil {
		rctx.PhaseOutputVars = make(map[string]string)
	}
	rctx.PhaseOutputVars[varName] = content

	if phaseID == "qa_e2e_test" {
		result, err := ParseQAE2ETestResult(content)
		if err == nil {
			vars[varName] = result.FormatFindingsForFix()
		} else {
			vars[varName] = content
		}
		rctx.QAFindings = vars[varName]
	}

	if rctx.PriorOutputs != nil {
		rctx.PriorOutputs[phaseID] = content
	}
}

// evaluateLoopCondition checks if a legacy loop condition is met based on phase output.
func (we *WorkflowExecutor) evaluateLoopCondition(condition, phaseID string, vars map[string]string, rctx *variable.ResolutionContext) bool {
	output := ""
	if o, ok := rctx.PriorOutputs[phaseID]; ok {
		output = o
	} else if o, ok := vars["OUTPUT_"+phaseID]; ok {
		output = o
	}

	if output == "" {
		return false
	}

	switch condition {
	case "has_findings":
		var result QAE2ETestResult
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			we.logger.Warn("failed to parse QA findings for loop condition", "error", err)
			return false
		}
		hasFindingsToFix := len(result.Findings) > 0
		we.logger.Debug("loop condition has_findings evaluated", "findings_count", len(result.Findings), "should_loop", hasFindingsToFix)
		return hasFindingsToFix
	case "not_empty":
		return output != "" && output != "{}" && output != "[]"
	case "status_needs_fix":
		var statusCheck struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(output), &statusCheck); err != nil {
			return false
		}
		return statusCheck.Status == "needs_fix" || statusCheck.Status == "findings"
	default:
		we.logger.Warn("unknown loop condition", "condition", condition)
		return false
	}
}

// createTaskForRunProto creates a proto task for a default context run.
func (we *WorkflowExecutor) createTaskForRunProto(opts WorkflowRunOptions, workflowID string) (*orcv1.Task, error) {
	taskID, err := we.backend.GetNextTaskID()
	if err != nil {
		return nil, fmt.Errorf("get next task ID: %w", err)
	}

	t := task.NewProtoTask(taskID, truncateTitle(opts.Prompt))
	task.SetDescriptionProto(t, opts.Prompt)
	t.WorkflowId = &workflowID

	if opts.Category != orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED {
		t.Category = opts.Category
	}

	if err := we.saveTaskStrict(t, "save task"); err != nil {
		return nil, err
	}

	return t, nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func truncateTitle(s string) string {
	const maxLen = 80
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// extractPhaseOutput extracts the phase output content from JSON.
func extractPhaseOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}

	var generic map[string]any
	if err := json.Unmarshal([]byte(output), &generic); err != nil {
		return ""
	}

	if content, ok := generic["content"].(string); ok && content != "" {
		return content
	}

	return output
}
