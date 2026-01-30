// Package task provides proto-based task helper functions.
// These functions operate on orcv1.Task proto types, providing the same
// functionality as the original Task methods but as standalone functions.
package task

import (
	"slices"
	"sort"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewProtoTask creates a new proto Task with sensible defaults.
func NewProtoTask(id, title string) *orcv1.Task {
	now := timestamppb.Now()
	return &orcv1.Task{
		Id:        id,
		Title:     title,
		Status:    orcv1.TaskStatus_TASK_STATUS_CREATED,
		Branch:    "orc/" + id,
		Queue:     orcv1.TaskQueue_TASK_QUEUE_ACTIVE,
		Priority:  orcv1.TaskPriority_TASK_PRIORITY_NORMAL,
		Category:  orcv1.TaskCategory_TASK_CATEGORY_FEATURE,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
		Execution: InitProtoExecutionState(),
	}
}

// InitProtoExecutionState creates a new ExecutionState with initialized maps.
func InitProtoExecutionState() *orcv1.ExecutionState {
	return &orcv1.ExecutionState{
		Phases: make(map[string]*orcv1.PhaseState),
		Tokens: &orcv1.TokenUsage{},
		Cost:   &orcv1.CostTracking{},
	}
}

// IsTerminalProto returns true if the task is in a terminal state.
func IsTerminalProto(t *orcv1.Task) bool {
	if t == nil {
		return false
	}
	return t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED ||
		t.Status == orcv1.TaskStatus_TASK_STATUS_FAILED ||
		t.Status == orcv1.TaskStatus_TASK_STATUS_RESOLVED
}

// CanRunProto returns true if the task can be executed.
func CanRunProto(t *orcv1.Task) bool {
	if t == nil {
		return false
	}
	return t.Status == orcv1.TaskStatus_TASK_STATUS_CREATED ||
		t.Status == orcv1.TaskStatus_TASK_STATUS_PLANNED ||
		t.Status == orcv1.TaskStatus_TASK_STATUS_PAUSED ||
		t.Status == orcv1.TaskStatus_TASK_STATUS_BLOCKED
}

// IsDoneProto returns true if the status indicates the task has completed its work.
func IsDoneProto(s orcv1.TaskStatus) bool {
	return s == orcv1.TaskStatus_TASK_STATUS_COMPLETED ||
		s == orcv1.TaskStatus_TASK_STATUS_RESOLVED
}

// GetQueueProto returns the task's queue, defaulting to active if not set or unspecified.
func GetQueueProto(t *orcv1.Task) orcv1.TaskQueue {
	if t == nil || t.Queue == orcv1.TaskQueue_TASK_QUEUE_UNSPECIFIED {
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	}
	return t.Queue
}

// GetPriorityProto returns the task's priority, defaulting to normal if not set.
func GetPriorityProto(t *orcv1.Task) orcv1.TaskPriority {
	if t == nil || t.Priority == orcv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED {
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	}
	return t.Priority
}

// GetCategoryProto returns the task's category, defaulting to feature if not set.
func GetCategoryProto(t *orcv1.Task) orcv1.TaskCategory {
	if t == nil || t.Category == orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED {
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	}
	return t.Category
}

// ElapsedProto returns the duration since execution started.
// Returns 0 if the task hasn't started (StartedAt is nil).
func ElapsedProto(t *orcv1.Task) time.Duration {
	if t == nil || t.StartedAt == nil {
		return 0
	}
	startTime := t.StartedAt.AsTime()
	if startTime.IsZero() {
		return 0
	}
	return time.Since(startTime)
}

// IsBacklogProto returns true if the task is in the backlog queue.
func IsBacklogProto(t *orcv1.Task) bool {
	return GetQueueProto(t) == orcv1.TaskQueue_TASK_QUEUE_BACKLOG
}

// MoveToBacklogProto moves the task to the backlog queue.
func MoveToBacklogProto(t *orcv1.Task) {
	if t != nil {
		t.Queue = orcv1.TaskQueue_TASK_QUEUE_BACKLOG
	}
}

// MoveToActiveProto moves the task to the active queue.
func MoveToActiveProto(t *orcv1.Task) {
	if t != nil {
		t.Queue = orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	}
}

// SetInitiativeProto links the task to an initiative.
// Pass an empty string to unlink the task from any initiative.
func SetInitiativeProto(t *orcv1.Task, initiativeID string) {
	if t == nil {
		return
	}
	if initiativeID == "" {
		t.InitiativeId = nil
	} else {
		t.InitiativeId = &initiativeID
	}
}

// GetInitiativeIDProto returns the task's initiative ID, or empty string if not linked.
func GetInitiativeIDProto(t *orcv1.Task) string {
	if t == nil || t.InitiativeId == nil {
		return ""
	}
	return *t.InitiativeId
}

// HasInitiativeProto returns true if the task is linked to an initiative.
func HasInitiativeProto(t *orcv1.Task) bool {
	return t != nil && t.InitiativeId != nil && *t.InitiativeId != ""
}

// HasPRProto returns true if the task has an associated pull request.
func HasPRProto(t *orcv1.Task) bool {
	return t != nil && t.Pr != nil && t.Pr.Url != nil && *t.Pr.Url != ""
}

// GetPRStatusProto returns the PR status, or PR_STATUS_NONE if no PR exists.
func GetPRStatusProto(t *orcv1.Task) orcv1.PRStatus {
	if t == nil || t.Pr == nil {
		return orcv1.PRStatus_PR_STATUS_NONE
	}
	return t.Pr.Status
}

// GetPRURLProto returns the PR URL, or empty string if no PR exists.
func GetPRURLProto(t *orcv1.Task) string {
	if t == nil || t.Pr == nil || t.Pr.Url == nil {
		return ""
	}
	return *t.Pr.Url
}

// SetPRInfoProto sets or updates the PR information for the task.
func SetPRInfoProto(t *orcv1.Task, url string, number int) {
	if t == nil {
		return
	}
	if t.Pr == nil {
		t.Pr = &orcv1.PRInfo{}
	}
	t.Pr.Url = &url
	num := int32(number)
	t.Pr.Number = &num
	// Default to pending review for new PRs
	if t.Pr.Status == orcv1.PRStatus_PR_STATUS_NONE ||
		t.Pr.Status == orcv1.PRStatus_PR_STATUS_UNSPECIFIED {
		t.Pr.Status = orcv1.PRStatus_PR_STATUS_PENDING_REVIEW
	}
}

// SetMergedInfoProto marks the PR as merged with the given target branch.
func SetMergedInfoProto(t *orcv1.Task, prURL, targetBranch string) {
	if t == nil {
		return
	}
	if t.Pr == nil {
		t.Pr = &orcv1.PRInfo{}
	}
	t.Pr.Url = &prURL
	t.Pr.Merged = true
	t.Pr.MergedAt = timestamppb.Now()
	t.Pr.TargetBranch = &targetBranch
	t.Pr.Status = orcv1.PRStatus_PR_STATUS_MERGED
}

// UpdatePRStatusProto updates the PR status fields from fetched data.
func UpdatePRStatusProto(t *orcv1.Task, status orcv1.PRStatus, checksStatus string, mergeable bool, reviewCount, approvalCount int) {
	if t == nil {
		return
	}
	if t.Pr == nil {
		t.Pr = &orcv1.PRInfo{}
	}
	t.Pr.Status = status
	if checksStatus != "" {
		t.Pr.ChecksStatus = &checksStatus
	}
	t.Pr.Mergeable = mergeable
	t.Pr.ReviewCount = int32(reviewCount)
	t.Pr.ApprovalCount = int32(approvalCount)
	t.Pr.LastCheckedAt = timestamppb.Now()
}

// SetTestingRequirementsProto configures testing requirements based on project and task context.
func SetTestingRequirementsProto(t *orcv1.Task, hasFrontend bool) {
	if t == nil {
		return
	}

	title := t.Title
	description := ""
	if t.Description != nil {
		description = *t.Description
	}

	// Auto-detect UI testing from task description
	t.RequiresUiTesting = DetectUITesting(title, description)

	// Initialize testing requirements if not set
	if t.TestingRequirements == nil {
		t.TestingRequirements = &orcv1.TestingRequirements{}
	}

	// Unit tests are always recommended for non-trivial tasks
	if t.Weight != orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL {
		t.TestingRequirements.Unit = true
	}

	// E2E tests for frontend projects with UI tasks
	if hasFrontend && t.RequiresUiTesting {
		t.TestingRequirements.E2E = true
	}

	// Visual tests for tasks explicitly mentioning visual/design concerns
	text := title + " " + description
	if visualKeywordPattern.MatchString(text) {
		t.TestingRequirements.Visual = true
	}
}

// ComputeBlocksProto calculates tasks that are waiting on this task.
// Returns task IDs that have this task in their BlockedBy list.
func ComputeBlocksProto(taskID string, allTasks []*orcv1.Task) []string {
	var blocks []string
	for _, t := range allTasks {
		if slices.Contains(t.BlockedBy, taskID) {
			blocks = append(blocks, t.Id)
		}
	}
	sort.Strings(blocks)
	return blocks
}

// ComputeReferencedByProto finds tasks whose descriptions mention this task ID.
// Excludes self-references, tasks in BlockedBy, and tasks in RelatedTo.
func ComputeReferencedByProto(taskID string, allTasks []*orcv1.Task) []string {
	var referencedBy []string

	for _, t := range allTasks {
		// Skip self
		if t.Id == taskID {
			continue
		}

		// Check if this task mentions taskID
		description := ""
		if t.Description != nil {
			description = *t.Description
		}
		refs := DetectTaskReferences(t.Title + " " + description)
		if !slices.Contains(refs, taskID) {
			continue
		}

		// Exclude if taskID is already in this task's BlockedBy or RelatedTo
		if slices.Contains(t.BlockedBy, taskID) || slices.Contains(t.RelatedTo, taskID) {
			continue
		}

		referencedBy = append(referencedBy, t.Id)
	}
	sort.Strings(referencedBy)
	return referencedBy
}

// PopulateComputedFieldsProto fills in computed fields for all tasks:
// - Blocks: tasks that are waiting on this task
// - ReferencedBy: tasks whose descriptions mention this task
// - IsBlocked: whether this task has unmet dependencies
// - UnmetBlockers: list of task IDs that block this task and are incomplete
// - DependencyStatus: BLOCKED, READY, or NONE for filtering
func PopulateComputedFieldsProto(tasks []*orcv1.Task) {
	// Build task map for dependency checking
	taskMap := make(map[string]*orcv1.Task)
	for _, t := range tasks {
		taskMap[t.Id] = t
	}

	// Build reverse lookup maps in O(N) instead of O(N²)
	blocksMap := make(map[string][]string)    // blockerID -> []taskIDs that it blocks
	referencedByMap := make(map[string][]string) // refID -> []taskIDs that reference it

	for _, t := range tasks {
		// Build blocks map: for each blocker, this task is blocked by it
		for _, blockerID := range t.BlockedBy {
			blocksMap[blockerID] = append(blocksMap[blockerID], t.Id)
		}

		// Build referencedBy map: extract task references from description
		description := ""
		if t.Description != nil {
			description = *t.Description
		}
		refs := DetectTaskReferences(t.Title + " " + description)

		// Create sets for quick lookup
		blockedBySet := make(map[string]bool)
		for _, id := range t.BlockedBy {
			blockedBySet[id] = true
		}
		relatedToSet := make(map[string]bool)
		for _, id := range t.RelatedTo {
			relatedToSet[id] = true
		}

		for _, refID := range refs {
			// Exclude self-references
			if refID == t.Id {
				continue
			}
			// Exclude if refID is already in BlockedBy or RelatedTo
			if blockedBySet[refID] || relatedToSet[refID] {
				continue
			}
			referencedByMap[refID] = append(referencedByMap[refID], t.Id)
		}
	}

	// Sort the maps once
	for k := range blocksMap {
		sort.Strings(blocksMap[k])
	}
	for k := range referencedByMap {
		sort.Strings(referencedByMap[k])
	}

	// Apply computed fields - O(N)
	for _, t := range tasks {
		t.Blocks = blocksMap[t.Id]
		t.ReferencedBy = referencedByMap[t.Id]
		t.UnmetBlockers = GetUnmetDependenciesProto(t, taskMap)
		t.IsBlocked = len(t.UnmetBlockers) > 0
		t.DependencyStatus = ComputeDependencyStatusProto(t)
	}
}

// ComputeDependencyStatusProto returns the dependency status for filtering.
func ComputeDependencyStatusProto(t *orcv1.Task) orcv1.DependencyStatus {
	if t == nil || len(t.BlockedBy) == 0 {
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_NONE
	}
	if len(t.UnmetBlockers) > 0 {
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_BLOCKED
	}
	return orcv1.DependencyStatus_DEPENDENCY_STATUS_READY
}

// HasUnmetDependenciesProto returns true if any task in BlockedBy is not completed.
func HasUnmetDependenciesProto(t *orcv1.Task, tasks map[string]*orcv1.Task) bool {
	if t == nil {
		return false
	}
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists {
			// Missing task is treated as unmet dependency
			return true
		}
		if !IsDoneProto(blocker.Status) {
			return true
		}
	}
	return false
}

// GetUnmetDependenciesProto returns the IDs of tasks that block this one and aren't completed.
func GetUnmetDependenciesProto(t *orcv1.Task, tasks map[string]*orcv1.Task) []string {
	if t == nil {
		return nil
	}
	var unmet []string
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists || !IsDoneProto(blocker.Status) {
			unmet = append(unmet, blockerID)
		}
	}
	return unmet
}

// ProtoBlockerInfo contains information about a blocking task for display purposes.
type ProtoBlockerInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// GetIncompleteBlockersProto returns full information about blocking tasks that aren't completed.
func GetIncompleteBlockersProto(t *orcv1.Task, tasks map[string]*orcv1.Task) []ProtoBlockerInfo {
	if t == nil {
		return nil
	}
	var blockers []ProtoBlockerInfo
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists {
			// Reference to non-existent task - treat as blocker
			blockers = append(blockers, ProtoBlockerInfo{
				ID:     blockerID,
				Title:  "(task not found)",
				Status: orcv1.TaskStatus_TASK_STATUS_UNSPECIFIED.String(),
			})
			continue
		}
		if !IsDoneProto(blocker.Status) {
			blockers = append(blockers, ProtoBlockerInfo{
				ID:     blocker.Id,
				Title:  blocker.Title,
				Status: blocker.Status.String(),
			})
		}
	}
	return blockers
}

// EnsureQualityMetricsProto initializes the Quality field if nil.
func EnsureQualityMetricsProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	if t.Quality == nil {
		t.Quality = &orcv1.QualityMetrics{
			PhaseRetries: make(map[string]int32),
		}
	}
	if t.Quality.PhaseRetries == nil {
		t.Quality.PhaseRetries = make(map[string]int32)
	}
}

// RecordPhaseRetryProto increments the retry count for a specific phase.
func RecordPhaseRetryProto(t *orcv1.Task, phase string) {
	EnsureQualityMetricsProto(t)
	if t == nil {
		return
	}
	t.Quality.PhaseRetries[phase]++
	t.Quality.TotalRetries++
}

// RecordReviewRejectionProto increments the review rejection count.
func RecordReviewRejectionProto(t *orcv1.Task) {
	EnsureQualityMetricsProto(t)
	if t == nil {
		return
	}
	t.Quality.ReviewRejections++
}

// RecordManualInterventionProto marks that manual intervention was required.
func RecordManualInterventionProto(t *orcv1.Task, reason string) {
	EnsureQualityMetricsProto(t)
	if t == nil {
		return
	}
	t.Quality.ManualIntervention = true
	t.Quality.ManualInterventionReason = &reason
}

// GetPhaseRetriesProto returns the retry count for a specific phase, or 0 if not tracked.
func GetPhaseRetriesProto(t *orcv1.Task, phase string) int {
	if t == nil || t.Quality == nil || t.Quality.PhaseRetries == nil {
		return 0
	}
	return int(t.Quality.PhaseRetries[phase])
}

// GetTotalRetriesProto returns the total retry count across all phases.
func GetTotalRetriesProto(t *orcv1.Task) int {
	if t == nil || t.Quality == nil {
		return 0
	}
	return int(t.Quality.TotalRetries)
}

// GetReviewRejectionsProto returns the review rejection count.
func GetReviewRejectionsProto(t *orcv1.Task) int {
	if t == nil || t.Quality == nil {
		return 0
	}
	return int(t.Quality.ReviewRejections)
}

// HadManualInterventionProto returns true if manual intervention was required.
func HadManualInterventionProto(t *orcv1.Task) bool {
	return t != nil && t.Quality != nil && t.Quality.ManualIntervention
}

// GetDescriptionProto returns the task description, or empty string if nil.
func GetDescriptionProto(t *orcv1.Task) string {
	if t == nil || t.Description == nil {
		return ""
	}
	return *t.Description
}

// SetDescriptionProto sets the task description.
func SetDescriptionProto(t *orcv1.Task, description string) {
	if t == nil {
		return
	}
	if description == "" {
		t.Description = nil
	} else {
		t.Description = &description
	}
}

// GetCurrentPhaseProto returns the current phase, or empty string if nil.
func GetCurrentPhaseProto(t *orcv1.Task) string {
	if t == nil || t.CurrentPhase == nil {
		return ""
	}
	return *t.CurrentPhase
}

// SetCurrentPhaseProto sets the current phase.
func SetCurrentPhaseProto(t *orcv1.Task, phase string) {
	if t == nil {
		return
	}
	if phase == "" {
		t.CurrentPhase = nil
	} else {
		t.CurrentPhase = &phase
	}
}

// GetWorkflowIDProto returns the workflow ID, or empty string if nil.
func GetWorkflowIDProto(t *orcv1.Task) string {
	if t == nil || t.WorkflowId == nil {
		return ""
	}
	return *t.WorkflowId
}

// SetWorkflowIDProto sets the workflow ID.
func SetWorkflowIDProto(t *orcv1.Task, workflowID string) {
	if t == nil {
		return
	}
	if workflowID == "" {
		t.WorkflowId = nil
	} else {
		t.WorkflowId = &workflowID
	}
}

// GetTargetBranchProto returns the target branch, or empty string if nil.
func GetTargetBranchProto(t *orcv1.Task) string {
	if t == nil || t.TargetBranch == nil {
		return ""
	}
	return *t.TargetBranch
}

// SetTargetBranchProto sets the target branch.
func SetTargetBranchProto(t *orcv1.Task, branch string) {
	if t == nil {
		return
	}
	if branch == "" {
		t.TargetBranch = nil
	} else {
		t.TargetBranch = &branch
	}
}

// GetBranchNameProto returns the user-specified branch name or empty string.
func GetBranchNameProto(t *orcv1.Task) string {
	if t.BranchName != nil {
		return *t.BranchName
	}
	return ""
}

// SetBranchNameProto sets the user-specified branch name.
func SetBranchNameProto(t *orcv1.Task, name string) {
	t.BranchName = &name
}

// GetPRDraftProto returns the PR draft override or nil if not set.
func GetPRDraftProto(t *orcv1.Task) *bool {
	return t.PrDraft
}

// SetPRDraftProto sets the PR draft override.
func SetPRDraftProto(t *orcv1.Task, draft bool) {
	t.PrDraft = &draft
}

// GetPRLabelsProto returns PR label overrides. Check PrLabelsSet to determine if set.
func GetPRLabelsProto(t *orcv1.Task) []string {
	return t.PrLabels
}

// SetPRLabelsProto sets PR label overrides.
func SetPRLabelsProto(t *orcv1.Task, labels []string) {
	t.PrLabels = labels
	t.PrLabelsSet = true
}

// ClearPRLabelsProto clears PR label overrides (reverts to project default).
func ClearPRLabelsProto(t *orcv1.Task) {
	t.PrLabels = nil
	t.PrLabelsSet = false
}

// GetPRReviewersProto returns PR reviewer overrides. Check PrReviewersSet to determine if set.
func GetPRReviewersProto(t *orcv1.Task) []string {
	return t.PrReviewers
}

// SetPRReviewersProto sets PR reviewer overrides.
func SetPRReviewersProto(t *orcv1.Task, reviewers []string) {
	t.PrReviewers = reviewers
	t.PrReviewersSet = true
}

// ClearPRReviewersProto clears PR reviewer overrides (reverts to project default).
func ClearPRReviewersProto(t *orcv1.Task) {
	t.PrReviewers = nil
	t.PrReviewersSet = false
}

// MarkStartedProto marks the task as started with the current timestamp.
func MarkStartedProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	now := timestamppb.Now()
	t.StartedAt = now
	t.UpdatedAt = now
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
}

// MarkCompletedProto marks the task as completed with the current timestamp.
func MarkCompletedProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	now := timestamppb.Now()
	t.CompletedAt = now
	t.UpdatedAt = now
	t.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
}

// MarkFailedProto marks the task as failed with the current timestamp.
func MarkFailedProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	now := timestamppb.Now()
	t.CompletedAt = now
	t.UpdatedAt = now
	t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
}

// UpdateTimestampProto sets the UpdatedAt field to the current time.
func UpdateTimestampProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	t.UpdatedAt = timestamppb.Now()
}

// EnsureExecutionProto initializes the Execution field if nil.
func EnsureExecutionProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	if t.Execution == nil {
		t.Execution = InitProtoExecutionState()
	}
}

// EnsureMetadataProto initializes the Metadata field if nil.
func EnsureMetadataProto(t *orcv1.Task) {
	if t == nil {
		return
	}
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
}

// CheckOrphanedProto checks if a task is orphaned (executor process died mid-run).
// A task is orphaned if:
// 1. Its status is "running" but no executor PID is tracked
// 2. Its status is "running" with a PID that no longer exists
//
// Note: Heartbeat staleness is only used for additional context when the PID is dead.
// A live PID always indicates a healthy task - this prevents false positives during
// long-running phases where heartbeats may not be updated frequently.
//
// Returns (isOrphaned, reason) where reason explains why.
func CheckOrphanedProto(t *orcv1.Task) (bool, string) {
	if t == nil {
		return false, ""
	}

	// Only running tasks can be orphaned
	if t.Status != orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return false, ""
	}

	// No execution info means potentially orphaned (legacy or incomplete state)
	if t.ExecutorPid == 0 {
		return true, "no execution info (legacy state or incomplete)"
	}

	// Primary check: Is the executor process alive?
	if !IsPIDAlive(int(t.ExecutorPid)) {
		// PID is dead - task is definitely orphaned
		// Use heartbeat to provide additional context in the reason
		if t.LastHeartbeat != nil && time.Since(t.LastHeartbeat.AsTime()) > StaleHeartbeatThreshold {
			return true, "executor process not running (heartbeat stale)"
		}
		return true, "executor process not running"
	}

	// PID is alive - task is NOT orphaned, regardless of heartbeat
	return false, ""
}

// InterruptPhaseOnTaskProto marks a phase as interrupted on a task's execution state.
// This is a convenience wrapper around the ExecutionState-level function.
func InterruptPhaseOnTaskProto(t *orcv1.Task, phaseID string) {
	if t == nil {
		return
	}
	EnsureExecutionProto(t)
	InterruptPhaseProto(t.Execution, phaseID)
}

// GetTotalTokensProto returns the total token count from the task's execution state.
func GetTotalTokensProto(t *orcv1.Task) int {
	if t == nil || t.Execution == nil || t.Execution.Tokens == nil {
		return 0
	}
	return int(t.Execution.Tokens.TotalTokens)
}

// DetectCircularDependencyWithAllProto checks if adding newBlockers to taskID creates a cycle.
// Returns the cycle path if found, nil otherwise.
func DetectCircularDependencyWithAllProto(taskID string, newBlockers []string, tasks map[string]*orcv1.Task) []string {
	// Build adjacency list: task -> tasks it's blocked by
	// Copy slices to avoid mutating original task data
	blockedByMap := make(map[string][]string)
	for _, t := range tasks {
		if t.Id == taskID {
			// Use the new blockers for this task
			blockedByMap[t.Id] = append([]string(nil), newBlockers...)
		} else {
			blockedByMap[t.Id] = append([]string(nil), t.BlockedBy...)
		}
	}

	// If the task doesn't exist in the map yet, add it with new blockers
	if _, exists := blockedByMap[taskID]; !exists {
		blockedByMap[taskID] = append([]string(nil), newBlockers...)
	}

	// DFS to detect cycle starting from taskID
	visited := make(map[string]bool)
	path := make(map[string]bool)
	var cyclePath []string

	var dfs func(id string) bool
	dfs = func(id string) bool {
		if path[id] {
			// Found a cycle, reconstruct path
			cyclePath = append(cyclePath, id)
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		path[id] = true

		for _, blocker := range blockedByMap[id] {
			if dfs(blocker) {
				cyclePath = append(cyclePath, id)
				return true
			}
		}

		path[id] = false
		return false
	}

	// Start DFS from each of the new blockers to see if they can reach taskID
	for _, blocker := range newBlockers {
		if dfs(blocker) {
			// Reverse to get proper order
			for i, j := 0, len(cyclePath)-1; i < j; i, j = i+1, j-1 {
				cyclePath[i], cyclePath[j] = cyclePath[j], cyclePath[i]
			}
			return cyclePath
		}
		// Reset for next blocker
		visited = make(map[string]bool)
		path = make(map[string]bool)
		cyclePath = nil
	}

	return nil
}
