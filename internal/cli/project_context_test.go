package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/project"
)

func TestResolveProjectID_FromFlag(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Set up a test registry
	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "proj-1", Name: "test-project", Path: tmpDir},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Test resolution by ID
	projectFlag = "proj-1"
	t.Cleanup(func() { projectFlag = "" })

	id, err := ResolveProjectID()
	if err != nil {
		t.Fatalf("ResolveProjectID failed: %v", err)
	}
	if id != "proj-1" {
		t.Errorf("expected proj-1, got %s", id)
	}
}

func TestResolveProjectID_FromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Set up a test registry
	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "env-project", Name: "env-test", Path: tmpDir},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Set env var
	t.Setenv("ORC_PROJECT", "env-project")

	// Flag should be empty
	projectFlag = ""

	id, err := ResolveProjectID()
	if err != nil {
		t.Fatalf("ResolveProjectID failed: %v", err)
	}
	if id != "env-project" {
		t.Errorf("expected env-project, got %s", id)
	}
}

func TestResolveProjectID_FlagTakesPriority(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Set up a test registry with two projects
	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "flag-project", Name: "flag-test", Path: tmpDir},
			{ID: "env-project", Name: "env-test", Path: filepath.Join(tmpDir, "other")},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Set both flag and env
	projectFlag = "flag-project"
	t.Setenv("ORC_PROJECT", "env-project")
	t.Cleanup(func() {
		projectFlag = ""
	})

	id, err := ResolveProjectID()
	if err != nil {
		t.Fatalf("ResolveProjectID failed: %v", err)
	}
	if id != "flag-project" {
		t.Errorf("expected flag-project (flag priority), got %s", id)
	}
}

func TestResolveProjectID_FromCwd(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Create .orc directory to make it a project root
	// Also serves as the HOME/.orc dir for registry
	projectDir := filepath.Join(tmpDir, "myproject")
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create HOME .orc dir: %v", err)
	}

	// Initialize config (force=true since .orc dir already exists)
	if err := config.InitAt(projectDir, true); err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	// Set up registry with this project
	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "cwd-project", Name: "cwd-test", Path: projectDir},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// Clear flag and env
	projectFlag = ""
	t.Setenv("ORC_PROJECT", "")

	// Change to project dir
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldCwd) })

	id, err := ResolveProjectID()
	if err != nil {
		t.Fatalf("ResolveProjectID failed: %v", err)
	}
	if id != "cwd-project" {
		t.Errorf("expected cwd-project, got %s", id)
	}
}

func TestResolveProjectRef_ByName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "abc123", Name: "my-project", Path: tmpDir},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	id, err := resolveProjectRef("my-project")
	if err != nil {
		t.Fatalf("resolveProjectRef failed: %v", err)
	}
	if id != "abc123" {
		t.Errorf("expected abc123, got %s", id)
	}
}

func TestResolveProjectRef_AmbiguousName(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "proj-1", Name: "dupe", Path: filepath.Join(tmpDir, "a")},
			{ID: "proj-2", Name: "dupe", Path: filepath.Join(tmpDir, "b")},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	_, err := resolveProjectRef("dupe")
	if err == nil {
		t.Error("expected error for ambiguous name")
	}
}

func TestResolveProjectRef_ByPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	if err := os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	reg := &project.Registry{
		Projects: []project.Project{
			{ID: "path-proj", Name: "test", Path: tmpDir},
		},
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	id, err := resolveProjectRef(tmpDir)
	if err != nil {
		t.Fatalf("resolveProjectRef failed: %v", err)
	}
	if id != "path-proj" {
		t.Errorf("expected path-proj, got %s", id)
	}
}

func TestIsMultiProjectMode(t *testing.T) {
	// Clear state
	projectFlag = ""
	t.Setenv("ORC_PROJECT", "")

	if IsMultiProjectMode() {
		t.Error("expected single-project mode")
	}

	projectFlag = "some-project"
	if !IsMultiProjectMode() {
		t.Error("expected multi-project mode with flag")
	}
	projectFlag = ""

	t.Setenv("ORC_PROJECT", "other-project")
	if !IsMultiProjectMode() {
		t.Error("expected multi-project mode with env")
	}
	t.Setenv("ORC_PROJECT", "")
}
