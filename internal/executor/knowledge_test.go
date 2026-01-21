package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashKnowledgeSection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  string
		wantHash bool // true if should return a hash
	}{
		{
			name: "with knowledge section",
			content: `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge
Some content here
<!-- orc:knowledge:end -->
`,
			wantHash: true,
		},
		{
			name:     "without knowledge section",
			content:  "# Project\n\nSome content.\n",
			wantHash: false,
		},
		{
			name:     "empty file",
			content:  "",
			wantHash: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			if tt.content != "" {
				err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(tt.content), 0644)
				if err != nil {
					t.Fatalf("failed to write CLAUDE.md: %v", err)
				}
			}

			hash := HashKnowledgeSection(tmpDir)

			if tt.wantHash && hash == "" {
				t.Error("expected hash, got empty string")
			}
			if !tt.wantHash && hash != "" {
				t.Errorf("expected empty hash, got %s", hash)
			}
		})
	}
}

func TestHashKnowledgeSection_SameContent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	content := `# Project

<!-- orc:knowledge:begin -->
## Project Knowledge
Fixed content here
<!-- orc:knowledge:end -->
`
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	hash1 := HashKnowledgeSection(tmpDir)
	hash2 := HashKnowledgeSection(tmpDir)

	if hash1 != hash2 {
		t.Errorf("same content should produce same hash: %s != %s", hash1, hash2)
	}
}

func TestShouldExtractKnowledge(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		beforeHash string
		afterHash  string
		want       bool
	}{
		{
			name:       "hashes match - should extract",
			beforeHash: "abc123",
			afterHash:  "abc123",
			want:       true,
		},
		{
			name:       "hashes differ - no extract",
			beforeHash: "abc123",
			afterHash:  "def456",
			want:       false,
		},
		{
			name:       "no before hash",
			beforeHash: "",
			afterHash:  "abc123",
			want:       false,
		},
		{
			name:       "both empty",
			beforeHash: "",
			afterHash:  "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldExtractKnowledge(tt.beforeHash, tt.afterHash)
			if got != tt.want {
				t.Errorf("ShouldExtractKnowledge(%q, %q) = %v, want %v",
					tt.beforeHash, tt.afterHash, got, tt.want)
			}
		})
	}
}

func TestExtractKnowledgeFromTranscript(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		transcript    string
		wantDecisions int
		wantPatterns  int
		wantGotchas   int
		checkContent  func(*KnowledgeCapture) bool
	}{
		{
			name:          "decision - I decided to",
			transcript:    "After reviewing the options, I decided to use PostgreSQL because it handles concurrent writes better.",
			wantDecisions: 1,
		},
		{
			name:          "decision - I chose",
			transcript:    "I chose the functional options pattern because it provides better API ergonomics.",
			wantDecisions: 1,
		},
		{
			name:         "pattern - Following pattern",
			transcript:   "Following the repository abstraction pattern for data access.",
			wantPatterns: 1,
		},
		{
			name:        "gotcha - doesn't work",
			transcript:  "SQLite connection pooling doesn't work because it locks the entire database during writes.",
			wantGotchas: 1,
		},
		{
			name:        "gotcha - Watch out for",
			transcript:  "Watch out for race conditions in the cache layer.",
			wantGotchas: 1,
		},
		{
			name:       "no knowledge",
			transcript: "Just implementing the function as requested. No special decisions needed.",
		},
		{
			name: "multiple types",
			transcript: `I decided to use Redis for caching because of its speed and reliability.
Following the singleton abstraction pattern for the cache client.
Watch out for connection timeouts in high-load scenarios.`,
			wantDecisions: 1,
			wantPatterns:  1,
			wantGotchas:   1,
		},
		{
			name:          "too short - ignored",
			transcript:    "I decided to x. I chose y.",
			wantDecisions: 0, // Too short to be meaningful
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := ExtractKnowledgeFromTranscript(tt.transcript, "TASK-001")

			if len(capture.Decisions) != tt.wantDecisions {
				t.Errorf("decisions: got %d, want %d", len(capture.Decisions), tt.wantDecisions)
			}
			if len(capture.Patterns) != tt.wantPatterns {
				t.Errorf("patterns: got %d, want %d", len(capture.Patterns), tt.wantPatterns)
			}
			if len(capture.Gotchas) != tt.wantGotchas {
				t.Errorf("gotchas: got %d, want %d", len(capture.Gotchas), tt.wantGotchas)
			}

			// Verify source is set
			for _, d := range capture.Decisions {
				if d.Source != "TASK-001" {
					t.Errorf("decision source = %q, want TASK-001", d.Source)
				}
			}

			if tt.checkContent != nil && !tt.checkContent(capture) {
				t.Error("content check failed")
			}
		})
	}
}

func TestKnowledgeCapture_HasEntries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		capture *KnowledgeCapture
		want    bool
	}{
		{
			name:    "empty",
			capture: &KnowledgeCapture{},
			want:    false,
		},
		{
			name: "with pattern",
			capture: &KnowledgeCapture{
				Patterns: []KnowledgeEntry{{Name: "test"}},
			},
			want: true,
		},
		{
			name: "with gotcha",
			capture: &KnowledgeCapture{
				Gotchas: []KnowledgeEntry{{Name: "test"}},
			},
			want: true,
		},
		{
			name: "with decision",
			capture: &KnowledgeCapture{
				Decisions: []KnowledgeEntry{{Name: "test"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.capture.HasEntries(); got != tt.want {
				t.Errorf("HasEntries() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendKnowledgeToClaudeMD(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	initialContent := `# Project

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
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	capture := &KnowledgeCapture{
		Patterns: []KnowledgeEntry{
			{Name: "Repository", Description: "Data access abstraction", Source: "TASK-001"},
		},
		Gotchas: []KnowledgeEntry{
			{Name: "SQLite locks", Description: "Use WAL mode", Source: "TASK-002"},
		},
		Decisions: []KnowledgeEntry{
			{Name: "PostgreSQL", Description: "Better for concurrent writes", Source: "TASK-003"},
		},
	}

	err = AppendKnowledgeToClaudeMD(tmpDir, capture)
	if err != nil {
		t.Fatalf("AppendKnowledgeToClaudeMD() error = %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(filepath.Join(tmpDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Check entries were added
	if !strings.Contains(content, "Repository") {
		t.Error("pattern entry not found")
	}
	if !strings.Contains(content, "SQLite locks") {
		t.Error("gotcha entry not found")
	}
	if !strings.Contains(content, "PostgreSQL") {
		t.Error("decision entry not found")
	}
	if !strings.Contains(content, "TASK-001") {
		t.Error("task source not found")
	}
}

func TestAppendKnowledgeToClaudeMD_NoEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	content := "# Project\n"
	err := os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write CLAUDE.md: %v", err)
	}

	// Empty capture should return nil without error
	err = AppendKnowledgeToClaudeMD(tmpDir, &KnowledgeCapture{})
	if err != nil {
		t.Errorf("AppendKnowledgeToClaudeMD() with empty capture should not error, got %v", err)
	}

	// Nil capture should return nil without error
	err = AppendKnowledgeToClaudeMD(tmpDir, nil)
	if err != nil {
		t.Errorf("AppendKnowledgeToClaudeMD() with nil capture should not error, got %v", err)
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a longer string", 10, "this is..."},
		{"with\nnewlines\nhere", 20, "with newlines here"},
		{"multiple  spaces", 20, "multiple spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestLoadDocsPhaseTranscript(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create transcript directory
	transcriptDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-001", "transcripts")
	err := os.MkdirAll(transcriptDir, 0755)
	if err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	// Create docs phase transcript
	transcript1 := "# docs - Iteration 1\n\nSome content\n"
	err = os.WriteFile(filepath.Join(transcriptDir, "docs-001.md"), []byte(transcript1), 0644)
	if err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	transcript2 := "# docs - Iteration 2\n\nMore content\n"
	err = os.WriteFile(filepath.Join(transcriptDir, "docs-002.md"), []byte(transcript2), 0644)
	if err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	// Load transcripts
	content, err := LoadDocsPhaseTranscript(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadDocsPhaseTranscript() error = %v", err)
	}

	if !strings.Contains(content, "Some content") {
		t.Error("transcript 1 content not found")
	}
	if !strings.Contains(content, "More content") {
		t.Error("transcript 2 content not found")
	}
}

func TestLoadDocsPhaseTranscript_NoTranscripts(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	content, err := LoadDocsPhaseTranscript(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadDocsPhaseTranscript() error = %v", err)
	}
	if content != "" {
		t.Errorf("expected empty content for missing transcripts, got %q", content)
	}
}
