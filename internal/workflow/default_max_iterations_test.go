package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowDefaultMaxIterations_YAMLParsing tests SC-2: YAML marshaling includes the field.
func TestWorkflowDefaultMaxIterations_YAMLParsing(t *testing.T) {
	t.Run("parses default_max_iterations from YAML", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
default_max_iterations: 50
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		assert.Equal(t, "test-workflow", wf.ID)
		assert.Equal(t, 50, wf.DefaultMaxIterations)
	})

	t.Run("defaults to zero when not specified", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		assert.Equal(t, 0, wf.DefaultMaxIterations)
	})

	t.Run("preserves zero value explicitly", func(t *testing.T) {
		yaml := []byte(`
id: test-workflow
name: Test Workflow
default_max_iterations: 0
phases:
  - template: implement
    sequence: 1
`)
		wf, err := parseWorkflowYAML(yaml)
		require.NoError(t, err)

		// Zero is valid (means "inherit") and should be preserved
		assert.Equal(t, 0, wf.DefaultMaxIterations)
	})
}

// TestWorkflowDefaultMaxIterations_StructField tests that the Workflow struct has the field.
func TestWorkflowDefaultMaxIterations_StructField(t *testing.T) {
	t.Run("workflow struct has DefaultMaxIterations field", func(t *testing.T) {
		wf := Workflow{
			ID:                   "test",
			Name:                 "Test",
			DefaultMaxIterations: 42,
		}

		assert.Equal(t, 42, wf.DefaultMaxIterations)
	})
}
