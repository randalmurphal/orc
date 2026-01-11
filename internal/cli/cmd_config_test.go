package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestConfigShowCmd_OutputsValidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .orc directory with config
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	configContent := `version: 1
profile: safe
model: test-model
`
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Change to temp dir
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Capture output
	var buf bytes.Buffer
	cmd := newConfigShowCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}

	output := buf.String()

	// Check it contains expected YAML keys
	if !strings.Contains(output, "version:") {
		t.Error("Output missing 'version:' key")
	}
	if !strings.Contains(output, "model:") {
		t.Error("Output missing 'model:' key")
	}
	if !strings.Contains(output, "profile:") {
		t.Error("Output missing 'profile:' key")
	}
}

func TestConfigShowCmd_WithSource(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	configContent := `model: custom-model
`
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	var buf bytes.Buffer
	cmd := newConfigShowCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--source"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config show --source failed: %v", err)
	}

	output := buf.String()

	// Check format: key = value (source)
	if !strings.Contains(output, "model =") {
		t.Error("Output missing 'model =' format")
	}
	// Should show source in parentheses
	if !strings.Contains(output, "(") || !strings.Contains(output, ")") {
		t.Error("Output missing source annotation in parentheses")
	}
}

func TestConfigGetCmd_RetrievesNestedKeys(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	configContent := `gates:
  default_type: human
retry:
  enabled: true
`
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	tests := []struct {
		key  string
		want string
	}{
		{"gates.default_type", "human"},
		{"retry.enabled", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			var buf bytes.Buffer
			cmd := newConfigGetCmd()
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{tt.key})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("config get %s failed: %v", tt.key, err)
			}

			got := strings.TrimSpace(buf.String())
			if got != tt.want {
				t.Errorf("config get %s = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestConfigGetCmd_WithSource(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	configContent := `model: test-model
`
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	var buf bytes.Buffer
	cmd := newConfigGetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"model", "--source"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config get model --source failed: %v", err)
	}

	output := buf.String()

	// Should contain value and source
	if !strings.Contains(output, "test-model") {
		t.Error("Output missing value")
	}
	if !strings.Contains(output, "(from") {
		t.Error("Output missing source annotation")
	}
}

func TestConfigSetCmd_WritesToCorrectFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .orc directory
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Test setting to project config
	var buf bytes.Buffer
	cmd := newConfigSetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--project", "model", "new-model"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config set --project failed: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(orcDir, config.ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	if !strings.Contains(string(data), "new-model") {
		t.Error("Config file missing set value")
	}

	// Verify output message
	output := buf.String()
	if !strings.Contains(output, "Set model = new-model") {
		t.Error("Missing confirmation message")
	}
	if !strings.Contains(output, ".orc/config.yaml") {
		t.Error("Missing target file in output")
	}
}

func TestConfigSetCmd_WritesToShared(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	sharedDir := filepath.Join(orcDir, "shared")
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("create shared dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	var buf bytes.Buffer
	cmd := newConfigSetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--shared", "profile", "strict"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config set --shared failed: %v", err)
	}

	// Verify file was created in shared directory
	configPath := filepath.Join(sharedDir, config.ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read shared config: %v", err)
	}

	if !strings.Contains(string(data), "strict") {
		t.Error("Shared config missing set value")
	}
}

func TestConfigResolutionCmd_ShowsAllLevels(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	configContent := `model: project-model
`
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	var buf bytes.Buffer
	cmd := newConfigResolutionCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"model"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config resolution failed: %v", err)
	}

	output := buf.String()

	// Check it shows the key
	if !strings.Contains(output, "Resolution chain for 'model'") {
		t.Error("Missing header")
	}

	// Check it shows level names
	if !strings.Contains(output, "DEFAULT") {
		t.Error("Missing DEFAULT level")
	}

	// Check it shows final value
	if !strings.Contains(output, "Final value:") {
		t.Error("Missing final value")
	}
}

func TestLevelPriority(t *testing.T) {
	tests := []struct {
		level config.ConfigLevel
		want  string
	}{
		{config.LevelRuntime, "highest priority"},
		{config.LevelPersonal, "second priority"},
		{config.LevelShared, "third priority"},
		{config.LevelDefaults, "lowest priority"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			got := levelPriority(tt.level)
			if got != tt.want {
				t.Errorf("levelPriority(%v) = %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

func TestFormatResolutionPath(t *testing.T) {
	tests := []struct {
		name  string
		entry config.ResolutionEntry
		want  string
	}{
		{
			name: "env var",
			entry: config.ResolutionEntry{
				Source: config.SourceEnv,
				Path:   "ORC_MODEL",
			},
			want: "env (ORC_MODEL)",
		},
		{
			name: "flag",
			entry: config.ResolutionEntry{
				Source: config.SourceFlag,
				Path:   "--model",
			},
			want: "flags (--model)",
		},
		{
			name: "file path",
			entry: config.ResolutionEntry{
				Source: config.SourcePersonal,
				Path:   "~/.orc/config.yaml",
			},
			want: "~/.orc/config.yaml",
		},
		{
			name: "shared file",
			entry: config.ResolutionEntry{
				Source: config.SourceShared,
				Path:   ".orc/config.yaml",
			},
			want: ".orc/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResolutionPath(tt.entry)
			if got != tt.want {
				t.Errorf("formatResolutionPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigGetCmd_InvalidKey(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	cmd := newConfigGetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"nonexistent.key.path"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid key, got nil")
	}
}

func TestConfigSetCmd_InvalidKey(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	cmd := newConfigSetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"nonexistent.key.path", "value"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid key, got nil")
	}
}

func TestConfigSetCmd_MutuallyExclusiveFlags(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	cmd := newConfigSetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--project", "--shared", "model", "test"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") && !strings.Contains(err.Error(), "if any flags in the group") {
		t.Errorf("expected mutual exclusivity error, got: %v", err)
	}
}

func TestConfigSetCmd_WritesToUser(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .orc directory for project
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	// Create fake home dir for user config
	homeDir := filepath.Join(tmpDir, "home")
	userOrcDir := filepath.Join(homeDir, ".orc")
	if err := os.MkdirAll(userOrcDir, 0755); err != nil {
		t.Fatalf("create user .orc dir: %v", err)
	}

	// Save old HOME and set new
	oldHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	defer func() { _ = os.Setenv("HOME", oldHome) }()

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	cmd := newConfigSetCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--user", "model", "user-model"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config set --user failed: %v", err)
	}

	// Verify file was created in user directory
	configPath := filepath.Join(userOrcDir, config.ConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read user config: %v", err)
	}

	if !strings.Contains(string(data), "user-model") {
		t.Error("User config missing set value")
	}

	// Verify output message
	output := buf.String()
	if !strings.Contains(output, "Set model = user-model") {
		t.Error("Missing confirmation message")
	}
	if !strings.Contains(output, "~/.orc/config.yaml") {
		t.Error("Missing target file in output")
	}
}

func TestConfigResolutionCmd_UnknownKey(t *testing.T) {
	// Resolution for unknown keys shows the chain with empty values
	// This is expected behavior - it doesn't error, just shows nothing is set
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	cmd := newConfigResolutionCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"nonexistent.key.path"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	// Should still show the resolution chain header
	if !strings.Contains(output, "Resolution chain for 'nonexistent.key.path'") {
		t.Error("Missing resolution chain header")
	}
	// Final value should be empty
	if !strings.Contains(output, "Final value:") {
		t.Error("Missing final value line")
	}
}

func TestConfigResolutionCmd_FormatsRuntimeEntries(t *testing.T) {
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer func() { _ = os.Chdir(oldWd) }()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	cmd := newConfigResolutionCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"model"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("config resolution failed: %v", err)
	}

	output := buf.String()

	// Check format matches spec: "env (ORC_MODEL):" and "flags (--model):"
	if !strings.Contains(output, "env (ORC_MODEL)") {
		t.Error("Output missing 'env (ORC_MODEL)' format")
	}
	if !strings.Contains(output, "flags (--model)") {
		t.Error("Output missing 'flags (--model)' format")
	}
}

func TestConfigEditCmd_MutuallyExclusiveFlags(t *testing.T) {
	var buf bytes.Buffer
	cmd := newConfigEditCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--project", "--shared"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for mutually exclusive flags, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") && !strings.Contains(err.Error(), "if any flags in the group") {
		t.Errorf("expected mutual exclusivity error, got: %v", err)
	}
}
