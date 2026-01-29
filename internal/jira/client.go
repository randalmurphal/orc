package jira

import (
	"context"
	"fmt"
	"net/http"
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
}

// Client wraps the go-atlassian Jira v3 client with orc-specific convenience methods.
type Client struct {
	jira *v3.Client
	cfg  ClientConfig
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

	client, err := v3.New(&http.Client{Timeout: 30 * time.Second}, cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("create jira client: %w", err)
	}

	client.Auth.SetBasicAuth(cfg.Email, cfg.APIToken)
	client.Auth.SetUserAgent("orc-jira-import/1.0")

	return &Client{jira: client, cfg: cfg}, nil
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

// convertIssue maps a go-atlassian IssueScheme to our simplified Issue type.
func convertIssue(issue *models.IssueScheme) Issue {
	if issue == nil || issue.Fields == nil {
		return Issue{Key: safeKey(issue)}
	}

	f := issue.Fields

	result := Issue{
		Key:       issue.Key,
		Summary:   f.Summary,
		IssueType: safeIssueTypeName(f.IssueType),
		IsSubtask: f.IssueType != nil && f.IssueType.Subtask,
		Status:    safeStatusName(f.Status),
		StatusKey: safeStatusCategoryKey(f.Status),
		Priority:  safePriorityName(f.Priority),
		Labels:    f.Labels,
		ParentKey: safeParentKey(f.Parent),
	}

	// Convert ADF description to markdown
	result.Description = ADFToMarkdown(f.Description)

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
