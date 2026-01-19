// Package task provides task management for orc.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package task

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Path utility functions (no I/O, just path computation)

// TaskDir returns the task directory path for the current working directory.
func TaskDir(id string) string {
	return TaskDirIn("", id)
}

// TaskDirIn returns the task directory path for a specific project directory.
func TaskDirIn(projectDir, id string) string {
	return filepath.Join(projectDir, OrcDir, TasksDir, id)
}

// SpecPath returns the spec file path for the current working directory.
func SpecPath(id string) string {
	return SpecPathIn("", id)
}

// SpecPathIn returns the spec file path for a specific project directory.
func SpecPathIn(projectDir, id string) string {
	return filepath.Join(TaskDirIn(projectDir, id), "spec.md")
}

const (
	// OrcDir is the default orc configuration directory
	OrcDir = ".orc"
	// TasksDir is the subdirectory for tasks
	TasksDir = "tasks"
	// ExportsDir is the subdirectory for exports
	ExportsDir = "exports"
)

// ExportPath returns the default export directory path.
func ExportPath(projectDir string) string {
	return filepath.Join(projectDir, OrcDir, ExportsDir)
}

// Weight represents the complexity classification of a task.
type Weight string

const (
	WeightTrivial    Weight = "trivial"
	WeightSmall      Weight = "small"
	WeightMedium     Weight = "medium"
	WeightLarge      Weight = "large"
	WeightGreenfield Weight = "greenfield"
)

// ValidWeights returns all valid weight values.
func ValidWeights() []Weight {
	return []Weight{WeightTrivial, WeightSmall, WeightMedium, WeightLarge, WeightGreenfield}
}

// IsValidWeight returns true if the weight is a valid weight value.
func IsValidWeight(w Weight) bool {
	switch w {
	case WeightTrivial, WeightSmall, WeightMedium, WeightLarge, WeightGreenfield:
		return true
	default:
		return false
	}
}

// Status represents the current state of a task.
type Status string

const (
	StatusCreated     Status = "created"
	StatusClassifying Status = "classifying"
	StatusPlanned     Status = "planned"
	StatusRunning     Status = "running"
	StatusPaused      Status = "paused"
	StatusBlocked     Status = "blocked"
	StatusFinalizing  Status = "finalizing" // Post-completion: cleanup, PR creation, branch sync
	StatusCompleted   Status = "completed"  // Terminal: all phases AND sync/PR/merge succeeded
	StatusFailed      Status = "failed"
	StatusResolved    Status = "resolved" // Terminal: failed task marked as resolved without re-running
)

// ValidStatuses returns all valid status values.
func ValidStatuses() []Status {
	return []Status{
		StatusCreated, StatusClassifying, StatusPlanned, StatusRunning,
		StatusPaused, StatusBlocked, StatusFinalizing, StatusCompleted,
		StatusFailed, StatusResolved,
	}
}

// IsValidStatus returns true if the status is a valid status value.
func IsValidStatus(s Status) bool {
	switch s {
	case StatusCreated, StatusClassifying, StatusPlanned, StatusRunning,
		StatusPaused, StatusBlocked, StatusFinalizing, StatusCompleted,
		StatusFailed, StatusResolved:
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

// PRInfo contains pull request information for a task.
type PRInfo struct {
	// URL is the full URL to the pull request.
	URL string `yaml:"url,omitempty" json:"url,omitempty"`
	// Number is the PR number (e.g., 123 for PR #123).
	Number int `yaml:"number,omitempty" json:"number,omitempty"`
	// Status is the review/approval status.
	Status PRStatus `yaml:"status,omitempty" json:"status,omitempty"`
	// ChecksStatus summarizes CI check results (pending, success, failure).
	ChecksStatus string `yaml:"checks_status,omitempty" json:"checks_status,omitempty"`
	// Mergeable indicates if the PR can be merged (no conflicts).
	Mergeable bool `yaml:"mergeable,omitempty" json:"mergeable,omitempty"`
	// ReviewCount is the number of reviews received.
	ReviewCount int `yaml:"review_count,omitempty" json:"review_count,omitempty"`
	// ApprovalCount is the number of approvals received.
	ApprovalCount int `yaml:"approval_count,omitempty" json:"approval_count,omitempty"`
	// LastCheckedAt is when the PR status was last polled.
	LastCheckedAt *time.Time `yaml:"last_checked_at,omitempty" json:"last_checked_at,omitempty"`

	// Merged indicates if the PR has been merged.
	Merged bool `yaml:"merged,omitempty" json:"merged,omitempty"`
	// MergedAt is when the PR was merged.
	MergedAt *time.Time `yaml:"merged_at,omitempty" json:"merged_at,omitempty"`
	// MergeCommitSHA is the SHA of the merge commit.
	MergeCommitSHA string `yaml:"merge_commit_sha,omitempty" json:"merge_commit_sha,omitempty"`
	// TargetBranch is the branch the PR was merged into.
	TargetBranch string `yaml:"target_branch,omitempty" json:"target_branch,omitempty"`
}

// TestingRequirements specifies what types of testing are needed for a task.
type TestingRequirements struct {
	// Unit indicates if unit tests are required
	Unit bool `yaml:"unit,omitempty" json:"unit,omitempty"`
	// E2E indicates if end-to-end/integration tests are required
	E2E bool `yaml:"e2e,omitempty" json:"e2e,omitempty"`
	// Visual indicates if visual regression tests are required
	Visual bool `yaml:"visual,omitempty" json:"visual,omitempty"`
}

// QualityMetrics tracks execution quality signals for analysis.
// These metrics help identify patterns in task failures and quality issues.
type QualityMetrics struct {
	// PhaseRetries counts how many times each phase was retried due to failure.
	// Key is phase name (e.g., "implement", "review"), value is retry count.
	PhaseRetries map[string]int `yaml:"phase_retries,omitempty" json:"phase_retries,omitempty"`

	// ReviewRejections counts how many times the review phase rejected implementation.
	// This indicates quality issues caught during review.
	ReviewRejections int `yaml:"review_rejections,omitempty" json:"review_rejections,omitempty"`

	// ManualIntervention indicates if a human had to manually fix something
	// that the automated execution couldn't handle.
	ManualIntervention bool `yaml:"manual_intervention,omitempty" json:"manual_intervention,omitempty"`

	// ManualInterventionReason describes what required manual intervention.
	ManualInterventionReason string `yaml:"manual_intervention_reason,omitempty" json:"manual_intervention_reason,omitempty"`

	// TotalRetries is the sum of all phase retries for quick filtering.
	TotalRetries int `yaml:"total_retries,omitempty" json:"total_retries,omitempty"`
}

// Task represents a unit of work to be orchestrated.
type Task struct {
	// ID is the unique identifier (e.g., TASK-001)
	ID string `yaml:"id" json:"id"`

	// Title is a short description of the task
	Title string `yaml:"title" json:"title"`

	// Description is the full task description
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Weight is the complexity classification
	Weight Weight `yaml:"weight" json:"weight"`

	// Status is the current execution state
	Status Status `yaml:"status" json:"status"`

	// CurrentPhase is the phase currently being executed
	CurrentPhase string `yaml:"current_phase,omitempty" json:"current_phase,omitempty"`

	// Branch is the git branch for this task (e.g., orc/TASK-001)
	Branch string `yaml:"branch" json:"branch"`

	// Queue indicates whether the task is in the active work queue or backlog.
	// Active tasks are shown on the board, backlog tasks are hidden by default.
	Queue Queue `yaml:"queue,omitempty" json:"queue,omitempty"`

	// Priority indicates the urgency/importance of the task.
	// Higher priority tasks are shown first within their column.
	Priority Priority `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Category indicates the type of task (feature, bug, refactor, etc).
	Category Category `yaml:"category,omitempty" json:"category,omitempty"`

	// InitiativeID links this task to an initiative (e.g., INIT-001).
	// Empty/null means the task is standalone and not part of any initiative.
	InitiativeID string `yaml:"initiative_id,omitempty" json:"initiative_id,omitempty"`

	// TargetBranch overrides where this task's PR targets.
	// When set, takes precedence over initiative branch and project config.
	// Use for hotfixes or tasks that need to target a specific branch.
	// Example: "hotfix/v2.1" or "release/v3.0"
	TargetBranch string `yaml:"target_branch,omitempty" json:"target_branch,omitempty"`

	// BlockedBy lists task IDs that must complete before this task can run.
	// These are user-editable and stored in task.yaml.
	BlockedBy []string `yaml:"blocked_by,omitempty" json:"blocked_by,omitempty"`

	// Blocks lists task IDs that are waiting on this task.
	// This is computed (not stored) by scanning other tasks' BlockedBy fields.
	Blocks []string `yaml:"-" json:"blocks,omitempty"`

	// RelatedTo lists task IDs that are related (soft connection, informational).
	// Stored in task.yaml, user-editable.
	RelatedTo []string `yaml:"related_to,omitempty" json:"related_to,omitempty"`

	// ReferencedBy lists task IDs whose descriptions mention this task.
	// This is auto-detected and computed (not stored).
	ReferencedBy []string `yaml:"-" json:"referenced_by,omitempty"`

	// IsBlocked indicates if this task has incomplete blockers.
	// This is computed (not stored) from BlockedBy and blocker statuses.
	IsBlocked bool `yaml:"-" json:"is_blocked,omitempty"`

	// UnmetBlockers lists task IDs from BlockedBy that are not yet complete.
	// This is computed (not stored) during PopulateComputedFields.
	UnmetBlockers []string `yaml:"-" json:"unmet_blockers,omitempty"`

	// DependencyStatus indicates the task's dependency state for filtering.
	// Values: "blocked" (has incomplete blockers), "ready" (all deps satisfied or no deps), "none" (no deps)
	// This is computed (not stored) during PopulateComputedFields.
	DependencyStatus DependencyStatus `yaml:"-" json:"dependency_status,omitempty"`

	// IsAutomation indicates this is an automation task (AUTO-XXX prefix).
	// Used for efficient querying via is_automation database column.
	IsAutomation bool `yaml:"is_automation,omitempty" json:"is_automation,omitempty"`

	// RequiresUITesting indicates if this task involves UI changes
	// that should be validated with Playwright or similar tools
	RequiresUITesting bool `yaml:"requires_ui_testing,omitempty" json:"requires_ui_testing,omitempty"`

	// TestingRequirements specifies what types of testing are needed
	TestingRequirements *TestingRequirements `yaml:"testing_requirements,omitempty" json:"testing_requirements,omitempty"`

	// Quality tracks execution quality metrics for analysis.
	// Used to identify patterns in failures and measure improvement over time.
	Quality *QualityMetrics `yaml:"quality,omitempty" json:"quality,omitempty"`

	// CreatedAt is when the task was created
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// UpdatedAt is when the task was last updated
	UpdatedAt time.Time `yaml:"updated_at" json:"updated_at"`

	// StartedAt is when execution began
	StartedAt *time.Time `yaml:"started_at,omitempty" json:"started_at,omitempty"`

	// CompletedAt is when the task finished
	CompletedAt *time.Time `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`

	// Metadata holds arbitrary key-value data
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`

	// PR contains pull request information for this task.
	// This is populated when a PR is created and updated via polling.
	PR *PRInfo `yaml:"pr,omitempty" json:"pr,omitempty"`
}

// New creates a new task with the given title.
func New(id, title string) *Task {
	now := time.Now()
	return &Task{
		ID:        id,
		Title:     title,
		Status:    StatusCreated,
		Branch:    "orc/" + id,
		Queue:     QueueActive,
		Priority:  PriorityNormal,
		Category:  CategoryFeature,
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  make(map[string]string),
	}
}

// GetQueue returns the task's queue, defaulting to active if not set.
func (t *Task) GetQueue() Queue {
	if t.Queue == "" {
		return QueueActive
	}
	return t.Queue
}

// GetPriority returns the task's priority, defaulting to normal if not set.
func (t *Task) GetPriority() Priority {
	if t.Priority == "" {
		return PriorityNormal
	}
	return t.Priority
}

// GetCategory returns the task's category, defaulting to feature if not set.
func (t *Task) GetCategory() Category {
	if t.Category == "" {
		return CategoryFeature
	}
	return t.Category
}

// IsBacklog returns true if the task is in the backlog queue.
func (t *Task) IsBacklog() bool {
	return t.GetQueue() == QueueBacklog
}

// MoveToBacklog moves the task to the backlog queue.
func (t *Task) MoveToBacklog() {
	t.Queue = QueueBacklog
}

// MoveToActive moves the task to the active queue.
func (t *Task) MoveToActive() {
	t.Queue = QueueActive
}

// SetInitiative links the task to an initiative.
// Pass an empty string to unlink the task from any initiative.
func (t *Task) SetInitiative(initiativeID string) {
	t.InitiativeID = initiativeID
}

// GetInitiativeID returns the task's initiative ID, or empty string if not linked.
func (t *Task) GetInitiativeID() string {
	return t.InitiativeID
}

// HasInitiative returns true if the task is linked to an initiative.
func (t *Task) HasInitiative() bool {
	return t.InitiativeID != ""
}

// HasPR returns true if the task has an associated pull request.
func (t *Task) HasPR() bool {
	return t.PR != nil && t.PR.URL != ""
}

// GetPRStatus returns the PR status, or PRStatusNone if no PR exists.
func (t *Task) GetPRStatus() PRStatus {
	if t.PR == nil {
		return PRStatusNone
	}
	return t.PR.Status
}

// SetPRInfo sets or updates the PR information for the task.
func (t *Task) SetPRInfo(url string, number int) {
	if t.PR == nil {
		t.PR = &PRInfo{}
	}
	t.PR.URL = url
	t.PR.Number = number
	// Default to pending review for new PRs
	if t.PR.Status == PRStatusNone {
		t.PR.Status = PRStatusPendingReview
	}
}

// GetPRURL returns the PR URL, or empty string if no PR exists.
func (t *Task) GetPRURL() string {
	if t.PR == nil {
		return ""
	}
	return t.PR.URL
}

// SetMergedInfo marks the PR as merged with the given target branch.
func (t *Task) SetMergedInfo(prURL, targetBranch string) {
	if t.PR == nil {
		t.PR = &PRInfo{}
	}
	t.PR.URL = prURL
	t.PR.Merged = true
	now := time.Now()
	t.PR.MergedAt = &now
	t.PR.TargetBranch = targetBranch
	t.PR.Status = PRStatusMerged
}

// UpdatePRStatus updates the PR status fields from fetched data.
func (t *Task) UpdatePRStatus(status PRStatus, checksStatus string, mergeable bool, reviewCount, approvalCount int) {
	if t.PR == nil {
		t.PR = &PRInfo{}
	}
	t.PR.Status = status
	t.PR.ChecksStatus = checksStatus
	t.PR.Mergeable = mergeable
	t.PR.ReviewCount = reviewCount
	t.PR.ApprovalCount = approvalCount
	now := time.Now()
	t.PR.LastCheckedAt = &now
}

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusResolved
}

// CanRun returns true if the task can be executed.
func (t *Task) CanRun() bool {
	return t.Status == StatusCreated ||
		t.Status == StatusPlanned ||
		t.Status == StatusPaused ||
		t.Status == StatusBlocked
}

// uiKeywords contains words that suggest a task involves UI work.
// These are used to auto-detect tasks that require UI testing.
// NOTE: These are matched as whole words (word boundaries), not substrings.
// For example, "form" matches "form" but not "information" or "transform".
var uiKeywords = []string{
	// UI framework/component terms
	"frontend", "button", "form", "modal", "dialog",
	"component", "widget", "layout", "sidebar", "header", "footer",
	"dashboard", "navbar", "toolbar",
	// Form elements
	"input", "dropdown", "select", "checkbox", "radio",
	"textarea", "datepicker",
	// UI feedback elements
	"tooltip", "popover", "toast", "notification", "alert",
	"spinner", "loader", "progress bar",
	// Visual/styling terms
	"css", "stylesheet", "responsive", "dark mode", "light mode",
	"animation", "transition", "theme",
	// Accessibility
	"a11y", "screen reader", "keyboard navigation", "aria",
	// Specific UI interaction patterns (explicit, not generic verbs)
	"drag and drop", "click handler", "onclick", "hover state",
}

// uiKeywordPattern is a compiled regex for matching UI keywords as whole words.
// Built from uiKeywords at init time.
var uiKeywordPattern *regexp.Regexp

// visualKeywords contains words that suggest visual/design testing is needed.
var visualKeywords = []string{
	"visual", "design", "style", "css", "theme", "layout", "responsive",
	"screenshot", "pixel", "color", "colour", "font", "typography",
}

// visualKeywordPattern is a compiled regex for matching visual keywords as whole words.
var visualKeywordPattern *regexp.Regexp

// buildKeywordPattern creates a case-insensitive word-boundary regex from keywords.
func buildKeywordPattern(keywords []string) *regexp.Regexp {
	// Sort by length descending so longer phrases match first
	sorted := make([]string, len(keywords))
	copy(sorted, keywords)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	// Escape special regex characters and join with |
	escaped := make([]string, len(sorted))
	for i, kw := range sorted {
		escaped[i] = regexp.QuoteMeta(kw)
	}
	pattern := `\b(` + strings.Join(escaped, "|") + `)\b`
	return regexp.MustCompile("(?i)" + pattern)
}

func init() {
	uiKeywordPattern = buildKeywordPattern(uiKeywords)
	visualKeywordPattern = buildKeywordPattern(visualKeywords)
}

// DetectUITesting checks if a task description suggests UI testing is needed.
// Returns true if the title or description contains UI-related keywords.
// Keywords are matched as whole words to avoid false positives
// (e.g., "form" matches but "information" does not).
func DetectUITesting(title, description string) bool {
	text := title + " " + description
	return uiKeywordPattern.MatchString(text)
}

// SetTestingRequirements configures testing requirements based on project and task context.
func (t *Task) SetTestingRequirements(hasFrontend bool) {
	// Auto-detect UI testing from task description
	t.RequiresUITesting = DetectUITesting(t.Title, t.Description)

	// Initialize testing requirements if not set
	if t.TestingRequirements == nil {
		t.TestingRequirements = &TestingRequirements{}
	}

	// Unit tests are always recommended for non-trivial tasks
	if t.Weight != WeightTrivial {
		t.TestingRequirements.Unit = true
	}

	// E2E tests for frontend projects with UI tasks
	if hasFrontend && t.RequiresUITesting {
		t.TestingRequirements.E2E = true
	}

	// Visual tests for tasks explicitly mentioning visual/design concerns
	if visualKeywordPattern.MatchString(t.Title + " " + t.Description) {
		t.TestingRequirements.Visual = true
	}
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

// ComputeBlocks calculates the Blocks field for a task by scanning all tasks.
// Returns task IDs that have this task in their BlockedBy list.
func ComputeBlocks(taskID string, allTasks []*Task) []string {
	var blocks []string
	for _, t := range allTasks {
		for _, blocker := range t.BlockedBy {
			if blocker == taskID {
				blocks = append(blocks, t.ID)
				break
			}
		}
	}
	sort.Strings(blocks)
	return blocks
}

// taskRefPattern matches TASK-XXX patterns (at least 3 digits).
var taskRefPattern = regexp.MustCompile(`\bTASK-\d{3,}\b`)

// DetectTaskReferences scans text for TASK-XXX patterns and returns unique matches.
// Returns a sorted, deduplicated list of task IDs found in the text.
func DetectTaskReferences(text string) []string {
	matches := taskRefPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate and sort
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	sort.Strings(unique)
	return unique
}

// ComputeReferencedBy finds tasks whose descriptions mention this task ID.
// Excludes:
//   - Self-references (task referencing itself)
//   - Tasks already in BlockedBy (those are explicit blocking dependencies)
//   - Tasks already in RelatedTo (those are explicit related links)
//
// This provides "mentioned in" style soft links, similar to GitHub's backlinks.
func ComputeReferencedBy(taskID string, allTasks []*Task) []string {
	var referencedBy []string

	for _, t := range allTasks {
		// Skip self
		if t.ID == taskID {
			continue
		}

		// Check if this task mentions taskID
		refs := DetectTaskReferences(t.Title + " " + t.Description)
		mentions := false
		for _, ref := range refs {
			if ref == taskID {
				mentions = true
				break
			}
		}

		if !mentions {
			continue
		}

		// Exclude if taskID is already in this task's BlockedBy
		inBlockedBy := false
		for _, b := range t.BlockedBy {
			if b == taskID {
				inBlockedBy = true
				break
			}
		}
		if inBlockedBy {
			continue
		}

		// Exclude if taskID is already in this task's RelatedTo
		inRelatedTo := false
		for _, r := range t.RelatedTo {
			if r == taskID {
				inRelatedTo = true
				break
			}
		}
		if inRelatedTo {
			continue
		}

		referencedBy = append(referencedBy, t.ID)
	}
	sort.Strings(referencedBy)
	return referencedBy
}

// PopulateComputedFields fills in computed fields for all tasks:
// - Blocks: tasks that are waiting on this task
// - ReferencedBy: tasks whose descriptions mention this task
// - IsBlocked: whether this task has unmet dependencies
// - UnmetBlockers: list of task IDs that block this task and are incomplete
// - DependencyStatus: "blocked", "ready", or "none" for filtering
// This should be called after loading all tasks.
func PopulateComputedFields(tasks []*Task) {
	// Build task map for dependency checking
	taskMap := make(map[string]*Task)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	for _, t := range tasks {
		t.Blocks = ComputeBlocks(t.ID, tasks)
		t.ReferencedBy = ComputeReferencedBy(t.ID, tasks)
		t.UnmetBlockers = t.GetUnmetDependencies(taskMap)
		t.IsBlocked = len(t.UnmetBlockers) > 0
		t.DependencyStatus = t.ComputeDependencyStatus()
	}
}

// ComputeDependencyStatus returns the dependency status for filtering.
// - "none": task has no dependencies defined
// - "blocked": task has incomplete blockers
// - "ready": all dependencies are satisfied
func (t *Task) ComputeDependencyStatus() DependencyStatus {
	if len(t.BlockedBy) == 0 {
		return DependencyStatusNone
	}
	if len(t.UnmetBlockers) > 0 {
		return DependencyStatusBlocked
	}
	return DependencyStatusReady
}

// isDone returns true if the status indicates the task has completed its work.
func isDone(s Status) bool {
	return s == StatusCompleted
}

// HasUnmetDependencies returns true if any task in BlockedBy is not completed.
func (t *Task) HasUnmetDependencies(tasks map[string]*Task) bool {
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists {
			// Missing task is treated as unmet dependency
			return true
		}
		if !isDone(blocker.Status) {
			return true
		}
	}
	return false
}

// GetUnmetDependencies returns the IDs of tasks that block this one and aren't completed.
func (t *Task) GetUnmetDependencies(tasks map[string]*Task) []string {
	var unmet []string
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists || !isDone(blocker.Status) {
			unmet = append(unmet, blockerID)
		}
	}
	return unmet
}

// BlockerInfo contains information about a blocking task for display purposes.
type BlockerInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
}

// EnsureQualityMetrics initializes the Quality field if nil.
func (t *Task) EnsureQualityMetrics() {
	if t.Quality == nil {
		t.Quality = &QualityMetrics{
			PhaseRetries: make(map[string]int),
		}
	}
	if t.Quality.PhaseRetries == nil {
		t.Quality.PhaseRetries = make(map[string]int)
	}
}

// RecordPhaseRetry increments the retry count for a specific phase.
func (t *Task) RecordPhaseRetry(phase string) {
	t.EnsureQualityMetrics()
	t.Quality.PhaseRetries[phase]++
	t.Quality.TotalRetries++
}

// RecordReviewRejection increments the review rejection count.
func (t *Task) RecordReviewRejection() {
	t.EnsureQualityMetrics()
	t.Quality.ReviewRejections++
}

// RecordManualIntervention marks that manual intervention was required.
func (t *Task) RecordManualIntervention(reason string) {
	t.EnsureQualityMetrics()
	t.Quality.ManualIntervention = true
	t.Quality.ManualInterventionReason = reason
}

// GetPhaseRetries returns the retry count for a specific phase, or 0 if not tracked.
func (t *Task) GetPhaseRetries(phase string) int {
	if t.Quality == nil || t.Quality.PhaseRetries == nil {
		return 0
	}
	return t.Quality.PhaseRetries[phase]
}

// GetTotalRetries returns the total retry count across all phases.
func (t *Task) GetTotalRetries() int {
	if t.Quality == nil {
		return 0
	}
	return t.Quality.TotalRetries
}

// GetReviewRejections returns the review rejection count.
func (t *Task) GetReviewRejections() int {
	if t.Quality == nil {
		return 0
	}
	return t.Quality.ReviewRejections
}

// HadManualIntervention returns true if manual intervention was required.
func (t *Task) HadManualIntervention() bool {
	return t.Quality != nil && t.Quality.ManualIntervention
}

// GetIncompleteBlockers returns full information about blocking tasks that aren't completed.
// This is useful for displaying blocking information to users.
func (t *Task) GetIncompleteBlockers(tasks map[string]*Task) []BlockerInfo {
	var blockers []BlockerInfo
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists {
			// Reference to non-existent task - treat as blocker
			blockers = append(blockers, BlockerInfo{
				ID:     blockerID,
				Title:  "(task not found)",
				Status: "",
			})
			continue
		}
		if !isDone(blocker.Status) {
			blockers = append(blockers, BlockerInfo{
				ID:     blocker.ID,
				Title:  blocker.Title,
				Status: blocker.Status,
			})
		}
	}
	return blockers
}














