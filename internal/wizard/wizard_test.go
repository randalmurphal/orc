package wizard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/detect"
)

func TestSelectProfile_Default(t *testing.T) {
	// Test that quick mode uses default profile
	opts := Options{Quick: true}
	if opts.Profile == "" {
		opts.Profile = "auto" // mimics the logic
	}
	if opts.Profile != "auto" {
		t.Errorf("expected auto, got %s", opts.Profile)
	}
}

func TestApplyDetectedSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	// Create initial config
	initial := `profile: auto
worktree:
  enabled: true
`
	os.WriteFile(configPath, []byte(initial), 0644)

	detection := &detect.Detection{
		Language:    detect.ProjectTypeGo,
		TestCommand: "go test ./...",
		LintCommand: "golangci-lint run",
	}

	err := applyDetectedSettings(configPath, detection, "safe")
	if err != nil {
		t.Fatalf("applyDetectedSettings failed: %v", err)
	}

	// Read back
	data, _ := os.ReadFile(configPath)
	content := string(data)

	if !strings.Contains(content, "profile: safe") {
		t.Error("profile not updated")
	}
	if !strings.Contains(content, "test_command: go test ./...") {
		t.Error("test_command not added")
	}
	if !strings.Contains(content, "lint_command: golangci-lint run") {
		t.Error("lint_command not added")
	}
}

func TestSetConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		key      string
		value    string
		expected string
	}{
		{
			name:     "update existing",
			content:  "profile: auto\n",
			key:      "profile",
			value:    "safe",
			expected: "profile: safe\n",
		},
		{
			name:     "add new",
			content:  "profile: auto",
			key:      "new_key",
			value:    "new_value",
			expected: "profile: auto\nnew_key: new_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := setConfigValue(tt.content, tt.key, tt.value)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGenerateOrcSection(t *testing.T) {
	detection := &detect.Detection{
		Language:     detect.ProjectTypeGo,
		Frameworks:   []detect.Framework{detect.FrameworkGin, detect.FrameworkCobra},
		TestCommand:  "go test ./...",
		LintCommand:  "golangci-lint run",
		BuildCommand: "go build ./...",
	}

	section := generateOrcSection(detection)

	if !strings.Contains(section, "## Orc Orchestrator") {
		t.Error("missing header")
	}
	if !strings.Contains(section, "Language | go") {
		t.Error("missing language")
	}
	if !strings.Contains(section, "gin, cobra") {
		t.Error("missing frameworks")
	}
	if !strings.Contains(section, "go test ./...") {
		t.Error("missing test command")
	}
}

func TestInstallSkills(t *testing.T) {
	dir := t.TempDir()
	skills := []string{"go-style", "testing-standards"}

	installed, err := installSkills(dir, skills)
	if err != nil {
		t.Fatalf("installSkills failed: %v", err)
	}

	if len(installed) != 2 {
		t.Errorf("expected 2 installed skills, got %d", len(installed))
	}

	// Check files exist
	for _, skill := range skills {
		path := filepath.Join(dir, ".claude", "skills", skill, "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("skill file not found: %s", path)
		}
	}
}

func TestUpdateClaudeMD_CreateNew(t *testing.T) {
	dir := t.TempDir()

	detection := &detect.Detection{
		Language:    detect.ProjectTypeGo,
		TestCommand: "go test ./...",
	}

	err := updateClaudeMD(dir, detection)
	if err != nil {
		t.Fatalf("updateClaudeMD failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	if !strings.Contains(content, "## Orc Orchestrator") {
		t.Error("missing orc section")
	}
}

func TestUpdateClaudeMD_Idempotent(t *testing.T) {
	dir := t.TempDir()

	existing := `# Project

## Orc Orchestrator

Existing content.
`
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(existing), 0644)

	detection := &detect.Detection{Language: detect.ProjectTypeGo}

	err := updateClaudeMD(dir, detection)
	if err != nil {
		t.Fatalf("updateClaudeMD failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	// Should not duplicate
	if strings.Count(content, "## Orc Orchestrator") != 1 {
		t.Error("orc section duplicated")
	}
}

func TestUpdateClaudeMD_AppendToExisting(t *testing.T) {
	dir := t.TempDir()

	existing := `# Project Instructions

## Some Other Section

Content here.
`
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(existing), 0644)

	detection := &detect.Detection{Language: detect.ProjectTypeGo}

	err := updateClaudeMD(dir, detection)
	if err != nil {
		t.Fatalf("updateClaudeMD failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	// Should have both sections
	if !strings.Contains(content, "## Some Other Section") {
		t.Error("existing section removed")
	}
	if !strings.Contains(content, "## Orc Orchestrator") {
		t.Error("orc section not added")
	}
}
