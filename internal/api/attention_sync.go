package api

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
)

func transitionTaskWithAttentionSync(
	backend storage.Backend,
	publisher events.Publisher,
	projectID string,
	originalTask *orcv1.Task,
	nextTask *orcv1.Task,
	resolvedBy string,
) error {
	if err := persistTaskWithAttentionSync(backend, publisher, projectID, nextTask, resolvedBy); err != nil {
		if originalTask != nil {
			rollbackTask := proto.Clone(originalTask).(*orcv1.Task)
			if rollbackErr := persistTaskWithAttentionSync(backend, publisher, projectID, rollbackTask, resolvedBy+"_rollback"); rollbackErr != nil {
				return fmt.Errorf("persist task state with attention sync: %w (rollback failed: %v)", err, rollbackErr)
			}
		}
		return err
	}
	return nil
}

func persistTaskWithAttentionSync(
	backend storage.Backend,
	publisher events.Publisher,
	projectID string,
	taskItem *orcv1.Task,
	resolvedBy string,
) error {
	if taskItem == nil {
		return nil
	}
	if err := backend.SaveTask(taskItem); err != nil {
		return fmt.Errorf("save task %s: %w", taskItem.GetId(), err)
	}
	if err := syncTaskAttentionSignals(backend, publisher, projectID, taskItem, resolvedBy); err != nil {
		return fmt.Errorf("sync attention signals for task %s: %w", taskItem.GetId(), err)
	}
	publishTaskUpdatedEvent(publisher, projectID, taskItem)
	return nil
}

func syncTaskAttentionSignals(
	backend storage.Backend,
	publisher events.Publisher,
	projectID string,
	taskItem *orcv1.Task,
	resolvedBy string,
) error {
	if taskItem == nil {
		return nil
	}

	switch taskItem.GetStatus() {
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return saveTaskAttentionSignal(backend, publisher, projectID, taskItem, controlplane.AttentionSignalStatusBlocked)
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return saveTaskAttentionSignal(backend, publisher, projectID, taskItem, controlplane.AttentionSignalStatusFailed)
	case orcv1.TaskStatus_TASK_STATUS_RUNNING,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return resolveTaskAttentionSignals(backend, publisher, projectID, taskItem.GetId(), resolvedBy)
	default:
		return nil
	}
}

func saveTaskAttentionSignal(
	backend storage.Backend,
	publisher events.Publisher,
	projectID string,
	taskItem *orcv1.Task,
	status string,
) error {
	signal := &controlplane.PersistedAttentionSignal{
		ProjectID:     projectID,
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        status,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskItem.GetId(),
		Title:         taskItem.GetTitle(),
		Summary:       controlplane.TaskAttentionSummary(taskItem),
	}
	if err := backend.SaveAttentionSignal(signal); err != nil {
		return err
	}
	publishAttentionSignalCreated(publisher, projectID, taskItem.GetId(), signal)
	return nil
}

func resolveTaskAttentionSignals(
	backend storage.Backend,
	publisher events.Publisher,
	projectID string,
	taskID string,
	resolvedBy string,
) error {
	if taskID == "" {
		return nil
	}

	signals, err := backend.LoadActiveAttentionSignals()
	if err != nil {
		return fmt.Errorf("load active attention signals: %w", err)
	}

	for _, signal := range signals {
		if signal == nil {
			continue
		}
		if signal.ReferenceType != controlplane.AttentionSignalReferenceTypeTask || signal.ReferenceID != taskID {
			continue
		}

		resolvedSignal, err := backend.ResolveAttentionSignal(signal.ID, resolvedBy)
		if err != nil {
			return fmt.Errorf("resolve attention signal %s: %w", signal.ID, err)
		}
		publishAttentionSignalResolved(publisher, projectID, taskID, resolvedSignal)
	}

	return nil
}

func publishTaskUpdatedEvent(publisher events.Publisher, projectID string, taskItem *orcv1.Task) {
	if publisher == nil || taskItem == nil {
		return
	}
	if projectID != "" {
		publisher.Publish(events.NewProjectEvent(events.EventTaskUpdated, projectID, taskItem.GetId(), taskItem))
		return
	}
	publisher.Publish(events.NewEvent(events.EventTaskUpdated, taskItem.GetId(), taskItem))
}

func publishAttentionSignalCreated(
	publisher events.Publisher,
	projectID string,
	taskID string,
	signal *controlplane.PersistedAttentionSignal,
) {
	if publisher == nil || signal == nil {
		return
	}

	publisher.Publish(events.NewProjectEvent(
		events.EventAttentionSignalCreated,
		projectID,
		taskID,
		events.AttentionSignalCreatedData{
			SignalID:      signal.ID,
			Kind:          string(signal.Kind),
			Status:        signal.Status,
			ReferenceType: signal.ReferenceType,
			ReferenceID:   signal.ReferenceID,
			Title:         signal.Title,
			Summary:       signal.Summary,
		},
	))
}

func publishAttentionSignalResolved(
	publisher events.Publisher,
	projectID string,
	taskID string,
	signal *controlplane.PersistedAttentionSignal,
) {
	if publisher == nil || signal == nil || signal.ResolvedAt == nil {
		return
	}

	publisher.Publish(events.NewProjectEvent(
		events.EventAttentionSignalResolved,
		projectID,
		taskID,
		events.AttentionSignalResolvedData{
			SignalID:      signal.ID,
			Kind:          string(signal.Kind),
			ReferenceType: signal.ReferenceType,
			ReferenceID:   signal.ReferenceID,
			ResolvedBy:    signal.ResolvedBy,
			ResolvedAt:    *signal.ResolvedAt,
		},
	))
}
