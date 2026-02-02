package executor

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/workflow"
	"google.golang.org/protobuf/proto"
)

// TestResolveTargetBranchWithWorkflow_Hierarchy tests the 6-level priority hierarchy
// for target branch resolution that includes workflow.
func TestResolveTargetBranchWithWorkflow_Hierarchy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		task           *orcv1.Task
		workflow       *workflow.Workflow
		initiative     *initiative.Initiative
		config         *config.Config
		expectedBranch string
		expectedSource string
	}{
		{
			name:           "all nil - defaults to main",
			task:           nil,
			workflow:       nil,
			initiative:     nil,
			config:         nil,
			expectedBranch: "main",
			expectedSource: "default",
		},
		{
			name: "level 1: task explicit override takes precedence over all",
			task: &orcv1.Task{
				Id:           "TASK-001",
				TargetBranch: proto.String("hotfix/v2.1"),
			},
			workflow: &workflow.Workflow{
				ID:           "custom-workflow",
				TargetBranch: "workflow-branch",
			},
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "hotfix/v2.1",
			expectedSource: "task override",
		},
		{
			name: "level 2: workflow target_branch takes precedence over initiative",
			task: &orcv1.Task{Id: "TASK-001"},
			workflow: &workflow.Workflow{
				ID:           "custom-workflow",
				TargetBranch: "workflow-branch",
			},
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "workflow-branch",
			expectedSource: "workflow default",
		},
		{
			name:     "level 3: initiative branch_base when workflow has no target_branch",
			task:     &orcv1.Task{Id: "TASK-001"},
			workflow: &workflow.Workflow{ID: "custom-workflow"}, // No target_branch
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "feature/auth",
			expectedSource: "initiative branch",
		},
		{
			name:       "level 4: developer staging when workflow and initiative have no branch",
			task:       &orcv1.Task{Id: "TASK-001"},
			workflow:   &workflow.Workflow{ID: "custom-workflow"}, // No target_branch
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
			expectedSource: "developer staging",
		},
		{
			name:       "level 5: project config when nothing else is set",
			task:       &orcv1.Task{Id: "TASK-001"},
			workflow:   nil,
			initiative: nil,
			config: &config.Config{
				Completion: config.CompletionConfig{
					TargetBranch: "develop",
				},
			},
			expectedBranch: "develop",
			expectedSource: "project config",
		},
		{
			name:           "level 6: hardcoded fallback when all else is nil/empty",
			task:           &orcv1.Task{Id: "TASK-001"},
			workflow:       nil,
			initiative:     nil,
			config:         nil,
			expectedBranch: "main",
			expectedSource: "default",
		},
		{
			name: "nil workflow is skipped in hierarchy",
			task: &orcv1.Task{Id: "TASK-001"},
			workflow: nil, // Explicitly nil
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config:         nil,
			expectedBranch: "feature/auth",
			expectedSource: "initiative branch",
		},
		{
			name:     "empty workflow target_branch is skipped",
			task:     &orcv1.Task{Id: "TASK-001"},
			workflow: &workflow.Workflow{ID: "custom-workflow", TargetBranch: ""}, // Empty string
			initiative: &initiative.Initiative{
				ID:         "INIT-001",
				BranchBase: "feature/auth",
			},
			config:         nil,
			expectedBranch: "feature/auth",
			expectedSource: "initiative branch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			branch, source := ResolveTargetBranchWithWorkflowSource(tc.task, tc.workflow, tc.initiative, tc.config)

			if branch != tc.expectedBranch {
				t.Errorf("expected branch %q, got %q", tc.expectedBranch, branch)
			}
			if source != tc.expectedSource {
				t.Errorf("expected source %q, got %q", tc.expectedSource, source)
			}
		})
	}
}

// TestResolveTargetBranchWithWorkflow_Convenience tests the non-source version.
func TestResolveTargetBranchWithWorkflow_Convenience(t *testing.T) {
	t.Parallel()

	task := &orcv1.Task{Id: "TASK-001"}
	wf := &workflow.Workflow{
		ID:           "custom-workflow",
		TargetBranch: "workflow-branch",
	}

	branch := ResolveTargetBranchWithWorkflow(task, wf, nil, nil)
	if branch != "workflow-branch" {
		t.Errorf("expected 'workflow-branch', got %q", branch)
	}
}
