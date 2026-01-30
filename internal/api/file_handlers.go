package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/randalmurphal/orc/internal/task"
)

// registerFileRoutes sets up routes for binary file serving.
// These are the ONLY HTTP routes remaining after Connect RPC migration.
// All structured data access should go through Connect RPC at /rpc/*.
func (s *Server) registerFileRoutes() {
	// CORS middleware wrapper for file routes
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	// Attachments
	s.mux.HandleFunc("GET /files/tasks/{id}/attachments/{filename}", cors(s.serveAttachment))
	s.mux.HandleFunc("POST /files/tasks/{id}/attachments", cors(s.uploadAttachment))

	// Test results (Playwright)
	s.mux.HandleFunc("GET /files/tasks/{id}/test-results/screenshots/{filename}", cors(s.serveScreenshot))
	s.mux.HandleFunc("GET /files/tasks/{id}/test-results/traces/{filename}", cors(s.serveTrace))
	s.mux.HandleFunc("GET /files/tasks/{id}/test-results/html-report", cors(s.serveHTMLReport))

	// Export/Import API (tar.gz archive operations)
	exportServer := NewExportServer(s.backend, s.workDir, s.logger)
	exportServer.SetProjectCache(s.projectCache)
	s.mux.HandleFunc("POST /api/export", cors(exportServer.HandleExport))
	s.mux.HandleFunc("POST /api/import", cors(exportServer.HandleImport))

	// Static files (embedded frontend) - catch-all for non-API routes
	s.mux.Handle("/", staticHandler())
}

// serveAttachment returns a specific attachment file.
// GET /files/tasks/{id}/attachments/{filename}
func (s *Server) serveAttachment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	filename := r.PathValue("filename")

	// Verify task exists
	exists, err := s.backend.TaskExists(taskID)
	if err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Sanitize filename
	filename = filepath.Base(filename)

	attachment, data, err := s.backend.GetAttachment(taskID, filename)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "attachment not found", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "invalid filename") {
			s.jsonError(w, "invalid filename", http.StatusBadRequest)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to get attachment: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Set response headers
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.Size, 10))

	// Set content disposition based on whether it's an image
	if attachment.IsImage {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", attachment.Filename))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", attachment.Filename))
	}

	// Write the file data
	if _, err := w.Write(data); err != nil {
		// Client may have disconnected; just log and return
		slog.Debug("error writing attachment", "filename", attachment.Filename, "error", err)
	}
}

// uploadAttachment uploads a new attachment to a task.
// POST /files/tasks/{id}/attachments
func (s *Server) uploadAttachment(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	exists, err := s.backend.TaskExists(taskID)
	if err != nil || !exists {
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
	defer func() { _ = file.Close() }()

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

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	// Save the attachment
	attachment, err := s.backend.SaveAttachment(taskID, filename, header.Header.Get("Content-Type"), data)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save attachment: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, attachment)
}

// serveScreenshot returns a specific screenshot file.
// GET /files/tasks/{id}/test-results/screenshots/{filename}
func (s *Server) serveScreenshot(w http.ResponseWriter, r *http.Request) {
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
	defer func() { _ = reader.Close() }()

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
	_, _ = io.Copy(w, reader)
}

// serveTrace returns a trace file if available.
// GET /files/tasks/{id}/test-results/traces/{filename}
func (s *Server) serveTrace(w http.ResponseWriter, r *http.Request) {
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
	defer func() { _ = reader.Close() }()

	// Traces are typically zip files
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	_, _ = io.Copy(w, reader)
}

// serveHTMLReport returns the Playwright HTML report if available.
// GET /files/tasks/{id}/test-results/html-report
func (s *Server) serveHTMLReport(w http.ResponseWriter, r *http.Request) {
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
	defer func() { _ = reader.Close() }()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, reader)
}
