package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_GetResolutionChain(t *testing.T) {
	// Create temp directories for test
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	userDir := filepath.Join(tmpDir, "user")

	// Create directory structure
	os.MkdirAll(filepath.Join(projectDir, OrcDir), 0755)
	os.MkdirAll(filepath.Join(projectDir, OrcDir, "shared"), 0755)
	os.MkdirAll(filepath.Join(projectDir, OrcDir, "local"), 0755)
	os.MkdirAll(userDir, 0755)

	// Write shared config with custom model
	sharedConfig := `model: shared-model
profile: safe
`
	err := os.WriteFile(filepath.Join(projectDir, OrcDir, ConfigFileName), []byte(sharedConfig), 0644)
	if err != nil {
		t.Fatalf("write shared config: %v", err)
	}

	// Write personal config with different model
	personalConfig := `model: personal-model
`
	err = os.WriteFile(filepath.Join(userDir, ConfigFileName), []byte(personalConfig), 0644)
	if err != nil {
		t.Fatalf("write personal config: %v", err)
	}

	// Create loader with custom paths (no os.Chdir needed)
	loader := &Loader{
		projectDir: projectDir,
		userDir:    userDir,
	}

	// Get resolution chain for model
	chain, err := loader.GetResolutionChain("model")
	if err != nil {
		t.Fatalf("GetResolutionChain: %v", err)
	}

	// Verify chain has entries
	if len(chain.Entries) == 0 {
		t.Fatal("Expected entries in resolution chain")
	}

	// Verify key
	if chain.Key != "model" {
		t.Errorf("Key = %q, want %q", chain.Key, "model")
	}

	// Verify final value comes from personal (highest non-runtime)
	if chain.FinalValue != "personal-model" {
		t.Errorf("FinalValue = %q, want %q", chain.FinalValue, "personal-model")
	}

	// Verify we have entries at different levels
	levelCounts := make(map[ConfigLevel]int)
	for _, e := range chain.Entries {
		levelCounts[e.Level]++
	}

	if levelCounts[LevelDefaults] == 0 {
		t.Error("Missing defaults level entries")
	}
	if levelCounts[LevelShared] == 0 {
		t.Error("Missing shared level entries")
	}
	if levelCounts[LevelPersonal] == 0 {
		t.Error("Missing personal level entries")
	}
}

func TestLoader_GetResolutionChain_EnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	userDir := filepath.Join(tmpDir, "user")

	os.MkdirAll(filepath.Join(projectDir, OrcDir), 0755)
	os.MkdirAll(userDir, 0755)

	// Write config with model
	config := `model: file-model
`
	os.WriteFile(filepath.Join(projectDir, OrcDir, ConfigFileName), []byte(config), 0644)

	// Set env var override
	t.Setenv("ORC_MODEL", "env-model")

	loader := &Loader{
		projectDir: projectDir,
		userDir:    userDir,
	}

	chain, err := loader.GetResolutionChain("model")
	if err != nil {
		t.Fatalf("GetResolutionChain: %v", err)
	}

	// Env should win
	if chain.FinalValue != "env-model" {
		t.Errorf("FinalValue = %q, want %q (env should override)", chain.FinalValue, "env-model")
	}

	// Verify env entry is marked as set
	var envEntry *ResolutionEntry
	for i := range chain.Entries {
		if chain.Entries[i].Source == SourceEnv {
			envEntry = &chain.Entries[i]
			break
		}
	}

	if envEntry == nil {
		t.Fatal("Missing env entry in chain")
	}

	if !envEntry.IsSet {
		t.Error("Env entry should be marked as set")
	}

	if envEntry.Value != "env-model" {
		t.Errorf("Env entry value = %q, want %q", envEntry.Value, "env-model")
	}
}

func TestResolutionEntry_Levels(t *testing.T) {
	tests := []struct {
		level ConfigLevel
		want  string
	}{
		{LevelDefaults, "default"},
		{LevelShared, "shared"},
		{LevelPersonal, "personal"},
		{LevelRuntime, "runtime"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.level.String(); got != tt.want {
				t.Errorf("ConfigLevel.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetEnvVarForPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"profile", "ORC_PROFILE"},
		{"model", "ORC_MODEL"},
		{"max_iterations", "ORC_MAX_ITERATIONS"},
		{"timeout", "ORC_TIMEOUT"},
		{"retry.enabled", "ORC_RETRY_ENABLED"},
		{"gates.default_type", "ORC_GATES_DEFAULT"},
		{"worktree.enabled", "ORC_WORKTREE_ENABLED"},
		{"nonexistent", ""}, // No env var for this
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := getEnvVarForPath(tt.path)
			if got != tt.want {
				t.Errorf("getEnvVarForPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitKeyPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"model", []string{"model"}},
		{"gates.default_type", []string{"gates", "default_type"}},
		{"completion.pr.title", []string{"completion", "pr", "title"}},
		{"server.auth.enabled", []string{"server", "auth", "enabled"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitKeyPath(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitKeyPath(%q) = %v, want %v", tt.input, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitKeyPath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
