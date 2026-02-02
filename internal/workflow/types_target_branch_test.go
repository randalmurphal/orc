package workflow

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowTargetBranch_YAMLRoundtrip tests that target_branch survives YAML parsing.
func TestWorkflowTargetBranch_YAMLRoundtrip(t *testing.T) {
	t.Run("parses target_branch from YAML", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
target_branch: develop
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)
		assert.Equal(t, "develop", wf.TargetBranch)
	})

	t.Run("empty target_branch means inherit from config", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)
		assert.Equal(t, "", wf.TargetBranch)
	})

	t.Run("target_branch accepts various branch names", func(t *testing.T) {
		branches := []string{"main", "master", "develop", "feature/auth", "release-v2"}

		for _, branch := range branches {
			yaml := []byte(`
id: test-` + branch + `
name: Test
target_branch: ` + branch + `
phases:
  - template: implement
    sequence: 1
`)
			wf, err := parseWorkflowYAML(yaml)
			require.NoError(t, err, "failed to parse YAML for branch %s", branch)
			assert.Equal(t, branch, wf.TargetBranch, "expected target_branch to be %q", branch)
		}
	})
}

// TestWorkflowTargetBranch_DBConversion tests that target_branch survives DB conversion.
func TestWorkflowTargetBranch_DBConversion(t *testing.T) {
	t.Run("workflowToDBWorkflow preserves target_branch", func(t *testing.T) {
		wf := &Workflow{
			ID:           "test-workflow",
			Name:         "Test Workflow",
			TargetBranch: "develop",
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)
		assert.Equal(t, "develop", dbWf.TargetBranch)
	})

	t.Run("DBWorkflowToWorkflow preserves target_branch", func(t *testing.T) {
		dbWf := &db.Workflow{
			ID:           "test-workflow",
			Name:         "Test Workflow",
			TargetBranch: "feature/auth",
		}

		wf := DBWorkflowToWorkflow(dbWf)
		assert.Equal(t, "feature/auth", wf.TargetBranch)
	})

	t.Run("empty target_branch is preserved", func(t *testing.T) {
		wf := &Workflow{
			ID:           "test-workflow",
			Name:         "Test Workflow",
			TargetBranch: "", // Inherit from config
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)
		assert.Equal(t, "", dbWf.TargetBranch)

		// Round-trip back
		wf2 := DBWorkflowToWorkflow(dbWf)
		assert.Equal(t, "", wf2.TargetBranch)
	})
}
