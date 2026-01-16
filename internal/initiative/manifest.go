// Package initiative provides initiative/feature grouping for related tasks.
package initiative

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/task"
	"gopkg.in/yaml.v3"
)

// ManifestVersion is the current manifest format version.
const ManifestVersion = 1

// ManifestTask represents a task definition within a manifest.
type ManifestTask struct {
	// ID is the local identifier for dependency references within the manifest.
	// This is not the final TASK-XXX ID, which is assigned during creation.
	ID int `yaml:"id"`

	// Title is the task title (required).
	Title string `yaml:"title"`

	// Description is an optional detailed description.
	Description string `yaml:"description,omitempty"`

	// Weight is the task complexity (trivial, small, medium, large, greenfield).
	// Defaults to medium if not specified.
	Weight string `yaml:"weight,omitempty"`

	// Category is the task type (feature, bug, refactor, chore, docs, test).
	// Defaults to feature if not specified.
	Category string `yaml:"category,omitempty"`

	// Priority is the task urgency (critical, high, normal, low).
	// Defaults to normal if not specified.
	Priority string `yaml:"priority,omitempty"`

	// DependsOn lists local IDs of tasks that must complete before this task.
	DependsOn []int `yaml:"depends_on,omitempty"`

	// Spec is the inline specification content.
	// If provided, the task will skip the spec phase during execution.
	Spec string `yaml:"spec,omitempty"`
}

// CreateInitiative contains details for creating a new initiative.
type CreateInitiative struct {
	Title  string `yaml:"title"`
	Vision string `yaml:"vision,omitempty"`
}

// Manifest represents a bulk task creation manifest.
type Manifest struct {
	// Version is the manifest format version (currently 1).
	Version int `yaml:"version"`

	// Initiative is the ID of an existing initiative to add tasks to.
	// Mutually exclusive with CreateInitiative.
	Initiative string `yaml:"initiative,omitempty"`

	// CreateInitiative contains details for creating a new initiative.
	// Mutually exclusive with Initiative.
	CreateInitiative *CreateInitiative `yaml:"create_initiative,omitempty"`

	// Tasks is the list of tasks to create.
	Tasks []ManifestTask `yaml:"tasks"`
}

// ParseManifest reads and parses a manifest file.
func ParseManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	return ParseManifestBytes(data)
}

// ParseManifestBytes parses manifest content from bytes.
func ParseManifestBytes(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, nil
}

// ValidationError represents a manifest validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidateManifest validates a manifest and returns all validation errors.
func ValidateManifest(m *Manifest) []error {
	var errs []error

	// Check version
	if m.Version == 0 {
		errs = append(errs, &ValidationError{
			Field:   "version",
			Message: "version is required",
		})
	} else if m.Version != ManifestVersion {
		errs = append(errs, &ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version %d (expected %d)", m.Version, ManifestVersion),
		})
	}

	// Check initiative specification
	if m.Initiative == "" && m.CreateInitiative == nil {
		errs = append(errs, &ValidationError{
			Message: "either 'initiative' or 'create_initiative' must be specified",
		})
	}
	if m.Initiative != "" && m.CreateInitiative != nil {
		errs = append(errs, &ValidationError{
			Message: "'initiative' and 'create_initiative' are mutually exclusive",
		})
	}

	// Validate create_initiative if specified
	if m.CreateInitiative != nil {
		if m.CreateInitiative.Title == "" {
			errs = append(errs, &ValidationError{
				Field:   "create_initiative.title",
				Message: "title is required",
			})
		}
	}

	// Check for at least one task
	if len(m.Tasks) == 0 {
		errs = append(errs, &ValidationError{
			Field:   "tasks",
			Message: "at least one task is required",
		})
	}

	// Validate individual tasks and collect IDs
	localIDs := make(map[int]bool)
	for i, t := range m.Tasks {
		taskPrefix := fmt.Sprintf("tasks[%d]", i)

		// Check for duplicate local IDs
		if t.ID != 0 {
			if localIDs[t.ID] {
				errs = append(errs, &ValidationError{
					Field:   taskPrefix + ".id",
					Message: fmt.Sprintf("duplicate local ID %d", t.ID),
				})
			}
			localIDs[t.ID] = true
		} else {
			errs = append(errs, &ValidationError{
				Field:   taskPrefix + ".id",
				Message: "local ID is required (must be a positive integer)",
			})
		}

		// Validate title
		if t.Title == "" {
			errs = append(errs, &ValidationError{
				Field:   taskPrefix + ".title",
				Message: "title is required",
			})
		}

		// Validate weight if specified
		if t.Weight != "" {
			w := task.Weight(t.Weight)
			if !task.IsValidWeight(w) {
				errs = append(errs, &ValidationError{
					Field:   taskPrefix + ".weight",
					Message: fmt.Sprintf("invalid weight %q (valid: %s)", t.Weight, formatWeights()),
				})
			}
		}

		// Validate category if specified
		if t.Category != "" {
			c := task.Category(t.Category)
			if !task.IsValidCategory(c) {
				errs = append(errs, &ValidationError{
					Field:   taskPrefix + ".category",
					Message: fmt.Sprintf("invalid category %q (valid: %s)", t.Category, formatCategories()),
				})
			}
		}

		// Validate priority if specified
		if t.Priority != "" {
			p := task.Priority(t.Priority)
			if !task.IsValidPriority(p) {
				errs = append(errs, &ValidationError{
					Field:   taskPrefix + ".priority",
					Message: fmt.Sprintf("invalid priority %q (valid: %s)", t.Priority, formatPriorities()),
				})
			}
		}
	}

	// Validate dependencies reference valid local IDs
	for i, t := range m.Tasks {
		taskPrefix := fmt.Sprintf("tasks[%d]", i)
		for _, depID := range t.DependsOn {
			if !localIDs[depID] {
				errs = append(errs, &ValidationError{
					Field:   taskPrefix + ".depends_on",
					Message: fmt.Sprintf("references unknown local ID %d", depID),
				})
			}
			if depID == t.ID {
				errs = append(errs, &ValidationError{
					Field:   taskPrefix + ".depends_on",
					Message: "task cannot depend on itself",
				})
			}
		}
	}

	// Check for circular dependencies
	if cycle := detectCircularDependencies(m.Tasks); cycle != nil {
		cycleStr := make([]string, len(cycle))
		for i, id := range cycle {
			cycleStr[i] = fmt.Sprintf("%d", id)
		}
		errs = append(errs, &ValidationError{
			Field:   "depends_on",
			Message: fmt.Sprintf("circular dependency detected: %s", strings.Join(cycleStr, " -> ")),
		})
	}

	return errs
}

// detectCircularDependencies checks for cycles in task dependencies.
// Returns the cycle path if found, nil otherwise.
func detectCircularDependencies(tasks []ManifestTask) []int {
	// Build adjacency list
	deps := make(map[int][]int)
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}

	// DFS for cycle detection
	visited := make(map[int]bool)
	path := make(map[int]bool)
	var cyclePath []int

	var dfs func(id int) bool
	dfs = func(id int) bool {
		if path[id] {
			cyclePath = append(cyclePath, id)
			return true
		}
		if visited[id] {
			return false
		}

		visited[id] = true
		path[id] = true

		for _, dep := range deps[id] {
			if dfs(dep) {
				cyclePath = append(cyclePath, id)
				return true
			}
		}

		path[id] = false
		return false
	}

	for _, t := range tasks {
		if dfs(t.ID) {
			// Reverse to show cycle in order
			for i, j := 0, len(cyclePath)-1; i < j; i, j = i+1, j-1 {
				cyclePath[i], cyclePath[j] = cyclePath[j], cyclePath[i]
			}
			return cyclePath
		}
	}

	return nil
}

// TopologicalSort returns task indices in dependency order.
// Tasks with no dependencies come first, followed by tasks whose
// dependencies have already been listed.
func TopologicalSort(tasks []ManifestTask) ([]int, error) {
	// Build maps
	idToIndex := make(map[int]int)
	for i, t := range tasks {
		idToIndex[t.ID] = i
	}

	// Kahn's algorithm for topological sort
	inDegree := make(map[int]int)
	for _, t := range tasks {
		if _, exists := inDegree[t.ID]; !exists {
			inDegree[t.ID] = 0
		}
		for _, dep := range t.DependsOn {
			inDegree[dep] = inDegree[dep] // Ensure dep exists
		}
	}

	// Count incoming edges
	for _, t := range tasks {
		for range t.DependsOn {
			inDegree[t.ID]++
		}
	}

	// Find all nodes with no incoming edges
	var queue []int
	for _, t := range tasks {
		if inDegree[t.ID] == 0 {
			queue = append(queue, t.ID)
		}
	}

	// Sort queue for deterministic ordering
	sort.Ints(queue)

	var result []int
	for len(queue) > 0 {
		// Pop from queue
		id := queue[0]
		queue = queue[1:]
		result = append(result, idToIndex[id])

		// Find tasks that depend on this one
		for _, t := range tasks {
			for _, dep := range t.DependsOn {
				if dep == id {
					inDegree[t.ID]--
					if inDegree[t.ID] == 0 {
						queue = append(queue, t.ID)
						sort.Ints(queue)
					}
				}
			}
		}
	}

	if len(result) != len(tasks) {
		return nil, fmt.Errorf("circular dependency prevents ordering")
	}

	return result, nil
}

// Helper functions for formatting valid values

func formatWeights() string {
	weights := task.ValidWeights()
	strs := make([]string, len(weights))
	for i, w := range weights {
		strs[i] = string(w)
	}
	return strings.Join(strs, ", ")
}

func formatCategories() string {
	cats := task.ValidCategories()
	strs := make([]string, len(cats))
	for i, c := range cats {
		strs[i] = string(c)
	}
	return strings.Join(strs, ", ")
}

func formatPriorities() string {
	pris := task.ValidPriorities()
	strs := make([]string, len(pris))
	for i, p := range pris {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}
