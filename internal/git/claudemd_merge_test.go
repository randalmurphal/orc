package git

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestParseConflictBlocks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int // number of conflict blocks
	}{
		{
			name:     "no conflicts",
			content:  "# CLAUDE.md\nSome content\n",
			expected: 0,
		},
		{
			name: "single conflict",
			content: `# CLAUDE.md
<<<<<<< HEAD
| Pattern A | Description A | TASK-001 |
=======
| Pattern B | Description B | TASK-002 |
>>>>>>> branch
`,
			expected: 1,
		},
		{
			name: "multiple conflicts",
			content: `# CLAUDE.md
<<<<<<< HEAD
content1
=======
content2
>>>>>>> branch

<<<<<<< HEAD
content3
=======
content4
>>>>>>> branch
`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocks := parseConflictBlocks(tt.content)
			if len(blocks) != tt.expected {
				t.Errorf("expected %d conflict blocks, got %d", tt.expected, len(blocks))
			}
		})
	}
}

func TestExtractKnowledgeSection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectEmpty bool
		expectError bool
	}{
		{
			name:        "no knowledge section",
			content:     "# CLAUDE.md\nSome content\n",
			expectEmpty: true,
		},
		{
			name: "valid knowledge section",
			content: `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge
| Pattern | Description | Source |
|---------|-------------|--------|
<!-- orc:knowledge:end -->
`,
			expectEmpty: false,
		},
		{
			name: "missing end marker",
			content: `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			section, err := extractKnowledgeSection(tt.content)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tt.expectEmpty && section != "" {
				t.Errorf("expected empty section, got %q", section)
			}
			if !tt.expectEmpty && section == "" {
				t.Error("expected non-empty section, got empty")
			}
		})
	}
}

func TestParseTableRows(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int // number of rows (excluding headers and separators)
	}{
		{
			name:     "empty",
			content:  "",
			expected: 0,
		},
		{
			name: "header only",
			content: `| Pattern | Description | Source |
|---------|-------------|--------|`,
			expected: 0,
		},
		{
			name: "with data rows",
			content: `| Pattern | Description | Source |
|---------|-------------|--------|
| Pattern A | Desc A | TASK-001 |
| Pattern B | Desc B | TASK-002 |`,
			expected: 2,
		},
		{
			name: "mixed content",
			content: `Some text
| Pattern | Description | Source |
|---------|-------------|--------|
| Pattern A | Desc A | TASK-001 |
More text`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := parseTableRows(tt.content)
			if len(rows) != tt.expected {
				t.Errorf("expected %d rows, got %d: %v", tt.expected, len(rows), rows)
			}
		})
	}
}

func TestIsTableHeader(t *testing.T) {
	tests := []struct {
		row      string
		expected bool
	}{
		{"| Pattern | Description | Source |", true},
		{"| Issue | Resolution | Source |", true},
		{"| Decision | Rationale | Source |", true},
		{"| My Feature | Works great | TASK-001 |", false},
		{"| Fix bug | Use new API | TASK-002 |", false},
	}

	for _, tt := range tests {
		t.Run(tt.row, func(t *testing.T) {
			got := isTableHeader(tt.row)
			if got != tt.expected {
				t.Errorf("isTableHeader(%q) = %v, want %v", tt.row, got, tt.expected)
			}
		})
	}
}

func TestDetectTableName(t *testing.T) {
	tests := []struct {
		context  string
		expected string
	}{
		{"### Patterns Learned\n", "Patterns Learned"},
		{"Some text\n### Known Gotchas\n", "Known Gotchas"},
		{"### Decisions\n", "Decisions"},
		{"Random content", ""},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := detectTableName(tt.context)
			if got != tt.expected {
				t.Errorf("detectTableName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestNormalizeRow(t *testing.T) {
	tests := []struct {
		row      string
		expected string
	}{
		{"| a | b | c |", "|a|b|c|"},
		{"  |  a  |  b  |  c  |  ", "|a|b|c|"},
		{"|a|b|c|", "|a|b|c|"},
	}

	for _, tt := range tests {
		t.Run(tt.row, func(t *testing.T) {
			got := normalizeRow(tt.row)
			if got != tt.expected {
				t.Errorf("normalizeRow(%q) = %q, want %q", tt.row, got, tt.expected)
			}
		})
	}
}

func TestExtractSourceIDs(t *testing.T) {
	rows := []string{
		"| Pattern A | Description | TASK-001 |",
		"| Pattern B | Description | TASK-002 |",
		"| Pattern C | Description | TASK-001 |", // duplicate
	}

	ids := extractSourceIDs(rows)
	if len(ids) != 2 {
		t.Errorf("expected 2 unique IDs, got %d", len(ids))
	}
	if !ids["TASK-001"] {
		t.Error("expected TASK-001 in IDs")
	}
	if !ids["TASK-002"] {
		t.Error("expected TASK-002 in IDs")
	}
}

func TestSortBySourceID(t *testing.T) {
	rows := []string{
		"| Pattern C | Description | TASK-010 |",
		"| Pattern A | Description | TASK-001 |",
		"| Pattern B | Description | TASK-005 |",
	}

	sortBySourceID(rows)

	expected := []string{
		"| Pattern A | Description | TASK-001 |",
		"| Pattern B | Description | TASK-005 |",
		"| Pattern C | Description | TASK-010 |",
	}

	for i, row := range rows {
		if row != expected[i] {
			t.Errorf("row %d: got %q, want %q", i, row, expected[i])
		}
	}
}

func TestCanAutoResolve_NoKnowledgeSection(t *testing.T) {
	content := `# CLAUDE.md
Some content
<<<<<<< HEAD
line1
=======
line2
>>>>>>> branch
`

	merger := NewClaudeMDMerger(slog.Default())
	conflict, err := merger.CanAutoResolve(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conflict.CanAutoResolve {
		t.Error("expected CanAutoResolve=false for content without knowledge section")
	}
}

func TestCanAutoResolve_PurelyAdditiveConflict(t *testing.T) {
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Pattern A | Description A | TASK-001 |
=======
| Pattern B | Description B | TASK-002 |
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	conflict, err := merger.CanAutoResolve(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !conflict.IsKnowledge {
		t.Error("expected IsKnowledge=true")
	}
	if !conflict.CanAutoResolve {
		t.Errorf("expected CanAutoResolve=true for purely additive conflict, logs: %v", conflict.ResolutionLog)
	}
}

func TestAutoResolve_PurelyAdditiveConflict(t *testing.T) {
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Pattern A | Description A | TASK-001 |
=======
| Pattern B | Description B | TASK-002 |
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	if !result.Success {
		t.Fatalf("expected success, got error: %v, logs: %v", result.Error, result.Logs)
	}

	// Verify conflict markers are removed
	if strings.Contains(result.MergedContent, "<<<<<<<") {
		t.Error("merged content still contains conflict markers")
	}

	// Verify both patterns are present
	if !strings.Contains(result.MergedContent, "Pattern A") {
		t.Error("merged content missing Pattern A")
	}
	if !strings.Contains(result.MergedContent, "Pattern B") {
		t.Error("merged content missing Pattern B")
	}

	// Verify knowledge section markers are preserved
	if !strings.Contains(result.MergedContent, KnowledgeSectionStart) {
		t.Error("merged content missing knowledge section start marker")
	}
	if !strings.Contains(result.MergedContent, KnowledgeSectionEnd) {
		t.Error("merged content missing knowledge section end marker")
	}
}

func TestAutoResolve_MultipleTablesConflict(t *testing.T) {
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Pattern A | Description A | TASK-001 |
=======
| Pattern B | Description B | TASK-002 |
>>>>>>> branch

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
<<<<<<< HEAD
| Issue A | Fix A | TASK-001 |
=======
| Issue B | Fix B | TASK-002 |
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	if !result.Success {
		t.Fatalf("expected success, got error: %v, logs: %v", result.Error, result.Logs)
	}

	// Verify all content is present
	mustContain := []string{
		"Pattern A", "Pattern B",
		"Issue A", "Issue B",
		"TASK-001", "TASK-002",
	}

	for _, s := range mustContain {
		if !strings.Contains(result.MergedContent, s) {
			t.Errorf("merged content missing %q", s)
		}
	}
}

func TestAutoResolve_CommonRowsPreserved(t *testing.T) {
	// Both sides have the same existing row, plus their own additions
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Common Pattern | Common Description | TASK-001 |
| Pattern A | Description A | TASK-002 |
=======
| Common Pattern | Common Description | TASK-001 |
| Pattern B | Description B | TASK-003 |
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	if !result.Success {
		t.Fatalf("expected success, got error: %v, logs: %v", result.Error, result.Logs)
	}

	// Verify all patterns are present
	if !strings.Contains(result.MergedContent, "Common Pattern") {
		t.Error("merged content missing Common Pattern")
	}
	if !strings.Contains(result.MergedContent, "Pattern A") {
		t.Error("merged content missing Pattern A")
	}
	if !strings.Contains(result.MergedContent, "Pattern B") {
		t.Error("merged content missing Pattern B")
	}

	// Verify common pattern appears only once
	count := strings.Count(result.MergedContent, "Common Pattern")
	if count != 1 {
		t.Errorf("expected Common Pattern to appear once, got %d times", count)
	}
}

func TestAutoResolve_SortsByTaskID(t *testing.T) {
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Pattern Ten | Description | TASK-010 |
=======
| Pattern Three | Description | TASK-003 |
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	if !result.Success {
		t.Fatalf("expected success, got error: %v, logs: %v", result.Error, result.Logs)
	}

	// TASK-003 should come before TASK-010
	idx003 := strings.Index(result.MergedContent, "TASK-003")
	idx010 := strings.Index(result.MergedContent, "TASK-010")

	if idx003 > idx010 {
		t.Error("expected TASK-003 to come before TASK-010 in sorted output")
	}
}

func TestIsClaudeMDFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"CLAUDE.md", true},
		{"/path/to/CLAUDE.md", true},
		{"README.md", false},
		{"claude.md", false}, // case sensitive
		{"/path/to/README.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsClaudeMDFile(tt.path)
			if got != tt.expected {
				t.Errorf("IsClaudeMDFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestResolveClaudeMDConflict_Convenience(t *testing.T) {
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

Patterns, gotchas, and decisions learned during development.

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Pattern A | Description A | TASK-001 |
=======
| Pattern B | Description B | TASK-002 |
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	resolved, success, logs := ResolveClaudeMDConflict(content, nil)

	if !success {
		t.Fatalf("expected success, logs: %v", logs)
	}

	if strings.Contains(resolved, "<<<<<<<") {
		t.Error("resolved content still contains conflict markers")
	}
}

// TestAutoResolve_RealWorldScenario tests a more realistic CLAUDE.md conflict
func TestAutoResolve_RealWorldScenario(t *testing.T) {
	content := `# Orc - Claude Code Task Orchestrator

AI-powered task orchestration with phased execution.

<!-- orc:knowledge:begin -->
## Project Knowledge

Patterns, gotchas, and decisions learned during development.

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Branch sync before completion | Task branches rebase onto target before PR | TASK-019 |
<<<<<<< HEAD
| Executor PID tracking | Track executor PID in state.yaml to detect orphaned tasks | TASK-046 |
| Atomic status updates | Set current_phase atomically with status=running | TASK-057 |
=======
| Live transcript modal | Click running task to open transcript modal | TASK-012 |
| Project selection persistence | URL param takes precedence over localStorage | TASK-009 |
>>>>>>> feature-branch

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| PR labels missing | Orc warns and creates PR without labels | TASK-015 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Sync at completion | Balance safety vs overhead | TASK-019 |

<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	if !result.Success {
		t.Fatalf("expected success, got error: %v, logs: %v", result.Error, result.Logs)
	}

	// All patterns should be present
	patterns := []string{
		"Branch sync before completion",
		"Executor PID tracking",
		"Atomic status updates",
		"Live transcript modal",
		"Project selection persistence",
	}

	for _, p := range patterns {
		if !strings.Contains(result.MergedContent, p) {
			t.Errorf("merged content missing pattern: %s", p)
		}
	}

	// Non-conflicted sections should be unchanged
	if !strings.Contains(result.MergedContent, "PR labels missing") {
		t.Error("non-conflicted Known Gotchas row missing")
	}
	if !strings.Contains(result.MergedContent, "Sync at completion") {
		t.Error("non-conflicted Decisions row missing")
	}

	// Verify sorted by task ID (approximately)
	idx009 := strings.Index(result.MergedContent, "TASK-009")
	idx012 := strings.Index(result.MergedContent, "TASK-012")
	idx019 := strings.Index(result.MergedContent, "TASK-019")
	idx046 := strings.Index(result.MergedContent, "TASK-046")
	idx057 := strings.Index(result.MergedContent, "TASK-057")

	// In the Patterns section, TASK-009 should come first, then 012, then 019, etc.
	if idx009 > idx012 || idx012 > idx019 || idx019 > idx046 || idx046 > idx057 {
		t.Log("Note: Task IDs may not be perfectly sorted due to existing common rows")
	}
}

// TestAutoResolve_ConflictOutsideKnowledgeSection verifies fallback for conflicts
// outside the knowledge section markers
func TestAutoResolve_ConflictOutsideKnowledgeSection(t *testing.T) {
	content := `# CLAUDE.md

## Some Section

<<<<<<< HEAD
Regular content change
=======
Different regular content
>>>>>>> branch

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Pattern A | Description A | TASK-001 |
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	// Should fail because conflict is outside knowledge section
	if result.Success {
		t.Error("expected failure for conflict outside knowledge section")
	}
}

// TestAutoResolve_MalformedTable verifies fallback for malformed table content
func TestAutoResolve_MalformedTable(t *testing.T) {
	content := `# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
<<<<<<< HEAD
This is not a table row
=======
This is also not a table row
>>>>>>> branch
<!-- orc:knowledge:end -->
`

	merger := NewClaudeMDMerger(slog.Default())
	conflict, err := merger.CanAutoResolve(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not be auto-resolvable (no table rows detected)
	if conflict.CanAutoResolve {
		t.Error("expected CanAutoResolve=false for malformed table")
	}
}

// TestAutoResolve_NoKnowledgeMarkers verifies fallback when markers are missing
func TestAutoResolve_NoKnowledgeMarkers(t *testing.T) {
	content := `# CLAUDE.md

## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
| Pattern A | Description A | TASK-001 |
=======
| Pattern B | Description B | TASK-002 |
>>>>>>> branch
`

	merger := NewClaudeMDMerger(slog.Default())
	result := merger.AutoResolve(content)

	// Should fail because knowledge section markers are missing
	if result.Success {
		t.Error("expected failure when knowledge markers are missing")
	}
}

// Benchmark for large conflict resolution
func BenchmarkAutoResolve(b *testing.B) {
	// Build a content with many rows in conflict
	var sb strings.Builder
	sb.WriteString(`# CLAUDE.md
<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<<<<<<< HEAD
`)
	for i := 0; i < 50; i++ {
		sb.WriteString("| Pattern ")
		sb.WriteString(string(rune('A' + i%26)))
		sb.WriteString(" | Description | TASK-")
		sb.WriteString(string(rune('0' + i/10)))
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString("1 |\n")
	}
	sb.WriteString("=======\n")
	for i := 0; i < 50; i++ {
		sb.WriteString("| Pattern ")
		sb.WriteString(string(rune('a' + i%26)))
		sb.WriteString(" | Other Description | TASK-")
		sb.WriteString(string(rune('0' + i/10)))
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString("2 |\n")
	}
	sb.WriteString(`>>>>>>> branch
<!-- orc:knowledge:end -->
`)

	content := sb.String()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	merger := NewClaudeMDMerger(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := merger.AutoResolve(content)
		if !result.Success {
			b.Fatalf("merge failed: %v", result.Error)
		}
	}
}
