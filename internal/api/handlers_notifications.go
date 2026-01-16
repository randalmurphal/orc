package api

import (
	"net/http"
	"strings"

	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/storage"
)

// handleListNotifications returns all active notifications.
// GET /api/notifications
func (s *Server) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.jsonError(w, "database backend required for notifications", http.StatusInternalServerError)
		return
	}

	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	notifications, err := adapter.GetActiveNotifications(r.Context())
	if err != nil {
		s.logger.Error("failed to get notifications", "error", err)
		s.jsonError(w, "failed to get notifications", http.StatusInternalServerError)
		return
	}

	// Convert to API response format
	resp := make([]automation.NotificationResponse, 0, len(notifications))
	for _, n := range notifications {
		resp = append(resp, n.ToResponse())
	}

	s.jsonResponse(w, map[string]any{
		"notifications": resp,
	})
}

// handleDismissNotification dismisses a single notification.
// PUT /api/notifications/{id}/dismiss
func (s *Server) handleDismissNotification(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[1] != "dismiss" {
		s.jsonError(w, "invalid path", http.StatusBadRequest)
		return
	}
	id := parts[0]

	if id == "" {
		s.jsonError(w, "notification ID required", http.StatusBadRequest)
		return
	}

	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.jsonError(w, "database backend required for notifications", http.StatusInternalServerError)
		return
	}

	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	if err := adapter.DismissNotification(r.Context(), id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			s.jsonError(w, "notification not found", http.StatusNotFound)
			return
		}
		s.logger.Error("failed to dismiss notification", "id", id, "error", err)
		s.jsonError(w, "failed to dismiss notification", http.StatusInternalServerError)
		return
	}

	s.logger.Info("notification dismissed", "id", id)
	s.jsonResponse(w, map[string]string{
		"status": "dismissed",
		"id":     id,
	})
}

// handleDismissAllNotifications dismisses all active notifications.
// PUT /api/notifications/dismiss-all
func (s *Server) handleDismissAllNotifications(w http.ResponseWriter, r *http.Request) {
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.jsonError(w, "database backend required for notifications", http.StatusInternalServerError)
		return
	}

	adapter := automation.NewProjectDBAdapter(dbBackend.DB())
	if err := adapter.DismissAllNotifications(r.Context()); err != nil {
		s.logger.Error("failed to dismiss all notifications", "error", err)
		s.jsonError(w, "failed to dismiss all notifications", http.StatusInternalServerError)
		return
	}

	s.logger.Info("all notifications dismissed")
	s.jsonResponse(w, map[string]string{
		"status": "dismissed_all",
	})
}
