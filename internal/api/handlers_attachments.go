package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

// handleListAttachments returns all attachments for a task.
// GET /api/tasks/{id}/attachments
func (s *Server) handleListAttachments(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	// Verify task exists
	exists, err := s.backend.TaskExists(taskID)
	if err != nil || !exists {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	attachments, err := s.backend.ListAttachments(taskID)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list attachments: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, attachments)
}

// handleUploadAttachment uploads a new attachment to a task.
// POST /api/tasks/{id}/attachments
func (s *Server) handleUploadAttachment(w http.ResponseWriter, r *http.Request) {
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

// handleGetAttachment returns a specific attachment file.
// GET /api/tasks/{id}/attachments/{filename}
func (s *Server) handleGetAttachment(w http.ResponseWriter, r *http.Request) {
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

// handleDeleteAttachment deletes an attachment from a task.
// DELETE /api/tasks/{id}/attachments/{filename}
func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
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

	if err := s.backend.DeleteAttachment(taskID, filename); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "attachment not found", http.StatusNotFound)
		} else if strings.Contains(err.Error(), "invalid filename") {
			s.jsonError(w, "invalid filename", http.StatusBadRequest)
		} else {
			s.jsonError(w, fmt.Sprintf("failed to delete attachment: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
