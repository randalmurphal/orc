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

	// Load task to get branch
	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	base := r.URL.Query().Get("base")
	if base == "" {
		base = "main"
	}

	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	// Check if only file list requested (for virtual scrolling of large diffs)
	filesOnly := r.URL.Query().Get("files") == "true"

	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)

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
			Head:  head,
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

	s.jsonResponse(w, result)
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

	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	base := r.URL.Query().Get("base")
	if base == "" {
		base = "main"
	}

	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)
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

	t, err := task.LoadFrom(s.workDir, taskID)
	if err != nil {
		s.handleOrcError(w, orcerrors.ErrTaskNotFound(taskID))
		return
	}

	base := r.URL.Query().Get("base")
	if base == "" {
		base = "main"
	}

	head := t.Branch
	if head == "" {
		head = "HEAD"
	}

	diffSvc := diff.NewService(s.getProjectRoot(), s.diffCache)
	stats, err := diffSvc.GetStats(r.Context(), base, head)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, stats)
}
