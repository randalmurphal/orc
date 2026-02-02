package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowCompletionAction_StructField tests SC-1:
// Workflow struct has CompletionAction field of type string.
func TestWorkflowCompletionAction_StructField(t *testing.T) {
	t.Run("workflow struct has CompletionAction field", func(t *testing.T) {
		wf := Workflow{
			ID:               "test",
			Name:             "Test",
			CompletionAction: "pr",
		}

		assert.Equal(t, "pr", wf.CompletionAction)
	})

	t.Run("accepts valid completion action values", func(t *testing.T) {
		validValues := []string{"", "pr", "commit", "none"}

		for _, value := range validValues {
			wf := Workflow{
				ID:               "test-" + value,
				Name:             "Test",
				CompletionAction: value,
			}
			assert.Equal(t, value, wf.CompletionAction, "expected %q to be valid", value)
		}
	})

	t.Run("empty string means inherit", func(t *testing.T) {
		wf := Workflow{
			ID:               "test",
			Name:             "Test",
			CompletionAction: "", // Inherit from config
		}

		// Empty string is the default "inherit" value
		assert.Equal(t, "", wf.CompletionAction)
	})
}

// TestWorkflowCompletionAction_YAMLParsing tests SC-2:
// YAML parsing includes the completion_action field.
func TestWorkflowCompletionAction_YAMLParsing(t *testing.T) {
	t.Run("parses completion_action from YAML", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
completion_action: pr
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		assert.Equal(t, "test-workflow", wf.ID)
		assert.Equal(t, "pr", wf.CompletionAction)
	})

	t.Run("parses completion_action as commit", func(t *testing.T) {
		yaml := []byte(`
id: commit-only-workflow
name: Commit Only Workflow
completion_action: commit
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		assert.Equal(t, "commit", wf.CompletionAction)
	})

	t.Run("parses completion_action as none", func(t *testing.T) {
		yaml := []byte(`
id: no-action-workflow
name: No Action Workflow
completion_action: none
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		assert.Equal(t, "none", wf.CompletionAction)
	})

	t.Run("defaults to empty string when not specified", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		// Default is empty (inherit from config)
		assert.Equal(t, "", wf.CompletionAction)
	})

	t.Run("preserves empty string explicitly", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
completion_action: ""
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		// Empty string means "inherit" - should be preserved
		assert.Equal(t, "", wf.CompletionAction)
	})

	t.Run("parses alongside other workflow fields", func(t *testing.T) {
		yaml := []byte(`
id: complete-workflow
name: Complete Workflow
description: A workflow with all settings
default_model: opus
default_thinking: true
default_max_iterations: 30
completion_action: pr
phases:
  - template: spec
    sequence: 1
  - template: implement
    sequence: 2
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		assert.Equal(t, "complete-workflow", wf.ID)
		assert.Equal(t, "Complete Workflow", wf.Name)
		assert.Equal(t, "A workflow with all settings", wf.Description)
		assert.Equal(t, "opus", wf.DefaultModel)
		assert.True(t, wf.DefaultThinking)
		assert.Equal(t, 30, wf.DefaultMaxIterations)
		assert.Equal(t, "pr", wf.CompletionAction)
		assert.Len(t, wf.Phases, 2)
	})
}
