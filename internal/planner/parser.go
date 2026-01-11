package planner

import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/task"
)

// ProposedTask represents a task proposed by Claude.
type ProposedTask struct {
	Index       int         `yaml:"index" json:"index"`
	Title       string      `yaml:"title" json:"title"`
	Description string      `yaml:"description" json:"description"`
	Weight      task.Weight `yaml:"weight" json:"weight"`
	DependsOn   []int       `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
}

// TaskBreakdown represents the parsed output from Claude.
type TaskBreakdown struct {
	Summary string          `yaml:"summary" json:"summary"`
	Tasks   []*ProposedTask `yaml:"tasks" json:"tasks"`
}

// xmlTaskBreakdown matches the Claude output format.
type xmlTaskBreakdown struct {
	Tasks []xmlTask `xml:"task"`
}

type xmlTask struct {
	ID          string `xml:"id,attr"`
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Weight      string `xml:"weight"`
	DependsOn   string `xml:"depends_on"`
}

var taskBreakdownPattern = regexp.MustCompile(`(?s)<task_breakdown>(.*?)</task_breakdown>`)

// ParseTaskBreakdown extracts tasks from Claude's response.
func ParseTaskBreakdown(content string) (*TaskBreakdown, error) {
	// Extract <task_breakdown>...</task_breakdown>
	matches := taskBreakdownPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return nil, fmt.Errorf("no task breakdown found in response")
	}

	xmlContent := "<task_breakdown>" + matches[1] + "</task_breakdown>"

	var breakdown xmlTaskBreakdown
	if err := xml.Unmarshal([]byte(xmlContent), &breakdown); err != nil {
		return nil, fmt.Errorf("parse task breakdown XML: %w", err)
	}

	if len(breakdown.Tasks) == 0 {
		return nil, fmt.Errorf("task breakdown contains no tasks")
	}

	result := &TaskBreakdown{
		Tasks: make([]*ProposedTask, len(breakdown.Tasks)),
	}

	for i, t := range breakdown.Tasks {
		idx, err := strconv.Atoi(t.ID)
		if err != nil {
			idx = i + 1 // Default to position-based index
		}

		weight := normalizeWeight(strings.TrimSpace(t.Weight))

		result.Tasks[i] = &ProposedTask{
			Index:       idx,
			Title:       strings.TrimSpace(t.Title),
			Description: strings.TrimSpace(t.Description),
			Weight:      weight,
			DependsOn:   parseDependencies(t.DependsOn),
		}
	}

	// Extract summary if present (text before task_breakdown)
	summaryIdx := strings.Index(content, "<task_breakdown>")
	if summaryIdx > 0 {
		result.Summary = strings.TrimSpace(content[:summaryIdx])
	}

	return result, nil
}

// parseDependencies parses a comma-separated list of task indices.
func parseDependencies(s string) []int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var deps []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if id, err := strconv.Atoi(p); err == nil && id > 0 {
			deps = append(deps, id)
		}
	}
	return deps
}

// normalizeWeight converts a weight string to task.Weight.
func normalizeWeight(s string) task.Weight {
	s = strings.ToLower(s)
	switch s {
	case "trivial":
		return task.WeightTrivial
	case "small":
		return task.WeightSmall
	case "medium":
		return task.WeightMedium
	case "large":
		return task.WeightLarge
	case "greenfield":
		return task.WeightGreenfield
	default:
		// Default to medium for unknown weights
		return task.WeightMedium
	}
}

// ValidateDependencies checks that all dependencies reference valid task indices.
func ValidateDependencies(breakdown *TaskBreakdown) error {
	// Build set of valid indices
	validIndices := make(map[int]bool)
	for _, t := range breakdown.Tasks {
		validIndices[t.Index] = true
	}

	// Check each task's dependencies
	for _, t := range breakdown.Tasks {
		for _, dep := range t.DependsOn {
			if !validIndices[dep] {
				return fmt.Errorf("task %d depends on non-existent task %d", t.Index, dep)
			}
			if dep >= t.Index {
				return fmt.Errorf("task %d depends on task %d (forward reference)", t.Index, dep)
			}
		}
	}

	// Check for circular dependencies
	if err := detectCycles(breakdown); err != nil {
		return err
	}

	return nil
}

// detectCycles checks for circular dependencies in the task graph.
func detectCycles(breakdown *TaskBreakdown) error {
	// Build adjacency list
	deps := make(map[int][]int)
	for _, t := range breakdown.Tasks {
		deps[t.Index] = t.DependsOn
	}

	// Track visited and in-progress nodes for DFS
	visited := make(map[int]bool)
	inProgress := make(map[int]bool)

	var dfs func(idx int, path []int) error
	dfs = func(idx int, path []int) error {
		if inProgress[idx] {
			// Found cycle - build path string
			cycleStart := 0
			for i, p := range path {
				if p == idx {
					cycleStart = i
					break
				}
			}
			cycle := append(path[cycleStart:], idx)
			return fmt.Errorf("circular dependency detected: %v", cycle)
		}

		if visited[idx] {
			return nil
		}

		inProgress[idx] = true
		path = append(path, idx)

		for _, dep := range deps[idx] {
			if err := dfs(dep, path); err != nil {
				return err
			}
		}

		inProgress[idx] = false
		visited[idx] = true
		return nil
	}

	for _, t := range breakdown.Tasks {
		if !visited[t.Index] {
			if err := dfs(t.Index, nil); err != nil {
				return err
			}
		}
	}

	return nil
}
