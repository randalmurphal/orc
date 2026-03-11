package api

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func pendingDecisionHasOption(decision *gate.PendingDecision, optionID string) bool {
	if decision == nil || optionID == "" {
		return false
	}

	for _, option := range decision.Options {
		if option.ID == optionID {
			return true
		}
	}

	return false
}

func resolvePendingDecision(
	backend storage.Backend,
	pendingDecisions *gate.PendingDecisionStore,
	publisher events.Publisher,
	projectID string,
	decisionID string,
	approved bool,
	reason string,
	resolvedBy string,
	selectedOption string,
) (*orcv1.ResolvedDecision, error) {
	if pendingDecisions == nil {
		return nil, fmt.Errorf("pending decisions not available")
	}

	decision, ok := pendingDecisions.Get(projectID, decisionID)
	if !ok {
		return nil, fmt.Errorf("decision not found: %s", decisionID)
	}
	if selectedOption != "" && !pendingDecisionHasOption(decision, selectedOption) {
		return nil, fmt.Errorf("decision option not found: %s", selectedOption)
	}

	t, err := backend.LoadTask(decision.TaskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %s", decision.TaskID)
	}
	if t.Status != orcv1.TaskStatus_TASK_STATUS_BLOCKED {
		return nil, fmt.Errorf("task is not blocked (status: %s)", t.Status.String())
	}

	currentPhase := task.GetCurrentPhaseProto(t)
	if currentPhase != decision.Phase {
		return nil, fmt.Errorf(
			"decision phase mismatch: task is at phase %q, decision is for phase %q",
			currentPhase,
			decision.Phase,
		)
	}

	now := time.Now()
	originalTask := proto.Clone(t).(*orcv1.Task)

	task.EnsureExecutionProto(t)
	gateDecision := &orcv1.GateDecision{
		Phase:     decision.Phase,
		GateType:  decision.GateType,
		Approved:  approved,
		Timestamp: timestamppb.New(now),
	}
	if reason != "" {
		gateDecision.Reason = &reason
	}
	t.Execution.Gates = append(t.Execution.Gates, gateDecision)

	if approved {
		t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	} else {
		t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	}
	task.UpdateTimestampProto(t)

	if err := transitionTaskWithAttentionSync(backend, publisher, projectID, originalTask, t, resolvedBy); err != nil {
		return nil, fmt.Errorf("failed to update task attention state: %w", err)
	}

	if publisher != nil {
		publisher.Publish(events.NewProjectEvent(
			events.EventDecisionResolved,
			projectID,
			decision.TaskID,
			events.DecisionResolvedData{
				DecisionID: decisionID,
				TaskID:     decision.TaskID,
				Phase:      decision.Phase,
				Approved:   approved,
				Reason:     reason,
				ResolvedBy: resolvedBy,
				ResolvedAt: now,
			},
		))
	}

	pendingDecisions.Remove(projectID, decisionID)

	resolved := &orcv1.ResolvedDecision{
		Id:         decisionID,
		TaskId:     decision.TaskID,
		Phase:      decision.Phase,
		Approved:   approved,
		ResolvedBy: resolvedBy,
		ResolvedAt: timestamppb.New(now),
	}
	if selectedOption != "" {
		resolved.SelectedOption = &selectedOption
	}
	if reason != "" {
		resolved.Reason = &reason
	}

	return resolved, nil
}
