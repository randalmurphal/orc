package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	v3 "github.com/ctreminiom/go-atlassian/v2/jira/v3"
	"github.com/ctreminiom/go-atlassian/v2/pkg/infra/models"
)

// ClientConfig holds the configuration for connecting to a Jira Cloud instance.
type ClientConfig struct {
	// BaseURL is the Jira Cloud instance URL (e.g., "https://acme.atlassian.net").
	BaseURL string
	// Email is the user's email address for basic auth.
	Email string
	// APIToken is the API token for basic auth.
	APIToken string
	// CustomFields maps Jira custom field IDs to metadata key names.
	// Example: {"customfield_10020": "jira_sprint"}
	// If empty, no custom field extraction is performed.
	CustomFields map[string]string
}

// Client wraps the go-atlassian Jira v3 client with orc-specific convenience methods.
type Client struct {
	jira       *v3.Client
	httpClient *http.Client
	cfg        ClientConfig
}

// NewClient creates a new Jira Cloud client with basic auth.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("jira base URL is required")
	}
	if cfg.Email == "" {
		return nil, fmt.Errorf("jira email is required")
	}
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("jira API token is required")
	}

	// Ensure URL doesn't have trailing slash
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	httpClient := &http.Client{Timeout: 30 * time.Second}

	client, err := v3.New(httpClient, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("create jira client: %w", err)
	}

	client.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)
	client.Auth.SetUserAgent("orc-jira-import/1.0")

	return &Client{jira: client, httpClient: httpClient, cfg: cfg}, nil
}

// searchFields are the Jira fields we request in search results.
// Keeping this explicit avoids fetching unnecessary data.
var searchFields = []string{
	"summary",
	"description",
	"issuetype",
	"status",
	"priority",
	"labels",
	"components",
	"parent",
	"issuelinks",
	"created",
	"updated",
	"assignee",
	"reporter",
	"resolution",
	"fixVersions",
	"duedate",
	"project",
}

// SearchAllIssues fetches all issues matching the JQL query, handling pagination.
// Returns the full list of issues converted to our simplified Issue type.
func (c *Client) SearchAllIssues(ctx context.Context, jql string) ([]Issue, error) {
	var all []Issue
	nextPageToken := ""

	for {
		result, resp, err := c.jira.Issue.Search.SearchJQL(
			ctx,
			jql,
			searchFields,
			nil, // no expand
			50,  // maxResults per page
			nextPageToken,
		)
		if err != nil {
			if resp != nil {
				return nil, fmt.Errorf("jira search (status %d): %w", resp.StatusCode, err)
			}
			return nil, fmt.Errorf("jira search: %w", err)
		}

		for _, issue := range result.Issues {
			all = append(all, convertIssue(issue))
		}

		if result.NextPageToken == "" || len(result.Issues) == 0 {
			break
		}
		nextPageToken = result.NextPageToken
	}

	return all, nil
}

// CheckAuth verifies the client can authenticate with Jira.
func (c *Client) CheckAuth(ctx context.Context) error {
	_, resp, err := c.jira.MySelf.Details(ctx, nil)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("jira auth check failed (status %d): %w", resp.StatusCode, err)
		}
		return fmt.Errorf("jira auth check failed: %w", err)
	}
	return nil
}

// FetchCustomFields fetches custom field values for the given issues.
// go-atlassian's IssueFieldsScheme is a fixed struct that drops unknown JSON keys,
// so we make a raw HTTP request to extract custom field values.
// Returns a map of issueKey → (customFieldID → stringValue).
// Only called when CustomFields is configured (len > 0).
func (c *Client) FetchCustomFields(ctx context.Context, jql string) (map[string]map[string]string, error) {
	if len(c.cfg.CustomFields) == 0 {
		return nil, nil
	}

	// Build the fields list: key + configured custom field IDs
	fields := []string{"key"}
	for cfID := range c.cfg.CustomFields {
		fields = append(fields, cfID)
	}

	result := make(map[string]map[string]string)
	startAt := 0

	for {
		// Build search URL with query params
		params := url.Values{}
		params.Set("jql", jql)
		params.Set("fields", strings.Join(fields, ","))
		params.Set("maxResults", "50")
		params.Set("startAt", strconv.Itoa(startAt))

		searchURL := c.cfg.BaseURL + "/rest/api/3/search?" + params.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
		if err != nil {
			return nil, fmt.Errorf("build custom field request: %w", err)
		}
		req.SetBasicAuth(c.cfg.Email, c.cfg.APIToken)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch custom fields: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read custom field response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("custom field search failed (status %d): %s", resp.StatusCode, string(body))
		}

		// Parse into generic structure
		var searchResult struct {
			Issues []struct {
				Key    string                 `json:"key"`
				Fields map[string]any `json:"fields"`
			} `json:"issues"`
			Total int `json:"total"`
		}
		if err := json.Unmarshal(body, &searchResult); err != nil {
			return nil, fmt.Errorf("parse custom field response: %w", err)
		}

		for _, issue := range searchResult.Issues {
			cfValues := make(map[string]string)
			for cfID, metadataKey := range c.cfg.CustomFields {
				if val, ok := issue.Fields[cfID]; ok && val != nil {
					cfValues[metadataKey] = coerceToString(val)
				}
			}
			if len(cfValues) > 0 {
				result[issue.Key] = cfValues
			}
		}

		startAt += len(searchResult.Issues)
		if startAt >= searchResult.Total || len(searchResult.Issues) == 0 {
			break
		}
	}

	return result, nil
}

// coerceToString converts a custom field value to a string representation.
// Handles common Jira custom field types: strings, numbers, objects with name/value, arrays.
func coerceToString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case map[string]any:
		// Objects with name field (select fields, sprint objects, etc.)
		if name, ok := v["name"]; ok {
			return fmt.Sprintf("%v", name)
		}
		if value, ok := v["value"]; ok {
			return fmt.Sprintf("%v", value)
		}
		// Fallback to JSON
		b, _ := json.Marshal(v)
		return string(b)
	case []any:
		// Arrays (multi-select, etc.)
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, coerceToString(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// convertIssue maps a go-atlassian IssueScheme to our simplified Issue type.
func convertIssue(issue *models.IssueScheme) Issue {
	if issue == nil || issue.Fields == nil {
		return Issue{Key: safeKey(issue)}
	}

	f := issue.Fields

	result := Issue{
		Key:        issue.Key,
		Summary:    f.Summary,
		IssueType:  safeIssueTypeName(f.IssueType),
		IsSubtask:  f.IssueType != nil && f.IssueType.Subtask,
		Status:     safeStatusName(f.Status),
		StatusKey:  safeStatusCategoryKey(f.Status),
		Priority:   safePriorityName(f.Priority),
		Labels:     f.Labels,
		ParentKey:  safeParentKey(f.Parent),
		Assignee:   safeUserDisplayName(f.Assignee),
		Reporter:   safeUserDisplayName(f.Reporter),
		Resolution: safeResolutionName(f.Resolution),
		Project:    safeProjectKey(f.Project),
	}

	// Convert ADF description to markdown
	result.Description = ADFToMarkdown(f.Description)

	// Extract fix version names
	for _, v := range f.FixVersions {
		if v != nil && v.Name != "" {
			result.FixVersions = append(result.FixVersions, v.Name)
		}
	}

	// Extract due date (DateScheme is a time.Time alias, format as YYYY-MM-DD)
	if f.DueDate != nil {
		result.DueDate = time.Time(*f.DueDate).Format("2006-01-02")
	}

	// Extract component names
	for _, comp := range f.Components {
		if comp != nil && comp.Name != "" {
			result.Components = append(result.Components, comp.Name)
		}
	}

	// Convert issue links
	for _, link := range f.IssueLinks {
		if link == nil || link.Type == nil {
			continue
		}
		if link.OutwardIssue != nil {
			result.IssueLinks = append(result.IssueLinks, IssueLink{
				Type:      link.Type.Name,
				Direction: LinkOutward,
				LinkedKey: link.OutwardIssue.Key,
			})
		}
		if link.InwardIssue != nil {
			result.IssueLinks = append(result.IssueLinks, IssueLink{
				Type:      link.Type.Name,
				Direction: LinkInward,
				LinkedKey: link.InwardIssue.Key,
			})
		}
	}

	// Parse timestamps
	if f.Created != nil {
		result.Created = time.Time(*f.Created)
	}
	if f.Updated != nil {
		result.Updated = time.Time(*f.Updated)
	}

	return result
}

func safeKey(issue *models.IssueScheme) string {
	if issue == nil {
		return ""
	}
	return issue.Key
}

func safeIssueTypeName(it *models.IssueTypeScheme) string {
	if it == nil {
		return ""
	}
	return it.Name
}

func safeStatusName(s *models.StatusScheme) string {
	if s == nil {
		return ""
	}
	return s.Name
}

func safeStatusCategoryKey(s *models.StatusScheme) string {
	if s == nil || s.StatusCategory == nil {
		return ""
	}
	return s.StatusCategory.Key
}

func safePriorityName(p *models.PriorityScheme) string {
	if p == nil {
		return ""
	}
	return p.Name
}

func safeParentKey(p *models.ParentScheme) string {
	if p == nil {
		return ""
	}
	return p.Key
}

func safeUserDisplayName(u *models.UserScheme) string {
	if u == nil {
		return ""
	}
	return u.DisplayName
}

func safeResolutionName(r *models.ResolutionScheme) string {
	if r == nil {
		return ""
	}
	return r.Name
}

func safeProjectKey(p *models.ProjectScheme) string {
	if p == nil {
		return ""
	}
	return p.Key
}
