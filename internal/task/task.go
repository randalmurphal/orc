// Package task provides task management for orc.
// Note: File I/O functions have been removed. Use storage.Backend for persistence.
package task

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/project"
)

const (
	// OrcDir is the default orc configuration directory
	OrcDir = ".orc"
	// ExportsDir is the subdirectory for exports
	ExportsDir = "exports"
)

// ExportPath returns the export directory path.
// Resolves to ~/.orc/projects/<id>/exports/ for registered projects.
// Falls back to <projectDir>/.orc/exports/ for unregistered projects.
func ExportPath(projectDir string) string {
	projectID, err := project.ResolveProjectID(projectDir)
	if err == nil {
		exportDir, err := project.ProjectExportDir(projectID)
		if err == nil {
			return exportDir
		}
	}
	return filepath.Join(projectDir, OrcDir, ExportsDir)
}

// uiKeywords contains words that suggest a task involves UI work.
// These are used to auto-detect tasks that require UI testing.
// NOTE: These are matched as whole words (word boundaries), not substrings.
// For example, "form" matches "form" but not "information" or "transform".
var uiKeywords = []string{
	// UI framework/component terms
	"frontend", "button", "form", "modal", "dialog",
	"component", "widget", "layout", "sidebar", "header", "footer",
	"dashboard", "navbar", "toolbar",
	// Form elements
	"input", "dropdown", "select", "checkbox", "radio",
	"textarea", "datepicker",
	// UI feedback elements
	"tooltip", "popover", "toast", "notification", "alert",
	"spinner", "loader", "progress bar",
	// Visual/styling terms
	"css", "stylesheet", "responsive", "dark mode", "light mode",
	"animation", "transition", "theme",
	// Accessibility
	"a11y", "screen reader", "keyboard navigation", "aria",
	// Specific UI interaction patterns (explicit, not generic verbs)
	"drag and drop", "click handler", "onclick", "hover state",
}

// uiKeywordPattern is a compiled regex for matching UI keywords as whole words.
// Built from uiKeywords at init time.
var uiKeywordPattern *regexp.Regexp

// visualKeywords contains words that suggest visual/design testing is needed.
var visualKeywords = []string{
	"visual", "design", "style", "css", "theme", "layout", "responsive",
	"screenshot", "pixel", "color", "colour", "font", "typography",
}

// visualKeywordPattern is a compiled regex for matching visual keywords as whole words.
var visualKeywordPattern *regexp.Regexp

// buildKeywordPattern creates a case-insensitive word-boundary regex from keywords.
func buildKeywordPattern(keywords []string) *regexp.Regexp {
	// Sort by length descending so longer phrases match first
	sorted := make([]string, len(keywords))
	copy(sorted, keywords)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) > len(sorted[j])
	})

	// Escape special regex characters and join with |
	escaped := make([]string, len(sorted))
	for i, kw := range sorted {
		escaped[i] = regexp.QuoteMeta(kw)
	}
	pattern := `\b(` + strings.Join(escaped, "|") + `)\b`
	return regexp.MustCompile("(?i)" + pattern)
}

func init() {
	uiKeywordPattern = buildKeywordPattern(uiKeywords)
	visualKeywordPattern = buildKeywordPattern(visualKeywords)
}

// DetectUITesting checks if a task description suggests UI testing is needed.
// Returns true if the title or description contains UI-related keywords.
// Keywords are matched as whole words to avoid false positives
// (e.g., "form" matches but "information" does not).
func DetectUITesting(title, description string) bool {
	text := title + " " + description
	return uiKeywordPattern.MatchString(text)
}



// taskRefPattern matches TASK-XXX patterns (at least 3 digits).
var taskRefPattern = regexp.MustCompile(`\bTASK-\d{3,}\b`)

// DetectTaskReferences scans text for TASK-XXX patterns and returns unique matches.
// Returns a sorted, deduplicated list of task IDs found in the text.
func DetectTaskReferences(text string) []string {
	matches := taskRefPattern.FindAllString(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// Deduplicate and sort
	seen := make(map[string]bool)
	var unique []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	sort.Strings(unique)
	return unique
}




