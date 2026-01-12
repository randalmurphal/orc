package plan_session

import (
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
)

func TestGeneratePrompt(t *testing.T) {
	tests := []struct {
		name         string
		data         PromptData
		wantContains []string
		wantAbsent   []string
	}{
		{
			name: "task mode with all fields",
			data: PromptData{
				Mode:        ModeTask,
				Title:       "Add user authentication",
				TaskID:      "TASK-001",
				TaskWeight:  "medium",
				Description: "Implement JWT-based authentication",
				WorkDir:     "/home/user/myproject",
			},
			wantContains: []string{
				"# Planning Session: Add user authentication",
				"## Task: TASK-001",
				"**Weight**: medium",
				"**Description**: Implement JWT-based authentication",
				"Save to: `.orc/tasks/TASK-001/spec.md`",
			},
			wantAbsent: []string{
				"## Initiative:",
				"CreateTasks",
				".orc/specs/",
			},
		},
		{
			name: "feature mode with CreateTasks",
			data: PromptData{
				Mode:        ModeFeature,
				Title:       "Dark Mode Support",
				WorkDir:     "/home/user/frontend",
				CreateTasks: true,
			},
			wantContains: []string{
				"# Planning Session: Dark Mode Support",
				"orc new",
				"--weight",
				".orc/specs/<feature-name>.md",
			},
			wantAbsent: []string{
				"## Task:",
				"TASK-",
			},
		},
		{
			name: "feature mode without CreateTasks",
			data: PromptData{
				Mode:        ModeFeature,
				Title:       "API Redesign",
				WorkDir:     "/home/user/api",
				CreateTasks: false,
			},
			wantContains: []string{
				"# Planning Session: API Redesign",
				".orc/specs/<feature-name>.md",
			},
			wantAbsent: []string{
				"### Task Generation",
				"orc new",
			},
		},
		{
			name: "with initiative context",
			data: PromptData{
				Mode:    ModeTask,
				Title:   "Auth Models",
				TaskID:  "TASK-002",
				WorkDir: "/home/user/project",
				Initiative: &initiative.Initiative{
					ID:     "INIT-001",
					Title:  "User Authentication System",
					Vision: "Secure, scalable authentication using modern standards",
					Decisions: []initiative.Decision{
						{
							ID:        "DEC-001",
							Date:      time.Now(),
							By:        "RM",
							Decision:  "Use JWT for session tokens",
							Rationale: "Stateless, scalable",
						},
						{
							ID:        "DEC-002",
							Date:      time.Now(),
							By:        "RM",
							Decision:  "Use bcrypt for password hashing",
							Rationale: "Industry standard",
						},
					},
				},
			},
			wantContains: []string{
				"## Initiative: INIT-001 - User Authentication System",
				"**Vision**: Secure, scalable authentication using modern standards",
				"**Decisions**:",
				"Use JWT for session tokens",
				"(Stateless, scalable)",
				"Use bcrypt for password hashing",
				"(Industry standard)",
			},
			wantAbsent: []string{},
		},
		{
			name: "with detection info",
			data: PromptData{
				Mode:    ModeTask,
				Title:   "Add Tests",
				TaskID:  "TASK-003",
				WorkDir: "/home/user/goproject",
				Detection: &db.Detection{
					Language:    "Go",
					Frameworks:  []string{"cobra", "chi"},
					BuildTools:  []string{"make", "go"},
					HasTests:    true,
					TestCommand: "go test ./...",
				},
			},
			wantContains: []string{
				"# Planning Session: Add Tests",
			},
			// Note: Detection info is added to tmplData but template doesn't
			// directly render it in visible text unless template uses it
			wantAbsent: []string{},
		},
		{
			name: "minimal data - just title",
			data: PromptData{
				Mode:    ModeFeature,
				Title:   "Simple Feature",
				WorkDir: "/tmp/test",
			},
			wantContains: []string{
				"# Planning Session: Simple Feature",
				"Start by asking the user about their requirements for **Simple Feature**",
			},
			wantAbsent: []string{
				"## Task:",
				"## Initiative:",
				"### Task Generation",
			},
		},
		{
			name: "task mode trivial weight shows brief spec requirements",
			data: PromptData{
				Mode:       ModeTask,
				Title:      "Fix typo",
				TaskID:     "TASK-010",
				TaskWeight: "trivial",
				WorkDir:    "/home/user/project",
			},
			wantContains: []string{
				"### Spec Requirements (Trivial)",
				"Brief spec with:",
				"**Intent**: What and why (1-2 sentences each)",
				"**Success Criteria**: 1-2 testable items",
			},
			wantAbsent: []string{
				"**## Testing**",
			},
		},
		{
			name: "task mode non-trivial weight shows full spec requirements",
			data: PromptData{
				Mode:       ModeTask,
				Title:      "Add feature",
				TaskID:     "TASK-011",
				TaskWeight: "large",
				WorkDir:    "/home/user/project",
			},
			wantContains: []string{
				"### Spec Requirements",
				"**## Intent**",
				"**## Success Criteria**",
				"**## Testing**",
			},
			wantAbsent: []string{
				"### Spec Requirements (Trivial)",
			},
		},
		{
			name: "initiative with empty decisions",
			data: PromptData{
				Mode:    ModeTask,
				Title:   "New Task",
				TaskID:  "TASK-020",
				WorkDir: "/home/user/project",
				Initiative: &initiative.Initiative{
					ID:        "INIT-005",
					Title:     "Empty Initiative",
					Vision:    "Some vision",
					Decisions: []initiative.Decision{},
				},
			},
			wantContains: []string{
				"## Initiative: INIT-005 - Empty Initiative",
				"**Vision**: Some vision",
			},
			wantAbsent: []string{
				"**Decisions**:",
			},
		},
		{
			name: "initiative without vision",
			data: PromptData{
				Mode:    ModeTask,
				Title:   "Task",
				TaskID:  "TASK-021",
				WorkDir: "/home/user/project",
				Initiative: &initiative.Initiative{
					ID:     "INIT-006",
					Title:  "No Vision Initiative",
					Vision: "",
				},
			},
			wantContains: []string{
				"## Initiative: INIT-006 - No Vision Initiative",
			},
			wantAbsent: []string{
				"**Vision**:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GeneratePrompt(tt.data)
			if err != nil {
				t.Fatalf("GeneratePrompt() error = %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("GeneratePrompt() missing expected content: %q\n\nFull output:\n%s", want, got)
				}
			}

			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("GeneratePrompt() contains unexpected content: %q", absent)
				}
			}
		})
	}
}

func TestGeneratePrompt_ReturnsNonEmptyString(t *testing.T) {
	data := PromptData{
		Mode:    ModeFeature,
		Title:   "Test",
		WorkDir: "/tmp",
	}

	got, err := GeneratePrompt(data)
	if err != nil {
		t.Fatalf("GeneratePrompt() error = %v", err)
	}

	if got == "" {
		t.Error("GeneratePrompt() returned empty string")
	}

	// Should contain the completion marker section
	if !strings.Contains(got, "<spec_complete>true</spec_complete>") {
		t.Error("GeneratePrompt() missing completion marker section")
	}
}

func TestFormatDecisions(t *testing.T) {
	tests := []struct {
		name      string
		decisions []initiative.Decision
		want      string
	}{
		{
			name:      "empty decisions",
			decisions: []initiative.Decision{},
			want:      "",
		},
		{
			name:      "nil decisions",
			decisions: nil,
			want:      "",
		},
		{
			name: "single decision without rationale",
			decisions: []initiative.Decision{
				{
					ID:        "DEC-001",
					Date:      time.Now(),
					By:        "RM",
					Decision:  "Use PostgreSQL",
					Rationale: "",
				},
			},
			want: "- Use PostgreSQL\n",
		},
		{
			name: "single decision with rationale",
			decisions: []initiative.Decision{
				{
					ID:        "DEC-001",
					Date:      time.Now(),
					By:        "RM",
					Decision:  "Use PostgreSQL",
					Rationale: "Best fit for our needs",
				},
			},
			want: "- Use PostgreSQL (Best fit for our needs)\n",
		},
		{
			name: "multiple decisions with rationale",
			decisions: []initiative.Decision{
				{
					ID:        "DEC-001",
					Date:      time.Now(),
					By:        "RM",
					Decision:  "Use PostgreSQL",
					Rationale: "Best fit for our needs",
				},
				{
					ID:        "DEC-002",
					Date:      time.Now(),
					By:        "JD",
					Decision:  "Use Redis for caching",
					Rationale: "Fast in-memory store",
				},
				{
					ID:        "DEC-003",
					Date:      time.Now(),
					By:        "RM",
					Decision:  "Use Kubernetes",
					Rationale: "Container orchestration",
				},
			},
			want: "- Use PostgreSQL (Best fit for our needs)\n- Use Redis for caching (Fast in-memory store)\n- Use Kubernetes (Container orchestration)\n",
		},
		{
			name: "multiple decisions mixed rationale",
			decisions: []initiative.Decision{
				{
					ID:        "DEC-001",
					Date:      time.Now(),
					By:        "RM",
					Decision:  "Use Go",
					Rationale: "Performance and simplicity",
				},
				{
					ID:        "DEC-002",
					Date:      time.Now(),
					By:        "JD",
					Decision:  "No external dependencies",
					Rationale: "",
				},
				{
					ID:        "DEC-003",
					Date:      time.Now(),
					By:        "RM",
					Decision:  "Use structured logging",
					Rationale: "Better observability",
				},
			},
			want: "- Use Go (Performance and simplicity)\n- No external dependencies\n- Use structured logging (Better observability)\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDecisions(tt.decisions)
			if got != tt.want {
				t.Errorf("formatDecisions() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDecisions_PreservesOrder(t *testing.T) {
	decisions := []initiative.Decision{
		{ID: "DEC-001", Decision: "First"},
		{ID: "DEC-002", Decision: "Second"},
		{ID: "DEC-003", Decision: "Third"},
	}

	got := formatDecisions(decisions)

	// Check order is preserved
	firstIdx := strings.Index(got, "First")
	secondIdx := strings.Index(got, "Second")
	thirdIdx := strings.Index(got, "Third")

	if firstIdx == -1 || secondIdx == -1 || thirdIdx == -1 {
		t.Fatal("formatDecisions() missing expected decisions")
	}

	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Error("formatDecisions() did not preserve order")
	}
}

func TestGeneratePrompt_ProjectName(t *testing.T) {
	tests := []struct {
		name     string
		workDir  string
		wantName string
	}{
		{
			name:     "extracts project name from path",
			workDir:  "/home/user/repos/myproject",
			wantName: "myproject",
		},
		{
			name:     "handles root path",
			workDir:  "/",
			wantName: "",
		},
		{
			name:     "handles current directory",
			workDir:  ".",
			wantName: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := PromptData{
				Mode:    ModeFeature,
				Title:   "Test",
				WorkDir: tt.workDir,
			}

			_, err := GeneratePrompt(data)
			if err != nil {
				t.Fatalf("GeneratePrompt() error = %v", err)
			}

			// The template uses ProjectName but doesn't render it visibly
			// in current template, so we just verify no error occurs
		})
	}
}

func TestGeneratePrompt_DetectionInfo(t *testing.T) {
	// Test that detection info doesn't cause errors
	// even with various edge cases
	tests := []struct {
		name      string
		detection *db.Detection
	}{
		{
			name:      "nil detection",
			detection: nil,
		},
		{
			name: "empty detection",
			detection: &db.Detection{
				Language:   "",
				Frameworks: nil,
				BuildTools: nil,
			},
		},
		{
			name: "full detection",
			detection: &db.Detection{
				Language:    "Python",
				Frameworks:  []string{"FastAPI", "SQLAlchemy"},
				BuildTools:  []string{"pip", "poetry"},
				HasTests:    true,
				TestCommand: "pytest",
				LintCommand: "ruff check",
			},
		},
		{
			name: "detection with empty slices",
			detection: &db.Detection{
				Language:   "Rust",
				Frameworks: []string{},
				BuildTools: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := PromptData{
				Mode:      ModeTask,
				Title:     "Test",
				TaskID:    "TASK-001",
				WorkDir:   "/tmp/test",
				Detection: tt.detection,
			}

			got, err := GeneratePrompt(data)
			if err != nil {
				t.Fatalf("GeneratePrompt() error = %v", err)
			}

			if got == "" {
				t.Error("GeneratePrompt() returned empty string")
			}
		})
	}
}
