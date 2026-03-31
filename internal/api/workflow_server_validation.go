package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// ValidateWorkflow checks workflow structure for cycles, invalid dependency
// references, and invalid loop_to_phase references.
func (s *workflowServer) ValidateWorkflow(
	ctx context.Context,
	req *connect.Request[orcv1.ValidateWorkflowRequest],
) (*connect.Response[orcv1.ValidateWorkflowResponse], error) {
	if req.Msg.WorkflowId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workflow_id is required"))
	}

	wf, err := s.globalDB.GetWorkflow(req.Msg.WorkflowId)
	if err != nil || wf == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("workflow %s not found", req.Msg.WorkflowId))
	}

	phases, err := s.globalDB.GetWorkflowPhases(req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get workflow phases: %w", err))
	}

	var issues []*orcv1.ValidationIssue
	phaseIDs := make(map[string]bool, len(phases))
	for _, p := range phases {
		phaseIDs[p.PhaseTemplateID] = true
	}

	for _, p := range phases {
		deps := parseDependsOnJSON(p.DependsOn)
		for _, dep := range deps {
			if !phaseIDs[dep] {
				issues = append(issues, &orcv1.ValidationIssue{
					Severity: "error",
					Message:  fmt.Sprintf("phase %q depends on non-existent phase %q", p.PhaseTemplateID, dep),
					PhaseIds: []string{p.PhaseTemplateID},
				})
			}
		}
	}

	cyclePhases := detectCycles(phases, phaseIDs)
	if len(cyclePhases) > 0 {
		issues = append(issues, &orcv1.ValidationIssue{
			Severity: "error",
			Message:  fmt.Sprintf("cycle detected involving phases: %s", strings.Join(cyclePhases, ", ")),
			PhaseIds: cyclePhases,
		})
	}

	for _, p := range phases {
		if p.LoopConfig == "" {
			continue
		}
		var lc struct {
			LoopToPhase string `json:"loop_to_phase"`
		}
		if err := json.Unmarshal([]byte(p.LoopConfig), &lc); err != nil {
			continue
		}
		if lc.LoopToPhase != "" && !phaseIDs[lc.LoopToPhase] {
			issues = append(issues, &orcv1.ValidationIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("phase %q has loop_to_phase referencing non-existent phase %q", p.PhaseTemplateID, lc.LoopToPhase),
				PhaseIds: []string{p.PhaseTemplateID},
			})
		}
	}

	hasErrors := false
	for _, issue := range issues {
		if issue.Severity == "error" {
			hasErrors = true
			break
		}
	}

	return connect.NewResponse(&orcv1.ValidateWorkflowResponse{
		Valid:  !hasErrors,
		Issues: issues,
	}), nil
}

// parseDependsOnJSON extracts phase template IDs from a JSON array string.
func parseDependsOnJSON(raw string) []string {
	if raw == "" || raw == "[]" {
		return nil
	}
	var deps []string
	if err := json.Unmarshal([]byte(raw), &deps); err != nil {
		return nil
	}
	return deps
}

// detectCycles uses Kahn's algorithm to find phases involved in cycles.
// Only considers dependencies that reference phases in the provided phaseIDs set.
func detectCycles(phases []*db.WorkflowPhase, phaseIDs map[string]bool) []string {
	adjacency := make(map[string][]string, len(phases))
	inDegree := make(map[string]int, len(phases))

	for _, p := range phases {
		inDegree[p.PhaseTemplateID] = 0
	}

	for _, p := range phases {
		deps := parseDependsOnJSON(p.DependsOn)
		seen := make(map[string]bool, len(deps))
		for _, dep := range deps {
			if seen[dep] || !phaseIDs[dep] {
				continue
			}
			seen[dep] = true
			adjacency[dep] = append(adjacency[dep], p.PhaseTemplateID)
			inDegree[p.PhaseTemplateID]++
		}
	}

	var queue []string
	for _, p := range phases {
		if inDegree[p.PhaseTemplateID] == 0 {
			queue = append(queue, p.PhaseTemplateID)
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++
		for _, depID := range adjacency[current] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				queue = append(queue, depID)
			}
		}
	}

	if processed == len(phases) {
		return nil
	}

	var cycled []string
	for id, deg := range inDegree {
		if deg > 0 {
			cycled = append(cycled, id)
		}
	}
	sort.Strings(cycled)
	return cycled
}
