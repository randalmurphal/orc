package api

import (
	"net/http"
	"strings"

	"github.com/randalmurphal/orc/internal/diff"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/task"
)

// handleGetDiff returns the diff for a task's changes.
// Query params:
//   - base: base branch (default: "main")
//   - files: if "true", only return file list without hunks (for virtual scrolling)
func (s *Server) handleGetDiff(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Load task to get branch and PR info
	t, err := s.backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	// Check if only file list requested (for virtual scrolling of large diffs)
	filesOnly := r.URL.Query().Get("files") == "true"

	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)

	// Determine which diff strategy to use:
	// 1. Merged PR with merge commit SHA → show merge commit diff
	// 2. Task with commit SHAs from phase states → use commit range
	// 3. Default → branch comparison

	// Strategy 1: Merged PR
	if t.PR != nil && t.PR.Merged && t.PR.MergeCommitSHA != "" {
		s.handleMergedPRDiff(w, r, diffSvc, t.PR.MergeCommitSHA, filesOnly)
		return
	}

	// Strategy 2: Use commit SHAs from task state if available
	firstCommit, lastCommit := s.getTaskCommitRange(taskID)
	if firstCommit != "" && lastCommit != "" {
		s.handleCommitRangeDiff(w, r, diffSvc, firstCommit, lastCommit, filesOnly)
		return
	}

	// Strategy 3: Fall back to branch comparison
	s.handleBranchDiff(w, r, diffSvc, t, filesOnly)
}

// handleMergedPRDiff returns the diff for a merged PR using its merge commit.
func (s *Server) handleMergedPRDiff(w http.ResponseWriter, r *http.Request, diffSvc *diff.Service, mergeCommitSHA string, filesOnly bool) {
	displayBase := mergeCommitSHA + "^"
	displayHead := mergeCommitSHA

	if filesOnly {
		files, stats, err := diffSvc.GetMergeCommitFileList(r.Context(), mergeCommitSHA)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, diff.DiffResult{
			Base:  displayBase,
			Head:  displayHead,
			Stats: *stats,
			Files: files,
		})
		return
	}

	result, err := diffSvc.GetMergeCommitDiff(r.Context(), mergeCommitSHA)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, result)
}

// handleCommitRangeDiff returns the diff for a range of commits.
func (s *Server) handleCommitRangeDiff(w http.ResponseWriter, r *http.Request, diffSvc *diff.Service, firstCommit, lastCommit string, filesOnly bool) {
	displayBase := firstCommit + "^"
	displayHead := lastCommit

	if filesOnly {
		files, stats, err := diffSvc.GetCommitRangeFileList(r.Context(), firstCommit, lastCommit)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, diff.DiffResult{
			Base:  displayBase,
			Head:  displayHead,
			Stats: *stats,
			Files: files,
		})
		return
	}

	result, err := diffSvc.GetCommitRangeDiff(r.Context(), firstCommit, lastCommit)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.jsonResponse(w, result)
}

// handleBranchDiff returns the diff using branch comparison (original logic).
func (s *Server) handleBranchDiff(w http.ResponseWriter, r *http.Request, diffSvc *diff.Service, t *task.Task, filesOnly bool) {
	base := r.URL.Query().Get("base")
	if base == "" {
		base = "main"
	}

	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	// Resolve refs (handles remote-only branches)
	base = diffSvc.ResolveRef(r.Context(), base)
	head = diffSvc.ResolveRef(r.Context(), head)

	// Check if we should include uncommitted working tree changes.
	// This happens when the task branch hasn't diverged from base but has uncommitted changes.
	useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(r.Context(), base, head)
	if useWorkingTree {
		head = effectiveHead // Will be "" to indicate working tree comparison
	}

	// For display purposes, show what we're comparing
	displayHead := head
	if displayHead == "" {
		displayHead = "working tree"
	}

	if filesOnly {
		files, err := diffSvc.GetFileList(r.Context(), base, head)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Calculate stats from file list
		stats := &diff.DiffStats{FilesChanged: len(files)}
		for _, f := range files {
			stats.Additions += f.Additions
			stats.Deletions += f.Deletions
		}

		s.jsonResponse(w, diff.DiffResult{
			Base:  base,
			Head:  displayHead,
			Stats: *stats,
			Files: files,
		})
		return
	}

	// Full diff with hunks
	result, err := diffSvc.GetFullDiff(r.Context(), base, head)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update display head for response
	result.Head = displayHead
	s.jsonResponse(w, result)
}

// getTaskCommitRange extracts the first and last commit SHAs from the task's state.
// Returns empty strings if no commits are found.
func (s *Server) getTaskCommitRange(taskID string) (firstCommit, lastCommit string) {
	// Try to load state
	taskState, err := s.backend.LoadState(taskID)
	if err != nil || taskState == nil {
		return "", ""
	}

	// Collect all commit SHAs from phase states
	var commits []string
	for _, phaseState := range taskState.Phases {
		if phaseState != nil && phaseState.CommitSHA != "" {
			commits = append(commits, phaseState.CommitSHA)
		}
	}

	if len(commits) == 0 {
		return "", ""
	}

	// Load task to get weight for phase ordering
	t, err := s.backend.LoadTask(taskID)
	if err != nil || t == nil {
		// Fallback: just use the commits we found (might not be in order)
		if len(commits) == 1 {
			return commits[0], commits[0]
		}
		return "", ""
	}

	// Get phase order from task weight
	phaseOrder := phasesForWeight(t.Weight)

	// Build ordered commit list based on phase order
	var orderedCommits []string
	for _, phaseID := range phaseOrder {
		if phaseState, ok := taskState.Phases[phaseID]; ok && phaseState != nil && phaseState.CommitSHA != "" {
			orderedCommits = append(orderedCommits, phaseState.CommitSHA)
		}
	}

	if len(orderedCommits) == 0 {
		return "", ""
	}

	// Deduplicate (some phases might have the same commit if combined)
	seen := make(map[string]bool)
	var uniqueCommits []string
	for _, c := range orderedCommits {
		if !seen[c] {
			seen[c] = true
			uniqueCommits = append(uniqueCommits, c)
		}
	}

	return uniqueCommits[0], uniqueCommits[len(uniqueCommits)-1]
}

// handleGetDiffFile returns the diff for a single file.
// This is used for on-demand loading in virtual scrolling.
// Query params:
//   - base: base branch (default: "main")
func (s *Server) handleGetDiffFile(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Extract file path from the wildcard portion of the URL
	// The route is /api/tasks/{id}/diff/file/{path...}
	filePath := r.PathValue("path")
	if filePath == "" {
		s.jsonError(w, "file path is required", http.StatusBadRequest)
		return
	}

	// Remove leading slash if present
	filePath = strings.TrimPrefix(filePath, "/")

	t, err := s.backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)

	// Use the same diff strategy as handleGetDiff:
	// 1. Merged PR with merge commit SHA → use merge commit
	// 2. Task with commit SHAs from phase states → use commit range
	// 3. Default → branch comparison

	// Strategy 1: Merged PR
	if t.PR != nil && t.PR.Merged && t.PR.MergeCommitSHA != "" {
		fileDiff, err := diffSvc.GetMergeCommitFileDiff(r.Context(), t.PR.MergeCommitSHA, filePath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, fileDiff)
		return
	}

	// Strategy 2: Use commit SHAs from task state if available
	firstCommit, lastCommit := s.getTaskCommitRange(taskID)
	if firstCommit != "" && lastCommit != "" {
		fileDiff, err := diffSvc.GetCommitRangeFileDiff(r.Context(), firstCommit, lastCommit, filePath)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, fileDiff)
		return
	}

	// Strategy 3: Branch comparison
	base := r.URL.Query().Get("base")
	if base == "" {
		base = "main"
	}

	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	// Resolve refs (handles remote-only branches)
	base = diffSvc.ResolveRef(r.Context(), base)
	head = diffSvc.ResolveRef(r.Context(), head)

	// Check if we should include uncommitted working tree changes
	useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(r.Context(), base, head)
	if useWorkingTree {
		head = effectiveHead // Will be "" to indicate working tree comparison
	}

	fileDiff, err := diffSvc.GetFileDiff(r.Context(), base, head, filePath)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, fileDiff)
}

// handleGetDiffStats returns just the diff statistics.
// Useful for quick summary without fetching file contents.
func (s *Server) handleGetDiffStats(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	t, err := s.backend.LoadTask(taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)

	// Use the same diff strategy as handleGetDiff:
	// 1. Merged PR with merge commit SHA → use merge commit
	// 2. Task with commit SHAs from phase states → use commit range
	// 3. Default → branch comparison

	// Strategy 1: Merged PR
	if t.PR != nil && t.PR.Merged && t.PR.MergeCommitSHA != "" {
		stats, err := diffSvc.GetStats(r.Context(), t.PR.MergeCommitSHA+"^", t.PR.MergeCommitSHA)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, stats)
		return
	}

	// Strategy 2: Use commit SHAs from task state if available
	firstCommit, lastCommit := s.getTaskCommitRange(taskID)
	if firstCommit != "" && lastCommit != "" {
		stats, err := diffSvc.GetStats(r.Context(), firstCommit+"^", lastCommit)
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.jsonResponse(w, stats)
		return
	}

	// Strategy 3: Branch comparison
	base := r.URL.Query().Get("base")
	if base == "" {
		base = "main"
	}

	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	// Resolve refs (handles remote-only branches)
	base = diffSvc.ResolveRef(r.Context(), base)
	head = diffSvc.ResolveRef(r.Context(), head)

	// Check if we should include uncommitted working tree changes
	useWorkingTree, effectiveHead := diffSvc.ShouldIncludeWorkingTree(r.Context(), base, head)
	if useWorkingTree {
		head = effectiveHead // Will be "" to indicate working tree comparison
	}

	stats, err := diffSvc.GetStats(r.Context(), base, head)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}

// phasesForWeight returns the phase IDs for a given task weight.
func phasesForWeight(weight task.Weight) []string {
	switch weight {
	case task.WeightTrivial:
		return []string{"tiny_spec", "implement"}
	case task.WeightSmall:
		return []string{"tiny_spec", "implement", "review"}
	case task.WeightMedium:
		return []string{"spec", "tdd_write", "implement", "review", "docs"}
	case task.WeightLarge:
		return []string{"spec", "tdd_write", "breakdown", "implement", "review", "docs"}
	default:
		return []string{"spec", "implement", "review"}
	}
}
