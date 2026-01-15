package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/task"
)

// handleGetTestResults returns test results for a task.
// GET /api/tasks/{id}/test-results
func (s *Server) handleGetTestResults(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	results, err := task.GetTestResults(s.workDir, taskID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to get test results: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, results)
}

// handleListScreenshots returns all screenshots for a task.
// GET /api/tasks/{id}/test-results/screenshots
func (s *Server) handleListScreenshots(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	screenshots, err := task.ListScreenshots(s.workDir, taskID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list screenshots: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, screenshots)
}

// handleGetScreenshot returns a specific screenshot file.
// GET /api/tasks/{id}/test-results/screenshots/{filename}
func (s *Server) handleGetScreenshot(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	filename := r.PathValue("filename")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Sanitize filename
	filename = filepath.Base(filename)

	screenshot, reader, err := task.GetScreenshot(s.workDir, taskID, filename)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "screenshot not found", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "invalid filename") {
			s.jsonError(w, "invalid filename", http.StatusBadRequest)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to get screenshot: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	// Detect content type from filename
	contentType := "image/png"
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".webp":
		contentType = "image/webp"
	case ".gif":
		contentType = "image/gif"
	}

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(screenshot.Size, 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", screenshot.Filename))
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	// Stream the file
	io.Copy(w, reader)
}

// handleUploadScreenshot uploads a screenshot for a task.
// POST /api/tasks/{id}/test-results/screenshots
func (s *Server) handleUploadScreenshot(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.jsonError(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get filename - use form field if provided, otherwise use uploaded filename
	filename := r.FormValue("filename")
	if filename == "" {
		filename = header.Filename
	}

	// Sanitize filename - remove path components
	filename = filepath.Base(filename)
	if filename == "." || filename == "" {
		s.jsonError(w, "invalid filename", http.StatusBadRequest)
		return
	}

	// Save the screenshot
	screenshot, err := task.SaveScreenshot(s.workDir, taskID, filename, file)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save screenshot: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, screenshot)
}

// handleGetHTMLReport returns the Playwright HTML report if available.
// GET /api/tasks/{id}/test-results/report
func (s *Server) handleGetHTMLReport(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	reader, err := task.GetHTMLReport(s.workDir, taskID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "HTML report not found", http.StatusNotFound)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to get HTML report: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.Copy(w, reader)
}

// handleGetTrace returns a trace file if available.
// GET /api/tasks/{id}/test-results/traces/{filename}
func (s *Server) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	filename := r.PathValue("filename")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Sanitize filename
	filename = filepath.Base(filename)

	reader, err := task.GetTrace(s.workDir, taskID, filename)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "trace not found", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "invalid filename") {
			s.jsonError(w, "invalid filename", http.StatusBadRequest)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to get trace: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	// Traces are typically zip files
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	io.Copy(w, reader)
}

// handleSaveTestReport saves a test report for a task.
// POST /api/tasks/{id}/test-results
func (s *Server) handleSaveTestReport(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	var report task.TestReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := task.SaveTestReport(s.workDir, taskID, &report); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save test report: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, map[string]string{"status": "saved"})
}

// handleInitTestResults initializes the test-results directory structure.
// POST /api/tasks/{id}/test-results/init
func (s *Server) handleInitTestResults(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	if exists, err := s.backend.TaskExists(taskID); err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if err := task.InitTestResultsDir(s.workDir, taskID); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to initialize test results: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the path for use in Playwright config
	path := task.TestResultsPath(s.workDir, taskID)
	s.jsonResponse(w, map[string]string{
		"status": "initialized",
		"path":   path,
	})
}
