package db

import (
	"fmt"
	"strings"
)

// FormatThreadLinksForPrompt renders typed thread links as prompt-friendly bullets.
func FormatThreadLinksForPrompt(links []ThreadLink, limit int) string {
	return formatPromptBullets(limitThreadLinks(links, limit), func(link ThreadLink) string {
		label := strings.TrimSpace(link.TargetID)
		if strings.TrimSpace(link.Title) != "" {
			label = strings.TrimSpace(link.Title)
		}
		return fmt.Sprintf("%s: %s", link.LinkType, label)
	})
}

// FormatThreadRecommendationDraftsForPrompt renders recommendation drafts as prompt-friendly bullets.
func FormatThreadRecommendationDraftsForPrompt(drafts []ThreadRecommendationDraft, limit int) string {
	return formatPromptBullets(limitThreadRecommendationDrafts(drafts, limit), func(draft ThreadRecommendationDraft) string {
		var lines []string
		lines = append(lines, fmt.Sprintf("[%s] %s", draft.Status, draft.Title))
		if strings.TrimSpace(draft.Summary) != "" {
			lines = append(lines, fmt.Sprintf("  Summary: %s", strings.TrimSpace(draft.Summary)))
		}
		return strings.Join(lines, "\n")
	})
}

// FormatThreadDecisionDraftsForPrompt renders decision drafts as prompt-friendly bullets.
func FormatThreadDecisionDraftsForPrompt(drafts []ThreadDecisionDraft, limit int) string {
	return formatPromptBullets(limitThreadDecisionDrafts(drafts, limit), func(draft ThreadDecisionDraft) string {
		var lines []string
		lines = append(lines, fmt.Sprintf("[%s] %s", draft.Status, draft.Decision))
		if strings.TrimSpace(draft.Rationale) != "" {
			lines = append(lines, fmt.Sprintf("  Rationale: %s", strings.TrimSpace(draft.Rationale)))
		}
		return strings.Join(lines, "\n")
	})
}

// FormatThreadMessagesForPrompt renders recent thread history as prompt-friendly bullets.
func FormatThreadMessagesForPrompt(messages []ThreadMessage, limit int) string {
	return formatPromptBullets(recentThreadMessages(messages, limit), func(message ThreadMessage) string {
		return fmt.Sprintf("%s: %s", message.Role, message.Content)
	})
}

func recentThreadMessages(messages []ThreadMessage, limit int) []ThreadMessage {
	if limit <= 0 || len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func limitThreadLinks(links []ThreadLink, limit int) []ThreadLink {
	if limit <= 0 || len(links) <= limit {
		return links
	}
	return links[:limit]
}

func limitThreadRecommendationDrafts(drafts []ThreadRecommendationDraft, limit int) []ThreadRecommendationDraft {
	if limit <= 0 || len(drafts) <= limit {
		return drafts
	}
	return drafts[:limit]
}

func limitThreadDecisionDrafts(drafts []ThreadDecisionDraft, limit int) []ThreadDecisionDraft {
	if limit <= 0 || len(drafts) <= limit {
		return drafts
	}
	return drafts[:limit]
}

func formatPromptBullets[T any](items []T, render func(T) string) string {
	if len(items) == 0 {
		return ""
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		rendered := strings.TrimSpace(render(item))
		if rendered == "" {
			continue
		}
		lines = append(lines, "- "+rendered)
	}
	return strings.Join(lines, "\n")
}
