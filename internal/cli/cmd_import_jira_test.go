package cli

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestResolveString(t *testing.T) {
	tests := []struct {
		name      string
		flag      string
		envVar    string
		configVal string
		envSet    map[string]string
		expected  string
	}{
		{
			name:     "flag takes priority",
			flag:     "from-flag",
			envVar:   "TEST_VAR",
			envSet:   map[string]string{"TEST_VAR": "from-env"},
			expected: "from-flag",
		},
		{
			name:      "env takes priority over config",
			flag:      "",
			envVar:    "TEST_VAR",
			configVal: "from-config",
			envSet:    map[string]string{"TEST_VAR": "from-env"},
			expected:  "from-env",
		},
		{
			name:      "config as fallback",
			flag:      "",
			envVar:    "TEST_VAR_UNSET",
			configVal: "from-config",
			expected:  "from-config",
		},
		{
			name:     "empty when nothing set",
			flag:     "",
			envVar:   "TEST_VAR_UNSET",
			expected: "",
		},
		{
			name:     "empty env var name skips env lookup",
			flag:     "",
			envVar:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envSet {
				t.Setenv(k, v)
			}
			got := resolveString(tt.flag, tt.envVar, tt.configVal)
			if got != tt.expected {
				t.Errorf("resolveString() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestResolveWeight(t *testing.T) {
	tests := []struct {
		flag      string
		configVal string
		expected  orcv1.TaskWeight
	}{
		{"trivial", "", orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL},
		{"small", "", orcv1.TaskWeight_TASK_WEIGHT_SMALL},
		{"medium", "", orcv1.TaskWeight_TASK_WEIGHT_MEDIUM},
		{"large", "", orcv1.TaskWeight_TASK_WEIGHT_LARGE},
		{"MEDIUM", "", orcv1.TaskWeight_TASK_WEIGHT_MEDIUM}, // case insensitive
		{"", "small", orcv1.TaskWeight_TASK_WEIGHT_SMALL},   // config fallback
		{"large", "small", orcv1.TaskWeight_TASK_WEIGHT_LARGE}, // flag wins
		{"", "", orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED},
		{"invalid", "", orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED},
	}

	for _, tt := range tests {
		name := tt.flag + "/" + tt.configVal
		t.Run(name, func(t *testing.T) {
			got := resolveWeight(tt.flag, tt.configVal)
			if got != tt.expected {
				t.Errorf("resolveWeight(%q, %q) = %v, want %v", tt.flag, tt.configVal, got, tt.expected)
			}
		})
	}
}

func TestResolveQueue(t *testing.T) {
	tests := []struct {
		flag      string
		configVal string
		expected  orcv1.TaskQueue
	}{
		{"active", "", orcv1.TaskQueue_TASK_QUEUE_ACTIVE},
		{"backlog", "", orcv1.TaskQueue_TASK_QUEUE_BACKLOG},
		{"ACTIVE", "", orcv1.TaskQueue_TASK_QUEUE_ACTIVE}, // case insensitive
		{"", "backlog", orcv1.TaskQueue_TASK_QUEUE_BACKLOG},
		{"", "", orcv1.TaskQueue_TASK_QUEUE_UNSPECIFIED},
		{"invalid", "", orcv1.TaskQueue_TASK_QUEUE_UNSPECIFIED},
	}

	for _, tt := range tests {
		name := tt.flag + "/" + tt.configVal
		t.Run(name, func(t *testing.T) {
			got := resolveQueue(tt.flag, tt.configVal)
			if got != tt.expected {
				t.Errorf("resolveQueue(%q, %q) = %v, want %v", tt.flag, tt.configVal, got, tt.expected)
			}
		})
	}
}

func TestNewImportJiraCmd_Registration(t *testing.T) {
	cmd := newImportJiraCmd()

	if cmd.Use != "jira" {
		t.Errorf("Use = %q, want %q", cmd.Use, "jira")
	}

	// Verify required flags exist
	expectedFlags := []string{"url", "email", "token", "project", "jql", "no-epics", "dry-run", "weight", "queue"}
	for _, name := range expectedFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("missing flag: %s", name)
		}
	}
}

func TestNewImportCmd_HasJiraSubcommand(t *testing.T) {
	cmd := newImportCmd()

	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "jira" {
			found = true
			break
		}
	}
	if !found {
		t.Error("import command should have 'jira' subcommand")
	}
}
