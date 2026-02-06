package brief

import (
	"fmt"
	"strings"
)

// categoryDisplayNames maps category IDs to display names for section headers.
var categoryDisplayNames = map[string]string{
	CategoryDecisions:      "Decisions",
	CategoryRecentFindings: "Recent Findings",
	CategoryHotFiles:       "Hot Files",
	CategoryPatterns:       "Patterns",
	CategoryKnownIssues:    "Known Issues",
}

// FormatBrief renders a brief as structured markdown text.
// Returns empty string if the brief has no entries.
func FormatBrief(b *Brief) string {
	if b == nil {
		return ""
	}

	// Check if there are any entries at all
	totalEntries := 0
	for _, s := range b.Sections {
		totalEntries += len(s.Entries)
	}
	if totalEntries == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Project Brief\n\n")

	for _, s := range b.Sections {
		if len(s.Entries) == 0 {
			continue
		}

		displayName := categoryDisplayNames[s.Category]
		if displayName == "" {
			displayName = s.Category
		}
		sb.WriteString(fmt.Sprintf("### %s\n", displayName))

		for _, e := range s.Entries {
			sb.WriteString(fmt.Sprintf("- %s [%s]\n", e.Content, e.Source))
		}
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n") + "\n"
}
