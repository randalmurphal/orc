package variable

import "testing"

func TestInterpolateString(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"TASK_ID": "TASK-001",
		"NAME":    "test",
		"PHASE":   "implement",
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single variable",
			input:    "--id={{TASK_ID}}",
			expected: "--id=TASK-001",
		},
		{
			name:     "multiple variables",
			input:    "{{NAME}}-{{TASK_ID}}",
			expected: "test-TASK-001",
		},
		{
			name:     "missing variable becomes empty",
			input:    "{{MISSING}}",
			expected: "",
		},
		{
			name:     "no variables",
			input:    "no vars here",
			expected: "no vars here",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "partial match not replaced",
			input:    "{TASK_ID}",
			expected: "{TASK_ID}",
		},
		{
			name:     "variable in URL",
			input:    "https://api.example.com/tasks/{{TASK_ID}}/status",
			expected: "https://api.example.com/tasks/TASK-001/status",
		},
		{
			name:     "variable at start",
			input:    "{{PHASE}}: running",
			expected: "implement: running",
		},
		{
			name:     "variable at end",
			input:    "Current phase is {{PHASE}}",
			expected: "Current phase is implement",
		},
		{
			name:     "mixed present and missing",
			input:    "{{TASK_ID}}-{{MISSING}}-{{NAME}}",
			expected: "TASK-001--test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := interpolateString(tt.input, vars)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestScriptConfigInterpolate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"TASK_ID":      "TASK-001",
		"PROJECT_ROOT": "/home/user/project",
	}

	cfg := &ScriptConfig{
		Path:    "scripts/{{TASK_ID}}.sh",
		Args:    []string{"--task", "{{TASK_ID}}", "--root", "{{PROJECT_ROOT}}"},
		WorkDir: "{{PROJECT_ROOT}}/work",
	}

	cfg.Interpolate(vars)

	if cfg.Path != "scripts/TASK-001.sh" {
		t.Errorf("Path: expected %q, got %q", "scripts/TASK-001.sh", cfg.Path)
	}
	if cfg.WorkDir != "/home/user/project/work" {
		t.Errorf("WorkDir: expected %q, got %q", "/home/user/project/work", cfg.WorkDir)
	}
	if len(cfg.Args) != 4 {
		t.Fatalf("Args: expected 4, got %d", len(cfg.Args))
	}
	if cfg.Args[1] != "TASK-001" {
		t.Errorf("Args[1]: expected %q, got %q", "TASK-001", cfg.Args[1])
	}
	if cfg.Args[3] != "/home/user/project" {
		t.Errorf("Args[3]: expected %q, got %q", "/home/user/project", cfg.Args[3])
	}
}

func TestAPIConfigInterpolate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"TASK_ID":   "TASK-001",
		"API_TOKEN": "secret123",
	}

	cfg := &APIConfig{
		URL: "https://api.example.com/tasks/{{TASK_ID}}",
		Headers: map[string]string{
			"Authorization": "Bearer {{API_TOKEN}}",
			"X-Task-ID":     "{{TASK_ID}}",
		},
	}

	cfg.Interpolate(vars)

	if cfg.URL != "https://api.example.com/tasks/TASK-001" {
		t.Errorf("URL: expected %q, got %q", "https://api.example.com/tasks/TASK-001", cfg.URL)
	}
	if cfg.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("Authorization header: expected %q, got %q", "Bearer secret123", cfg.Headers["Authorization"])
	}
	if cfg.Headers["X-Task-ID"] != "TASK-001" {
		t.Errorf("X-Task-ID header: expected %q, got %q", "TASK-001", cfg.Headers["X-Task-ID"])
	}
}

func TestEnvConfigInterpolate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"ENV_PREFIX": "PROD",
	}

	cfg := &EnvConfig{
		Var:     "{{ENV_PREFIX}}_DATABASE_URL",
		Default: "default-{{ENV_PREFIX}}-value",
	}

	cfg.Interpolate(vars)

	if cfg.Var != "PROD_DATABASE_URL" {
		t.Errorf("Var: expected %q, got %q", "PROD_DATABASE_URL", cfg.Var)
	}
	if cfg.Default != "default-PROD-value" {
		t.Errorf("Default: expected %q, got %q", "default-PROD-value", cfg.Default)
	}
}

func TestPhaseOutputConfigInterpolate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"PREVIOUS_PHASE": "spec",
	}

	cfg := &PhaseOutputConfig{
		Phase: "{{PREVIOUS_PHASE}}",
	}

	cfg.Interpolate(vars)

	if cfg.Phase != "spec" {
		t.Errorf("Phase: expected %q, got %q", "spec", cfg.Phase)
	}
}

func TestPromptFragmentConfigInterpolate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"LANG": "go",
	}

	cfg := &PromptFragmentConfig{
		Path: "{{LANG}}/best-practices.md",
	}

	cfg.Interpolate(vars)

	if cfg.Path != "go/best-practices.md" {
		t.Errorf("Path: expected %q, got %q", "go/best-practices.md", cfg.Path)
	}
}

func TestStaticConfigInterpolate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"VERSION": "1.2.3",
	}

	cfg := &StaticConfig{
		Value: "Version: {{VERSION}}",
	}

	cfg.Interpolate(vars)

	if cfg.Value != "Version: 1.2.3" {
		t.Errorf("Value: expected %q, got %q", "Version: 1.2.3", cfg.Value)
	}
}
