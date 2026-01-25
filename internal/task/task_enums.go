// Package task provides task management for orc.
package task

// Weight represents the complexity classification of a task.
type Weight string

const (
	WeightTrivial Weight = "trivial"
	WeightSmall   Weight = "small"
	WeightMedium  Weight = "medium"
	WeightLarge   Weight = "large"
)

// ValidWeights returns all valid weight values.
func ValidWeights() []Weight {
	return []Weight{WeightTrivial, WeightSmall, WeightMedium, WeightLarge}
}

// IsValidWeight returns true if the weight is a valid weight value.
func IsValidWeight(w Weight) bool {
	switch w {
	case WeightTrivial, WeightSmall, WeightMedium, WeightLarge:
		return true
	default:
		return false
	}
}

// Queue represents whether a task is in the active work queue or backlog.
type Queue string

const (
	// QueueActive indicates tasks in the current work queue (shown on board).
	QueueActive Queue = "active"
	// QueueBacklog indicates tasks for later (hidden by default, shown in backlog section).
	QueueBacklog Queue = "backlog"
)

// ValidQueues returns all valid queue values.
func ValidQueues() []Queue {
	return []Queue{QueueActive, QueueBacklog}
}

// IsValidQueue returns true if the queue is a valid queue value.
func IsValidQueue(q Queue) bool {
	switch q {
	case QueueActive, QueueBacklog:
		return true
	default:
		return false
	}
}

// Priority represents the urgency/importance of a task.
type Priority string

const (
	// PriorityCritical indicates urgent tasks that need immediate attention.
	PriorityCritical Priority = "critical"
	// PriorityHigh indicates important tasks that should be done soon.
	PriorityHigh Priority = "high"
	// PriorityNormal indicates regular tasks (default).
	PriorityNormal Priority = "normal"
	// PriorityLow indicates tasks that can wait.
	PriorityLow Priority = "low"
)

// ValidPriorities returns all valid priority values.
func ValidPriorities() []Priority {
	return []Priority{PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow}
}

// IsValidPriority returns true if the priority is a valid priority value.
func IsValidPriority(p Priority) bool {
	switch p {
	case PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow:
		return true
	default:
		return false
	}
}

// PriorityOrder returns a numeric value for sorting (lower = higher priority).
func PriorityOrder(p Priority) int {
	switch p {
	case PriorityCritical:
		return 0
	case PriorityHigh:
		return 1
	case PriorityNormal:
		return 2
	case PriorityLow:
		return 3
	default:
		return 2 // Default to normal
	}
}

// Category represents the type/category of a task.
type Category string

const (
	// CategoryFeature indicates a new feature or functionality.
	CategoryFeature Category = "feature"
	// CategoryBug indicates a bug fix.
	CategoryBug Category = "bug"
	// CategoryRefactor indicates code refactoring without behavior change.
	CategoryRefactor Category = "refactor"
	// CategoryChore indicates maintenance tasks (deps, cleanup, etc).
	CategoryChore Category = "chore"
	// CategoryDocs indicates documentation changes.
	CategoryDocs Category = "docs"
	// CategoryTest indicates test-related changes.
	CategoryTest Category = "test"
)

// ValidCategories returns all valid category values.
func ValidCategories() []Category {
	return []Category{CategoryFeature, CategoryBug, CategoryRefactor, CategoryChore, CategoryDocs, CategoryTest}
}

// IsValidCategory returns true if the category is a valid category value.
func IsValidCategory(c Category) bool {
	switch c {
	case CategoryFeature, CategoryBug, CategoryRefactor, CategoryChore, CategoryDocs, CategoryTest:
		return true
	default:
		return false
	}
}

// PRStatus represents the review/approval status of a pull request.
type PRStatus string

const (
	// PRStatusNone indicates no PR exists for this task.
	PRStatusNone PRStatus = ""
	// PRStatusDraft indicates the PR is in draft state.
	PRStatusDraft PRStatus = "draft"
	// PRStatusPendingReview indicates the PR is awaiting review.
	PRStatusPendingReview PRStatus = "pending_review"
	// PRStatusChangesRequested indicates reviewers have requested changes.
	PRStatusChangesRequested PRStatus = "changes_requested"
	// PRStatusApproved indicates the PR has been approved.
	PRStatusApproved PRStatus = "approved"
	// PRStatusMerged indicates the PR has been merged.
	PRStatusMerged PRStatus = "merged"
	// PRStatusClosed indicates the PR was closed without merging.
	PRStatusClosed PRStatus = "closed"
)

// ValidPRStatuses returns all valid PR status values.
func ValidPRStatuses() []PRStatus {
	return []PRStatus{
		PRStatusNone, PRStatusDraft, PRStatusPendingReview,
		PRStatusChangesRequested, PRStatusApproved, PRStatusMerged, PRStatusClosed,
	}
}

// IsValidPRStatus returns true if the PR status is a valid value.
func IsValidPRStatus(s PRStatus) bool {
	switch s {
	case PRStatusNone, PRStatusDraft, PRStatusPendingReview,
		PRStatusChangesRequested, PRStatusApproved, PRStatusMerged, PRStatusClosed:
		return true
	default:
		return false
	}
}

// DependencyStatus represents the dependency state of a task for filtering.
type DependencyStatus string

const (
	// DependencyStatusBlocked indicates the task has incomplete blockers.
	DependencyStatusBlocked DependencyStatus = "blocked"
	// DependencyStatusReady indicates all dependencies are satisfied (or no deps).
	DependencyStatusReady DependencyStatus = "ready"
	// DependencyStatusNone indicates the task has no dependencies defined.
	DependencyStatusNone DependencyStatus = "none"
)

// ValidDependencyStatuses returns all valid dependency status values.
func ValidDependencyStatuses() []DependencyStatus {
	return []DependencyStatus{DependencyStatusBlocked, DependencyStatusReady, DependencyStatusNone}
}

// IsValidDependencyStatus returns true if the dependency status is valid.
func IsValidDependencyStatus(ds DependencyStatus) bool {
	switch ds {
	case DependencyStatusBlocked, DependencyStatusReady, DependencyStatusNone:
		return true
	default:
		return false
	}
}
