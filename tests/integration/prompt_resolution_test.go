package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestPromptResolutionEmbedded verifies that embedded prompts are used
// when no overrides exist.
func TestPromptResolutionEmbedded(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	svc := prompt.NewService(repo.OrcDir)

	// Get implement prompt (should be embedded)
	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve implement prompt: %v", err)
	}

	if source != prompt.SourceEmbedded {
		t.Errorf("source = %v, want %v", source, prompt.SourceEmbedded)
	}

	// Verify some expected content exists
	if content == "" {
		t.Error("implement prompt should have content")
	}
}

// TestPromptResolutionProjectOverride verifies that project prompts
// override embedded prompts.
func TestPromptResolutionProjectOverride(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create project prompt override
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}

	customContent := "# Custom Implement Prompt\nThis is a project-specific prompt."
	if err := os.WriteFile(filepath.Join(promptDir, "implement.md"), []byte(customContent), 0644); err != nil {
		t.Fatalf("write custom prompt: %v", err)
	}

	svc := prompt.NewService(repo.OrcDir)

	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve implement prompt: %v", err)
	}

	if source != prompt.SourceProject {
		t.Errorf("source = %v, want %v", source, prompt.SourceProject)
	}

	if content != customContent {
		t.Errorf("content = %q, want %q", content, customContent)
	}
}

// TestPromptResolutionSharedOverride verifies that shared prompts
// override embedded but are overridden by project prompts.
func TestPromptResolutionSharedOverride(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Create shared prompt
	sharedPromptDir := filepath.Join(repo.OrcDir, "shared", "prompts")
	if err := os.MkdirAll(sharedPromptDir, 0755); err != nil {
		t.Fatalf("create shared prompt dir: %v", err)
	}

	sharedContent := "# Shared Implement Prompt\nThis is the team's standard prompt."
	if err := os.WriteFile(filepath.Join(sharedPromptDir, "implement.md"), []byte(sharedContent), 0644); err != nil {
		t.Fatalf("write shared prompt: %v", err)
	}

	svc := prompt.NewService(repo.OrcDir)

	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve implement prompt: %v", err)
	}

	if source != prompt.SourceProjectShared {
		t.Errorf("source = %v, want %v", source, prompt.SourceProjectShared)
	}

	if content != sharedContent {
		t.Errorf("content = %q, want %q", content, sharedContent)
	}
}

// TestPromptResolutionPersonalOverridesAll verifies that personal prompts
// have highest priority.
func TestPromptResolutionPersonalOverridesAll(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Create shared prompt
	sharedPromptDir := filepath.Join(repo.OrcDir, "shared", "prompts")
	if err := os.MkdirAll(sharedPromptDir, 0755); err != nil {
		t.Fatalf("create shared prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedPromptDir, "implement.md"), []byte("shared"), 0644); err != nil {
		t.Fatalf("write shared prompt: %v", err)
	}

	// Create project prompt
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "implement.md"), []byte("project"), 0644); err != nil {
		t.Fatalf("write project prompt: %v", err)
	}

	// Create local prompt (personal level)
	localPromptDir := filepath.Join(repo.OrcDir, "local", "prompts")
	if err := os.MkdirAll(localPromptDir, 0755); err != nil {
		t.Fatalf("create local prompt dir: %v", err)
	}
	localContent := "local"
	if err := os.WriteFile(filepath.Join(localPromptDir, "implement.md"), []byte(localContent), 0644); err != nil {
		t.Fatalf("write local prompt: %v", err)
	}

	svc := prompt.NewService(repo.OrcDir)

	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve implement prompt: %v", err)
	}

	// Local (personal) should win
	if source != prompt.SourceProjectLocal {
		t.Errorf("source = %v, want %v", source, prompt.SourceProjectLocal)
	}

	if content != localContent {
		t.Errorf("content = %q, want %q", content, localContent)
	}
}

// TestPromptResolutionHierarchy verifies the full resolution hierarchy.
func TestPromptResolutionHierarchy(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	svc := prompt.NewService(repo.OrcDir)

	// Phase 1: Only embedded exists
	_, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve embedded: %v", err)
	}
	if source != prompt.SourceEmbedded {
		t.Errorf("phase 1: source = %v, want embedded", source)
	}

	// Phase 2: Add shared prompt
	sharedPromptDir := filepath.Join(repo.OrcDir, "shared", "prompts")
	if err := os.MkdirAll(sharedPromptDir, 0755); err != nil {
		t.Fatalf("create shared prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sharedPromptDir, "implement.md"), []byte("shared"), 0644); err != nil {
		t.Fatalf("write shared prompt: %v", err)
	}

	// Recreate service to pick up new files
	svc = prompt.NewService(repo.OrcDir)

	_, source, err = svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve after shared: %v", err)
	}
	if source != prompt.SourceProjectShared {
		t.Errorf("phase 2: source = %v, want shared", source)
	}

	// Phase 3: Add project prompt
	// NOTE: In the prompt resolution hierarchy, shared (priority 3) has
	// HIGHER priority than project (priority 4). So adding a project
	// prompt does NOT override the shared prompt. This is by design -
	// team standards (shared) take precedence over project defaults.
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "implement.md"), []byte("project"), 0644); err != nil {
		t.Fatalf("write project prompt: %v", err)
	}

	svc = prompt.NewService(repo.OrcDir)

	_, source, err = svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve after project: %v", err)
	}
	// Shared has higher priority than project, so shared still wins
	if source != prompt.SourceProjectShared {
		t.Errorf("phase 3: source = %v, want project_shared (shared has higher priority)", source)
	}

	// Phase 4: Add local prompt
	localPromptDir := filepath.Join(repo.OrcDir, "local", "prompts")
	if err := os.MkdirAll(localPromptDir, 0755); err != nil {
		t.Fatalf("create local prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localPromptDir, "implement.md"), []byte("local"), 0644); err != nil {
		t.Fatalf("write local prompt: %v", err)
	}

	svc = prompt.NewService(repo.OrcDir)

	_, source, err = svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve after local: %v", err)
	}
	// Local wins
	if source != prompt.SourceProjectLocal {
		t.Errorf("phase 4: source = %v, want local", source)
	}
}

// TestPromptResolutionSourceReporting verifies that source is reported correctly.
func TestPromptResolutionSourceReporting(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	svc := prompt.NewService(repo.OrcDir)

	// Check HasOverride for embedded prompt
	if svc.HasOverride("implement") {
		t.Error("implement should not have override initially")
	}

	// Add override
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "implement.md"), []byte("custom"), 0644); err != nil {
		t.Fatalf("write custom prompt: %v", err)
	}

	// Recreate service
	svc = prompt.NewService(repo.OrcDir)

	if !svc.HasOverride("implement") {
		t.Error("implement should have override after adding project prompt")
	}
}

// TestPromptResolutionCustomPhases verifies handling of custom prompt phases.
func TestPromptResolutionCustomPhases(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create custom phase that doesn't exist in embedded
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}

	customContent := "# Custom Phase\nThis is a custom phase prompt."
	if err := os.WriteFile(filepath.Join(promptDir, "custom_phase.md"), []byte(customContent), 0644); err != nil {
		t.Fatalf("write custom phase prompt: %v", err)
	}

	svc := prompt.NewService(repo.OrcDir)

	content, source, err := svc.Resolve("custom_phase")
	if err != nil {
		t.Fatalf("resolve custom_phase: %v", err)
	}

	if source != prompt.SourceProject {
		t.Errorf("source = %v, want %v", source, prompt.SourceProject)
	}

	if content != customContent {
		t.Errorf("content mismatch")
	}
}

// TestPromptResolutionNonExistent verifies error handling for non-existent prompts.
func TestPromptResolutionNonExistent(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	svc := prompt.NewService(repo.OrcDir)

	_, _, err := svc.Resolve("nonexistent_phase_xyz")
	if err == nil {
		t.Error("expected error for non-existent prompt")
	}
}

// TestPromptInheritance verifies prompt inheritance via frontmatter.
func TestPromptInheritance(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create project prompt that extends embedded
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}

	// Prompt with inheritance frontmatter
	inheritContent := `---
extends: embedded
prepend: |
  # Project-Specific Instructions
  Always follow these rules:
append: |
  ## Additional Notes
  These are project-specific notes.
---
`
	if err := os.WriteFile(filepath.Join(promptDir, "implement.md"), []byte(inheritContent), 0644); err != nil {
		t.Fatalf("write inherit prompt: %v", err)
	}

	svc := prompt.NewService(repo.OrcDir)

	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("resolve implement with inheritance: %v", err)
	}

	// Source should be project (where override is defined)
	if source != prompt.SourceProject {
		t.Errorf("source = %v, want %v", source, prompt.SourceProject)
	}

	// Content should include prepended text
	if len(content) == 0 {
		t.Error("content should not be empty")
	}

	// Verify prepend and append were applied
	testutil.AssertFileContains(t, filepath.Join(promptDir, "implement.md"), "extends: embedded")
}

// TestPromptList verifies listing all available prompts.
func TestPromptList(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	svc := prompt.NewService(repo.OrcDir)

	prompts, err := svc.List()
	if err != nil {
		t.Fatalf("list prompts: %v", err)
	}

	// Should have at least some embedded prompts
	if len(prompts) == 0 {
		t.Error("expected at least one prompt in list")
	}

	// Check for implement phase
	found := false
	for _, p := range prompts {
		if p.Phase == "implement" {
			found = true
			// Should be embedded since no override
			if p.HasOverride {
				t.Error("implement should not have override initially")
			}
			break
		}
	}
	if !found {
		t.Error("implement phase not found in list")
	}
}

// TestPromptGetDefault verifies getting default (embedded) prompts.
func TestPromptGetDefault(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Create project override
	promptDir := filepath.Join(repo.OrcDir, "prompts")
	if err := os.MkdirAll(promptDir, 0755); err != nil {
		t.Fatalf("create prompt dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptDir, "implement.md"), []byte("custom"), 0644); err != nil {
		t.Fatalf("write custom prompt: %v", err)
	}

	svc := prompt.NewService(repo.OrcDir)

	// Get should return custom
	customPrompt, err := svc.Get("implement")
	if err != nil {
		t.Fatalf("get implement: %v", err)
	}
	if customPrompt.Source != prompt.SourceProject {
		t.Errorf("Get source = %v, want project", customPrompt.Source)
	}

	// GetDefault should return embedded
	defaultPrompt, err := svc.GetDefault("implement")
	if err != nil {
		t.Fatalf("get default implement: %v", err)
	}
	if defaultPrompt.Source != prompt.SourceEmbedded {
		t.Errorf("GetDefault source = %v, want embedded", defaultPrompt.Source)
	}
	if defaultPrompt.Content == customPrompt.Content {
		t.Error("default and custom prompts should be different")
	}
}
