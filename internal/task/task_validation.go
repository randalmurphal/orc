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

// Note: The Validate method on *Task was removed as part of the proto migration.
// For orcv1.Task validation, use the individual IsValid* functions or proto validation.

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

// Note: DetectCircularDependency and DetectCircularDependencyWithAll were removed.
// Use DetectCircularDependencyWithAllProto in proto_helpers.go for orcv1.Task instead.
