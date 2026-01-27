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

func TestWriteKnowledgeToClaudeMD_SinglePattern(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create CLAUDE.md with empty knowledge section
	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	// Write a pattern entry
	entry := &knowledgeEntryForWrite{
		Type:        "pattern",
		Name:        "Repository Pattern",
		Description: "Data access abstraction layer",
		SourceTask:  "TASK-001",
	}

	err = writeKnowledgeToClaudeMD(tmpDir, entry)
	if err != nil {
		t.Fatalf("writeKnowledgeToClaudeMD() error = %v", err)
	}

	// Verify the entry was written
	data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	content := string(data)

	if !strings.Contains(content, "Repository Pattern") {
		t.Error("pattern name not written")
	}
	if !strings.Contains(content, "Data access abstraction layer") {
		t.Error("pattern description not written")
	}
	if !strings.Contains(content, "TASK-001") {
		t.Error("pattern source not written")
	}
}

func TestWriteKnowledgeToClaudeMD_SingleGotcha(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	entry := &knowledgeEntryForWrite{
		Type:        "gotcha",
		Name:        "SQLite locks",
		Description: "Use WAL mode",
		SourceTask:  "TASK-002",
	}

	err = writeKnowledgeToClaudeMD(tmpDir, entry)
	if err != nil {
		t.Fatalf("writeKnowledgeToClaudeMD() error = %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	content := string(data)

	if !strings.Contains(content, "SQLite locks") {
		t.Error("gotcha name not written")
	}
	if !strings.Contains(content, "Use WAL mode") {
		t.Error("gotcha resolution not written")
	}
}

func TestWriteKnowledgeToClaudeMD_SingleDecision(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	entry := &knowledgeEntryForWrite{
		Type:        "decision",
		Name:        "Use PostgreSQL",
		Description: "Better concurrency than SQLite",
		SourceTask:  "TASK-003",
	}

	err = writeKnowledgeToClaudeMD(tmpDir, entry)
	if err != nil {
		t.Fatalf("writeKnowledgeToClaudeMD() error = %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	content := string(data)

	if !strings.Contains(content, "Use PostgreSQL") {
		t.Error("decision name not written")
	}
	if !strings.Contains(content, "Better concurrency than SQLite") {
		t.Error("decision rationale not written")
	}
}

func TestWriteKnowledgeToClaudeMD_MultipleEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	entries := []*knowledgeEntryForWrite{
		{Type: "pattern", Name: "Pattern1", Description: "Desc1", SourceTask: "TASK-001"},
		{Type: "gotcha", Name: "Gotcha1", Description: "Fix1", SourceTask: "TASK-002"},
		{Type: "decision", Name: "Decision1", Description: "Reason1", SourceTask: "TASK-003"},
	}

	err = writeMultipleKnowledgeToClaudeMD(tmpDir, entries)
	if err != nil {
		t.Fatalf("writeMultipleKnowledgeToClaudeMD() error = %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	content := string(data)

	if !strings.Contains(content, "Pattern1") {
		t.Error("pattern not written")
	}
	if !strings.Contains(content, "Gotcha1") {
		t.Error("gotcha not written")
	}
	if !strings.Contains(content, "Decision1") {
		t.Error("decision not written")
	}
}

func TestWriteKnowledgeToClaudeMD_NoKnowledgeSection(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// CLAUDE.md without knowledge section
	claudeMD := `# Project

Just some content without knowledge section.
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	entry := &knowledgeEntryForWrite{
		Type:        "pattern",
		Name:        "Test",
		Description: "Test",
		SourceTask:  "TASK-001",
	}

	err = writeKnowledgeToClaudeMD(tmpDir, entry)
	if err == nil {
		t.Error("expected error when knowledge section is missing")
	}
}

func TestWriteKnowledgeToClaudeMD_PreservesExistingEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// CLAUDE.md with existing entries
	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Existing | Already here | TASK-OLD |

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	entry := &knowledgeEntryForWrite{
		Type:        "pattern",
		Name:        "New Pattern",
		Description: "New description",
		SourceTask:  "TASK-NEW",
	}

	err = writeKnowledgeToClaudeMD(tmpDir, entry)
	if err != nil {
		t.Fatalf("writeKnowledgeToClaudeMD() error = %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	content := string(data)

	// Both old and new entries should exist
	if !strings.Contains(content, "Existing") {
		t.Error("existing entry was removed")
	}
	if !strings.Contains(content, "New Pattern") {
		t.Error("new entry was not added")
	}
}

func TestWriteKnowledgeToClaudeMD_EscapesPipes(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	claudeMD := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(claudeMD), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	// Entry with pipe character that needs escaping
	entry := &knowledgeEntryForWrite{
		Type:        "pattern",
		Name:        "A | B",
		Description: "Use A or B | but not C",
		SourceTask:  "TASK-001",
	}

	err = writeKnowledgeToClaudeMD(tmpDir, entry)
	if err != nil {
		t.Fatalf("writeKnowledgeToClaudeMD() error = %v", err)
	}

	// Verify table is still parseable (pipes escaped)
	patterns, _, _, _ := parseKnowledgeSection(tmpDir)
	if len(patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(patterns))
	}
}
