// Package initiative provides proto-based initiative helper functions.
// These functions operate on orcv1.Initiative proto types, providing the same
// functionality as the original Initiative methods but as standalone functions.
package initiative

import (
	"fmt"
	"slices"
	"sort"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NewProtoInitiative creates a new proto Initiative with sensible defaults.
func NewProtoInitiative(id, title string) *orcv1.Initiative {
	now := timestamppb.Now()
	return &orcv1.Initiative{
		Version:   1,
		Id:        id,
		Title:     title,
		Status:    orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddTaskProto adds a task reference to the initiative.
func AddTaskProto(i *orcv1.Initiative, id, title string, dependsOn []string, status orcv1.TaskStatus) {
	if i == nil {
		return
	}

	// Check if task already exists
	for idx, t := range i.Tasks {
		if t.Id == id {
			// Update existing task
			i.Tasks[idx].Title = title
			i.Tasks[idx].DependsOn = dependsOn
			i.Tasks[idx].Status = status
			i.UpdatedAt = timestamppb.Now()
			return
		}
	}

	// Add new task
	i.Tasks = append(i.Tasks, &orcv1.TaskRef{
		Id:        id,
		Title:     title,
		DependsOn: dependsOn,
		Status:    status,
	})
	i.UpdatedAt = timestamppb.Now()
}

// UpdateTaskStatusProto updates the status of a task in the initiative.
func UpdateTaskStatusProto(i *orcv1.Initiative, taskID string, status orcv1.TaskStatus) bool {
	if i == nil {
		return false
	}
	for idx, t := range i.Tasks {
		if t.Id == taskID {
			i.Tasks[idx].Status = status
			i.UpdatedAt = timestamppb.Now()
			return true
		}
	}
	return false
}

// RemoveTaskProto removes a task reference from the initiative.
func RemoveTaskProto(i *orcv1.Initiative, taskID string) bool {
	if i == nil {
		return false
	}
	for idx, t := range i.Tasks {
		if t.Id == taskID {
			i.Tasks = append(i.Tasks[:idx], i.Tasks[idx+1:]...)
			i.UpdatedAt = timestamppb.Now()
			return true
		}
	}
	return false
}

// HasTaskProto returns true if the task is in the initiative's task list.
func HasTaskProto(i *orcv1.Initiative, taskID string) bool {
	if i == nil {
		return false
	}
	for _, t := range i.Tasks {
		if t.Id == taskID {
			return true
		}
	}
	return false
}

// AddDecisionProto records a decision in the initiative.
func AddDecisionProto(i *orcv1.Initiative, decision, rationale, by string) {
	if i == nil {
		return
	}
	id := fmt.Sprintf("DEC-%03d", len(i.Decisions)+1)
	dec := &orcv1.InitiativeDecision{
		Id:       id,
		Date:     timestamppb.Now(),
		By:       by,
		Decision: decision,
	}
	if rationale != "" {
		dec.Rationale = &rationale
	}
	i.Decisions = append(i.Decisions, dec)
	i.UpdatedAt = timestamppb.Now()
}

// GetTaskDependenciesProto returns the dependencies for a specific task.
func GetTaskDependenciesProto(i *orcv1.Initiative, taskID string) []string {
	if i == nil {
		return nil
	}
	for _, t := range i.Tasks {
		if t.Id == taskID {
			return t.DependsOn
		}
	}
	return nil
}

// isRunnableStatusProto returns true if the status indicates a task that can be run.
func isRunnableStatusProto(status orcv1.TaskStatus) bool {
	switch status {
	case orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_PLANNED:
		return true
	default:
		return false
	}
}

// GetReadyTasksProto returns tasks that are pending/created/planned and have all
// dependencies completed.
func GetReadyTasksProto(i *orcv1.Initiative) []*orcv1.TaskRef {
	if i == nil {
		return nil
	}

	// Build a map of completed tasks
	completed := make(map[string]bool)
	for _, t := range i.Tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			completed[t.Id] = true
		}
	}

	// Find tasks that are in a runnable state and have all deps satisfied
	var ready []*orcv1.TaskRef
	for _, t := range i.Tasks {
		// Tasks that haven't started yet are candidates
		if !isRunnableStatusProto(t.Status) {
			continue
		}

		allDepsSatisfied := true
		for _, dep := range t.DependsOn {
			if !completed[dep] {
				allDepsSatisfied = false
				break
			}
		}

		if allDepsSatisfied {
			ready = append(ready, t)
		}
	}

	return ready
}

// ActivateProto sets the initiative status to active.
func ActivateProto(i *orcv1.Initiative) {
	if i == nil {
		return
	}
	i.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	i.UpdatedAt = timestamppb.Now()
}

// CompleteProto sets the initiative status to completed.
func CompleteProto(i *orcv1.Initiative) {
	if i == nil {
		return
	}
	i.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED
	i.UpdatedAt = timestamppb.Now()
}

// ArchiveProto sets the initiative status to archived.
func ArchiveProto(i *orcv1.Initiative) {
	if i == nil {
		return
	}
	i.Status = orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED
	i.UpdatedAt = timestamppb.Now()
}

// IsBlockedProto returns true if any blocking initiative is not completed.
func IsBlockedProto(i *orcv1.Initiative, initiatives map[string]*orcv1.Initiative) bool {
	if i == nil {
		return false
	}
	for _, depID := range i.BlockedBy {
		dep, exists := initiatives[depID]
		if !exists {
			// Missing initiative is treated as unmet dependency
			return true
		}
		if dep.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED {
			return true
		}
	}
	return false
}

// GetUnmetDependenciesProto returns the IDs of initiatives that block this one and aren't completed.
func GetUnmetDependenciesProto(i *orcv1.Initiative, initiatives map[string]*orcv1.Initiative) []string {
	if i == nil {
		return nil
	}
	var unmet []string
	for _, depID := range i.BlockedBy {
		dep, exists := initiatives[depID]
		if !exists || dep.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED {
			unmet = append(unmet, depID)
		}
	}
	return unmet
}

// ProtoBlockerInfo contains information about a blocking initiative for display purposes.
type ProtoBlockerInfo struct {
	ID     string                    `json:"id"`
	Title  string                    `json:"title"`
	Status orcv1.InitiativeStatus    `json:"status"`
}

// GetIncompleteBlockersProto returns full information about blocking initiatives that aren't completed.
func GetIncompleteBlockersProto(i *orcv1.Initiative, initiatives map[string]*orcv1.Initiative) []ProtoBlockerInfo {
	if i == nil {
		return nil
	}
	var blockers []ProtoBlockerInfo
	for _, blockerID := range i.BlockedBy {
		blocker, exists := initiatives[blockerID]
		if !exists {
			blockers = append(blockers, ProtoBlockerInfo{
				ID:     blockerID,
				Title:  "(initiative not found)",
				Status: orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED,
			})
			continue
		}
		if blocker.Status != orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED {
			blockers = append(blockers, ProtoBlockerInfo{
				ID:     blocker.Id,
				Title:  blocker.Title,
				Status: blocker.Status,
			})
		}
	}
	return blockers
}

// AddBlockerProto adds a single blocker to the initiative's BlockedBy list.
// Returns an error if the blocker would create a cycle or is invalid.
func AddBlockerProto(i *orcv1.Initiative, blockerID string, allInits map[string]*orcv1.Initiative) error {
	if i == nil {
		return fmt.Errorf("initiative is nil")
	}

	// Check for self-reference
	if blockerID == i.Id {
		return &DependencyError{
			InitiativeID: i.Id,
			Message:      "initiative cannot block itself",
		}
	}

	// Check if blocker exists
	if _, exists := allInits[blockerID]; !exists {
		return &DependencyError{
			InitiativeID: i.Id,
			Message:      fmt.Sprintf("blocked_by references non-existent initiative %s", blockerID),
		}
	}

	// Check for duplicate
	if slices.Contains(i.BlockedBy, blockerID) {
		return nil // Already blocked by this initiative
	}

	// Check for circular dependency
	if cycle := DetectCircularDependencyProto(i.Id, blockerID, allInits); cycle != nil {
		return &DependencyError{
			InitiativeID: i.Id,
			Message:      fmt.Sprintf("would create circular dependency: %s", formatCycle(cycle)),
		}
	}

	i.BlockedBy = append(i.BlockedBy, blockerID)
	sort.Strings(i.BlockedBy)
	i.UpdatedAt = timestamppb.Now()
	return nil
}

// formatCycle formats a cycle path as a string.
func formatCycle(cycle []string) string {
	result := ""
	for i, id := range cycle {
		if i > 0 {
			result += " -> "
		}
		result += id
	}
	return result
}

// RemoveBlockerProto removes a blocker from the initiative's BlockedBy list.
func RemoveBlockerProto(i *orcv1.Initiative, blockerID string) bool {
	if i == nil {
		return false
	}
	for idx, id := range i.BlockedBy {
		if id == blockerID {
			i.BlockedBy = append(i.BlockedBy[:idx], i.BlockedBy[idx+1:]...)
			i.UpdatedAt = timestamppb.Now()
			return true
		}
	}
	return false
}

// AllTasksCompleteProto returns true if all tasks in the initiative have a completed status.
func AllTasksCompleteProto(i *orcv1.Initiative) bool {
	if i == nil {
		return true
	}
	for _, t := range i.Tasks {
		if t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			return false
		}
	}
	return true
}

// HasBranchBaseProto returns true if the initiative has a branch base configured.
func HasBranchBaseProto(i *orcv1.Initiative) bool {
	return i != nil && i.BranchBase != nil && *i.BranchBase != ""
}

// GetBranchBaseProto returns the branch base, or empty string if not set.
func GetBranchBaseProto(i *orcv1.Initiative) string {
	if i == nil || i.BranchBase == nil {
		return ""
	}
	return *i.BranchBase
}

// SetBranchBaseProto sets the branch base.
func SetBranchBaseProto(i *orcv1.Initiative, branch string) {
	if i == nil {
		return
	}
	if branch == "" {
		i.BranchBase = nil
	} else {
		i.BranchBase = &branch
	}
	i.UpdatedAt = timestamppb.Now()
}

// GetBranchPrefixProto returns the branch prefix, or empty string if not set.
func GetBranchPrefixProto(i *orcv1.Initiative) string {
	if i == nil || i.BranchPrefix == nil {
		return ""
	}
	return *i.BranchPrefix
}

// SetBranchPrefixProto sets the branch prefix.
func SetBranchPrefixProto(i *orcv1.Initiative, prefix string) {
	if i == nil {
		return
	}
	if prefix == "" {
		i.BranchPrefix = nil
	} else {
		i.BranchPrefix = &prefix
	}
	i.UpdatedAt = timestamppb.Now()
}

// IsReadyForMergeProto returns true if the initiative is ready for branch merge.
func IsReadyForMergeProto(i *orcv1.Initiative) bool {
	if i == nil {
		return false
	}
	return HasBranchBaseProto(i) &&
		AllTasksCompleteProto(i) &&
		i.MergeStatus != orcv1.MergeStatus_MERGE_STATUS_MERGED
}

// SetBlockedByProto replaces the entire BlockedBy list with validation.
func SetBlockedByProto(i *orcv1.Initiative, blockerIDs []string, allInits map[string]*orcv1.Initiative) error {
	if i == nil {
		return fmt.Errorf("initiative is nil")
	}

	// Build existing IDs map
	existingIDs := make(map[string]bool)
	for id := range allInits {
		existingIDs[id] = true
	}

	// Validate all blockers
	if errs := ValidateBlockedBy(i.Id, blockerIDs, existingIDs); len(errs) > 0 {
		return errs[0]
	}

	// Check for circular dependencies
	if cycle := DetectCircularDependencyWithAllProto(i.Id, blockerIDs, allInits); cycle != nil {
		return &DependencyError{
			InitiativeID: i.Id,
			Message:      fmt.Sprintf("would create circular dependency: %s", formatCycle(cycle)),
		}
	}

	i.BlockedBy = blockerIDs
	if len(i.BlockedBy) > 0 {
		sort.Strings(i.BlockedBy)
	}
	i.UpdatedAt = timestamppb.Now()
	return nil
}

// ComputeBlocksProto calculates the Blocks field for an initiative by scanning all initiatives.
func ComputeBlocksProto(initID string, allInits []*orcv1.Initiative) []string {
	var blocks []string
	for _, init := range allInits {
		if slices.Contains(init.BlockedBy, initID) {
			blocks = append(blocks, init.Id)
		}
	}
	sort.Strings(blocks)
	return blocks
}

// PopulateComputedFieldsProto fills in Blocks for all initiatives.
func PopulateComputedFieldsProto(initiatives []*orcv1.Initiative) {
	for _, init := range initiatives {
		init.Blocks = ComputeBlocksProto(init.Id, initiatives)
	}
}

// DetectCircularDependencyProto checks if adding a dependency would create a cycle.
func DetectCircularDependencyProto(initID string, newBlocker string, initiatives map[string]*orcv1.Initiative) []string {
	// Build adjacency list: initiative -> initiatives it's blocked by
	blockedByMap := make(map[string][]string)
	for _, init := range initiatives {
		blockedByMap[init.Id] = append([]string(nil), init.BlockedBy...)
	}

	// Temporarily add the new dependency
	blockedByMap[initID] = append(blockedByMap[initID], newBlocker)

	// DFS to detect cycle starting from initID
	visited := make(map[string]bool)
	path := make(map[string]bool)
	var cyclePath []string

	var dfs func(id string) bool
	dfs = func(id string) bool {
		if path[id] {
			cyclePath = append(cyclePath, id)
			return true
		}
		if visited[id] {
			return false
		}

		visited[id] = true
		path[id] = true

		for _, dep := range blockedByMap[id] {
			if dfs(dep) {
				cyclePath = append(cyclePath, id)
				return true
			}
		}

		path[id] = false
		return false
	}

	if dfs(initID) {
		// Reverse the path to show the cycle in order
		for i, j := 0, len(cyclePath)-1; i < j; i, j = i+1, j-1 {
			cyclePath[i], cyclePath[j] = cyclePath[j], cyclePath[i]
		}
		return cyclePath
	}

	return nil
}

// DetectCircularDependencyWithAllProto checks if setting all blockers at once creates a cycle.
func DetectCircularDependencyWithAllProto(initID string, newBlockers []string, initiatives map[string]*orcv1.Initiative) []string {
	// Build adjacency list: initiative -> initiatives it's blocked by
	blockedByMap := make(map[string][]string)
	for _, init := range initiatives {
		if init.Id == initID {
			blockedByMap[init.Id] = append([]string(nil), newBlockers...)
		} else {
			blockedByMap[init.Id] = append([]string(nil), init.BlockedBy...)
		}
	}

	// If the initiative doesn't exist in the map yet, add it with new blockers
	if _, exists := blockedByMap[initID]; !exists {
		blockedByMap[initID] = append([]string(nil), newBlockers...)
	}

	// DFS to detect cycle starting from initID
	visited := make(map[string]bool)
	path := make(map[string]bool)
	var cyclePath []string

	var dfs func(id string) bool
	dfs = func(id string) bool {
		if path[id] {
			cyclePath = append(cyclePath, id)
			return true
		}
		if visited[id] {
			return false
		}

		visited[id] = true
		path[id] = true

		for _, dep := range blockedByMap[id] {
			if dfs(dep) {
				cyclePath = append(cyclePath, id)
				return true
			}
		}

		path[id] = false
		return false
	}

	if dfs(initID) {
		// Reverse the path to show the cycle in order
		for i, j := 0, len(cyclePath)-1; i < j; i, j = i+1, j-1 {
			cyclePath[i], cyclePath[j] = cyclePath[j], cyclePath[i]
		}
		return cyclePath
	}

	return nil
}

// GetVisionProto returns the vision, or empty string if not set.
func GetVisionProto(i *orcv1.Initiative) string {
	if i == nil || i.Vision == nil {
		return ""
	}
	return *i.Vision
}

// SetVisionProto sets the vision.
func SetVisionProto(i *orcv1.Initiative, vision string) {
	if i == nil {
		return
	}
	if vision == "" {
		i.Vision = nil
	} else {
		i.Vision = &vision
	}
	i.UpdatedAt = timestamppb.Now()
}

// GetMergeCommitProto returns the merge commit, or empty string if not set.
func GetMergeCommitProto(i *orcv1.Initiative) string {
	if i == nil || i.MergeCommit == nil {
		return ""
	}
	return *i.MergeCommit
}

// SetMergeCommitProto sets the merge commit.
func SetMergeCommitProto(i *orcv1.Initiative, sha string) {
	if i == nil {
		return
	}
	if sha == "" {
		i.MergeCommit = nil
	} else {
		i.MergeCommit = &sha
	}
	i.UpdatedAt = timestamppb.Now()
}

// UpdateTimestampProto sets the UpdatedAt field to the current time.
func UpdateTimestampProto(i *orcv1.Initiative) {
	if i == nil {
		return
	}
	i.UpdatedAt = timestamppb.Now()
}

// InitiativeStatusToProto converts a string status to proto InitiativeStatus.
func InitiativeStatusToProto(s string) orcv1.InitiativeStatus {
	switch s {
	case "draft":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT
	case "active":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE
	case "completed":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED
	case "archived":
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED
	default:
		return orcv1.InitiativeStatus_INITIATIVE_STATUS_UNSPECIFIED
	}
}

// InitiativeStatusFromProto converts a proto InitiativeStatus to string.
func InitiativeStatusFromProto(s orcv1.InitiativeStatus) string {
	switch s {
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_DRAFT:
		return "draft"
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_ACTIVE:
		return "active"
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_COMPLETED:
		return "completed"
	case orcv1.InitiativeStatus_INITIATIVE_STATUS_ARCHIVED:
		return "archived"
	default:
		return "draft"
	}
}

// MergeStatusToProto converts a string merge status to proto MergeStatus.
func MergeStatusToProto(s string) orcv1.MergeStatus {
	switch s {
	case "", "none":
		return orcv1.MergeStatus_MERGE_STATUS_NONE
	case "pending":
		return orcv1.MergeStatus_MERGE_STATUS_PENDING
	case "in_progress":
		return orcv1.MergeStatus_MERGE_STATUS_IN_PROGRESS
	case "merged":
		return orcv1.MergeStatus_MERGE_STATUS_MERGED
	case "failed":
		return orcv1.MergeStatus_MERGE_STATUS_FAILED
	default:
		return orcv1.MergeStatus_MERGE_STATUS_NONE
	}
}

// MergeStatusFromProto converts a proto MergeStatus to string.
func MergeStatusFromProto(s orcv1.MergeStatus) string {
	switch s {
	case orcv1.MergeStatus_MERGE_STATUS_NONE:
		return ""
	case orcv1.MergeStatus_MERGE_STATUS_PENDING:
		return "pending"
	case orcv1.MergeStatus_MERGE_STATUS_IN_PROGRESS:
		return "in_progress"
	case orcv1.MergeStatus_MERGE_STATUS_MERGED:
		return "merged"
	case orcv1.MergeStatus_MERGE_STATUS_FAILED:
		return "failed"
	default:
		return ""
	}
}
