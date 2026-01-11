package spec

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
)

func TestGeneratePrompt_Basic(t *testing.T) {
	data := PromptData{
		Title:       "User Authentication",
		WorkDir:     "/home/user/myproject",
		CreateTasks: true,
	}

	prompt, err := GeneratePrompt(data)
	if err != nil {
		t.Fatalf("GeneratePrompt() error = %v", err)
	}

	// Check that basic elements are present
	wantContains := []string{
		"User Authentication",
		"myproject",
		"Task Generation",
	}

	for _, want := range wantContains {
		if !containsString(prompt, want) {
			t.Errorf("GeneratePrompt() missing %q", want)
		}
	}
}

func TestGeneratePrompt_WithDetection(t *testing.T) {
	data := PromptData{
		Title:   "API Endpoint",
		WorkDir: "/home/user/goproject",
		Detection: &db.Detection{
			Language:    "Go",
			Frameworks:  []string{"chi", "gorm"},
			BuildTools:  []string{"make"},
			HasTests:    true,
			TestCommand: "go test ./...",
		},
		CreateTasks: false,
	}

	prompt, err := GeneratePrompt(data)
	if err != nil {
		t.Fatalf("GeneratePrompt() error = %v", err)
	}

	// Check that detection info is included
	wantContains := []string{
		"Go",
		"chi",
	}

	for _, want := range wantContains {
		if !containsString(prompt, want) {
			t.Errorf("GeneratePrompt() with detection missing %q", want)
		}
	}

	// Task generation should NOT be present
	if containsString(prompt, "Task Generation") {
		t.Error("GeneratePrompt() should not include Task Generation when CreateTasks=false")
	}
}

func TestGeneratePrompt_WithInitiative(t *testing.T) {
	data := PromptData{
		Title:   "Login Flow",
		WorkDir: "/home/user/project",
		Initiative: &initiative.Initiative{
			ID:     "INIT-001",
			Title:  "Authentication System",
			Vision: "Secure authentication using JWT tokens",
			Decisions: []initiative.Decision{
				{Decision: "Use bcrypt for password hashing", Rationale: "Industry standard"},
			},
		},
		CreateTasks: true,
	}

	prompt, err := GeneratePrompt(data)
	if err != nil {
		t.Fatalf("GeneratePrompt() error = %v", err)
	}

	// Check that initiative info is included
	wantContains := []string{
		"INIT-001",
		"Authentication System",
		"JWT tokens",
		"bcrypt",
	}

	for _, want := range wantContains {
		if !containsString(prompt, want) {
			t.Errorf("GeneratePrompt() with initiative missing %q", want)
		}
	}
}

func TestFormatDecisions(t *testing.T) {
	tests := []struct {
		name      string
		decisions []initiative.Decision
		want      string
	}{
		{
			name:      "empty",
			decisions: nil,
			want:      "",
		},
		{
			name: "single decision",
			decisions: []initiative.Decision{
				{Decision: "Use JWT"},
			},
			want: "- Use JWT\n",
		},
		{
			name: "with rationale",
			decisions: []initiative.Decision{
				{Decision: "Use JWT", Rationale: "Industry standard"},
			},
			want: "- Use JWT (Industry standard)\n",
		},
		{
			name: "multiple decisions",
			decisions: []initiative.Decision{
				{Decision: "Use JWT"},
				{Decision: "7-day expiry", Rationale: "Security"},
			},
			want: "- Use JWT\n- 7-day expiry (Security)\n",
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

func TestOptions_Defaults(t *testing.T) {
	opts := Options{
		WorkDir: "/home/user/project",
	}

	// Check that defaults are reasonable
	if opts.WorkDir == "" {
		t.Error("Options.WorkDir should not be empty")
	}

	// CreateTasks defaults to false (zero value)
	if opts.CreateTasks {
		t.Error("Options.CreateTasks should default to false")
	}

	// DryRun defaults to false (zero value)
	if opts.DryRun {
		t.Error("Options.DryRun should default to false")
	}
}

// containsString checks if s contains substr
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
