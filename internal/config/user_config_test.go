package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// SC-9: UserConfig struct in config_types.go with Name and Email fields
// =============================================================================

func TestUserConfig_HasNameField(t *testing.T) {
	t.Parallel()

	cfg := UserConfig{
		Name: "alice",
	}

	if cfg.Name != "alice" {
		t.Errorf("Name = %q, want alice", cfg.Name)
	}
}

func TestUserConfig_HasEmailField(t *testing.T) {
	t.Parallel()

	cfg := UserConfig{
		Email: "alice@example.com",
	}

	if cfg.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", cfg.Email)
	}
}

func TestUserConfig_YAMLTags(t *testing.T) {
	t.Parallel()

	// Marshal a UserConfig to YAML
	cfg := UserConfig{
		Name:  "bob",
		Email: "bob@example.com",
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal failed: %v", err)
	}

	yamlStr := string(data)

	// Verify YAML keys match expected tags
	if !containsSubstring(yamlStr, "name:") {
		t.Errorf("YAML missing 'name:' key, got: %s", yamlStr)
	}
	if !containsSubstring(yamlStr, "email:") {
		t.Errorf("YAML missing 'email:' key, got: %s", yamlStr)
	}
}

func TestUserConfig_UnmarshalFromYAML(t *testing.T) {
	t.Parallel()

	yamlData := `
name: charlie
email: charlie@example.com
`

	var cfg UserConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	if cfg.Name != "charlie" {
		t.Errorf("Name = %q, want charlie", cfg.Name)
	}
	if cfg.Email != "charlie@example.com" {
		t.Errorf("Email = %q, want charlie@example.com", cfg.Email)
	}
}

func TestUserConfig_EmptyEmailAllowed(t *testing.T) {
	t.Parallel()

	// Email is optional per the spec
	cfg := UserConfig{
		Name:  "diana",
		Email: "", // Empty is allowed
	}

	if cfg.Name != "diana" {
		t.Errorf("Name = %q, want diana", cfg.Name)
	}
	// Empty email should be allowed (it's optional)
	if cfg.Email != "" {
		t.Errorf("Email = %q, want empty string", cfg.Email)
	}
}

// helper function
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
