// Package task provides task management for orc.
package task

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/util"
	"gopkg.in/yaml.v3"
)

const (
	// OrcDir is the default orc configuration directory
	OrcDir = ".orc"
	// TasksDir is the subdirectory for tasks
	TasksDir = "tasks"
)

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
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
)

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

// TestingRequirements specifies what types of testing are needed for a task.
type TestingRequirements struct {
	// Unit indicates if unit tests are required
	Unit bool `yaml:"unit,omitempty" json:"unit,omitempty"`
	// E2E indicates if end-to-end/integration tests are required
	E2E bool `yaml:"e2e,omitempty" json:"e2e,omitempty"`
	// Visual indicates if visual regression tests are required
	Visual bool `yaml:"visual,omitempty" json:"visual,omitempty"`
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

	// RequiresUITesting indicates if this task involves UI changes
	// that should be validated with Playwright or similar tools
	RequiresUITesting bool `yaml:"requires_ui_testing,omitempty" json:"requires_ui_testing,omitempty"`

	// TestingRequirements specifies what types of testing are needed
	TestingRequirements *TestingRequirements `yaml:"testing_requirements,omitempty" json:"testing_requirements,omitempty"`

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

// IsTerminal returns true if the task is in a terminal state.
func (t *Task) IsTerminal() bool {
	return t.Status == StatusCompleted || t.Status == StatusFailed
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
var uiKeywords = []string{
	"ui", "frontend", "button", "form", "page", "modal", "dialog",
	"component", "widget", "layout", "style", "css", "design",
	"responsive", "mobile", "desktop", "navigation", "menu",
	"sidebar", "header", "footer", "dashboard", "table", "grid",
	"card", "input", "dropdown", "select", "checkbox", "radio",
	"tooltip", "popover", "toast", "notification", "alert",
	"animation", "transition", "theme", "dark mode", "light mode",
	"accessibility", "a11y", "screen reader", "keyboard navigation",
	"click", "hover", "focus", "scroll", "drag", "drop",
}

// DetectUITesting checks if a task description suggests UI testing is needed.
// Returns true if the title or description contains UI-related keywords.
func DetectUITesting(title, description string) bool {
	text := strings.ToLower(title + " " + description)
	for _, keyword := range uiKeywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
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
	text := strings.ToLower(t.Title + " " + t.Description)
	visualKeywords := []string{"visual", "design", "style", "css", "theme", "layout", "responsive"}
	for _, keyword := range visualKeywords {
		if strings.Contains(text, keyword) {
			t.TestingRequirements.Visual = true
			break
		}
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
	blockedByMap := make(map[string][]string)
	for _, t := range tasks {
		blockedByMap[t.ID] = t.BlockedBy
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

// ComputeReferencedBy finds tasks whose descriptions mention this task ID.
func ComputeReferencedBy(taskID string, allTasks []*Task) []string {
	var referencedBy []string
	// Match task ID in descriptions (with word boundaries)
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(taskID) + `\b`)

	for _, t := range allTasks {
		if t.ID == taskID {
			continue
		}
		if pattern.MatchString(t.Description) || pattern.MatchString(t.Title) {
			referencedBy = append(referencedBy, t.ID)
		}
	}
	sort.Strings(referencedBy)
	return referencedBy
}

// PopulateComputedFields fills in Blocks and ReferencedBy for all tasks.
// This should be called after loading all tasks.
func PopulateComputedFields(tasks []*Task) {
	for _, t := range tasks {
		t.Blocks = ComputeBlocks(t.ID, tasks)
		t.ReferencedBy = ComputeReferencedBy(t.ID, tasks)
	}
}

// HasUnmetDependencies returns true if any task in BlockedBy is not completed.
func (t *Task) HasUnmetDependencies(tasks map[string]*Task) bool {
	for _, blockerID := range t.BlockedBy {
		blocker, exists := tasks[blockerID]
		if !exists {
			// Missing task is treated as unmet dependency
			return true
		}
		if blocker.Status != StatusCompleted {
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
		if !exists || blocker.Status != StatusCompleted {
			unmet = append(unmet, blockerID)
		}
	}
	return unmet
}

// Load loads a task from disk by ID.
func Load(id string) (*Task, error) {
	return LoadFrom(".", id)
}

// LoadFrom loads a task from a specific project directory.
func LoadFrom(projectDir, id string) (*Task, error) {
	path := filepath.Join(projectDir, OrcDir, TasksDir, id, "task.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task %s not found", id)
		}
		return nil, fmt.Errorf("read task %s: %w", id, err)
	}

	var t Task
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse task %s: %w", id, err)
	}

	return &t, nil
}

// Save persists the task to disk using atomic writes.
func (t *Task) Save() error {
	dir := filepath.Join(OrcDir, TasksDir, t.ID)
	return t.SaveTo(dir)
}

// LoadAll loads all tasks from disk.
func LoadAll() ([]*Task, error) {
	return LoadAllFrom(filepath.Join(OrcDir, TasksDir))
}

// NextID generates the next task ID (TASK-001, TASK-002, etc.).
func NextID() (string, error) {
	return NextIDIn(filepath.Join(OrcDir, TasksDir))
}

// TaskDir returns the directory path for a task.
func TaskDir(id string) string {
	return TaskDirIn(".", id)
}

// TaskDirIn returns the directory path for a task in a specific project.
func TaskDirIn(projectDir, id string) string {
	return filepath.Join(projectDir, OrcDir, TasksDir, id)
}

// Exists returns true if a task exists.
func Exists(id string) bool {
	return ExistsIn(".", id)
}

// ExistsIn returns true if a task exists in a specific project.
func ExistsIn(projectDir, id string) bool {
	path := filepath.Join(projectDir, OrcDir, TasksDir, id, "task.yaml")
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes a task and all its associated files.
// Returns an error if the task is currently running.
func Delete(id string) error {
	return DeleteIn(".", id)
}

// DeleteIn removes a task from a specific project directory.
func DeleteIn(projectDir, id string) error {
	t, err := LoadFrom(projectDir, id)
	if err != nil {
		return fmt.Errorf("task %s not found", id)
	}

	if t.Status == StatusRunning {
		return fmt.Errorf("cannot delete running task %s", id)
	}

	taskDir := TaskDirIn(projectDir, id)
	return os.RemoveAll(taskDir)
}

// SaveTo persists the task to a specific directory using atomic writes.
func (t *Task) SaveTo(dir string) error {
	t.UpdatedAt = time.Now()

	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	path := filepath.Join(dir, "task.yaml")
	if err := util.AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write task: %w", err)
	}

	return nil
}

// LoadAllFrom loads all tasks from a specific tasks directory.
func LoadAllFrom(tasksDir string) ([]*Task, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read tasks directory: %w", err)
	}

	var tasks []*Task
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(tasksDir, entry.Name(), "task.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var t Task
		if err := yaml.Unmarshal(data, &t); err != nil {
			continue
		}
		tasks = append(tasks, &t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks, nil
}

// NextIDIn generates the next task ID in a specific tasks directory.
func NextIDIn(tasksDir string) (string, error) {
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "TASK-001", nil
		}
		return "", fmt.Errorf("read tasks directory: %w", err)
	}

	taskIDRegex := regexp.MustCompile(`^TASK-(\d+)$`)
	maxNum := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		matches := taskIDRegex.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("TASK-%03d", maxNum+1), nil
}
