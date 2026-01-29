package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v3 "github.com/ctreminiom/go-atlassian/v2/jira/v3"
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

func TestConvertIssue_AllNewFields(t *testing.T) {
	dueDate := models.DateScheme(time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC))
	created := models.DateTimeScheme(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	updated := models.DateTimeScheme(time.Date(2025, 1, 16, 12, 0, 0, 0, time.UTC))

	issue := &models.IssueScheme{
		Key: "PROJ-99",
		Fields: &models.IssueFieldsScheme{
			Summary: "Issue with all new fields",
			IssueType: &models.IssueTypeScheme{
				Name: "Story",
			},
			Status: &models.StatusScheme{
				Name: "In Progress",
				StatusCategory: &models.StatusCategoryScheme{
					Key: "indeterminate",
				},
			},
			Priority: &models.PriorityScheme{
				Name: "Medium",
			},
			Assignee: &models.UserScheme{
				DisplayName: "John Doe",
			},
			Reporter: &models.UserScheme{
				DisplayName: "Jane Smith",
			},
			Resolution: &models.ResolutionScheme{
				Name: "Done",
			},
			FixVersions: []*models.VersionScheme{
				{Name: "1.0"},
				{Name: "2.0"},
			},
			DueDate: &dueDate,
			Project: &models.ProjectScheme{
				Key: "PROJ",
			},
			Created: &created,
			Updated: &updated,
		},
	}

	result := convertIssue(issue)

	if result.Assignee != "John Doe" {
		t.Errorf("Assignee = %q, want %q", result.Assignee, "John Doe")
	}
	if result.Reporter != "Jane Smith" {
		t.Errorf("Reporter = %q, want %q", result.Reporter, "Jane Smith")
	}
	if result.Resolution != "Done" {
		t.Errorf("Resolution = %q, want %q", result.Resolution, "Done")
	}
	if len(result.FixVersions) != 2 || result.FixVersions[0] != "1.0" || result.FixVersions[1] != "2.0" {
		t.Errorf("FixVersions = %v, want [1.0 2.0]", result.FixVersions)
	}
	if result.DueDate != "2025-03-15" {
		t.Errorf("DueDate = %q, want %q", result.DueDate, "2025-03-15")
	}
	if result.Project != "PROJ" {
		t.Errorf("Project = %q, want %q", result.Project, "PROJ")
	}
}

func TestCoerceToString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"float64 integer", float64(42), "42"},
		{"float64 decimal", float64(3.14), "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"map with name", map[string]any{"name": "Sprint 5"}, "Sprint 5"},
		{"map with value", map[string]any{"value": "High"}, "High"},
		{"map with neither", map[string]any{"id": 123.0}, `{"id":123}`},
		{"slice", []any{"a", "b", "c"}, "a,b,c"},
		{"slice of maps", []any{map[string]any{"name": "v1"}, map[string]any{"name": "v2"}}, "v1,v2"},
		{"nil-like", 42, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coerceToString(tt.input)
			if got != tt.expected {
				t.Errorf("coerceToString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// newTestJiraClient creates a Client pointing to the given test server URL.
func newTestJiraClient(t *testing.T, serverURL string, customFields map[string]string) *Client {
	t.Helper()
	httpClient := &http.Client{}
	jiraClient, err := v3.New(httpClient, serverURL)
	if err != nil {
		t.Fatalf("v3.New() error: %v", err)
	}
	jiraClient.Auth.SetBasicAuth("test@example.com", "test-token")

	return &Client{
		jira:       jiraClient,
		httpClient: httpClient,
		cfg: ClientConfig{
			BaseURL:      serverURL,
			Email:        "test@example.com",
			APIToken:     "test-token",
			CustomFields: customFields,
		},
	}
}

func TestSearchAllIssues_MockServer(t *testing.T) {
	requestCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s, want POST", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the request body to get nextPageToken
		var reqBody struct {
			JQL           string   `json:"jql"`
			Fields        []string `json:"fields"`
			MaxResults    int      `json:"maxResults"`
			NextPageToken string   `json:"nextPageToken"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("decode request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		requestCount++

		var resp map[string]any
		if reqBody.NextPageToken == "" {
			// First page
			resp = map[string]any{
				"issues": []map[string]any{
					{
						"key": "PROJ-1",
						"fields": map[string]any{
							"summary":   "First issue",
							"issuetype": map[string]any{"name": "Story"},
							"status": map[string]any{
								"name":           "To Do",
								"statusCategory": map[string]any{"key": "new"},
							},
							"priority": map[string]any{"name": "High"},
						},
					},
				},
				"nextPageToken": "page2",
			}
		} else {
			// Second page
			resp = map[string]any{
				"issues": []map[string]any{
					{
						"key": "PROJ-2",
						"fields": map[string]any{
							"summary":   "Second issue",
							"issuetype": map[string]any{"name": "Bug"},
							"status": map[string]any{
								"name":           "Done",
								"statusCategory": map[string]any{"key": "done"},
							},
							"priority": map[string]any{"name": "Medium"},
						},
					},
				},
				"nextPageToken": "",
			}
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	client := newTestJiraClient(t, server.URL, nil)

	issues, err := client.SearchAllIssues(context.Background(), "project = PROJ")
	if err != nil {
		t.Fatalf("SearchAllIssues() error: %v", err)
	}

	if len(issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(issues))
	}
	if issues[0].Key != "PROJ-1" {
		t.Errorf("issues[0].Key = %q, want PROJ-1", issues[0].Key)
	}
	if issues[0].Summary != "First issue" {
		t.Errorf("issues[0].Summary = %q, want %q", issues[0].Summary, "First issue")
	}
	if issues[1].Key != "PROJ-2" {
		t.Errorf("issues[1].Key = %q, want PROJ-2", issues[1].Key)
	}
	if requestCount != 2 {
		t.Errorf("requestCount = %d, want 2", requestCount)
	}
}

func TestCheckAuth_MockServer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/3/myself" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"accountId":   "12345",
				"displayName": "Test User",
				"emailAddress": "test@example.com",
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := newTestJiraClient(t, server.URL, nil)
		err := client.CheckAuth(context.Background())
		if err != nil {
			t.Errorf("CheckAuth() error: %v", err)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/api/3/myself" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message": "Unauthorized"}`))
		}))
		defer server.Close()

		client := newTestJiraClient(t, server.URL, nil)
		err := client.CheckAuth(context.Background())
		if err == nil {
			t.Fatal("CheckAuth() expected error for 401")
		}
		if !contains(err.Error(), "401") {
			t.Errorf("error = %q, want containing '401'", err.Error())
		}
	})
}

func TestFetchCustomFields_MockServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"issues": []map[string]any{
				{
					"key": "PROJ-1",
					"fields": map[string]any{
						"customfield_10020": map[string]any{"name": "Sprint 5"},
						"customfield_10028": 5.0,
					},
				},
			},
			"total": 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	customFields := map[string]string{
		"customfield_10020": "jira_sprint",
		"customfield_10028": "jira_story_points",
	}
	client := newTestJiraClient(t, server.URL, customFields)

	result, err := client.FetchCustomFields(context.Background(), "project = PROJ")
	if err != nil {
		t.Fatalf("FetchCustomFields() error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	issueFields, ok := result["PROJ-1"]
	if !ok {
		t.Fatal("PROJ-1 not in result")
	}
	if issueFields["jira_sprint"] != "Sprint 5" {
		t.Errorf("jira_sprint = %q, want %q", issueFields["jira_sprint"], "Sprint 5")
	}
	if issueFields["jira_story_points"] != "5" {
		t.Errorf("jira_story_points = %q, want %q", issueFields["jira_story_points"], "5")
	}
}

func TestFetchCustomFields_NoConfig(t *testing.T) {
	client := &Client{
		cfg: ClientConfig{},
	}

	result, err := client.FetchCustomFields(context.Background(), "project = PROJ")
	if err != nil {
		t.Fatalf("FetchCustomFields() error: %v", err)
	}
	if result != nil {
		t.Errorf("result = %v, want nil for empty custom fields config", result)
	}
}
