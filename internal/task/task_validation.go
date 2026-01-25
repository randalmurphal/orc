// Package task provides task management for orc.
package task

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation error.
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (got %q)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []ValidationError

// Error returns a combined error message.
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// HasErrors returns true if there are any validation errors.
func (e ValidationErrors) HasErrors() bool {
	return len(e) > 0
}

// ToError returns an error if there are validation errors, nil otherwise.
func (e ValidationErrors) ToError() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

// Validate checks all field constraints on a task and returns validation errors.
func (t *Task) Validate() ValidationErrors {
	var errs ValidationErrors

	if t.Weight != "" && !IsValidWeight(t.Weight) {
		errs = append(errs, ValidationError{
			Field:   "weight",
			Value:   string(t.Weight),
			Message: "invalid weight",
		})
	}

	if t.Queue != "" && !IsValidQueue(t.Queue) {
		errs = append(errs, ValidationError{
			Field:   "queue",
			Value:   string(t.Queue),
			Message: "invalid queue",
		})
	}

	if t.Priority != "" && !IsValidPriority(t.Priority) {
		errs = append(errs, ValidationError{
			Field:   "priority",
			Value:   string(t.Priority),
			Message: "invalid priority",
		})
	}

	if t.Category != "" && !IsValidCategory(t.Category) {
		errs = append(errs, ValidationError{
			Field:   "category",
			Value:   string(t.Category),
			Message: "invalid category",
		})
	}

	if t.Status != "" && !IsValidStatus(t.Status) {
		errs = append(errs, ValidationError{
			Field:   "status",
			Value:   string(t.Status),
			Message: "invalid status",
		})
	}

	return errs
}

// DependencyError represents an error related to task dependencies.
type DependencyError struct {
	TaskID  string
	Message string
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependency error for %s: %s", e.TaskID, e.Message)
}

// ValidateBlockedBy checks that all blocked_by references are valid.
// Returns errors for non-existent tasks but doesn't modify the task.
func ValidateBlockedBy(taskID string, blockedBy []string, existingIDs map[string]bool) []error {
	var errs []error
	for _, depID := range blockedBy {
		if depID == taskID {
			errs = append(errs, &DependencyError{
				TaskID:  taskID,
				Message: "task cannot block itself",
			})
			continue
		}
		if !existingIDs[depID] {
			errs = append(errs, &DependencyError{
				TaskID:  taskID,
				Message: fmt.Sprintf("blocked_by references non-existent task %s", depID),
			})
		}
	}
	return errs
}

// ValidateRelatedTo checks that all related_to references are valid.
func ValidateRelatedTo(taskID string, relatedTo []string, existingIDs map[string]bool) []error {
	var errs []error
	for _, relID := range relatedTo {
		if relID == taskID {
			errs = append(errs, &DependencyError{
				TaskID:  taskID,
				Message: "task cannot be related to itself",
			})
			continue
		}
		if !existingIDs[relID] {
			errs = append(errs, &DependencyError{
				TaskID:  taskID,
				Message: fmt.Sprintf("related_to references non-existent task %s", relID),
			})
		}
	}
	return errs
}

// DetectCircularDependency checks if adding a dependency would create a cycle.
// Returns the cycle path if a cycle would be created, nil otherwise.
func DetectCircularDependency(taskID string, newBlocker string, tasks map[string]*Task) []string {
	// Build adjacency list: task -> tasks it's blocked by
	// Copy slices to avoid mutating original task data
	blockedByMap := make(map[string][]string)
	for _, t := range tasks {
		blockedByMap[t.ID] = append([]string(nil), t.BlockedBy...)
	}

	// Temporarily add the new dependency
	blockedByMap[taskID] = append(blockedByMap[taskID], newBlocker)

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

		for _, dep := range blockedByMap[id] {
			if dfs(dep) {
				cyclePath = append(cyclePath, id)
				return true
			}
		}

		path[id] = false
		return false
	}

	if dfs(taskID) {
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
func DetectCircularDependencyWithAll(taskID string, newBlockers []string, tasks map[string]*Task) []string {
	// Build adjacency list: task -> tasks it's blocked by
	// Copy slices to avoid mutating original task data
	blockedByMap := make(map[string][]string)
	for _, t := range tasks {
		if t.ID == taskID {
			// Use the new blockers for this task
			blockedByMap[t.ID] = append([]string(nil), newBlockers...)
		} else {
			blockedByMap[t.ID] = append([]string(nil), t.BlockedBy...)
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

		for _, dep := range blockedByMap[id] {
			if dfs(dep) {
				cyclePath = append(cyclePath, id)
				return true
			}
		}

		path[id] = false
		return false
	}

	if dfs(taskID) {
		// Reverse the path to show the cycle in order
		for i, j := 0, len(cyclePath)-1; i < j; i, j = i+1, j-1 {
			cyclePath[i], cyclePath[j] = cyclePath[j], cyclePath[i]
		}
		return cyclePath
	}

	return nil
}
