package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/randalmurphal/orc/internal/task"
)

const (
	// OrcSectionMarker identifies the orc-managed section in CLAUDE.md
	orcSectionStart = "<!-- orc:begin -->"
	orcSectionEnd   = "<!-- orc:end -->"

	// ClaudeMDFile is the filename for Claude instructions
	ClaudeMDFile = "CLAUDE.md"
)

// OrcClaudeMDTemplate is the template for the orc section in CLAUDE.md
const OrcClaudeMDTemplate = `## Orc Orchestration

This project uses [orc](https://github.com/randalmurphal/orc) for task orchestration.

### Slash Commands

| Command | Purpose |
|---------|---------|
| ` + "`/orc:init`" + ` | Initialize project or create spec |
| ` + "`/orc:continue`" + ` | Resume current task |
| ` + "`/orc:status`" + ` | Show progress and next steps |
| ` + "`/orc:review`" + ` | Multi-round code review |
| ` + "`/orc:qa`" + ` | E2E tests and documentation |
| ` + "`/orc:propose`" + ` | Create sub-task for later |

### Task Files

Task specifications and state are stored in ` + "`.orc/tasks/`" + `:

` + "```" + `
.orc/tasks/TASK-001/
├── task.yaml      # Task metadata
├── spec.md        # Task specification
├── plan.yaml      # Phase sequence
└── state.yaml     # Execution state
` + "```" + `

### CLI Commands

` + "```bash" + `
orc status           # View active tasks
orc run TASK-001     # Execute task
orc pause TASK-001   # Pause execution
orc resume TASK-001  # Continue task
` + "```" + `

See ` + "`.orc/`" + ` for configuration and task details.
`

// InjectOrcSection adds or updates the orc section in CLAUDE.md
func InjectOrcSection(projectDir string) error {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	// Read existing content or start fresh
	content := ""
	data, err := os.ReadFile(claudeMDPath)
	if err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	// Build the orc section
	orcSection := fmt.Sprintf("%s\n%s\n%s", orcSectionStart, OrcClaudeMDTemplate, orcSectionEnd)

	// Check if orc section already exists
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(orcSectionStart) + `.*?` + regexp.QuoteMeta(orcSectionEnd))
	if re.MatchString(content) {
		// Update existing section
		content = re.ReplaceAllString(content, orcSection)
	} else {
		// Append new section
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if content != "" {
			content += "\n"
		}
		content += orcSection + "\n"
	}

	// Write back
	if err := os.WriteFile(claudeMDPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write CLAUDE.md: %w", err)
	}

	return nil
}

// RemoveOrcSection removes the orc section from CLAUDE.md
func RemoveOrcSection(projectDir string) error {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	content := string(data)

	// Remove orc section
	re := regexp.MustCompile(`(?s)\n?` + regexp.QuoteMeta(orcSectionStart) + `.*?` + regexp.QuoteMeta(orcSectionEnd) + `\n?`)
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

// HasOrcSection checks if CLAUDE.md contains the orc section
func HasOrcSection(projectDir string) bool {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return false
	}

	return strings.Contains(string(data), orcSectionStart)
}

// UpdateTaskContext updates the current task context in CLAUDE.md (optional feature)
// This can be used to show the current active task in CLAUDE.md
func UpdateTaskContext(projectDir string, activeTask *task.Task) error {
	if activeTask == nil {
		return nil
	}

	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return nil // Skip if CLAUDE.md doesn't exist
	}

	content := string(data)

	// Build context section
	contextSection := fmt.Sprintf(`### Current Task

- **Task**: %s
- **Title**: %s
- **Phase**: %s
- **Status**: %s
`, activeTask.ID, activeTask.Title, activeTask.CurrentPhase, activeTask.Status)

	// Check if context section exists
	contextStart := "<!-- orc:context:begin -->"
	contextEnd := "<!-- orc:context:end -->"
	contextFull := fmt.Sprintf("%s\n%s%s", contextStart, contextSection, contextEnd)

	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(contextStart) + `.*?` + regexp.QuoteMeta(contextEnd))
	if re.MatchString(content) {
		content = re.ReplaceAllString(content, contextFull)
	} else {
		// Insert after orc section start if it exists
		insertRe := regexp.MustCompile(regexp.QuoteMeta(orcSectionStart) + `\n`)
		if insertRe.MatchString(content) {
			content = insertRe.ReplaceAllString(content, orcSectionStart+"\n"+contextFull+"\n\n")
		}
	}

	return os.WriteFile(claudeMDPath, []byte(content), 0644)
}

// ClearTaskContext removes the task context section from CLAUDE.md
func ClearTaskContext(projectDir string) error {
	claudeMDPath := filepath.Join(projectDir, ClaudeMDFile)
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return nil
	}

	content := string(data)

	contextStart := "<!-- orc:context:begin -->"
	contextEnd := "<!-- orc:context:end -->"
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(contextStart) + `.*?` + regexp.QuoteMeta(contextEnd) + `\n*`)
	content = re.ReplaceAllString(content, "")

	return os.WriteFile(claudeMDPath, []byte(content), 0644)
}
