package config

import (
	"testing"
	"time"
)

func TestConfig_GetValue(t *testing.T) {
	cfg := Default()
	cfg.Profile = ProfileSafe
	cfg.Model = "test-model"
	cfg.MaxIterations = 50
	cfg.Gates.DefaultType = "human"
	cfg.Retry.Enabled = false

	tests := []struct {
		path    string
		want    string
		wantErr bool
	}{
		{"profile", "safe", false},
		{"model", "test-model", false},
		{"max_iterations", "50", false},
		{"gates.default_type", "human", false},
		{"retry.enabled", "false", false},
		{"timeout", "10m0s", false},
		{"branch_prefix", "orc/", false},
		// Nested values
		{"worktree.enabled", "true", false},
		{"completion.action", "pr", false},
		{"completion.pr.title", "[orc] {{TASK_TITLE}}", false},
		{"server.host", "127.0.0.1", false},
		{"server.auth.enabled", "false", false},
		// Invalid paths
		{"nonexistent", "", true},
		{"gates.nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := cfg.GetValue(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValue(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetValue(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestConfig_SetValue(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		value   string
		check   func(*Config) bool
		wantErr bool
	}{
		{
			name:  "set string",
			path:  "model",
			value: "new-model",
			check: func(c *Config) bool { return c.Model == "new-model" },
		},
		{
			name:  "set profile",
			path:  "profile",
			value: "strict",
			check: func(c *Config) bool { return c.Profile == ProfileStrict },
		},
		{
			name:  "set int",
			path:  "max_iterations",
			value: "100",
			check: func(c *Config) bool { return c.MaxIterations == 100 },
		},
		{
			name:  "set bool true",
			path:  "retry.enabled",
			value: "true",
			check: func(c *Config) bool { return c.Retry.Enabled },
		},
		{
			name:  "set bool false",
			path:  "worktree.enabled",
			value: "false",
			check: func(c *Config) bool { return !c.Worktree.Enabled },
		},
		{
			name:  "set duration",
			path:  "timeout",
			value: "30m",
			check: func(c *Config) bool { return c.Timeout == 30*time.Minute },
		},
		{
			name:  "set nested string",
			path:  "gates.default_type",
			value: "ai",
			check: func(c *Config) bool { return c.Gates.DefaultType == "ai" },
		},
		{
			name:  "set nested int",
			path:  "gates.max_retries",
			value: "5",
			check: func(c *Config) bool { return c.Gates.MaxRetries == 5 },
		},
		{
			name:  "set server port",
			path:  "server.port",
			value: "9000",
			check: func(c *Config) bool { return c.Server.Port == 9000 },
		},
		{
			name:  "set deeply nested",
			path:  "completion.pr.title",
			value: "New Title",
			check: func(c *Config) bool { return c.Completion.PR.Title == "New Title" },
		},
		{
			name:    "invalid path",
			path:    "nonexistent",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid nested path",
			path:    "gates.nonexistent",
			value:   "value",
			wantErr: true,
		},
		{
			name:    "invalid int",
			path:    "max_iterations",
			value:   "not-a-number",
			wantErr: true,
		},
		{
			name:    "invalid duration",
			path:    "timeout",
			value:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			err := cfg.SetValue(tt.path, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetValue(%q, %q) error = %v, wantErr %v", tt.path, tt.value, err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil && !tt.check(cfg) {
				t.Errorf("SetValue(%q, %q) did not set correctly", tt.path, tt.value)
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"string", "hello", "hello"},
		{"int", 42, "42"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"duration", 10 * time.Minute, "10m0s"},
		{"string slice", []string{"a", "b"}, "a, b"},
		{"empty slice", []string{}, "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()

			// Test via round-trip through GetValue
			switch v := tt.input.(type) {
			case string:
				cfg.Model = v
				got, _ := cfg.GetValue("model")
				if got != tt.want {
					t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
				}
			case int:
				cfg.MaxIterations = v
				got, _ := cfg.GetValue("max_iterations")
				if got != tt.want {
					t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
				}
			case bool:
				cfg.Retry.Enabled = v
				got, _ := cfg.GetValue("retry.enabled")
				if got != tt.want {
					t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
				}
			case time.Duration:
				cfg.Timeout = v
				got, _ := cfg.GetValue("timeout")
				if got != tt.want {
					t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
				}
			case []string:
				cfg.Completion.PR.Labels = v
				got, _ := cfg.GetValue("completion.pr.labels")
				if got != tt.want {
					t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
				}
			}
		})
	}
}

func TestAllConfigPaths(t *testing.T) {
	paths := AllConfigPaths()

	// Check some expected paths exist
	expectedPaths := []string{
		"profile",
		"model",
		"max_iterations",
		"timeout",
		"gates.default_type",
		"retry.enabled",
		"worktree.enabled",
		"completion.action",
		"completion.pr.title",
		"server.host",
		"server.port",
		"team.activity_logging",
	}

	pathSet := make(map[string]bool)
	for _, p := range paths {
		pathSet[p] = true
	}

	for _, expected := range expectedPaths {
		if !pathSet[expected] {
			t.Errorf("AllConfigPaths() missing %q", expected)
		}
	}
}

func TestConfig_SetValue_Labels(t *testing.T) {
	cfg := Default()

	// Set labels as comma-separated string
	err := cfg.SetValue("completion.pr.labels", "automated, bug-fix, priority")
	if err != nil {
		t.Fatalf("SetValue failed: %v", err)
	}

	if len(cfg.Completion.PR.Labels) != 3 {
		t.Errorf("Labels length = %d, want 3", len(cfg.Completion.PR.Labels))
	}

	// Check values (should be trimmed)
	expected := []string{"automated", "bug-fix", "priority"}
	for i, want := range expected {
		if i >= len(cfg.Completion.PR.Labels) {
			break
		}
		if cfg.Completion.PR.Labels[i] != want {
			t.Errorf("Labels[%d] = %q, want %q", i, cfg.Completion.PR.Labels[i], want)
		}
	}
}
