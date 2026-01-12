package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// Knowledge section markers in CLAUDE.md
	knowledgeSectionStart = "<!-- orc:knowledge:begin -->"
	knowledgeSectionEnd   = "<!-- orc:knowledge:end -->"
)

// KnowledgeSectionTemplate is the initial empty knowledge section.
// Claude edits this directly during the docs phase - no XML parsing needed.
const KnowledgeSectionTemplate = `## Project Knowledge

Patterns, gotchas, and decisions learned during development.

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
`

// InjectKnowledgeSection adds or updates the knowledge section in CLAUDE.md.
// The section is placed after the orc section (if present) or at the end.
func InjectKnowledgeSection(projectDir string) error {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	// Read existing content or start fresh
	content := ""
	data, err := os.ReadFile(claudeMDPath)
	if err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	// Build the knowledge section
	knowledgeSection := fmt.Sprintf("%s\n%s\n%s", knowledgeSectionStart, KnowledgeSectionTemplate, knowledgeSectionEnd)

	// Check if knowledge section already exists
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(knowledgeSectionStart) + `.*?` + regexp.QuoteMeta(knowledgeSectionEnd))
	if re.MatchString(content) {
		// Section exists - don't overwrite (Claude maintains this)
		return nil
	}

	// Append new section (after orc section if present, otherwise at end)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	content += knowledgeSection + "\n"

	// Write back
	if err := os.WriteFile(claudeMDPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	return nil
}

// RemoveKnowledgeSection removes the knowledge section from CLAUDE.md.
func RemoveKnowledgeSection(projectDir string) error {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	content := string(data)

	// Remove knowledge section
	re := regexp.MustCompile(`(?s)\n?` + regexp.QuoteMeta(knowledgeSectionStart) + `.*?` + regexp.QuoteMeta(knowledgeSectionEnd) + `\n?`)
	content = re.ReplaceAllString(content, "\n")

	// Clean up multiple newlines
	content = regexp.MustCompile(`\n{3,}`).ReplaceAllString(content, "\n\n")
	content = strings.TrimSpace(content)
	if content != "" {
		content += "\n"
	}

	if err := os.WriteFile(claudeMDPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	return nil
}

// HasKnowledgeSection checks if CLAUDE.md contains the knowledge section.
func HasKnowledgeSection(projectDir string) bool {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return false
	}

	return strings.Contains(string(data), knowledgeSectionStart)
}

// KnowledgeSectionLineCount returns the line count of the knowledge section.
// Used to determine if split to agent_docs/ should be suggested.
func KnowledgeSectionLineCount(projectDir string) (int, error) {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read CLAUDE.md: %w", err)
	}

	content := string(data)

	// Extract knowledge section
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(knowledgeSectionStart) + `(.*?)` + regexp.QuoteMeta(knowledgeSectionEnd))
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return 0, nil
	}

	// Count non-empty lines
	lines := strings.Split(matches[1], "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}

	return count, nil
}

// ClaudeMDLineCount returns the total line count of CLAUDE.md.
// Used to determine if split to agent_docs/ should be suggested.
func ClaudeMDLineCount(projectDir string) (int, error) {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read CLAUDE.md: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	return len(lines), nil
}

// ShouldSuggestSplit returns true if CLAUDE.md is getting long (>200 lines).
func ShouldSuggestSplit(projectDir string) (bool, int, error) {
	count, err := ClaudeMDLineCount(projectDir)
	if err != nil {
		return false, 0, err
	}

	return count > 200, count, nil
}
