package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewResolver(t *testing.T) {
	r := NewResolver(
		WithPersonalDir("/home/test/.orc/prompts"),
		WithLocalDir("/project/.orc/local/prompts"),
		WithSharedDir("/project/.orc/shared/prompts"),
		WithProjectDir("/project/.orc/prompts"),
	)

	if r.personalDir != "/home/test/.orc/prompts" {
		t.Errorf("expected personalDir '/home/test/.orc/prompts', got %q", r.personalDir)
	}
	if r.localDir != "/project/.orc/local/prompts" {
		t.Errorf("expected localDir '/project/.orc/local/prompts', got %q", r.localDir)
	}
	if r.sharedDir != "/project/.orc/shared/prompts" {
		t.Errorf("expected sharedDir '/project/.orc/shared/prompts', got %q", r.sharedDir)
	}
	if r.projectDir != "/project/.orc/prompts" {
		t.Errorf("expected projectDir '/project/.orc/prompts', got %q", r.projectDir)
	}
	if !r.embedded {
		t.Error("expected embedded to be true by default")
	}
}

func TestResolverFromOrcDir(t *testing.T) {
	r := NewResolverFromOrcDir("/project/.orc")

	if r.localDir != "/project/.orc/local/prompts" {
		t.Errorf("expected localDir '/project/.orc/local/prompts', got %q", r.localDir)
	}
	if r.sharedDir != "/project/.orc/shared/prompts" {
		t.Errorf("expected sharedDir '/project/.orc/shared/prompts', got %q", r.sharedDir)
	}
	if r.projectDir != "/project/.orc/prompts" {
		t.Errorf("expected projectDir '/project/.orc/prompts', got %q", r.projectDir)
	}
}

func TestResolve_PersonalOverridesAll(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup directories
	personalDir := filepath.Join(tmpDir, "personal", "prompts")
	localDir := filepath.Join(tmpDir, "project", ".orc", "local", "prompts")
	sharedDir := filepath.Join(tmpDir, "project", ".orc", "shared", "prompts")
	projectDir := filepath.Join(tmpDir, "project", ".orc", "prompts")

	for _, dir := range []string{personalDir, localDir, sharedDir, projectDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write prompts at each level
	if err := os.WriteFile(filepath.Join(personalDir, "implement.md"), []byte("personal prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "implement.md"), []byte("local prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "implement.md"), []byte("shared prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte("project prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithPersonalDir(personalDir),
		WithLocalDir(localDir),
		WithSharedDir(sharedDir),
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Content != "personal prompt" {
		t.Errorf("expected personal prompt, got %q", resolved.Content)
	}
	if resolved.Source != SourcePersonalGlobal {
		t.Errorf("expected source personal_global, got %q", resolved.Source)
	}
}

func TestResolve_LocalOverridesShared(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup only local and shared
	localDir := filepath.Join(tmpDir, "local", "prompts")
	sharedDir := filepath.Join(tmpDir, "shared", "prompts")

	for _, dir := range []string{localDir, sharedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(localDir, "implement.md"), []byte("local prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "implement.md"), []byte("shared prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithLocalDir(localDir),
		WithSharedDir(sharedDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Content != "local prompt" {
		t.Errorf("expected local prompt, got %q", resolved.Content)
	}
	if resolved.Source != SourceProjectLocal {
		t.Errorf("expected source project_local, got %q", resolved.Source)
	}
}

func TestResolve_SharedOverridesProject(t *testing.T) {
	tmpDir := t.TempDir()

	sharedDir := filepath.Join(tmpDir, "shared", "prompts")
	projectDir := filepath.Join(tmpDir, "project", "prompts")

	for _, dir := range []string{sharedDir, projectDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(sharedDir, "implement.md"), []byte("shared prompt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte("project prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithSharedDir(sharedDir),
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Content != "shared prompt" {
		t.Errorf("expected shared prompt, got %q", resolved.Content)
	}
	if resolved.Source != SourceProjectShared {
		t.Errorf("expected source project_shared, got %q", resolved.Source)
	}
}

func TestResolve_ProjectOverridesEmbedded(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte("project prompt"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Content != "project prompt" {
		t.Errorf("expected project prompt, got %q", resolved.Content)
	}
	if resolved.Source != SourceProject {
		t.Errorf("expected source project, got %q", resolved.Source)
	}
}

func TestResolve_FallsBackToEmbedded(t *testing.T) {
	tmpDir := t.TempDir()

	r := NewResolver(
		WithPersonalDir(filepath.Join(tmpDir, "personal", "prompts")),
		WithLocalDir(filepath.Join(tmpDir, "local", "prompts")),
		WithSharedDir(filepath.Join(tmpDir, "shared", "prompts")),
		WithProjectDir(filepath.Join(tmpDir, "project", "prompts")),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Content == "" {
		t.Error("expected non-empty content from embedded")
	}
	if resolved.Source != SourceEmbedded {
		t.Errorf("expected source embedded, got %q", resolved.Source)
	}
}

func TestResolve_NotFoundWithoutEmbedded(t *testing.T) {
	tmpDir := t.TempDir()

	r := NewResolver(
		WithProjectDir(filepath.Join(tmpDir, "prompts")),
		WithEmbedded(false),
	)

	_, err := r.Resolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent prompt without embedded fallback")
	}
}

func TestResolve_InheritanceWithPrepend(t *testing.T) {
	tmpDir := t.TempDir()

	projectDir := filepath.Join(tmpDir, "project", "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write prompt that extends embedded and prepends
	content := `---
extends: embedded
prepend: |
  CUSTOM HEADER
  ==============
---
`
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Source != SourceProject {
		t.Errorf("expected source project, got %q", resolved.Source)
	}
	if len(resolved.InheritedFrom) == 0 {
		t.Error("expected InheritedFrom to be set")
	}
	if resolved.InheritedFrom[0] != SourceEmbedded {
		t.Errorf("expected inherited from embedded, got %q", resolved.InheritedFrom[0])
	}
	if len(resolved.Content) == 0 {
		t.Error("expected non-empty content")
	}
	// Check prepend is at start
	if resolved.Content[:14] != "CUSTOM HEADER\n" {
		t.Errorf("expected content to start with prepend, got %q", resolved.Content[:50])
	}
}

func TestResolve_InheritanceWithAppend(t *testing.T) {
	tmpDir := t.TempDir()

	projectDir := filepath.Join(tmpDir, "project", "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
extends: embedded
append: |
  CUSTOM FOOTER
---
`
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check append is at end
	if len(resolved.Content) < 14 {
		t.Fatal("content too short")
	}
	suffix := resolved.Content[len(resolved.Content)-14:]
	if suffix != "\nCUSTOM FOOTER" {
		t.Errorf("expected content to end with append, got suffix %q", suffix)
	}
}

func TestResolve_InheritanceWithPrependAndAppend(t *testing.T) {
	tmpDir := t.TempDir()

	projectDir := filepath.Join(tmpDir, "project", "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
extends: embedded
prepend: |
  HEADER
append: |
  FOOTER
---
`
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Content[:7] != "HEADER\n" {
		t.Errorf("expected content to start with HEADER, got %q", resolved.Content[:20])
	}
	if resolved.Content[len(resolved.Content)-7:] != "\nFOOTER" {
		t.Errorf("expected content to end with FOOTER, got %q", resolved.Content[len(resolved.Content)-20:])
	}
}

func TestResolve_InheritanceChain(t *testing.T) {
	tmpDir := t.TempDir()

	sharedDir := filepath.Join(tmpDir, "shared", "prompts")
	localDir := filepath.Join(tmpDir, "local", "prompts")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Shared extends embedded
	sharedContent := `---
extends: embedded
prepend: |
  SHARED PREPEND
---
`
	if err := os.WriteFile(filepath.Join(sharedDir, "implement.md"), []byte(sharedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Local extends shared
	localContent := `---
extends: shared
prepend: |
  LOCAL PREPEND
---
`
	if err := os.WriteFile(filepath.Join(localDir, "implement.md"), []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithLocalDir(localDir),
		WithSharedDir(sharedDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Source != SourceProjectLocal {
		t.Errorf("expected source project_local, got %q", resolved.Source)
	}
	if len(resolved.InheritedFrom) < 2 {
		t.Fatalf("expected at least 2 inherited sources, got %d", len(resolved.InheritedFrom))
	}
	if resolved.InheritedFrom[0] != SourceProjectShared {
		t.Errorf("expected first inherited from shared, got %q", resolved.InheritedFrom[0])
	}
	if resolved.InheritedFrom[1] != SourceEmbedded {
		t.Errorf("expected second inherited from embedded, got %q", resolved.InheritedFrom[1])
	}

	// Check order: LOCAL PREPEND then SHARED PREPEND then embedded
	if resolved.Content[:14] != "LOCAL PREPEND\n" {
		t.Errorf("expected content to start with LOCAL PREPEND, got %q", resolved.Content[:30])
	}
}

func TestResolveFromSource(t *testing.T) {
	tmpDir := t.TempDir()

	personalDir := filepath.Join(tmpDir, "personal", "prompts")
	projectDir := filepath.Join(tmpDir, "project", "prompts")
	if err := os.MkdirAll(personalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(personalDir, "test.md"), []byte("personal"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "test.md"), []byte("project"), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithPersonalDir(personalDir),
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	// Resolve from specific source
	resolved, err := r.ResolveFromSource("test", SourceProject)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content != "project" {
		t.Errorf("expected project content, got %q", resolved.Content)
	}

	// Should get personal when asking for it
	resolved, err = r.ResolveFromSource("test", SourcePersonalGlobal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content != "personal" {
		t.Errorf("expected personal content, got %q", resolved.Content)
	}
}

func TestResolveFromSource_Embedded(t *testing.T) {
	r := NewResolver(WithEmbedded(true))

	resolved, err := r.ResolveFromSource("implement", SourceEmbedded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Content == "" {
		t.Error("expected non-empty embedded content")
	}
	if resolved.Source != SourceEmbedded {
		t.Errorf("expected source embedded, got %q", resolved.Source)
	}
}

func TestResolveFromSource_NotConfigured(t *testing.T) {
	r := NewResolver() // No directories configured

	_, err := r.ResolveFromSource("test", SourcePersonalGlobal)
	if err == nil {
		t.Error("expected error for unconfigured personal directory")
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantExtends string
		wantPrepend string
		wantAppend  string
		wantBody    string
	}{
		{
			name:     "no frontmatter",
			content:  "Just plain content",
			wantBody: "Just plain content",
		},
		{
			name: "extends only",
			content: `---
extends: embedded
---
Body content`,
			wantExtends: "embedded",
			wantBody:    "Body content",
		},
		{
			name: "prepend only",
			content: `---
prepend: |
  First line
  Second line
---
Body`,
			wantPrepend: "First line\nSecond line\n",
			wantBody:    "Body",
		},
		{
			name: "all fields",
			content: `---
extends: shared
prepend: |
  Header
append: |
  Footer
---
Main content`,
			wantExtends: "shared",
			wantPrepend: "Header\n",
			wantAppend:  "Footer\n",
			wantBody:    "Main content",
		},
		{
			name:     "incomplete frontmatter",
			content:  "---\nincomplete",
			wantBody: "---\nincomplete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, body := parseFrontmatter(tt.content)

			if meta.Extends != tt.wantExtends {
				t.Errorf("extends: got %q, want %q", meta.Extends, tt.wantExtends)
			}
			if meta.Prepend != tt.wantPrepend {
				t.Errorf("prepend: got %q, want %q", meta.Prepend, tt.wantPrepend)
			}
			if meta.Append != tt.wantAppend {
				t.Errorf("append: got %q, want %q", meta.Append, tt.wantAppend)
			}
			if body != tt.wantBody {
				t.Errorf("body: got %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestSourcePriority(t *testing.T) {
	tests := []struct {
		source   Source
		priority int
	}{
		{SourcePersonalGlobal, 1},
		{SourceProjectLocal, 2},
		{SourceProjectShared, 3},
		{SourceProject, 4},
		{SourceEmbedded, 5},
		{Source("unknown"), 99},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			if got := SourcePriority(tt.source); got != tt.priority {
				t.Errorf("SourcePriority(%q) = %d, want %d", tt.source, got, tt.priority)
			}
		})
	}

	// Verify ordering
	if SourcePriority(SourcePersonalGlobal) >= SourcePriority(SourceProjectLocal) {
		t.Error("personal should have higher priority than local")
	}
	if SourcePriority(SourceProjectLocal) >= SourcePriority(SourceProjectShared) {
		t.Error("local should have higher priority than shared")
	}
	if SourcePriority(SourceProjectShared) >= SourcePriority(SourceProject) {
		t.Error("shared should have higher priority than project")
	}
	if SourcePriority(SourceProject) >= SourcePriority(SourceEmbedded) {
		t.Error("project should have higher priority than embedded")
	}
}

func TestSourceDisplayName(t *testing.T) {
	tests := []struct {
		source Source
		want   string
	}{
		{SourcePersonalGlobal, "Personal (~/.orc/prompts/)"},
		{SourceProjectLocal, "Local (.orc/local/prompts/)"},
		{SourceProjectShared, "Shared (.orc/shared/prompts/)"},
		{SourceProject, "Project (.orc/prompts/)"},
		{SourceEmbedded, "Embedded (built-in)"},
		{Source("custom"), "custom"},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			if got := SourceDisplayName(tt.source); got != tt.want {
				t.Errorf("SourceDisplayName(%q) = %q, want %q", tt.source, got, tt.want)
			}
		})
	}
}

func TestResolve_NoBodyAfterFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Prompt with only frontmatter, no body
	content := `---
extends: embedded
prepend: |
  PREPEND ONLY
---
`
	if err := os.WriteFile(filepath.Join(projectDir, "implement.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	resolved, err := r.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have prepend + embedded content
	if len(resolved.Content) == 0 {
		t.Error("expected non-empty content")
	}
	if resolved.Content[:12] != "PREPEND ONLY" {
		t.Errorf("expected to start with PREPEND ONLY, got %q", resolved.Content[:20])
	}
}

func TestResolve_InvalidExtends(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := `---
extends: invalid_source
---
Body`
	if err := os.WriteFile(filepath.Join(projectDir, "test.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(true),
	)

	_, err := r.Resolve("test")
	if err == nil {
		t.Error("expected error for invalid extends value")
	}
}

func TestResolve_InheritanceCycleDetection(t *testing.T) {
	tmpDir := t.TempDir()

	sharedDir := filepath.Join(tmpDir, "shared", "prompts")
	localDir := filepath.Join(tmpDir, "local", "prompts")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a cycle: local extends shared, shared extends local
	localContent := `---
extends: shared
prepend: |
  LOCAL
---
`
	sharedContent := `---
extends: local
prepend: |
  SHARED
---
`
	if err := os.WriteFile(filepath.Join(localDir, "cycle.md"), []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "cycle.md"), []byte(sharedContent), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithLocalDir(localDir),
		WithSharedDir(sharedDir),
		WithEmbedded(false),
	)

	_, err := r.Resolve("cycle")
	if err == nil {
		t.Error("expected error for inheritance cycle")
	}
	if err != nil && !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}

func TestResolve_SelfReferenceCycleDetection(t *testing.T) {
	tmpDir := t.TempDir()

	projectDir := filepath.Join(tmpDir, "prompts")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create self-reference: project extends project
	content := `---
extends: project
prepend: |
  SELF
---
`
	if err := os.WriteFile(filepath.Join(projectDir, "self.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewResolver(
		WithProjectDir(projectDir),
		WithEmbedded(false),
	)

	_, err := r.Resolve("self")
	if err == nil {
		t.Error("expected error for self-reference cycle")
	}
	if err != nil && !strings.Contains(err.Error(), "cycle") {
		t.Errorf("expected cycle error, got: %v", err)
	}
}
