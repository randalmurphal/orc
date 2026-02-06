package executor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/variable"
)

// scratchpadRawEntry represents a raw scratchpad entry from phase JSON output.
type scratchpadRawEntry struct {
	Category string `json:"category"`
	Content  string `json:"content"`
}

// scratchpadOutput represents the scratchpad portion of phase JSON output.
type scratchpadOutput struct {
	Scratchpad []scratchpadRawEntry `json:"scratchpad"`
}

// ExtractScratchpadEntries parses the "scratchpad" array from phase JSON output.
// Returns extracted entries with Category and Content populated.
// Entries missing category or content are silently skipped.
// Invalid JSON returns empty slice and no error (defensive extraction).
func ExtractScratchpadEntries(output string) ([]scratchpadRawEntry, error) {
	var parsed scratchpadOutput
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return []scratchpadRawEntry{}, nil
	}

	var entries []scratchpadRawEntry
	for _, raw := range parsed.Scratchpad {
		if raw.Category == "" || raw.Content == "" {
			continue
		}
		entries = append(entries, raw)
	}

	if entries == nil {
		entries = []scratchpadRawEntry{}
	}
	return entries, nil
}

// FormatScratchpadMarkdown formats scratchpad entries as categorized markdown.
// Groups entries by category with ## headings and bullet points.
// Returns empty string if no entries.
func FormatScratchpadMarkdown(entries []storage.ScratchpadEntry) string {
	if len(entries) == 0 {
		return ""
	}

	// Group by category, maintaining insertion order
	type categoryGroup struct {
		name    string
		entries []string
	}
	var groups []categoryGroup
	seen := map[string]int{}

	for _, e := range entries {
		title := formatCategoryTitle(e.Category)
		idx, ok := seen[title]
		if !ok {
			idx = len(groups)
			seen[title] = idx
			groups = append(groups, categoryGroup{name: title})
		}
		groups[idx].entries = append(groups[idx].entries, e.Content)
	}

	var sb strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&sb, "## %s\n\n", g.name)
		for _, content := range g.entries {
			fmt.Fprintf(&sb, "- %s\n", content)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// populateScratchpadContext loads scratchpad entries from the database
// and populates the resolution context with PREV_SCRATCHPAD and RETRY_SCRATCHPAD.
func (we *WorkflowExecutor) populateScratchpadContext(rctx *variable.ResolutionContext, taskID, phaseID string) {
	// Load all entries for the task to build PREV_SCRATCHPAD
	allEntries, err := we.backend.GetScratchpadEntries(taskID)
	if err != nil {
		we.logger.Warn("failed to load scratchpad entries", "task", taskID, "error", err)
		return
	}

	// PREV_SCRATCHPAD: entries from phases OTHER than the current phase
	var prevEntries []storage.ScratchpadEntry
	for _, e := range allEntries {
		if e.PhaseID != phaseID {
			prevEntries = append(prevEntries, e)
		}
	}
	rctx.PrevScratchpad = FormatScratchpadMarkdown(prevEntries)

	// RETRY_SCRATCHPAD: entries from prior attempt of current phase (only on retry)
	if rctx.RetryAttempt > 1 {
		priorAttempt := rctx.RetryAttempt - 1
		retryEntries, retryErr := we.backend.GetScratchpadEntriesByAttempt(taskID, phaseID, priorAttempt)
		if retryErr != nil {
			we.logger.Warn("failed to load retry scratchpad entries", "task", taskID, "phase", phaseID, "error", retryErr)
			return
		}
		rctx.RetryScratchpad = FormatScratchpadMarkdown(retryEntries)
	}
}

// persistScratchpadEntries extracts scratchpad entries from phase output
// and saves them to the database. Errors are logged but do not fail the phase.
func (we *WorkflowExecutor) persistScratchpadEntries(taskID, phaseID string, attempt int, phaseOutput string) {
	rawEntries, err := ExtractScratchpadEntries(phaseOutput)
	if err != nil {
		we.logger.Warn("failed to extract scratchpad entries", "task", taskID, "phase", phaseID, "error", err)
		return
	}

	if len(rawEntries) == 0 {
		return
	}

	// Use attempt 1 as default when not in retry mode
	if attempt <= 0 {
		attempt = 1
	}

	for _, raw := range rawEntries {
		entry := &storage.ScratchpadEntry{
			TaskID:   taskID,
			PhaseID:  phaseID,
			Category: raw.Category,
			Content:  raw.Content,
			Attempt:  attempt,
		}
		if err := we.backend.SaveScratchpadEntry(entry); err != nil {
			we.logger.Warn("failed to save scratchpad entry",
				"task", taskID,
				"phase", phaseID,
				"category", raw.Category,
				"error", err,
			)
		}
	}

	we.logger.Info("persisted scratchpad entries",
		"task", taskID,
		"phase", phaseID,
		"count", len(rawEntries),
	)
}

// formatCategoryTitle converts a category slug to a title.
func formatCategoryTitle(category string) string {
	switch category {
	case "observation":
		return "Observations"
	case "decision":
		return "Decisions"
	case "blocker":
		return "Blockers"
	case "todo":
		return "Todos"
	case "warning":
		return "Warnings"
	default:
		// Capitalize first letter of unknown categories
		if len(category) == 0 {
			return "Notes"
		}
		return strings.ToUpper(category[:1]) + category[1:]
	}
}
