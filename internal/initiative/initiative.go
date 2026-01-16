// Package initiative provides initiative/feature grouping for related tasks.
// Initiatives provide shared context, vision, and decisions across multiple tasks.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package initiative

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// InitiativeIDPattern validates initiative IDs (INIT-XXX format where XXX is alphanumeric with optional dashes).
// Examples: INIT-001, INIT-123, INIT-TEST-001, INIT-abc-def
// This prevents path traversal attacks by rejecting IDs containing special characters like /, \, .., etc.
var InitiativeIDPattern = regexp.MustCompile(`^INIT-[A-Za-z0-9][A-Za-z0-9-]*[A-Za-z0-9]$|^INIT-[A-Za-z0-9]$`)

// ValidateID checks if an initiative ID is valid.
// Valid IDs start with "INIT-" followed by alphanumeric characters (with optional dashes in between).
// This prevents path traversal attacks by rejecting IDs containing special characters.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("initiative ID cannot be empty")
	}
	if !InitiativeIDPattern.MatchString(id) {
		return fmt.Errorf("invalid initiative ID %q: must start with INIT- followed by alphanumeric characters", id)
	}
	// Additional check: ensure no path traversal characters
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		return fmt.Errorf("invalid initiative ID %q: contains path traversal characters", id)
	}
	return nil
}

// Status represents the status of an initiative.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusArchived  Status = "archived"
)

// MergeStatus constants for tracking initiative branch merge state.
const (
	MergeStatusNone       = ""            // No branch merge configured (no BranchBase)
	MergeStatusPending    = "pending"     // All tasks complete, ready to merge
	MergeStatusInProgress = "in_progress" // Merge/PR in progress
	MergeStatusMerged     = "merged"      // Successfully merged
	MergeStatusFailed     = "failed"      // Merge failed
)

// Identity represents the owner of an initiative.
type Identity struct {
	Initials    string `yaml:"initials" json:"initials"`
	DisplayName string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Email       string `yaml:"email,omitempty" json:"email,omitempty"`
}

// Decision represents a recorded decision within an initiative.
type Decision struct {
	ID        string    `yaml:"id" json:"id"`
	Date      time.Time `yaml:"date" json:"date"`
	By        string    `yaml:"by" json:"by"`
	Decision  string    `yaml:"decision" json:"decision"`
	Rationale string    `yaml:"rationale,omitempty" json:"rationale,omitempty"`
}

// TaskRef represents a reference to a task within an initiative.
type TaskRef struct {
	ID        string   `yaml:"id" json:"id"`
	Title     string   `yaml:"title" json:"title"`
	DependsOn []string `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Status    string   `yaml:"status" json:"status"`
}

// Initiative represents a grouping of related tasks with shared context.
type Initiative struct {
	Version      int        `yaml:"version" json:"version"`
	ID           string     `yaml:"id" json:"id"`
	Title        string     `yaml:"title" json:"title"`
	Status       Status     `yaml:"status" json:"status"`
	Owner        Identity   `yaml:"owner,omitempty" json:"owner,omitempty"`
	Vision       string     `yaml:"vision,omitempty" json:"vision,omitempty"`
	Decisions    []Decision `yaml:"decisions,omitempty" json:"decisions,omitempty"`
	ContextFiles []string   `yaml:"context_files,omitempty" json:"context_files,omitempty"`
	Tasks        []TaskRef  `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	// BlockedBy lists initiative IDs that must complete before this initiative can start
	BlockedBy []string `yaml:"blocked_by,omitempty" json:"blocked_by,omitempty"`
	// Blocks lists initiative IDs waiting on this (computed, not persisted)
	Blocks []string `yaml:"-" json:"blocks,omitempty"`

	// BranchBase is the target branch for tasks in this initiative.
	// When set, tasks in this initiative will target this branch instead of the project default.
	// Example: "feature/user-auth" for a feature branch that collects all auth-related tasks.
	BranchBase string `yaml:"branch_base,omitempty" json:"branch_base,omitempty"`

	// BranchPrefix overrides the task branch naming pattern for tasks in this initiative.
	// Example: "feature/auth-" would create branches like "feature/auth-TASK-001".
	// If empty, uses the default "orc/" prefix.
	BranchPrefix string `yaml:"branch_prefix,omitempty" json:"branch_prefix,omitempty"`

	// MergeStatus tracks the status of merging the initiative branch to the target.
	// Only relevant when BranchBase is set. Values: pending, in_progress, merged, failed
	MergeStatus string `yaml:"merge_status,omitempty" json:"merge_status,omitempty"`

	// MergeCommit is the commit SHA after the initiative branch was merged.
	// Only set when MergeStatus is "merged".
	MergeCommit string `yaml:"merge_commit,omitempty" json:"merge_commit,omitempty"`

	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`
}

// Directory constants
const (
	// InitiativesDir is the subdirectory for initiatives
	InitiativesDir = "initiatives"
	// SharedDir is the shared directory for P2P mode
	SharedDir = "shared"
)

// New creates a new initiative with the given ID and title.
func New(id, title string) *Initiative {
	now := time.Now()
	return &Initiative{
		Version:   1,
		ID:        id,
		Title:     title,
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddTask adds a task reference to the initiative.
func (i *Initiative) AddTask(id, title string, dependsOn []string) {
	// Check if task already exists
	for idx, t := range i.Tasks {
		if t.ID == id {
			// Update existing task
			i.Tasks[idx].Title = title
			i.Tasks[idx].DependsOn = dependsOn
			i.UpdatedAt = time.Now()
			return
		}
	}

	// Add new task
	i.Tasks = append(i.Tasks, TaskRef{
		ID:        id,
		Title:     title,
		DependsOn: dependsOn,
		Status:    "pending",
	})
	i.UpdatedAt = time.Now()
}

// UpdateTaskStatus updates the status of a task in the initiative.
func (i *Initiative) UpdateTaskStatus(taskID, status string) bool {
	for idx, t := range i.Tasks {
		if t.ID == taskID {
			i.Tasks[idx].Status = status
			i.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// RemoveTask removes a task reference from the initiative.
// Returns true if the task was found and removed.
func (i *Initiative) RemoveTask(taskID string) bool {
	for idx, t := range i.Tasks {
		if t.ID == taskID {
			i.Tasks = append(i.Tasks[:idx], i.Tasks[idx+1:]...)
			i.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// HasTask returns true if the task is in the initiative's task list.
func (i *Initiative) HasTask(taskID string) bool {
	for _, t := range i.Tasks {
		if t.ID == taskID {
			return true
		}
	}
	return false
}

// AddDecision records a decision in the initiative.
func (i *Initiative) AddDecision(decision, rationale, by string) {
	id := fmt.Sprintf("DEC-%03d", len(i.Decisions)+1)
	i.Decisions = append(i.Decisions, Decision{
		ID:        id,
		Date:      time.Now(),
		By:        by,
		Decision:  decision,
		Rationale: rationale,
	})
	i.UpdatedAt = time.Now()
}

// GetTaskDependencies returns the dependencies for a specific task.
func (i *Initiative) GetTaskDependencies(taskID string) []string {
	for _, t := range i.Tasks {
		if t.ID == taskID {
			return t.DependsOn
		}
	}
	return nil
}

// GetReadyTasks returns tasks that are pending and have all dependencies completed.
// Deprecated: Use GetReadyTasksWithLoader for accurate status from task.yaml files.
func (i *Initiative) GetReadyTasks() []TaskRef {
	return i.GetReadyTasksWithLoader(nil)
}

// GetReadyTasksWithLoader returns tasks that are pending/created/planned and have all
// dependencies completed. If loader is provided, uses actual task status from task.yaml.
// A task is considered "ready" if it's in a runnable state (created, planned, or pending)
// and all its dependencies are completed/finished.
func (i *Initiative) GetReadyTasksWithLoader(loader TaskLoader) []TaskRef {
	// Get tasks with actual status if loader provided
	tasks := i.Tasks
	if loader != nil {
		tasks = i.GetTasksWithStatus(loader)
	}

	// Build a map of completed tasks
	completed := make(map[string]bool)
	for _, t := range tasks {
		if t.Status == "completed" {
			completed[t.ID] = true
		}
	}

	// Find tasks that are in a runnable state and have all deps satisfied
	var ready []TaskRef
	for _, t := range tasks {
		// Tasks that haven't started yet are candidates
		if !isRunnableStatus(t.Status) {
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

// isRunnableStatus returns true if the status indicates a task that can be run.
func isRunnableStatus(status string) bool {
	switch status {
	case "pending", "created", "planned":
		return true
	default:
		return false
	}
}

// Activate sets the initiative status to active.
func (i *Initiative) Activate() {
	i.Status = StatusActive
	i.UpdatedAt = time.Now()
}

// Complete sets the initiative status to completed.
func (i *Initiative) Complete() {
	i.Status = StatusCompleted
	i.UpdatedAt = time.Now()
}

// Archive sets the initiative status to archived.
func (i *Initiative) Archive() {
	i.Status = StatusArchived
	i.UpdatedAt = time.Now()
}

// DependencyError represents an error related to initiative dependencies.
type DependencyError struct {
	InitiativeID string
	Message      string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency error for %s: %s", e.InitiativeID, e.Message)
}

// ValidateBlockedBy checks that all blocked_by references are valid.
// Returns errors for self-references and non-existent initiatives.
func ValidateBlockedBy(initID string, blockedBy []string, existingIDs map[string]bool) []error {
	var errs []error
	for _, depID := range blockedBy {
		if depID == initID {
			errs = append(errs, &DependencyError{
				InitiativeID: initID,
				Message:      "initiative cannot block itself",
			})
			continue
		}
		if !existingIDs[depID] {
			errs = append(errs, &DependencyError{
				InitiativeID: initID,
				Message:      fmt.Sprintf("blocked_by references non-existent initiative %s", depID),
			})
		}
	}
	return errs
}

// DetectCircularDependency checks if adding a dependency would create a cycle.
// Returns the cycle path if a cycle would be created, nil otherwise.
func DetectCircularDependency(initID string, newBlocker string, initiatives map[string]*Initiative) []string {
	// Build adjacency list: initiative -> initiatives it's blocked by
	blockedByMap := make(map[string][]string)
	for _, init := range initiatives {
		blockedByMap[init.ID] = append([]string(nil), init.BlockedBy...)
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

// DetectCircularDependencyWithAll checks if setting all blockers at once creates a cycle.
// This is used when replacing the entire BlockedBy list.
// Returns the cycle path if a cycle would be created, nil otherwise.
func DetectCircularDependencyWithAll(initID string, newBlockers []string, initiatives map[string]*Initiative) []string {
	// Build adjacency list: initiative -> initiatives it's blocked by
	blockedByMap := make(map[string][]string)
	for _, init := range initiatives {
		if init.ID == initID {
			blockedByMap[init.ID] = append([]string(nil), newBlockers...)
		} else {
			blockedByMap[init.ID] = append([]string(nil), init.BlockedBy...)
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

// ComputeBlocks calculates the Blocks field for an initiative by scanning all initiatives.
// Returns initiative IDs that have this initiative in their BlockedBy list.
func ComputeBlocks(initID string, allInits []*Initiative) []string {
	var blocks []string
	for _, init := range allInits {
		for _, blocker := range init.BlockedBy {
			if blocker == initID {
				blocks = append(blocks, init.ID)
				break
			}
		}
	}
	sort.Strings(blocks)
	return blocks
}

// PopulateComputedFields fills in Blocks for all initiatives.
// This should be called after loading all initiatives.
func PopulateComputedFields(initiatives []*Initiative) {
	for _, init := range initiatives {
		init.Blocks = ComputeBlocks(init.ID, initiatives)
	}
}

// IsBlocked returns true if any blocking initiative is not completed.
func (i *Initiative) IsBlocked(initiatives map[string]*Initiative) bool {
	for _, depID := range i.BlockedBy {
		dep, exists := initiatives[depID]
		if !exists {
			// Missing initiative is treated as unmet dependency
			return true
		}
		if dep.Status != StatusCompleted {
			return true
		}
	}
	return false
}

// GetUnmetDependencies returns the IDs of initiatives that block this one and aren't completed.
func (i *Initiative) GetUnmetDependencies(initiatives map[string]*Initiative) []string {
	var unmet []string
	for _, depID := range i.BlockedBy {
		dep, exists := initiatives[depID]
		if !exists || dep.Status != StatusCompleted {
			unmet = append(unmet, depID)
		}
	}
	return unmet
}

// BlockerInfo contains information about a blocking initiative for display purposes.
type BlockerInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
}

// GetIncompleteBlockers returns full information about blocking initiatives that aren't completed.
func (i *Initiative) GetIncompleteBlockers(initiatives map[string]*Initiative) []BlockerInfo {
	var blockers []BlockerInfo
	for _, blockerID := range i.BlockedBy {
		blocker, exists := initiatives[blockerID]
		if !exists {
			blockers = append(blockers, BlockerInfo{
				ID:     blockerID,
				Title:  "(initiative not found)",
				Status: "",
			})
			continue
		}
		if blocker.Status != StatusCompleted {
			blockers = append(blockers, BlockerInfo{
				ID:     blocker.ID,
				Title:  blocker.Title,
				Status: blocker.Status,
			})
		}
	}
	return blockers
}

// AddBlocker adds a single blocker to the initiative's BlockedBy list.
// Returns an error if the blocker would create a cycle or is invalid.
func (i *Initiative) AddBlocker(blockerID string, allInits map[string]*Initiative) error {
	// Check for self-reference
	if blockerID == i.ID {
		return &DependencyError{
			InitiativeID: i.ID,
			Message:      "initiative cannot block itself",
		}
	}

	// Check if blocker exists
	if _, exists := allInits[blockerID]; !exists {
		return &DependencyError{
			InitiativeID: i.ID,
			Message:      fmt.Sprintf("blocked_by references non-existent initiative %s", blockerID),
		}
	}

	// Check for duplicate
	for _, existing := range i.BlockedBy {
		if existing == blockerID {
			return nil // Already blocked by this initiative
		}
	}

	// Check for circular dependency
	if cycle := DetectCircularDependency(i.ID, blockerID, allInits); cycle != nil {
		return &DependencyError{
			InitiativeID: i.ID,
			Message:      fmt.Sprintf("would create circular dependency: %s", strings.Join(cycle, " -> ")),
		}
	}

	i.BlockedBy = append(i.BlockedBy, blockerID)
	sort.Strings(i.BlockedBy)
	i.UpdatedAt = time.Now()
	return nil
}

// TaskLoader is a function type that loads task status given a task ID.
// Returns the status as a string and any error. If the task is not found,
// returns empty string and nil (not an error - task may have been deleted).
type TaskLoader func(taskID string) (status string, title string, err error)

// EnrichTaskStatuses updates the Status and Title fields of all tasks in the initiative
// by fetching actual status from task.yaml files via the provided loader function.
// Tasks that cannot be loaded retain their existing status (fallback to stored value).
func (i *Initiative) EnrichTaskStatuses(loader TaskLoader) {
	for idx, t := range i.Tasks {
		status, title, err := loader(t.ID)
		if err != nil {
			// Keep existing status if task cannot be loaded
			continue
		}
		if status != "" {
			i.Tasks[idx].Status = status
		}
		if title != "" {
			i.Tasks[idx].Title = title
		}
	}
}

// GetTasksWithStatus returns a copy of the tasks with status enriched from the loader.
// This does not modify the original initiative.
func (i *Initiative) GetTasksWithStatus(loader TaskLoader) []TaskRef {
	result := make([]TaskRef, len(i.Tasks))
	copy(result, i.Tasks)

	for idx, t := range result {
		status, title, err := loader(t.ID)
		if err != nil {
			continue
		}
		if status != "" {
			result[idx].Status = status
		}
		if title != "" {
			result[idx].Title = title
		}
	}

	return result
}

// RemoveBlocker removes a blocker from the initiative's BlockedBy list.
// Returns true if the blocker was found and removed.
func (i *Initiative) RemoveBlocker(blockerID string) bool {
	for idx, id := range i.BlockedBy {
		if id == blockerID {
			i.BlockedBy = append(i.BlockedBy[:idx], i.BlockedBy[idx+1:]...)
			i.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// AllTasksComplete returns true if all tasks in the initiative have a done status.
// A task is considered done if its status is "completed" or "finished".
// Returns true if there are no tasks (empty initiative is trivially complete).
func (i *Initiative) AllTasksComplete() bool {
	for _, t := range i.Tasks {
		if !isDoneStatus(t.Status) {
			return false
		}
	}
	return true
}

// AllTasksCompleteWithLoader checks if all tasks are done using actual status from loader.
// This is more accurate than AllTasksComplete() as it fetches live status from task.yaml.
func (i *Initiative) AllTasksCompleteWithLoader(loader TaskLoader) bool {
	if len(i.Tasks) == 0 {
		return true
	}

	for _, t := range i.Tasks {
		status := t.Status
		if loader != nil {
			if actualStatus, _, err := loader(t.ID); err == nil && actualStatus != "" {
				status = actualStatus
			}
		}
		if !isDoneStatus(status) {
			return false
		}
	}
	return true
}

// isDoneStatus returns true if the status indicates a task is done.
func isDoneStatus(status string) bool {
	return status == "completed"
}

// HasBranchBase returns true if the initiative has a branch base configured.
func (i *Initiative) HasBranchBase() bool {
	return i.BranchBase != ""
}

// IsReadyForMerge returns true if the initiative is ready for branch merge.
// This means all tasks are complete and the initiative has a branch base configured.
func (i *Initiative) IsReadyForMerge() bool {
	return i.HasBranchBase() && i.AllTasksComplete() && i.MergeStatus != MergeStatusMerged
}

// SetBlockedBy replaces the entire BlockedBy list with validation.
// Returns an error if any blocker is invalid or would create a cycle.
func (i *Initiative) SetBlockedBy(blockerIDs []string, allInits map[string]*Initiative) error {
	// Build existing IDs map
	existingIDs := make(map[string]bool)
	for id := range allInits {
		existingIDs[id] = true
	}

	// Validate all blockers
	if errs := ValidateBlockedBy(i.ID, blockerIDs, existingIDs); len(errs) > 0 {
		return errs[0]
	}

	// Check for circular dependencies
	if cycle := DetectCircularDependencyWithAll(i.ID, blockerIDs, allInits); cycle != nil {
		return &DependencyError{
			InitiativeID: i.ID,
			Message:      fmt.Sprintf("would create circular dependency: %s", strings.Join(cycle, " -> ")),
		}
	}

	i.BlockedBy = blockerIDs
	if len(i.BlockedBy) > 0 {
		sort.Strings(i.BlockedBy)
	}
	i.UpdatedAt = time.Now()
	return nil
}
