package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService(".claude")
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.claudeDir != ".claude" {
		t.Errorf("expected claudeDir '.claude', got %q", svc.claudeDir)
	}
}

func TestDefaultService(t *testing.T) {
	svc := DefaultService()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestList_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	skills, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != 0 {
		t.Errorf("expected empty list, got %d skills", len(skills))
	}
}

func TestList_WithSkills(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test skill
	skillYAML := "name: test\ndescription: Test skill\nprompt: Do something"
	if err := os.WriteFile(filepath.Join(skillsDir, "test.yaml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	skills, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}

	if skills[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", skills[0].Name)
	}
}

func TestList_WithYmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test skill with .yml extension
	skillYAML := "name: test-yml\ndescription: Test yml skill\nprompt: Do something"
	if err := os.WriteFile(filepath.Join(skillsDir, "test-yml.yml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	skills, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("expected 1 skill, got %d", len(skills))
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillYAML := "name: test\ndescription: Test skill\nprompt: Do something useful"
	if err := os.WriteFile(filepath.Join(skillsDir, "test.yaml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	skill, err := svc.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name != "test" {
		t.Errorf("expected name 'test', got %q", skill.Name)
	}

	if skill.Description != "Test skill" {
		t.Errorf("expected description 'Test skill', got %q", skill.Description)
	}

	if skill.Prompt != "Do something useful" {
		t.Errorf("expected prompt 'Do something useful', got %q", skill.Prompt)
	}
}

func TestGet_YmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillYAML := "name: test-yml\ndescription: Test yml\nprompt: Prompt"
	if err := os.WriteFile(filepath.Join(skillsDir, "test-yml.yml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	skill, err := svc.Get("test-yml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if skill.Name != "test-yml" {
		t.Errorf("expected name 'test-yml', got %q", skill.Name)
	}
}

func TestGet_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestGet_DefaultsNameFromFilename(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Skill without name field
	skillYAML := "description: No name field\nprompt: Do something"
	if err := os.WriteFile(filepath.Join(skillsDir, "unnamed.yaml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	skill, err := svc.Get("unnamed")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Name should be inferred from filename
	if skill.Name != "unnamed" {
		t.Errorf("expected name 'unnamed', got %q", skill.Name)
	}
}

func TestCreate(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	skill := Skill{
		Name:        "test",
		Description: "Test skill",
		Prompt:      "Do something",
	}

	if err := svc.Create(skill); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, "skills", "test.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}

	// Verify content
	loaded, err := svc.Get("test")
	if err != nil {
		t.Fatalf("failed to load created skill: %v", err)
	}

	if loaded.Name != skill.Name {
		t.Errorf("expected name %q, got %q", skill.Name, loaded.Name)
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	skill := Skill{
		Name:   "test",
		Prompt: "Do something",
	}

	if err := svc.Create(skill); err != nil {
		t.Fatal(err)
	}

	// Try to create again
	err := svc.Create(skill)
	if err == nil {
		t.Error("expected error for duplicate skill")
	}
}

func TestCreate_MissingName(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	skill := Skill{
		Prompt: "Do something",
	}

	err := svc.Create(skill)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestCreate_MissingPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	skill := Skill{
		Name: "test",
	}

	err := svc.Create(skill)
	if err == nil {
		t.Error("expected error for missing prompt")
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// Create initial skill
	skill := Skill{
		Name:        "test",
		Description: "Original description",
		Prompt:      "Original prompt",
	}
	if err := svc.Create(skill); err != nil {
		t.Fatal(err)
	}

	// Update
	skill.Description = "Updated description"
	skill.Prompt = "Updated prompt"
	if err := svc.Update("test", skill); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify
	loaded, err := svc.Get("test")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Description != "Updated description" {
		t.Errorf("expected updated description, got %q", loaded.Description)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	skill := Skill{
		Name:   "nonexistent",
		Prompt: "Prompt",
	}

	err := svc.Update("nonexistent", skill)
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// Create skill
	skill := Skill{
		Name:   "test",
		Prompt: "Prompt",
	}
	if err := svc.Create(skill); err != nil {
		t.Fatal(err)
	}

	// Delete
	if err := svc.Delete("test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deleted
	if svc.Exists("test") {
		t.Error("expected skill to be deleted")
	}
}

func TestDelete_YmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .yml file directly
	skillYAML := "name: test-yml\nprompt: Prompt"
	if err := os.WriteFile(filepath.Join(skillsDir, "test-yml.yml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)

	// Delete
	if err := svc.Delete("test-yml"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deleted
	if svc.Exists("test-yml") {
		t.Error("expected skill to be deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	err := svc.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// Initially doesn't exist
	if svc.Exists("test") {
		t.Error("expected skill to not exist initially")
	}

	// Create skill
	skill := Skill{
		Name:   "test",
		Prompt: "Prompt",
	}
	if err := svc.Create(skill); err != nil {
		t.Fatal(err)
	}

	// Now exists
	if !svc.Exists("test") {
		t.Error("expected skill to exist after creation")
	}
}

func TestExists_YmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create .yml file directly
	skillYAML := "name: test-yml\nprompt: Prompt"
	if err := os.WriteFile(filepath.Join(skillsDir, "test-yml.yml"), []byte(skillYAML), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)

	if !svc.Exists("test-yml") {
		t.Error("expected .yml skill to exist")
	}
}
