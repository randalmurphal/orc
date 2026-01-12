package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectKnowledgeSection(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "empty CLAUDE.md",
			existingContent: "",
			wantContains: []string{
				knowledgeSectionStart,
				knowledgeSectionEnd,
				"## Project Knowledge",
				"### Patterns Learned",
				"### Known Gotchas",
				"### Decisions",
			},
		},
		{
			name:            "existing content without knowledge section",
			existingContent: "# My Project\n\nSome existing content.\n",
			wantContains: []string{
				"# My Project",
				"Some existing content.",
				knowledgeSectionStart,
				"## Project Knowledge",
			},
		},
		{
			name: "existing knowledge section should not be overwritten",
			existingContent: `# My Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Existing pattern | Already here | TASK-001 |
<!-- orc:knowledge:end -->
`,
			wantContains: []string{
				"Existing pattern",
				"Already here",
				"TASK-001",
			},
			wantNotContains: []string{
				// Should not duplicate the section
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create existing CLAUDE.md if needed
			if tt.existingContent != "" {
				err := os.WriteFile(filepath.Join(tmpDir, ClaudeMDFile), []byte(tt.existingContent), 0644)
				if err != nil {
					t.Fatalf("failed to create CLAUDE.md: %v", err)
				}
			}

			// Inject knowledge section
			err := InjectKnowledgeSection(tmpDir)
			if err != nil {
				t.Fatalf("InjectKnowledgeSection() error = %v", err)
			}

			// Read result
			data, err := os.ReadFile(filepath.Join(tmpDir, ClaudeMDFile))
			if err != nil {
				t.Fatalf("failed to read CLAUDE.md: %v", err)
			}
			content := string(data)

			// Check expected content
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("content should contain %q", want)
				}
			}

			// Check unexpected content
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("content should not contain %q", notWant)
				}
			}

			// Verify only one knowledge section exists
			count := strings.Count(content, knowledgeSectionStart)
			if count != 1 {
				t.Errorf("expected exactly 1 knowledge section start marker, got %d", count)
			}
		})
	}
}

func TestRemoveKnowledgeSection(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name: "remove knowledge section",
			existingContent: `# My Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
<!-- orc:knowledge:end -->

## Other Content
`,
			wantContains: []string{
				"# My Project",
				"## Other Content",
			},
			wantNotContains: []string{
				knowledgeSectionStart,
				knowledgeSectionEnd,
				"## Project Knowledge",
			},
		},
		{
			name:            "no knowledge section to remove",
			existingContent: "# My Project\n\nSome content.\n",
			wantContains: []string{
				"# My Project",
				"Some content.",
			},
		},
		{
			name:            "no file",
			existingContent: "",
			// Should not error, just return nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create existing CLAUDE.md if needed
			if tt.existingContent != "" {
				err := os.WriteFile(filepath.Join(tmpDir, ClaudeMDFile), []byte(tt.existingContent), 0644)
				if err != nil {
					t.Fatalf("failed to create CLAUDE.md: %v", err)
				}
			}

			// Remove knowledge section
			err := RemoveKnowledgeSection(tmpDir)
			if err != nil {
				t.Fatalf("RemoveKnowledgeSection() error = %v", err)
			}

			// Skip file checks if no file was created
			if tt.existingContent == "" {
				return
			}

			// Read result
			data, err := os.ReadFile(filepath.Join(tmpDir, ClaudeMDFile))
			if err != nil {
				t.Fatalf("failed to read CLAUDE.md: %v", err)
			}
			content := string(data)

			// Check expected content
			for _, want := range tt.wantContains {
				if !strings.Contains(content, want) {
					t.Errorf("content should contain %q", want)
				}
			}

			// Check unexpected content
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(content, notWant) {
					t.Errorf("content should not contain %q", notWant)
				}
			}
		})
	}
}

func TestHasKnowledgeSection(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		want            bool
	}{
		{
			name:            "has knowledge section",
			existingContent: "# Project\n\n" + knowledgeSectionStart + "\nContent\n" + knowledgeSectionEnd,
			want:            true,
		},
		{
			name:            "no knowledge section",
			existingContent: "# Project\n\nSome content.\n",
			want:            false,
		},
		{
			name:            "no file",
			existingContent: "",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.existingContent != "" {
				err := os.WriteFile(filepath.Join(tmpDir, ClaudeMDFile), []byte(tt.existingContent), 0644)
				if err != nil {
					t.Fatalf("failed to create CLAUDE.md: %v", err)
				}
			}

			got := HasKnowledgeSection(tmpDir)
			if got != tt.want {
				t.Errorf("HasKnowledgeSection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKnowledgeSectionLineCount(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		want            int
	}{
		{
			name: "count lines in knowledge section",
			existingContent: `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Pattern1 | Desc1 | TASK-001 |
| Pattern2 | Desc2 | TASK-002 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Dec1 | Reason1 | TASK-003 |
<!-- orc:knowledge:end -->
`,
			want: 10, // Non-empty lines within the section
		},
		{
			name:            "no knowledge section",
			existingContent: "# Project\n\nSome content.\n",
			want:            0,
		},
		{
			name:            "no file",
			existingContent: "",
			want:            0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.existingContent != "" {
				err := os.WriteFile(filepath.Join(tmpDir, ClaudeMDFile), []byte(tt.existingContent), 0644)
				if err != nil {
					t.Fatalf("failed to create CLAUDE.md: %v", err)
				}
			}

			got, err := KnowledgeSectionLineCount(tmpDir)
			if err != nil {
				t.Fatalf("KnowledgeSectionLineCount() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("KnowledgeSectionLineCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudeMDLineCount(t *testing.T) {
	tests := []struct {
		name            string
		existingContent string
		want            int
	}{
		{
			name:            "count total lines",
			existingContent: "Line 1\nLine 2\nLine 3\n",
			want:            4, // 3 lines + trailing empty after split
		},
		{
			name:            "no file",
			existingContent: "",
			want:            0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.existingContent != "" {
				err := os.WriteFile(filepath.Join(tmpDir, ClaudeMDFile), []byte(tt.existingContent), 0644)
				if err != nil {
					t.Fatalf("failed to create CLAUDE.md: %v", err)
				}
			}

			got, err := ClaudeMDLineCount(tmpDir)
			if err != nil {
				t.Fatalf("ClaudeMDLineCount() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ClaudeMDLineCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldSuggestSplit(t *testing.T) {
	tests := []struct {
		name      string
		lineCount int
		want      bool
	}{
		{
			name:      "under threshold",
			lineCount: 100,
			want:      false,
		},
		{
			name:      "at threshold",
			lineCount: 199, // 199 lines + trailing = 200 total
			want:      false,
		},
		{
			name:      "over threshold",
			lineCount: 201,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create a file with the specified number of lines
			content := strings.Repeat("line\n", tt.lineCount)
			err := os.WriteFile(filepath.Join(tmpDir, ClaudeMDFile), []byte(content), 0644)
			if err != nil {
				t.Fatalf("failed to create CLAUDE.md: %v", err)
			}

			got, count, err := ShouldSuggestSplit(tmpDir)
			if err != nil {
				t.Fatalf("ShouldSuggestSplit() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ShouldSuggestSplit() = %v, want %v (count: %d)", got, tt.want, count)
			}
		})
	}
}
