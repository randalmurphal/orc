package executor

import (
	"log/slog"
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testExecutorEnv holds the test environment for executor resolution tests.
type testExecutorEnv struct {
	projectDB *db.ProjectDB
	executor  *WorkflowExecutor
	tmpDir    string
}

// setupTestExecutor creates a minimal WorkflowExecutor for testing resolution functions.
func setupTestExecutor(t *testing.T, cfg *config.Config) *testExecutorEnv {
	t.Helper()

	tmpDir := t.TempDir()

	// Create project database (in-memory for fast tests)
	projectDB, err := db.OpenProjectInMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = projectDB.Close() })

	if cfg == nil {
		cfg = config.Default()
	}

	// Create executor with minimal config
	executor := &WorkflowExecutor{
		projectDB:  projectDB,
		orcConfig:  cfg,
		logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		workingDir: tmpDir,
	}

	return &testExecutorEnv{
		projectDB: projectDB,
		executor:  executor,
		tmpDir:    tmpDir,
	}
}

func TestResolveExecutorAgent(t *testing.T) {
	t.Run("returns nil when no agent configured", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		agent := env.executor.resolveExecutorAgent(tmpl, phase)

		assert.Nil(t, agent)
	})

	t.Run("returns agent from phase template", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Save an agent
		testAgent := &db.Agent{
			ID:          "impl-executor",
			Name:        "Implementation Executor",
			Description: "Executor for implementation",
			Model:       "opus",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{}

		agent := env.executor.resolveExecutorAgent(tmpl, phase)

		require.NotNil(t, agent)
		assert.Equal(t, "impl-executor", agent.ID)
		assert.Equal(t, "opus", agent.Model)
	})

	t.Run("agent override takes precedence over template", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Save two agents
		templateAgent := &db.Agent{
			ID:    "default-executor",
			Name:  "Default Executor",
			Model: "sonnet",
		}
		overrideAgent := &db.Agent{
			ID:    "custom-executor",
			Name:  "Custom Executor",
			Model: "opus",
		}
		require.NoError(t, env.projectDB.SaveAgent(templateAgent))
		require.NoError(t, env.projectDB.SaveAgent(overrideAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "default-executor",
		}
		phase := &db.WorkflowPhase{
			AgentOverride: "custom-executor",
		}

		agent := env.executor.resolveExecutorAgent(tmpl, phase)

		require.NotNil(t, agent)
		assert.Equal(t, "custom-executor", agent.ID)
		assert.Equal(t, "opus", agent.Model)
	})

	t.Run("returns nil when agent not found in database", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "nonexistent-agent",
		}
		phase := &db.WorkflowPhase{}

		agent := env.executor.resolveExecutorAgent(tmpl, phase)

		assert.Nil(t, agent)
	})
}

func TestResolvePhaseModel(t *testing.T) {
	t.Run("uses workflow phase model override first", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{
			ModelOverride: "haiku",
		}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "haiku", model)
	})

	t.Run("uses agent model when no override", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Save an agent with model
		testAgent := &db.Agent{
			ID:    "impl-executor",
			Name:  "Implementation Executor",
			Model: "sonnet",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "sonnet", model)
	})

	t.Run("uses config model when no agent model", func(t *testing.T) {
		cfg := config.Default()
		cfg.Model = "haiku"
		env := setupTestExecutor(t, cfg)

		// Agent without model
		testAgent := &db.Agent{
			ID:   "no-model-agent",
			Name: "No Model Agent",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "no-model-agent",
		}
		phase := &db.WorkflowPhase{}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "haiku", model)
	})

	t.Run("falls back to opus when no config", func(t *testing.T) {
		cfg := config.Default()
		cfg.Model = "" // No model configured
		env := setupTestExecutor(t, cfg)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "opus", model)
	})

	t.Run("model override beats agent model", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:    "impl-executor",
			Name:  "Implementation Executor",
			Model: "sonnet",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{
			ModelOverride: "opus", // Override the agent model
		}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "opus", model)
	})
}

func TestGetEffectivePhaseClaudeConfig(t *testing.T) {
	t.Run("returns nil when no agent or override configured", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		cfg := env.executor.getEffectivePhaseClaudeConfig(tmpl, phase)

		// Function returns nil when config is empty (no special configuration)
		assert.Nil(t, cfg)
	})

	t.Run("loads agent claude config", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:           "review-executor",
			Name:         "Review Executor",
			ClaudeConfig: `{"disallowed_tools": ["Write", "Edit"]}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "review",
			AgentID: "review-executor",
		}
		phase := &db.WorkflowPhase{}

		cfg := env.executor.getEffectivePhaseClaudeConfig(tmpl, phase)

		require.NotNil(t, cfg)
		assert.ElementsMatch(t, []string{"Write", "Edit"}, cfg.DisallowedTools)
	})

	t.Run("merges workflow phase override with agent config", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:           "impl-executor",
			Name:         "Implementation Executor",
			ClaudeConfig: `{"max_turns": 50}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{
			ClaudeConfigOverride: `{"disallowed_tools": ["NotebookEdit"]}`,
		}

		cfg := env.executor.getEffectivePhaseClaudeConfig(tmpl, phase)

		require.NotNil(t, cfg)
		assert.Equal(t, 50, cfg.MaxTurns)                               // From agent
		assert.ElementsMatch(t, []string{"NotebookEdit"}, cfg.DisallowedTools) // From override
	})

	t.Run("workflow override takes precedence on conflict", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:           "impl-executor",
			Name:         "Implementation Executor",
			ClaudeConfig: `{"max_turns": 50, "disallowed_tools": ["Bash"]}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{
			ClaudeConfigOverride: `{"disallowed_tools": ["Write", "Edit"]}`,
		}

		cfg := env.executor.getEffectivePhaseClaudeConfig(tmpl, phase)

		require.NotNil(t, cfg)
		assert.Equal(t, 50, cfg.MaxTurns) // Preserved from agent
		// Override replaces disallowed_tools completely
		assert.ElementsMatch(t, []string{"Write", "Edit"}, cfg.DisallowedTools)
	})

	t.Run("works with empty phase (no overrides)", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:           "impl-executor",
			Name:         "Implementation Executor",
			ClaudeConfig: `{"max_turns": 50}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		// Phase with no overrides still uses agent config
		phase := &db.WorkflowPhase{}

		cfg := env.executor.getEffectivePhaseClaudeConfig(tmpl, phase)

		require.NotNil(t, cfg)
		assert.Equal(t, 50, cfg.MaxTurns)
	})
}

func TestShouldUseThinking(t *testing.T) {
	t.Run("phase override takes precedence", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		thinkingEnabled := true
		tmpl := &db.PhaseTemplate{
			ID:              "implement",
			ThinkingEnabled: &thinkingEnabled,
		}
		thinkingOverride := false
		phase := &db.WorkflowPhase{
			ThinkingOverride: &thinkingOverride,
		}

		result := env.executor.shouldUseThinking(tmpl, phase)

		assert.False(t, result)
	})

	t.Run("uses template default when no override", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		thinkingEnabled := true
		tmpl := &db.PhaseTemplate{
			ID:              "spec",
			ThinkingEnabled: &thinkingEnabled,
		}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		assert.True(t, result)
	})

	t.Run("spec phase defaults to thinking", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "spec"}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		assert.True(t, result)
	})

	t.Run("review phase defaults to thinking", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "review"}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		assert.True(t, result)
	})

	t.Run("implement phase defaults to no thinking", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		assert.False(t, result)
	})
}
