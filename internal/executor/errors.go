package executor

import "errors"

// ErrSyncConflict is returned when sync encounters merge conflicts.
var ErrSyncConflict = errors.New("sync conflict detected")

// ErrTaskBlocked is returned when task execution completes but requires
// user intervention (e.g., sync conflicts, merge failures).
var ErrTaskBlocked = errors.New("task blocked")

// SyncPhase indicates when sync is being performed.
type SyncPhase string

const (
	// SyncPhaseStart indicates sync at task start or phase start
	SyncPhaseStart SyncPhase = "start"
	// SyncPhaseCompletion indicates sync before PR/merge
	SyncPhaseCompletion SyncPhase = "completion"
)
