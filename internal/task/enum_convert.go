// Package task provides proto enum conversion utilities.
package task

import (
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// StatusToProto converts a string status to proto TaskStatus.
func StatusToProto(s string) orcv1.TaskStatus {
	switch s {
	case "created":
		return orcv1.TaskStatus_TASK_STATUS_CREATED
	case "classifying":
		return orcv1.TaskStatus_TASK_STATUS_CLASSIFYING
	case "planned":
		return orcv1.TaskStatus_TASK_STATUS_PLANNED
	case "running":
		return orcv1.TaskStatus_TASK_STATUS_RUNNING
	case "paused":
		return orcv1.TaskStatus_TASK_STATUS_PAUSED
	case "blocked":
		return orcv1.TaskStatus_TASK_STATUS_BLOCKED
	case "finalizing":
		return orcv1.TaskStatus_TASK_STATUS_FINALIZING
	case "completed":
		return orcv1.TaskStatus_TASK_STATUS_COMPLETED
	case "failed":
		return orcv1.TaskStatus_TASK_STATUS_FAILED
	case "resolved":
		return orcv1.TaskStatus_TASK_STATUS_RESOLVED
	default:
		return orcv1.TaskStatus_TASK_STATUS_UNSPECIFIED
	}
}

// StatusFromProto converts a proto TaskStatus to string.
func StatusFromProto(s orcv1.TaskStatus) string {
	switch s {
	case orcv1.TaskStatus_TASK_STATUS_CREATED:
		return "created"
	case orcv1.TaskStatus_TASK_STATUS_CLASSIFYING:
		return "classifying"
	case orcv1.TaskStatus_TASK_STATUS_PLANNED:
		return "planned"
	case orcv1.TaskStatus_TASK_STATUS_RUNNING:
		return "running"
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return "paused"
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return "blocked"
	case orcv1.TaskStatus_TASK_STATUS_FINALIZING:
		return "finalizing"
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		return "completed"
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return "failed"
	case orcv1.TaskStatus_TASK_STATUS_RESOLVED:
		return "resolved"
	default:
		return "created"
	}
}

// WeightToProto converts a string weight to proto TaskWeight.
func WeightToProto(w string) orcv1.TaskWeight {
	switch w {
	case "trivial":
		return orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL
	case "small":
		return orcv1.TaskWeight_TASK_WEIGHT_SMALL
	case "medium":
		return orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	case "large":
		return orcv1.TaskWeight_TASK_WEIGHT_LARGE
	default:
		return orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED
	}
}

// WeightFromProto converts a proto TaskWeight to string.
func WeightFromProto(w orcv1.TaskWeight) string {
	switch w {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		return "trivial"
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		return "small"
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		return "medium"
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return "large"
	default:
		return "medium" // Default to medium
	}
}

// QueueToProto converts a string queue to proto TaskQueue.
func QueueToProto(q string) orcv1.TaskQueue {
	switch q {
	case "active":
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	case "backlog":
		return orcv1.TaskQueue_TASK_QUEUE_BACKLOG
	default:
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	}
}

// QueueFromProto converts a proto TaskQueue to string.
func QueueFromProto(q orcv1.TaskQueue) string {
	switch q {
	case orcv1.TaskQueue_TASK_QUEUE_ACTIVE:
		return "active"
	case orcv1.TaskQueue_TASK_QUEUE_BACKLOG:
		return "backlog"
	default:
		return "active"
	}
}

// PriorityToProto converts a string priority to proto TaskPriority.
func PriorityToProto(p string) orcv1.TaskPriority {
	switch p {
	case "critical":
		return orcv1.TaskPriority_TASK_PRIORITY_CRITICAL
	case "high":
		return orcv1.TaskPriority_TASK_PRIORITY_HIGH
	case "normal":
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	case "low":
		return orcv1.TaskPriority_TASK_PRIORITY_LOW
	default:
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	}
}

// PriorityFromProto converts a proto TaskPriority to string.
func PriorityFromProto(p orcv1.TaskPriority) string {
	switch p {
	case orcv1.TaskPriority_TASK_PRIORITY_CRITICAL:
		return "critical"
	case orcv1.TaskPriority_TASK_PRIORITY_HIGH:
		return "high"
	case orcv1.TaskPriority_TASK_PRIORITY_NORMAL:
		return "normal"
	case orcv1.TaskPriority_TASK_PRIORITY_LOW:
		return "low"
	default:
		return "normal"
	}
}

// PriorityOrderFromProto returns a numeric value for sorting (lower = higher priority).
func PriorityOrderFromProto(p orcv1.TaskPriority) int {
	switch p {
	case orcv1.TaskPriority_TASK_PRIORITY_CRITICAL:
		return 0
	case orcv1.TaskPriority_TASK_PRIORITY_HIGH:
		return 1
	case orcv1.TaskPriority_TASK_PRIORITY_NORMAL:
		return 2
	case orcv1.TaskPriority_TASK_PRIORITY_LOW:
		return 3
	default:
		return 2
	}
}

// CategoryToProto converts a string category to proto TaskCategory.
func CategoryToProto(c string) orcv1.TaskCategory {
	switch c {
	case "feature":
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	case "bug":
		return orcv1.TaskCategory_TASK_CATEGORY_BUG
	case "refactor":
		return orcv1.TaskCategory_TASK_CATEGORY_REFACTOR
	case "chore":
		return orcv1.TaskCategory_TASK_CATEGORY_CHORE
	case "docs":
		return orcv1.TaskCategory_TASK_CATEGORY_DOCS
	case "test":
		return orcv1.TaskCategory_TASK_CATEGORY_TEST
	default:
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	}
}

// CategoryFromProto converts a proto TaskCategory to string.
func CategoryFromProto(c orcv1.TaskCategory) string {
	switch c {
	case orcv1.TaskCategory_TASK_CATEGORY_FEATURE:
		return "feature"
	case orcv1.TaskCategory_TASK_CATEGORY_BUG:
		return "bug"
	case orcv1.TaskCategory_TASK_CATEGORY_REFACTOR:
		return "refactor"
	case orcv1.TaskCategory_TASK_CATEGORY_CHORE:
		return "chore"
	case orcv1.TaskCategory_TASK_CATEGORY_DOCS:
		return "docs"
	case orcv1.TaskCategory_TASK_CATEGORY_TEST:
		return "test"
	default:
		return "feature"
	}
}

// PhaseStatusToProto converts a string phase status to proto PhaseStatus.
// Phase status tracks completion only (pending, completed, skipped).
// Execution state (running, paused, etc.) is tracked at task level via TaskStatus.
func PhaseStatusToProto(s string) orcv1.PhaseStatus {
	switch s {
	case "pending":
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	case "completed":
		return orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	case "skipped":
		return orcv1.PhaseStatus_PHASE_STATUS_SKIPPED
	// Legacy values from before migration 038 - all map to pending (not completed)
	case "running", "failed", "paused", "interrupted", "blocked":
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	default:
		return orcv1.PhaseStatus_PHASE_STATUS_PENDING
	}
}

// PhaseStatusFromProto converts a proto PhaseStatus to string.
// Only pending, completed, skipped are valid phase statuses.
func PhaseStatusFromProto(s orcv1.PhaseStatus) string {
	switch s {
	case orcv1.PhaseStatus_PHASE_STATUS_PENDING:
		return "pending"
	case orcv1.PhaseStatus_PHASE_STATUS_COMPLETED:
		return "completed"
	case orcv1.PhaseStatus_PHASE_STATUS_SKIPPED:
		return "skipped"
	default:
		return "pending"
	}
}

// PRStatusToProto converts a string PR status to proto PRStatus.
func PRStatusToProto(s string) orcv1.PRStatus {
	switch s {
	case "", "none":
		return orcv1.PRStatus_PR_STATUS_NONE
	case "draft":
		return orcv1.PRStatus_PR_STATUS_DRAFT
	case "pending_review":
		return orcv1.PRStatus_PR_STATUS_PENDING_REVIEW
	case "changes_requested":
		return orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED
	case "approved":
		return orcv1.PRStatus_PR_STATUS_APPROVED
	case "merged":
		return orcv1.PRStatus_PR_STATUS_MERGED
	case "closed":
		return orcv1.PRStatus_PR_STATUS_CLOSED
	default:
		return orcv1.PRStatus_PR_STATUS_NONE
	}
}

// PRStatusFromProto converts a proto PRStatus to string.
func PRStatusFromProto(s orcv1.PRStatus) string {
	switch s {
	case orcv1.PRStatus_PR_STATUS_NONE:
		return ""
	case orcv1.PRStatus_PR_STATUS_DRAFT:
		return "draft"
	case orcv1.PRStatus_PR_STATUS_PENDING_REVIEW:
		return "pending_review"
	case orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED:
		return "changes_requested"
	case orcv1.PRStatus_PR_STATUS_APPROVED:
		return "approved"
	case orcv1.PRStatus_PR_STATUS_MERGED:
		return "merged"
	case orcv1.PRStatus_PR_STATUS_CLOSED:
		return "closed"
	default:
		return ""
	}
}

// DependencyStatusToProto converts a string dependency status to proto DependencyStatus.
func DependencyStatusToProto(s string) orcv1.DependencyStatus {
	switch s {
	case "blocked":
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_BLOCKED
	case "ready":
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_READY
	case "none":
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_NONE
	default:
		return orcv1.DependencyStatus_DEPENDENCY_STATUS_NONE
	}
}

// DependencyStatusFromProto converts a proto DependencyStatus to string.
func DependencyStatusFromProto(s orcv1.DependencyStatus) string {
	switch s {
	case orcv1.DependencyStatus_DEPENDENCY_STATUS_BLOCKED:
		return "blocked"
	case orcv1.DependencyStatus_DEPENDENCY_STATUS_READY:
		return "ready"
	case orcv1.DependencyStatus_DEPENDENCY_STATUS_NONE:
		return "none"
	default:
		return "none"
	}
}

// ValidWeightsProto returns all valid weight proto values.
func ValidWeightsProto() []orcv1.TaskWeight {
	return []orcv1.TaskWeight{
		orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL,
		orcv1.TaskWeight_TASK_WEIGHT_SMALL,
		orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		orcv1.TaskWeight_TASK_WEIGHT_LARGE,
	}
}

// IsValidWeightProto returns true if the weight is a valid weight value.
func IsValidWeightProto(w orcv1.TaskWeight) bool {
	switch w {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL,
		orcv1.TaskWeight_TASK_WEIGHT_SMALL,
		orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return true
	default:
		return false
	}
}

// ValidStatusesProto returns all valid status proto values.
func ValidStatusesProto() []orcv1.TaskStatus {
	return []orcv1.TaskStatus{
		orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_CLASSIFYING,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_RUNNING,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_BLOCKED,
		orcv1.TaskStatus_TASK_STATUS_FINALIZING,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		orcv1.TaskStatus_TASK_STATUS_FAILED,
		orcv1.TaskStatus_TASK_STATUS_RESOLVED,
	}
}

// IsValidStatusProto returns true if the status is a valid status value.
func IsValidStatusProto(s orcv1.TaskStatus) bool {
	switch s {
	case orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_CLASSIFYING,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_RUNNING,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_BLOCKED,
		orcv1.TaskStatus_TASK_STATUS_FINALIZING,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		orcv1.TaskStatus_TASK_STATUS_FAILED,
		orcv1.TaskStatus_TASK_STATUS_RESOLVED:
		return true
	default:
		return false
	}
}

// ValidQueuesProto returns all valid queue proto values.
func ValidQueuesProto() []orcv1.TaskQueue {
	return []orcv1.TaskQueue{
		orcv1.TaskQueue_TASK_QUEUE_ACTIVE,
		orcv1.TaskQueue_TASK_QUEUE_BACKLOG,
	}
}

// IsValidQueueProto returns true if the queue is a valid queue value.
func IsValidQueueProto(q orcv1.TaskQueue) bool {
	switch q {
	case orcv1.TaskQueue_TASK_QUEUE_ACTIVE, orcv1.TaskQueue_TASK_QUEUE_BACKLOG:
		return true
	default:
		return false
	}
}

// ValidPrioritiesProto returns all valid priority proto values.
func ValidPrioritiesProto() []orcv1.TaskPriority {
	return []orcv1.TaskPriority{
		orcv1.TaskPriority_TASK_PRIORITY_CRITICAL,
		orcv1.TaskPriority_TASK_PRIORITY_HIGH,
		orcv1.TaskPriority_TASK_PRIORITY_NORMAL,
		orcv1.TaskPriority_TASK_PRIORITY_LOW,
	}
}

// IsValidPriorityProto returns true if the priority is a valid priority value.
func IsValidPriorityProto(p orcv1.TaskPriority) bool {
	switch p {
	case orcv1.TaskPriority_TASK_PRIORITY_CRITICAL,
		orcv1.TaskPriority_TASK_PRIORITY_HIGH,
		orcv1.TaskPriority_TASK_PRIORITY_NORMAL,
		orcv1.TaskPriority_TASK_PRIORITY_LOW:
		return true
	default:
		return false
	}
}

// ValidCategoriesProto returns all valid category proto values.
func ValidCategoriesProto() []orcv1.TaskCategory {
	return []orcv1.TaskCategory{
		orcv1.TaskCategory_TASK_CATEGORY_FEATURE,
		orcv1.TaskCategory_TASK_CATEGORY_BUG,
		orcv1.TaskCategory_TASK_CATEGORY_REFACTOR,
		orcv1.TaskCategory_TASK_CATEGORY_CHORE,
		orcv1.TaskCategory_TASK_CATEGORY_DOCS,
		orcv1.TaskCategory_TASK_CATEGORY_TEST,
	}
}

// IsValidCategoryProto returns true if the category is a valid category value.
func IsValidCategoryProto(c orcv1.TaskCategory) bool {
	switch c {
	case orcv1.TaskCategory_TASK_CATEGORY_FEATURE,
		orcv1.TaskCategory_TASK_CATEGORY_BUG,
		orcv1.TaskCategory_TASK_CATEGORY_REFACTOR,
		orcv1.TaskCategory_TASK_CATEGORY_CHORE,
		orcv1.TaskCategory_TASK_CATEGORY_DOCS,
		orcv1.TaskCategory_TASK_CATEGORY_TEST:
		return true
	default:
		return false
	}
}

// ParseStatusProto parses a status string and returns the proto enum with validity.
func ParseStatusProto(s string) (orcv1.TaskStatus, bool) {
	status := StatusToProto(s)
	// StatusToProto returns UNSPECIFIED for unknown strings
	if status == orcv1.TaskStatus_TASK_STATUS_UNSPECIFIED {
		return status, false
	}
	return status, true
}

// ParsePriorityProto parses a priority string and returns the proto enum with validity.
func ParsePriorityProto(s string) (orcv1.TaskPriority, bool) {
	switch s {
	case "critical", "high", "normal", "low":
		return PriorityToProto(s), true
	default:
		return orcv1.TaskPriority_TASK_PRIORITY_UNSPECIFIED, false
	}
}

// ParseWeightProto parses a weight string and returns the proto enum with validity.
func ParseWeightProto(s string) (orcv1.TaskWeight, bool) {
	weight := WeightToProto(s)
	// WeightToProto returns UNSPECIFIED for unknown strings
	if weight == orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED {
		return weight, false
	}
	return weight, true
}

// ParseCategoryProto parses a category string and returns the proto enum with validity.
func ParseCategoryProto(s string) (orcv1.TaskCategory, bool) {
	switch s {
	case "feature", "bug", "refactor", "chore", "docs", "test":
		return CategoryToProto(s), true
	default:
		return orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED, false
	}
}
