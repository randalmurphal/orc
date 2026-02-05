package task

import (
	"fmt"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// IsClaimStale checks whether a task's execution claim is stale based on its
// heartbeat timestamp. A claim is stale if the task is running and its last
// heartbeat exceeds StaleHeartbeatThreshold (15 minutes).
//
// This is a read-only check — it does NOT modify the task or auto-release claims.
// Returns (isStale, detail) where detail explains why the claim is considered stale.
func IsClaimStale(t *orcv1.Task) (bool, string) {
	if t == nil {
		return false, ""
	}

	// Only running tasks can have stale claims
	if t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return false, ""
	}

	// No heartbeat recorded — treat as stale
	if t.LastHeartbeat == nil {
		return true, "no heartbeat recorded"
	}

	age := time.Since(t.LastHeartbeat.AsTime())
	if age > StaleHeartbeatThreshold {
		return true, fmt.Sprintf("heartbeat stale (last update %s ago, threshold %s)", age.Truncate(time.Second), StaleHeartbeatThreshold)
	}

	return false, ""
}

// FormatHeartbeatStatus returns a human-readable heartbeat status string for display.
// Returns empty string for nil or non-running tasks (heartbeat is irrelevant).
func FormatHeartbeatStatus(t *orcv1.Task) string {
	if t == nil || t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return ""
	}

	isStale, detail := IsClaimStale(t)
	if isStale {
		return fmt.Sprintf("heartbeat: stale (%s)", detail)
	}

	age := time.Since(t.LastHeartbeat.AsTime())
	return fmt.Sprintf("heartbeat: healthy (last update %s ago)", age.Truncate(time.Second))
}
