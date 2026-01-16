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

func TestRegistrySave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	reg := &Registry{Projects: []Project{}}
	if _, err := reg.Register(projectDir); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err := reg.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	regPath, _ := RegistryPath()
	if _, err := os.Stat(regPath); os.IsNotExist(err) {
		t.Error("Save() did not create registry file")
	}
}

func TestLoadRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	reg := &Registry{Projects: []Project{}}
	if _, err := reg.Register(projectDir); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := reg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loadedReg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() failed: %v", err)
	}

	if len(loadedReg.Projects) != 1 {
		t.Errorf("LoadRegistry() returned %d projects, want 1", len(loadedReg.Projects))
	}
}

func TestLoadRegistry_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	reg, err := LoadRegistry()
	if err != nil {
		t.Fatalf("LoadRegistry() failed: %v", err)
	}

	if len(reg.Projects) != 0 {
		t.Errorf("LoadRegistry() returned %d projects for empty, want 0", len(reg.Projects))
	}
}

func TestLoadRegistry_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	orcDir := filepath.Join(tmpDir, GlobalDir)
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, RegistryFile), []byte("invalid: yaml: [broken"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := LoadRegistry()
	if err == nil {
		t.Error("LoadRegistry() should fail for invalid YAML")
	}
}

func TestGlobalPath(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	path, err := GlobalPath()
	if err != nil {
		t.Fatalf("GlobalPath() failed: %v", err)
	}

	expected := filepath.Join(tmpDir, GlobalDir)
	if path != expected {
		t.Errorf("GlobalPath() = %s, want %s", path, expected)
	}
}

func TestRegistryPath(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	path, err := RegistryPath()
	if err != nil {
		t.Fatalf("RegistryPath() failed: %v", err)
	}

	expected := filepath.Join(tmpDir, GlobalDir, RegistryFile)
	if path != expected {
		t.Errorf("RegistryPath() = %s, want %s", path, expected)
	}
}

func TestRegisterProject(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	projectDir := filepath.Join(tmpDir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	proj, err := RegisterProject(projectDir)
	if err != nil {
		t.Fatalf("RegisterProject() failed: %v", err)
	}

	if proj.Name != "my-project" {
		t.Errorf("Name = %s, want my-project", proj.Name)
	}

	reg, _ := LoadRegistry()
	if len(reg.Projects) != 1 {
		t.Errorf("RegisterProject() did not save to registry")
	}
}

func TestListProjects(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	proj1Dir := filepath.Join(tmpDir, "project1")
	proj2Dir := filepath.Join(tmpDir, "project2")
	if err := os.MkdirAll(proj1Dir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(proj2Dir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	if _, err := RegisterProject(proj1Dir); err != nil {
		t.Fatalf("RegisterProject failed: %v", err)
	}
	if _, err := RegisterProject(proj2Dir); err != nil {
		t.Fatalf("RegisterProject failed: %v", err)
	}

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() failed: %v", err)
	}

	if len(projects) != 2 {
		t.Errorf("ListProjects() returned %d projects, want 2", len(projects))
	}
}

func TestRegister_File(t *testing.T) {
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	reg := &Registry{Projects: []Project{}}
	_, err := reg.Register(filePath)
	if err == nil {
		t.Error("Register() should fail for file (not directory)")
	}
}

func TestUnregister_ByPath(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	_ = os.MkdirAll(projectDir, 0755)

	reg := &Registry{Projects: []Project{}}
	_, _ = reg.Register(projectDir)

	err := reg.Unregister(projectDir)
	if err != nil {
		t.Fatalf("Unregister() by path failed: %v", err)
	}

	if len(reg.Projects) != 0 {
		t.Error("Unregister() by path did not remove project")
	}
}

func TestUnregister_NotFound(t *testing.T) {
	reg := &Registry{Projects: []Project{}}

	err := reg.Unregister("nonexistent")
	if err == nil {
		t.Error("Unregister() should fail for nonexistent project")
	}
}

func TestDefaultProject(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("create project dir: %v", err)
	}

	reg := &Registry{Projects: []Project{}}

	// Register a project
	proj, err := reg.Register(projectDir)
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}

	// Initially no default
	if reg.GetDefault() != "" {
		t.Error("GetDefault() should return empty string initially")
	}

	// Set default project
	err = reg.SetDefault(proj.ID)
	if err != nil {
		t.Fatalf("SetDefault() failed: %v", err)
	}

	if reg.GetDefault() != proj.ID {
		t.Errorf("GetDefault() = %s, want %s", reg.GetDefault(), proj.ID)
	}

	// Clear default
	err = reg.SetDefault("")
	if err != nil {
		t.Fatalf("SetDefault('') failed: %v", err)
	}

	if reg.GetDefault() != "" {
		t.Error("GetDefault() should return empty string after clearing")
	}
}

func TestDefaultProject_NotFound(t *testing.T) {
	reg := &Registry{Projects: []Project{}}

	err := reg.SetDefault("nonexistent-id")
	if err == nil {
		t.Error("SetDefault() should fail for nonexistent project")
	}
}

func TestDefaultProject_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	projectDir := filepath.Join(tmpDir, "my-project")
	_ = os.MkdirAll(projectDir, 0755)

	// Register and set default
	proj, err := RegisterProject(projectDir)
	if err != nil {
		t.Fatalf("RegisterProject() failed: %v", err)
	}

	err = SetDefaultProject(proj.ID)
	if err != nil {
		t.Fatalf("SetDefaultProject() failed: %v", err)
	}

	// Load fresh and verify
	defaultID, err := GetDefaultProject()
	if err != nil {
		t.Fatalf("GetDefaultProject() failed: %v", err)
	}

	if defaultID != proj.ID {
		t.Errorf("GetDefaultProject() = %s, want %s", defaultID, proj.ID)
	}
}

func TestDefaultProject_SetNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", tmpDir)
	defer func() { _ = os.Setenv("HOME", origHome) }()

	err := SetDefaultProject("nonexistent-id")
	if err == nil {
		t.Error("SetDefaultProject() should fail for nonexistent project")
	}
}
