package controlplane

import (
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
)

// TaskAttentionSummary returns the operator-facing summary for a task that
// needs attention. Metadata wins because blocked/failed tasks often carry a
// specific reason there instead of in the task description.
func TaskAttentionSummary(taskItem *orcv1.Task) string {
	if taskItem == nil {
		return ""
	}
	if taskItem.Metadata != nil {
		if blockedReason := taskItem.Metadata["blocked_reason"]; blockedReason != "" {
			return blockedReason
		}
		if failedError := taskItem.Metadata["failed_error"]; failedError != "" {
			return failedError
		}
	}
	return task.GetDescriptionProto(taskItem)
}

// MergeTaskAttentionSignals combines persisted attention signals with
// compatibility signals synthesized from blocked and failed tasks that have not
// been backfilled yet. This preserves rollout parity while persisted state is
// becoming the source of truth.
func MergeTaskAttentionSignals(
	projectID string,
	tasks []*orcv1.Task,
	persisted []*PersistedAttentionSignal,
) []*PersistedAttentionSignal {
	merged := make([]*PersistedAttentionSignal, 0, len(persisted)+len(tasks))
	coveredTaskIDs := make(map[string]struct{}, len(persisted))

	for _, signal := range persisted {
		if signal == nil {
			continue
		}
		copied := *signal
		if copied.ProjectID == "" && projectID != "" {
			copied.ProjectID = projectID
		}
		merged = append(merged, &copied)
		if copied.ReferenceType == AttentionSignalReferenceTypeTask && copied.ReferenceID != "" {
			coveredTaskIDs[copied.ReferenceID] = struct{}{}
		}
	}

	now := time.Now()
	for _, taskItem := range tasks {
		if taskItem == nil {
			continue
		}

		status, ok := attentionSignalStatusForTask(taskItem.GetStatus())
		if !ok {
			continue
		}
		if _, exists := coveredTaskIDs[taskItem.GetId()]; exists {
			continue
		}

		timestamp := taskTimestampOrNow(taskItem, now)
		merged = append(merged, &PersistedAttentionSignal{
			ProjectID:     projectID,
			Kind:          AttentionSignalKindBlocker,
			Status:        status,
			ReferenceType: AttentionSignalReferenceTypeTask,
			ReferenceID:   taskItem.GetId(),
			Title:         taskItem.GetTitle(),
			Summary:       TaskAttentionSummary(taskItem),
			CreatedAt:     timestamp,
			UpdatedAt:     timestamp,
		})
	}

	return merged
}

func attentionSignalStatusForTask(status orcv1.TaskStatus) (string, bool) {
	switch status {
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return AttentionSignalStatusBlocked, true
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return AttentionSignalStatusFailed, true
	default:
		return "", false
	}
}

func taskTimestampOrNow(taskItem *orcv1.Task, fallback time.Time) time.Time {
	if taskItem == nil {
		return fallback
	}
	if taskItem.GetUpdatedAt() != nil {
		return taskItem.GetUpdatedAt().AsTime()
	}
	if taskItem.GetCreatedAt() != nil {
		return taskItem.GetCreatedAt().AsTime()
	}
	return fallback
}
