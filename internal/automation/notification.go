package automation

import (
	"time"
)

// NotificationType constants for typed notification types.
const (
	// NotificationTypeAutomationPending indicates automation tasks are pending approval.
	NotificationTypeAutomationPending = "automation_pending"
	// NotificationTypeAutomationFailed indicates an automation task failed.
	NotificationTypeAutomationFailed = "automation_failed"
	// NotificationTypeAutomationBlocked indicates a trigger is blocked by cooldown.
	NotificationTypeAutomationBlocked = "automation_blocked"
)

// NotificationSourceType constants for notification sources.
const (
	NotificationSourceTrigger = "trigger"
	NotificationSourceTask    = "task"
)

// NotificationResponse is the API response format for notifications.
type NotificationResponse struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Title      string               `json:"title"`
	Message    string               `json:"message,omitempty"`
	SourceType string               `json:"source_type,omitempty"`
	SourceID   string               `json:"source_id,omitempty"`
	Actions    []NotificationAction `json:"actions,omitempty"`
	CreatedAt  string               `json:"created_at"`
	ExpiresAt  string               `json:"expires_at,omitempty"`
}

// ToResponse converts a Notification to NotificationResponse.
func (n *Notification) ToResponse() NotificationResponse {
	resp := NotificationResponse{
		ID:         n.ID,
		Type:       n.Type,
		Title:      n.Title,
		Message:    n.Message,
		SourceType: n.SourceType,
		SourceID:   n.SourceID,
	}

	if !n.CreatedAt.IsZero() {
		resp.CreatedAt = n.CreatedAt.Format(time.RFC3339)
	}
	if n.ExpiresAt != nil && !n.ExpiresAt.IsZero() {
		resp.ExpiresAt = n.ExpiresAt.Format(time.RFC3339)
	}

	// Add actions based on notification type
	switch n.Type {
	case NotificationTypeAutomationPending:
		resp.Actions = []NotificationAction{
			{Label: "Review", Href: "/automation"},
			{Label: "Dismiss", Action: "dismiss"},
		}
	case NotificationTypeAutomationFailed:
		resp.Actions = []NotificationAction{
			{Label: "View Details", Href: "/automation"},
			{Label: "Dismiss", Action: "dismiss"},
		}
	case NotificationTypeAutomationBlocked:
		resp.Actions = []NotificationAction{
			{Label: "Dismiss", Action: "dismiss"},
		}
	default:
		// Use provided actions if any
		resp.Actions = n.Actions
	}

	return resp
}
