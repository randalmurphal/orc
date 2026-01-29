package jira

import (
	"testing"
	"time"

	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

func TestConvertIssue(t *testing.T) {
	created := models.DateTimeScheme(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	updated := models.DateTimeScheme(time.Date(2025, 1, 16, 12, 0, 0, 0, time.UTC))

	issue := &models.IssueScheme{
		Key: "PROJ-42",
		Fields: &models.IssueFieldsScheme{
			Summary: "Fix authentication bug",
			Description: &models.CommentNodeScheme{
				Type: "doc",
				Content: []*models.CommentNodeScheme{
					{
						Type: "paragraph",
						Content: []*models.CommentNodeScheme{
							{Type: "text", Text: "Auth is broken"},
						},
					},
				},
			},
			IssueType: &models.IssueTypeScheme{
				Name:    "Bug",
				Subtask: false,
			},
			Status: &models.StatusScheme{
				Name: "In Progress",
				StatusCategory: &models.StatusCategoryScheme{
					Key:  "indeterminate",
					Name: "In Progress",
				},
			},
			Priority: &models.PriorityScheme{
				Name: "High",
			},
			Labels: []string{"critical", "auth"},
			Components: []*models.ComponentScheme{
				{Name: "backend"},
				{Name: "security"},
			},
			Parent: &models.ParentScheme{
				Key: "PROJ-10",
			},
			IssueLinks: []*models.IssueLinkScheme{
				{
					Type: &models.LinkTypeScheme{
						Name:    "Blocks",
						Inward:  "is blocked by",
						Outward: "blocks",
					},
					OutwardIssue: &models.LinkedIssueScheme{
						Key: "PROJ-50",
					},
				},
				{
					Type: &models.LinkTypeScheme{
						Name:    "Relates",
						Inward:  "relates to",
						Outward: "relates to",
					},
					InwardIssue: &models.LinkedIssueScheme{
						Key: "PROJ-99",
					},
				},
			},
			Created: &created,
			Updated: &updated,
		},
	}

	result := convertIssue(issue)

	if result.Key != "PROJ-42" {
		t.Errorf("Key = %q, want PROJ-42", result.Key)
	}
	if result.Summary != "Fix authentication bug" {
		t.Errorf("Summary = %q", result.Summary)
	}
	if result.Description != "Auth is broken" {
		t.Errorf("Description = %q, want %q", result.Description, "Auth is broken")
	}
	if result.IssueType != "Bug" {
		t.Errorf("IssueType = %q", result.IssueType)
	}
	if result.IsSubtask {
		t.Error("IsSubtask should be false")
	}
	if result.Status != "In Progress" {
		t.Errorf("Status = %q", result.Status)
	}
	if result.StatusKey != "indeterminate" {
		t.Errorf("StatusKey = %q", result.StatusKey)
	}
	if result.Priority != "High" {
		t.Errorf("Priority = %q", result.Priority)
	}
	if len(result.Labels) != 2 || result.Labels[0] != "critical" {
		t.Errorf("Labels = %v", result.Labels)
	}
	if len(result.Components) != 2 || result.Components[0] != "backend" {
		t.Errorf("Components = %v", result.Components)
	}
	if result.ParentKey != "PROJ-10" {
		t.Errorf("ParentKey = %q", result.ParentKey)
	}
	if len(result.IssueLinks) != 2 {
		t.Fatalf("IssueLinks = %d, want 2", len(result.IssueLinks))
	}
	// First link: outward "Blocks" to PROJ-50
	if result.IssueLinks[0].Type != "Blocks" || result.IssueLinks[0].Direction != LinkOutward || result.IssueLinks[0].LinkedKey != "PROJ-50" {
		t.Errorf("Link[0] = %+v", result.IssueLinks[0])
	}
	// Second link: inward "Relates" to PROJ-99
	if result.IssueLinks[1].Type != "Relates" || result.IssueLinks[1].Direction != LinkInward || result.IssueLinks[1].LinkedKey != "PROJ-99" {
		t.Errorf("Link[1] = %+v", result.IssueLinks[1])
	}
	if result.Created.IsZero() {
		t.Error("Created should not be zero")
	}
	if result.Updated.IsZero() {
		t.Error("Updated should not be zero")
	}
}

func TestConvertIssue_NilFields(t *testing.T) {
	// Nil issue
	result := convertIssue(nil)
	if result.Key != "" {
		t.Errorf("nil issue: Key = %q, want empty", result.Key)
	}

	// Issue with nil fields
	result = convertIssue(&models.IssueScheme{
		Key:    "PROJ-1",
		Fields: nil,
	})
	if result.Key != "PROJ-1" {
		t.Errorf("nil fields: Key = %q, want PROJ-1", result.Key)
	}
	if result.Summary != "" {
		t.Errorf("nil fields: Summary = %q, want empty", result.Summary)
	}

	// Issue with minimal fields (nil sub-objects)
	result = convertIssue(&models.IssueScheme{
		Key: "PROJ-2",
		Fields: &models.IssueFieldsScheme{
			Summary: "Minimal",
		},
	})
	if result.Summary != "Minimal" {
		t.Errorf("minimal: Summary = %q", result.Summary)
	}
	if result.IssueType != "" {
		t.Errorf("minimal: IssueType = %q, want empty", result.IssueType)
	}
	if result.Status != "" {
		t.Errorf("minimal: Status = %q, want empty", result.Status)
	}
}

func TestNewClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientConfig
		wantErr string
	}{
		{
			name:    "empty URL",
			cfg:     ClientConfig{Email: "a@b.com", APIToken: "tok"},
			wantErr: "base URL is required",
		},
		{
			name:    "empty email",
			cfg:     ClientConfig{BaseURL: "https://x.atlassian.net", APIToken: "tok"},
			wantErr: "email is required",
		},
		{
			name:    "empty token",
			cfg:     ClientConfig{BaseURL: "https://x.atlassian.net", Email: "a@b.com"},
			wantErr: "API token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNewClient_Success(t *testing.T) {
	client, err := NewClient(ClientConfig{
		BaseURL:  "https://test.atlassian.net",
		Email:    "test@example.com",
		APIToken: "test-token",
	})
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if client == nil {
		t.Fatal("client should not be nil")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
