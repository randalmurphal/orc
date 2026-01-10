package hooks

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

	hooks, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hooks) != 0 {
		t.Errorf("expected empty list, got %d hooks", len(hooks))
	}
}

func TestList_WithHooks(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test hook
	hookJSON := `{"name":"test","type":"pre:tool","pattern":"Bash","command":"echo test"}`
	if err := os.WriteFile(filepath.Join(hooksDir, "test.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	hooks, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hooks) != 1 {
		t.Errorf("expected 1 hook, got %d", len(hooks))
	}

	if hooks[0].Name != "test" {
		t.Errorf("expected name 'test', got %q", hooks[0].Name)
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	hooksDir := filepath.Join(tmpDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatal(err)
	}

	hookJSON := `{"name":"test","type":"pre:tool","pattern":"Bash","command":"echo test","timeout":30}`
	if err := os.WriteFile(filepath.Join(hooksDir, "test.json"), []byte(hookJSON), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(tmpDir)
	hook, err := svc.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hook.Name != "test" {
		t.Errorf("expected name 'test', got %q", hook.Name)
	}

	if hook.Type != HookPreTool {
		t.Errorf("expected type 'pre:tool', got %q", hook.Type)
	}

	if hook.Pattern != "Bash" {
		t.Errorf("expected pattern 'Bash', got %q", hook.Pattern)
	}

	if hook.Command != "echo test" {
		t.Errorf("expected command 'echo test', got %q", hook.Command)
	}

	if hook.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", hook.Timeout)
	}
}

func TestGet_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	_, err := svc.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent hook")
	}
}

func TestCreate(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	hook := Hook{
		Name:    "test",
		Type:    HookPreTool,
		Pattern: "Bash",
		Command: "echo test",
	}

	if err := svc.Create(hook); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, "hooks", "test.json")
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %v", err)
	}

	// Verify content
	loaded, err := svc.Get("test")
	if err != nil {
		t.Fatalf("failed to load created hook: %v", err)
	}

	if loaded.Name != hook.Name {
		t.Errorf("expected name %q, got %q", hook.Name, loaded.Name)
	}
}

func TestCreate_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	hook := Hook{
		Name:    "test",
		Type:    HookPreTool,
		Command: "echo test",
	}

	if err := svc.Create(hook); err != nil {
		t.Fatal(err)
	}

	// Try to create again
	err := svc.Create(hook)
	if err == nil {
		t.Error("expected error for duplicate hook")
	}
}

func TestCreate_MissingName(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	hook := Hook{
		Type:    HookPreTool,
		Command: "echo test",
	}

	err := svc.Create(hook)
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestCreate_MissingType(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	hook := Hook{
		Name:    "test",
		Command: "echo test",
	}

	err := svc.Create(hook)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestCreate_MissingCommand(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	hook := Hook{
		Name: "test",
		Type: HookPreTool,
	}

	err := svc.Create(hook)
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// Create initial hook
	hook := Hook{
		Name:    "test",
		Type:    HookPreTool,
		Command: "echo original",
	}
	if err := svc.Create(hook); err != nil {
		t.Fatal(err)
	}

	// Update
	hook.Command = "echo updated"
	if err := svc.Update("test", hook); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify
	loaded, err := svc.Get("test")
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Command != "echo updated" {
		t.Errorf("expected updated command, got %q", loaded.Command)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	hook := Hook{
		Name:    "nonexistent",
		Type:    HookPreTool,
		Command: "echo test",
	}

	err := svc.Update("nonexistent", hook)
	if err == nil {
		t.Error("expected error for nonexistent hook")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// Create hook
	hook := Hook{
		Name:    "test",
		Type:    HookPreTool,
		Command: "echo test",
	}
	if err := svc.Create(hook); err != nil {
		t.Fatal(err)
	}

	// Delete
	if err := svc.Delete("test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify deleted
	if svc.Exists("test") {
		t.Error("expected hook to be deleted")
	}
}

func TestDelete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	err := svc.Delete("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent hook")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(tmpDir)

	// Initially doesn't exist
	if svc.Exists("test") {
		t.Error("expected hook to not exist initially")
	}

	// Create hook
	hook := Hook{
		Name:    "test",
		Type:    HookPreTool,
		Command: "echo test",
	}
	if err := svc.Create(hook); err != nil {
		t.Fatal(err)
	}

	// Now exists
	if !svc.Exists("test") {
		t.Error("expected hook to exist after creation")
	}
}

func TestGetHookTypes(t *testing.T) {
	types := GetHookTypes()
	if len(types) == 0 {
		t.Error("expected at least one hook type")
	}

	// Check for required types
	expected := map[HookType]bool{
		HookPreTool:      false,
		HookPostTool:     false,
		HookPromptSubmit: false,
	}

	for _, ht := range types {
		if _, ok := expected[ht]; ok {
			expected[ht] = true
		}
	}

	for ht, found := range expected {
		if !found {
			t.Errorf("expected hook type %q to be present", ht)
		}
	}
}
