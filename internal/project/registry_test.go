package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	// Create temp dir for testing
	tmpDir := t.TempDir()

	// Create a fake project directory
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	// Create a registry
	reg := &Registry{Projects: []Project{}}

	// Register project
	proj, err := reg.Register(projectDir)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	if proj.Name != "test-project" {
		t.Errorf("Name = %s, want test-project", proj.Name)
	}

	if proj.Path != projectDir {
		t.Errorf("Path = %s, want %s", proj.Path, projectDir)
	}

	if proj.ID == "" {
		t.Error("ID should not be empty")
	}

	// List projects
	projects := reg.List()
	if len(projects) != 1 {
		t.Errorf("List() returned %d projects, want 1", len(projects))
	}

	// Get by ID
	found, err := reg.Get(proj.ID)
	if err != nil {
		t.Fatalf("Get() by ID failed: %v", err)
	}
	if found.Path != projectDir {
		t.Errorf("Get() returned wrong project")
	}

	// Get by path
	found, err = reg.Get(projectDir)
	if err != nil {
		t.Fatalf("Get() by path failed: %v", err)
	}
	if found.ID != proj.ID {
		t.Errorf("Get() returned wrong project")
	}

	// Re-register same project (should update, not duplicate)
	_, err = reg.Register(projectDir)
	if err != nil {
		t.Fatalf("Re-register failed: %v", err)
	}
	if len(reg.Projects) != 1 {
		t.Errorf("Re-register created duplicate: %d projects", len(reg.Projects))
	}

	// Unregister
	err = reg.Unregister(proj.ID)
	if err != nil {
		t.Fatalf("Unregister() failed: %v", err)
	}
	if len(reg.Projects) != 0 {
		t.Errorf("Unregister() didn't remove project")
	}
}

func TestRegistryInvalidPath(t *testing.T) {
	reg := &Registry{Projects: []Project{}}

	_, err := reg.Register("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("Register() should fail for nonexistent path")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	reg := &Registry{Projects: []Project{}}

	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Error("Get() should fail for nonexistent project")
	}
}

func TestValidProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create one valid directory
	validDir := filepath.Join(tmpDir, "valid")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("create valid dir: %v", err)
	}

	reg := &Registry{
		Projects: []Project{
			{ID: "valid", Name: "valid", Path: validDir},
			{ID: "invalid", Name: "invalid", Path: "/nonexistent/path"},
		},
	}

	valid := reg.ValidProjects()
	if len(valid) != 1 {
		t.Errorf("ValidProjects() returned %d, want 1", len(valid))
	}
	if valid[0].ID != "valid" {
		t.Errorf("ValidProjects() returned wrong project")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID("/path/one")
	id2 := generateID("/path/two")
	id3 := generateID("/path/one")

	if id1 == id2 {
		t.Error("Different paths should generate different IDs")
	}

	if id1 != id3 {
		t.Error("Same path should generate same ID")
	}

	if len(id1) != 8 {
		t.Errorf("ID length = %d, want 8", len(id1))
	}
}
