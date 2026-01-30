package executor

import (
	"os/exec"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/initiative"
	"google.golang.org/protobuf/proto"
)

func TestResolveTargetBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		task           *orcv1.Task
		initiative     *initiative.Initiative
		config         *config.Config
		expectedBranch string
	}{
		{
			name:           "all nil - defaults to main",
			task:           nil,
			initiative:     nil,
			config:         nil,
			expectedBranch: "main",
		},
		{
			name:           "task with no target branch - defaults to main",
			task:           &orcv1.Task{Id: "TASK-001"},
			initiative:     nil,
			config:         nil,
			expectedBranch: "main",
		},
		{
			name: "level 1: task explicit override takes precedence",
			task: &orcv1.Task{
				Id:           "TASK-001",
				TargetBranch: proto.String("hotfix/v2.1"),
			},
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config: &config.Config{
				Developer: config.DeveloperConfig{
					StagingEnabled: true,
					StagingBranch:  "dev/randy",
				},
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "hotfix/v2.1",
		},
		{
			name: "level 2: initiative branch takes precedence over developer staging",
			task: &orcv1.Task{Id: "TASK-001"},
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config: &config.Config{
				Developer: config.DeveloperConfig{
					StagingEnabled: true,
					StagingBranch:  "dev/randy",
				},
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "feature/auth",
		},
		{
			name:       "level 3: developer staging when enabled",
			task:       &orcv1.Task{Id: "TASK-001"},
			initiative: nil,
			config: &config.Config{
				Developer: config.DeveloperConfig{
					StagingEnabled: true,
					StagingBranch:  "dev/randy",
				},
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "dev/randy",
		},
		{
			name:       "developer staging disabled falls through to project config",
			task:       &orcv1.Task{Id: "TASK-001"},
			initiative: nil,
			config: &config.Config{
				Developer: config.DeveloperConfig{
					StagingEnabled: false,
					StagingBranch:  "dev/randy",
				},
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "develop",
		},
		{
			name:       "developer staging enabled but empty branch falls through",
			task:       &orcv1.Task{Id: "TASK-001"},
			initiative: nil,
			config: &config.Config{
				Developer: config.DeveloperConfig{
					StagingEnabled: true,
					StagingBranch:  "",
				},
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "develop",
		},
		{
			name:       "level 4: project config default",
			task:       &orcv1.Task{Id: "TASK-001"},
			initiative: nil,
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "develop",
		},
		{
			name:           "level 5: fallback to main when config has no target",
			task:           &orcv1.Task{Id: "TASK-001"},
			initiative:     nil,
			config:         &config.Config{},
			expectedBranch: "main",
		},
		{
			name: "initiative with empty branch base falls through",
			task: &orcv1.Task{Id: "TASK-001"},
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "",
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveTargetBranch(tt.task, tt.initiative, tt.config)
			if got != tt.expectedBranch {
				t.Errorf("ResolveTargetBranch() = %q, want %q", got, tt.expectedBranch)
			}
		})
	}
}

func TestResolveTargetBranchSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		task           *orcv1.Task
		initiative     *initiative.Initiative
		config         *config.Config
		expectedBranch string
		expectedSource string
	}{
		{
			name:           "default source",
			task:           nil,
			initiative:     nil,
			config:         nil,
			expectedBranch: "main",
			expectedSource: "default",
		},
		{
			name: "task override source",
			task: &orcv1.Task{
				Id:           "TASK-001",
				TargetBranch: proto.String("hotfix/v2.1"),
			},
			initiative:     nil,
			config:         nil,
			expectedBranch: "hotfix/v2.1",
			expectedSource: "task override",
		},
		{
			name: "initiative branch source",
			task: &orcv1.Task{Id: "TASK-001"},
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config:         nil,
			expectedBranch: "feature/auth",
			expectedSource: "initiative branch",
		},
		{
			name:       "developer staging source",
			task:       &orcv1.Task{Id: "TASK-001"},
			initiative: nil,
			config: &config.Config{
				Developer: config.DeveloperConfig{
					StagingEnabled: true,
					StagingBranch:  "dev/randy",
				},
			},
			expectedBranch: "dev/randy",
			expectedSource: "developer staging",
		},
		{
			name:       "project config source",
			task:       &orcv1.Task{Id: "TASK-001"},
			initiative: nil,
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "develop",
			expectedSource: "project config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch, source := ResolveTargetBranchSource(tt.task, tt.initiative, tt.config)
			if branch != tt.expectedBranch {
				t.Errorf("branch = %q, want %q", branch, tt.expectedBranch)
			}
			if source != tt.expectedSource {
				t.Errorf("source = %q, want %q", source, tt.expectedSource)
			}
		})
	}
}

func TestIsDefaultBranch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		branch   string
		expected bool
	}{
		{"main", true},
		{"master", true},
		{"develop", true},
		{"development", true},
		{"feature/auth", false},
		{"hotfix/v2.1", false},
		{"dev/randy", false},
		{"release/v3.0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := IsDefaultBranch(tt.branch)
			if got != tt.expected {
				t.Errorf("IsDefaultBranch(%q) = %v, want %v", tt.branch, got, tt.expected)
			}
		})
	}
}

// newTestGit creates a minimal git repo and returns a *git.Git for testing.
func newTestGit(t *testing.T) *git.Git {
	t.Helper()
	tmpDir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v failed: %v", args, err)
		}
	}

	g, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("git.New: %v", err)
	}
	return g
}

func TestResolveBranchName(t *testing.T) {
	t.Parallel()
	gitSvc := newTestGit(t)

	tests := []struct {
		name             string
		task             *orcv1.Task
		initiativePrefix string
		expected         string
	}{
		{
			name:             "no custom branch - auto-generated",
			task:             &orcv1.Task{Id: "TASK-001"},
			initiativePrefix: "",
			expected:         "orc/TASK-001",
		},
		{
			name:             "no custom branch with initiative prefix",
			task:             &orcv1.Task{Id: "TASK-001"},
			initiativePrefix: "feature/auth/",
			expected:         "feature/auth/TASK-001",
		},
		{
			name: "valid custom branch - used directly",
			task: &orcv1.Task{
				Id:         "TASK-001",
				BranchName: proto.String("my-feature-branch"),
			},
			initiativePrefix: "feature/auth/",
			expected:         "my-feature-branch",
		},
		{
			name: "valid custom branch with slashes",
			task: &orcv1.Task{
				Id:         "TASK-001",
				BranchName: proto.String("feature/my-work"),
			},
			initiativePrefix: "",
			expected:         "feature/my-work",
		},
		{
			name: "invalid custom branch - falls back to auto-generated",
			task: &orcv1.Task{
				Id:         "TASK-001",
				BranchName: proto.String("..invalid"),
			},
			initiativePrefix: "",
			expected:         "orc/TASK-001",
		},
		{
			name: "empty custom branch - falls back to auto-generated",
			task: &orcv1.Task{
				Id:         "TASK-001",
				BranchName: proto.String(""),
			},
			initiativePrefix: "",
			expected:         "orc/TASK-001",
		},
		{
			name: "nil BranchName - falls back to auto-generated",
			task: &orcv1.Task{
				Id: "TASK-001",
			},
			initiativePrefix: "",
			expected:         "orc/TASK-001",
		},
		{
			name: "invalid branch with @ alone - falls back",
			task: &orcv1.Task{
				Id:         "TASK-001",
				BranchName: proto.String("@"),
			},
			initiativePrefix: "",
			expected:         "orc/TASK-001",
		},
		{
			name: "invalid branch with .lock suffix - falls back",
			task: &orcv1.Task{
				Id:         "TASK-001",
				BranchName: proto.String("my-branch.lock"),
			},
			initiativePrefix: "",
			expected:         "orc/TASK-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBranchName(tt.task, gitSvc, tt.initiativePrefix)
			if got != tt.expected {
				t.Errorf("ResolveBranchName() = %q, want %q", got, tt.expected)
			}
		})
	}
}
