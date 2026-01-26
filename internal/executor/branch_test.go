package executor

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
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
