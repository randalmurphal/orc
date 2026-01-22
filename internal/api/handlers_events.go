package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// EventResponse represents an event in the API response.
type EventResponse struct {
	ID        int64   `json:"id"`
	TaskID    string  `json:"task_id"`
	TaskTitle string  `json:"task_title"`
	Phase     *string `json:"phase,omitempty"`
	Iteration *int    `json:"iteration,omitempty"`
	EventType string  `json:"event_type"`
	Data      any     `json:"data,omitempty"`
	Source    string  `json:"source"`
	CreatedAt string  `json:"created_at"` // ISO8601
}

// EventsListResponse represents the paginated events response.
type EventsListResponse struct {
	Events  []EventResponse `json:"events"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	HasMore bool            `json:"has_more"`
}

// handleGetEvents handles GET /api/events - query events with filters.
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	taskID := r.URL.Query().Get("task_id")
	initiativeID := r.URL.Query().Get("initiative_id")
	sinceStr := r.URL.Query().Get("since")
	untilStr := r.URL.Query().Get("until")
	typesStr := r.URL.Query().Get("types")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	// Parse and validate limit (default 100, max 1000)
	limit := 100
	if limitStr != "" {
		parsedLimit, err := strconv.Atoi(limitStr)
		if err != nil {
			s.jsonError(w, "invalid limit parameter", http.StatusBadRequest)
			return
		}
		if parsedLimit < 1 || parsedLimit > 1000 {
			s.jsonError(w, "limit must be between 1 and 1000", http.StatusBadRequest)
			return
		}
		limit = parsedLimit
	}

	// Parse and validate offset (default 0)
	offset := 0
	if offsetStr != "" {
		parsedOffset, err := strconv.Atoi(offsetStr)
		if err != nil {
			s.jsonError(w, "invalid offset parameter", http.StatusBadRequest)
			return
		}
		if parsedOffset < 0 {
			s.jsonError(w, "offset must be non-negative", http.StatusBadRequest)
			return
		}
		offset = parsedOffset
	}

	// Parse timestamps (ISO8601)
	var since, until *time.Time
	if sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			s.jsonError(w, "invalid since timestamp (expected ISO8601/RFC3339)", http.StatusBadRequest)
			return
		}
		since = &t
	}
	if untilStr != "" {
		t, err := time.Parse(time.RFC3339, untilStr)
		if err != nil {
			s.jsonError(w, "invalid until timestamp (expected ISO8601/RFC3339)", http.StatusBadRequest)
			return
		}
		until = &t
	}

	// Parse event types (comma-separated)
	var eventTypes []string
	if typesStr != "" {
		eventTypes = splitAndTrim(typesStr)
	}

	// Build query options
	opts := db.QueryEventsOptions{
		TaskID:       taskID,
		InitiativeID: initiativeID,
		Since:        since,
		Until:        until,
		EventTypes:   eventTypes,
		Limit:        limit,
		Offset:       offset,
	}

	// Get database handle (backend is always DatabaseBackend in practice)
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.logger.Error("backend is not DatabaseBackend")
		s.jsonError(w, "internal error: database not available", http.StatusInternalServerError)
		return
	}
	pdb := dbBackend.DB()

	// Query events with titles
	events, err := pdb.QueryEventsWithTitles(opts)
	if err != nil {
		s.logger.Error("failed to query events", "error", err)
		s.jsonError(w, "failed to query events", http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	total, err := pdb.CountEvents(opts)
	if err != nil {
		s.logger.Error("failed to count events", "error", err)
		s.jsonError(w, "failed to count events", http.StatusInternalServerError)
		return
	}

	// Convert to API response format
	apiEvents := make([]EventResponse, 0, len(events))
	for _, e := range events {
		apiEvents = append(apiEvents, EventResponse{
			ID:        e.ID,
			TaskID:    e.TaskID,
			TaskTitle: e.TaskTitle,
			Phase:     e.Phase,
			Iteration: e.Iteration,
			EventType: e.EventType,
			Data:      e.Data,
			Source:    e.Source,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}

	// Build response with pagination metadata
	response := EventsListResponse{
		Events:  apiEvents,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: offset+len(apiEvents) < total,
	}

	s.jsonResponse(w, response)
}
