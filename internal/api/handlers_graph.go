package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/task"
)

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

// GraphEdge represents a directed edge in the dependency graph.
type GraphEdge struct {
	From string `json:"from"` // Blocking task
	To   string `json:"to"`   // Blocked task
}

// DependencyGraphResponse is the API response for dependency graph endpoints.
type DependencyGraphResponse struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// handleGetInitiativeDependencyGraph returns the dependency graph for tasks within an initiative.
func (s *Server) handleGetInitiativeDependencyGraph(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Note: shared parameter ignored - all initiatives are loaded from backend
	_ = r.URL.Query().Get("shared")

	// Load initiative
	init, err := s.backend.LoadInitiative(id)
	if err != nil {
		s.jsonError(w, "initiative not found", http.StatusNotFound)
		return
	}

	// Build set of task IDs in this initiative
	taskIDs := make(map[string]bool)
	for _, t := range init.Tasks {
		taskIDs[t.ID] = true
	}

	// Load all tasks to get full dependency data (TaskRef doesn't store blocked_by)
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	// Build task map
	taskMap := make(map[string]*task.Task)
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	// Build graph from full task data
	graph := buildGraphFromTasks(taskMap, taskIDs)

	s.jsonResponse(w, graph)
}

// handleGetTasksDependencyGraph returns the dependency graph for an arbitrary set of tasks.
func (s *Server) handleGetTasksDependencyGraph(w http.ResponseWriter, r *http.Request) {
	// Get task IDs from query parameter
	idsParam := r.URL.Query().Get("ids")
	if idsParam == "" {
		s.jsonError(w, "ids parameter is required", http.StatusBadRequest)
		return
	}

	ids := strings.Split(idsParam, ",")
	if len(ids) == 0 {
		s.jsonError(w, "at least one task ID is required", http.StatusBadRequest)
		return
	}

	// Build set of requested task IDs
	requestedIDs := make(map[string]bool)
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			requestedIDs[id] = true
		}
	}

	if len(requestedIDs) == 0 {
		s.jsonError(w, "no valid task IDs provided", http.StatusBadRequest)
		return
	}

	// Load all tasks
	allTasks, err := s.backend.LoadAllTasks()
	if err != nil {
		s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	// Build task map
	taskMap := make(map[string]*task.Task)
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	// Build graph from requested tasks
	graph := buildGraphFromTasks(taskMap, requestedIDs)

	s.jsonResponse(w, graph)
}

// buildGraphFromTasks creates a dependency graph from a set of tasks.
func buildGraphFromTasks(taskMap map[string]*task.Task, requestedIDs map[string]bool) *DependencyGraphResponse {
	nodes := make([]GraphNode, 0, len(requestedIDs))
	edges := make([]GraphEdge, 0)
	edgeSet := make(map[string]bool) // Deduplicate edges

	for id := range requestedIDs {
		t, exists := taskMap[id]
		if !exists {
			// Skip non-existent tasks but continue with others
			continue
		}

		// Add node
		nodes = append(nodes, GraphNode{
			ID:     t.ID,
			Title:  t.Title,
			Status: mapTaskStatus(string(t.Status)),
		})

		// Add edges for blocked_by relationships within the requested set
		for _, blockerID := range t.BlockedBy {
			if requestedIDs[blockerID] {
				edgeKey := blockerID + "->" + t.ID
				if !edgeSet[edgeKey] {
					edges = append(edges, GraphEdge{
						From: blockerID,
						To:   t.ID,
					})
					edgeSet[edgeKey] = true
				}
			}
		}
	}

	// Sort nodes by ID for consistent output
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// Sort edges for consistent output
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	return &DependencyGraphResponse{
		Nodes: nodes,
		Edges: edges,
	}
}

// mapTaskStatus maps internal task status to display status for the graph.
// Returns simplified status values for visualization: done, running, blocked, ready, pending
func mapTaskStatus(status string) string {
	switch status {
	case "completed", "finished":
		return "done"
	case "running", "finalizing":
		return "running"
	case "blocked":
		return "blocked"
	case "paused":
		return "paused"
	case "failed":
		return "failed"
	case "created", "planned":
		return "ready"
	default:
		return "pending"
	}
}
