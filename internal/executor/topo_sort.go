package executor

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
)

// topologicalSort orders workflow phases respecting DependsOn constraints,
// using Sequence as a tiebreaker for phases at the same dependency level.
// Uses Kahn's algorithm (BFS). Only DependsOn is considered — loop_config
// and retry_from_phase are runtime control flow and are intentionally ignored.
func topologicalSort(phases []*db.WorkflowPhase) ([]*db.WorkflowPhase, error) {
	if len(phases) == 0 {
		return phases, nil
	}

	// Build lookup by PhaseTemplateID
	phaseByID := make(map[string]*db.WorkflowPhase, len(phases))
	for _, p := range phases {
		phaseByID[p.PhaseTemplateID] = p
	}

	// Parse DependsOn and build adjacency list + in-degree map.
	// adjacency: dependency -> list of phases that depend on it
	adjacency := make(map[string][]string, len(phases))
	inDegree := make(map[string]int, len(phases))

	for _, p := range phases {
		inDegree[p.PhaseTemplateID] = 0
	}

	for _, p := range phases {
		deps, err := parseDependsOn(p.DependsOn)
		if err != nil {
			return nil, fmt.Errorf("parse depends_on for phase %s: %w", p.PhaseTemplateID, err)
		}
		seen := make(map[string]bool, len(deps))
		for _, dep := range deps {
			if seen[dep] {
				continue // deduplicate
			}
			seen[dep] = true

			// Only count dependencies that reference phases in this workflow
			if _, exists := phaseByID[dep]; !exists {
				continue // missing dep is a no-op
			}
			adjacency[dep] = append(adjacency[dep], p.PhaseTemplateID)
			inDegree[p.PhaseTemplateID]++
		}
	}

	// Collect zero-indegree phases, sorted by Sequence
	queue := make([]*db.WorkflowPhase, 0)
	for _, p := range phases {
		if inDegree[p.PhaseTemplateID] == 0 {
			queue = append(queue, p)
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		return queue[i].Sequence < queue[j].Sequence
	})

	// Kahn's algorithm with Sequence tiebreaker
	result := make([]*db.WorkflowPhase, 0, len(phases))
	for len(queue) > 0 {
		// Dequeue first (lowest sequence)
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Find dependents and decrement their in-degree
		var newReady []*db.WorkflowPhase
		for _, depID := range adjacency[current.PhaseTemplateID] {
			inDegree[depID]--
			if inDegree[depID] == 0 {
				newReady = append(newReady, phaseByID[depID])
			}
		}

		// Insert newly ready phases into queue maintaining Sequence order
		if len(newReady) > 0 {
			queue = append(queue, newReady...)
			sort.Slice(queue, func(i, j int) bool {
				return queue[i].Sequence < queue[j].Sequence
			})
		}
	}

	if len(result) != len(phases) {
		// Cycle detected — collect involved phases
		var cyclePhases []string
		for id, deg := range inDegree {
			if deg > 0 {
				cyclePhases = append(cyclePhases, id)
			}
		}
		sort.Strings(cyclePhases)
		return nil, fmt.Errorf("cycle detected involving phases: %s", strings.Join(cyclePhases, ", "))
	}

	return result, nil
}

// parseDependsOn extracts phase template IDs from the JSON array string.
func parseDependsOn(raw string) ([]string, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var deps []string
	if err := json.Unmarshal([]byte(raw), &deps); err != nil {
		return nil, err
	}
	return deps, nil
}

// computeExecutionLevels groups phases by execution level based on dependencies.
// Phases in the same level have no dependencies on each other and can run in parallel.
// Returns: [[A], [B,C], [D]] for diamond pattern A→B,C→D
//
// Uses a modified Kahn's algorithm that assigns levels based on the maximum
// level of all dependencies plus one.
func computeExecutionLevels(phases []*db.WorkflowPhase) ([][]*db.WorkflowPhase, error) {
	if len(phases) == 0 {
		return nil, nil
	}

	// Build lookup by PhaseTemplateID
	phaseByID := make(map[string]*db.WorkflowPhase, len(phases))
	for _, p := range phases {
		phaseByID[p.PhaseTemplateID] = p
	}

	// Parse DependsOn and build adjacency list + in-degree map.
	// adjacency: dependency -> list of phases that depend on it
	adjacency := make(map[string][]string, len(phases))
	inDegree := make(map[string]int, len(phases))
	deps := make(map[string][]string, len(phases)) // phase -> its dependencies

	for _, p := range phases {
		inDegree[p.PhaseTemplateID] = 0
	}

	for _, p := range phases {
		parsed, err := parseDependsOn(p.DependsOn)
		if err != nil {
			return nil, fmt.Errorf("parse depends_on for phase %s: %w", p.PhaseTemplateID, err)
		}
		seen := make(map[string]bool, len(parsed))
		validDeps := make([]string, 0, len(parsed))
		for _, dep := range parsed {
			if seen[dep] {
				continue // deduplicate
			}
			seen[dep] = true

			// Only count dependencies that reference phases in this workflow
			if _, exists := phaseByID[dep]; !exists {
				continue // missing dep is a no-op
			}
			adjacency[dep] = append(adjacency[dep], p.PhaseTemplateID)
			inDegree[p.PhaseTemplateID]++
			validDeps = append(validDeps, dep)
		}
		deps[p.PhaseTemplateID] = validDeps
	}

	// Compute level for each phase using Kahn's algorithm
	// Level = max(levels of all dependencies) + 1, or 0 if no dependencies
	levels := make(map[string]int, len(phases))
	processed := 0

	// Process phases level by level
	for processed < len(phases) {
		var ready []string
		for _, p := range phases {
			if _, done := levels[p.PhaseTemplateID]; done {
				continue
			}
			if inDegree[p.PhaseTemplateID] == 0 {
				ready = append(ready, p.PhaseTemplateID)
			}
		}

		if len(ready) == 0 {
			// Cycle detected — collect involved phases
			var cyclePhases []string
			for _, p := range phases {
				if _, done := levels[p.PhaseTemplateID]; !done {
					cyclePhases = append(cyclePhases, p.PhaseTemplateID)
				}
			}
			sort.Strings(cyclePhases)
			return nil, fmt.Errorf("cycle detected involving phases: %s", strings.Join(cyclePhases, ", "))
		}

		// Compute level for each ready phase
		for _, id := range ready {
			maxDepLevel := -1
			for _, dep := range deps[id] {
				if levels[dep] > maxDepLevel {
					maxDepLevel = levels[dep]
				}
			}
			levels[id] = maxDepLevel + 1

			// Decrement in-degree of dependents
			for _, depID := range adjacency[id] {
				inDegree[depID]--
			}
			processed++
		}
	}

	// Group phases by level
	maxLevel := 0
	for _, lvl := range levels {
		if lvl > maxLevel {
			maxLevel = lvl
		}
	}

	result := make([][]*db.WorkflowPhase, maxLevel+1)
	for i := range result {
		result[i] = make([]*db.WorkflowPhase, 0)
	}

	for _, p := range phases {
		lvl := levels[p.PhaseTemplateID]
		result[lvl] = append(result[lvl], p)
	}

	// Sort phases within each level by sequence for determinism
	for i := range result {
		sort.Slice(result[i], func(a, b int) bool {
			return result[i][a].Sequence < result[i][b].Sequence
		})
	}

	return result, nil
}
