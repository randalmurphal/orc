package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseKnowledgeSection(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Repository | Data access abstraction | TASK-001 |
| Factory | Object creation | TASK-002 |

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| SQLite locks | Use WAL mode | TASK-003 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| PostgreSQL | Better concurrency | TASK-004 |
| Redis | Fast caching | TASK-005 |
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	patterns, gotchas, decisions, err := parseKnowledgeSection(tmpDir)
	if err != nil {
		t.Fatalf("parseKnowledgeSection() error = %v", err)
	}

	if len(patterns) != 2 {
		t.Errorf("patterns count = %d, want 2", len(patterns))
	}
	if len(gotchas) != 1 {
		t.Errorf("gotchas count = %d, want 1", len(gotchas))
	}
	if len(decisions) != 2 {
		t.Errorf("decisions count = %d, want 2", len(decisions))
	}

	// Check first pattern
	if len(patterns) > 0 && patterns[0][0] != "Repository" {
		t.Errorf("first pattern name = %q, want 'Repository'", patterns[0][0])
	}
}

func TestCountKnowledgeEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Pattern1 | Desc1 | TASK-001 |
| Pattern2 | Desc2 | TASK-002 |
| Pattern3 | Desc3 | TASK-003 |

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| Gotcha1 | Fix1 | TASK-004 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Decision1 | Reason1 | TASK-005 |
| Decision2 | Reason2 | TASK-006 |
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	patterns, gotchas, decisions := countKnowledgeEntries(tmpDir)

	if patterns != 3 {
		t.Errorf("patterns = %d, want 3", patterns)
	}
	if gotchas != 1 {
		t.Errorf("gotchas = %d, want 1", gotchas)
	}
	if decisions != 2 {
		t.Errorf("decisions = %d, want 2", decisions)
	}
}

func TestFormatFiles(t *testing.T) {
	t.Parallel()
	patterns := [][]string{
		{"Pattern1", "Description1", "TASK-001"},
		{"Pattern2", "Description2", "TASK-002"},
	}

	patternsFile := formatPatternsFile(patterns)
	if !strings.Contains(patternsFile, "# Code Patterns") {
		t.Error("patterns file missing header")
	}
	if !strings.Contains(patternsFile, "Pattern1") {
		t.Error("patterns file missing entry")
	}

	gotchas := [][]string{
		{"Issue1", "Resolution1", "TASK-003"},
	}

	gotchasFile := formatGotchasFile(gotchas)
	if !strings.Contains(gotchasFile, "# Known Gotchas") {
		t.Error("gotchas file missing header")
	}

	decisions := [][]string{
		{"Decision1", "Rationale1", "TASK-004"},
	}

	decisionsFile := formatDecisionsFile(decisions)
	if !strings.Contains(decisionsFile, "# Architectural Decisions") {
		t.Error("decisions file missing header")
	}
}

func TestReplaceKnowledgeSectionWithPointer(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

Some content here.

<!-- orc:knowledge:begin -->
## Project Knowledge

Old knowledge content here.
<!-- orc:knowledge:end -->

More content after.
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	err = replaceKnowledgeSectionWithPointer(tmpDir, 5, 3, 2)
	if err != nil {
		t.Fatalf("replaceKnowledgeSectionWithPointer() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "See [agent_docs/](agent_docs/)") {
		t.Error("pointer not added")
	}
	if !strings.Contains(content, "(5 items)") {
		t.Error("patterns count not correct")
	}
	if !strings.Contains(content, "More content after") {
		t.Error("content after section was lost")
	}
	if strings.Contains(content, "Old knowledge content") {
		t.Error("old content still present")
	}
}

func TestParseTable(t *testing.T) {
	t.Parallel()
	content := `
### Test Table
| Name | Description | Source |
|------|-------------|--------|
| Entry1 | Desc1 | SRC1 |
| Entry2 | Desc2 | SRC2 |

### Another Section
`
	rows := parseTable(content, "### Test Table")

	if len(rows) != 2 {
		t.Errorf("rows count = %d, want 2", len(rows))
	}

	if len(rows) > 0 && rows[0][0] != "Entry1" {
		t.Errorf("first row name = %q, want 'Entry1'", rows[0][0])
	}
}
