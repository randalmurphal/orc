package planner

import (
	"encoding/json"
	"fmt"
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

// jsonTaskBreakdown matches the Claude JSON output format.
type jsonTaskBreakdown struct {
	Summary string     `json:"summary"`
	Tasks   []jsonTask `json:"tasks"`
}

type jsonTask struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Weight      string `json:"weight"`
	DependsOn   []int  `json:"depends_on"`
}

// ParseTaskBreakdown extracts tasks from Claude's response.
// Expects JSON output with structure: {"summary": "...", "tasks": [...]}
func ParseTaskBreakdown(content string) (*TaskBreakdown, error) {
	// Find JSON in the response - it may be wrapped in markdown code blocks
	jsonContent := extractJSON(content)
	if jsonContent == "" {
		return nil, fmt.Errorf("no JSON task breakdown found in response")
	}

	var breakdown jsonTaskBreakdown
	if err := json.Unmarshal([]byte(jsonContent), &breakdown); err != nil {
		return nil, fmt.Errorf("parse task breakdown JSON: %w", err)
	}

	if len(breakdown.Tasks) == 0 {
		return nil, fmt.Errorf("task breakdown contains no tasks")
	}

	result := &TaskBreakdown{
		Summary: breakdown.Summary,
		Tasks:   make([]*ProposedTask, len(breakdown.Tasks)),
	}

	for i, t := range breakdown.Tasks {
		idx := t.ID
		if idx == 0 {
			idx = i + 1 // Default to position-based index
		}

		weight := normalizeWeight(strings.TrimSpace(t.Weight))

		result.Tasks[i] = &ProposedTask{
			Index:       idx,
			Title:       strings.TrimSpace(t.Title),
			Description: strings.TrimSpace(t.Description),
			Weight:      weight,
			DependsOn:   t.DependsOn,
		}
	}

	return result, nil
}

// extractJSON finds JSON content in the response.
// It handles JSON wrapped in code blocks or raw JSON.
func extractJSON(content string) string {
	// Try to find JSON in markdown code block
	if start := strings.Index(content, "```json"); start != -1 {
		start += 7 // skip "```json"
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}

	// Try to find JSON in generic code block
	if start := strings.Index(content, "```"); start != -1 {
		start += 3 // skip "```"
		// Skip language identifier if present
		if newline := strings.Index(content[start:], "\n"); newline != -1 {
			start += newline + 1
		}
		if end := strings.Index(content[start:], "```"); end != -1 {
			candidate := strings.TrimSpace(content[start : start+end])
			if strings.HasPrefix(candidate, "{") {
				return candidate
			}
		}
	}

	// Try to find raw JSON object
	if start := strings.Index(content, "{"); start != -1 {
		// Find the matching closing brace
		depth := 0
		for i := start; i < len(content); i++ {
			switch content[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return content[start : i+1]
				}
			}
		}
	}

	return ""
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
