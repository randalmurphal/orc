package executor

import (
	"log/slog"
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
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
		globalDB:   &db.GlobalDB{DB: projectDB.DB},
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

func TestGetEffectivePhaseRuntimeConfig(t *testing.T) {
	t.Run("returns nil when no agent or override configured", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		cfg, err := env.executor.getEffectivePhaseRuntimeConfig(tmpl, phase)
		require.NoError(t, err)

		// Function returns nil when config is empty (no special configuration)
		assert.Nil(t, cfg)
	})

	t.Run("loads agent runtime config", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:            "review-executor",
			Name:          "Review Executor",
			RuntimeConfig: `{"shared":{"disallowed_tools":["Write","Edit"]}}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "review",
			AgentID: "review-executor",
		}
		phase := &db.WorkflowPhase{}

		cfg, err := env.executor.getEffectivePhaseRuntimeConfig(tmpl, phase)
		require.NoError(t, err)

		require.NotNil(t, cfg)
		assert.ElementsMatch(t, []string{"Write", "Edit"}, cfg.Shared.DisallowedTools)
	})

	t.Run("merges workflow phase override with agent config", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:            "impl-executor",
			Name:          "Implementation Executor",
			RuntimeConfig: `{"shared":{"max_turns":50}}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{
			RuntimeConfigOverride: `{"shared":{"disallowed_tools":["NotebookEdit"]}}`,
		}

		cfg, err := env.executor.getEffectivePhaseRuntimeConfig(tmpl, phase)
		require.NoError(t, err)

		require.NotNil(t, cfg)
		assert.Equal(t, 50, cfg.Shared.MaxTurns)                                      // From agent
		assert.ElementsMatch(t, []string{"NotebookEdit"}, cfg.Shared.DisallowedTools) // From override
	})

	t.Run("workflow override takes precedence on conflict", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:            "impl-executor",
			Name:          "Implementation Executor",
			RuntimeConfig: `{"shared":{"max_turns":50,"disallowed_tools":["Bash"]}}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{
			RuntimeConfigOverride: `{"shared":{"disallowed_tools":["Write","Edit"]}}`,
		}

		cfg, err := env.executor.getEffectivePhaseRuntimeConfig(tmpl, phase)
		require.NoError(t, err)

		require.NotNil(t, cfg)
		assert.Equal(t, 50, cfg.Shared.MaxTurns) // Preserved from agent
		// Override replaces disallowed_tools completely
		assert.ElementsMatch(t, []string{"Write", "Edit"}, cfg.Shared.DisallowedTools)
	})

	t.Run("works with empty phase (no overrides)", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:            "impl-executor",
			Name:          "Implementation Executor",
			RuntimeConfig: `{"shared":{"max_turns":50}}`,
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		// Phase with no overrides still uses agent config
		phase := &db.WorkflowPhase{}

		cfg, err := env.executor.getEffectivePhaseRuntimeConfig(tmpl, phase)
		require.NoError(t, err)

		require.NotNil(t, cfg)
		assert.Equal(t, 50, cfg.Shared.MaxTurns)
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

func TestResolvePhaseModel_WorkflowDefaultModel(t *testing.T) {
	t.Run("uses workflow default model when no phase override", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_model
		env.executor.wf = &workflow.Workflow{
			ID:           "test-workflow",
			DefaultModel: "sonnet",
		}

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "sonnet", model)
	})

	t.Run("workflow default model beats agent model", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Save an agent with model
		testAgent := &db.Agent{
			ID:    "impl-executor",
			Name:  "Implementation Executor",
			Model: "haiku",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		// Set workflow with default_model
		env.executor.wf = &workflow.Workflow{
			ID:           "test-workflow",
			DefaultModel: "sonnet",
		}

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		// Workflow default_model should win over agent model
		assert.Equal(t, "sonnet", model)
	})

	t.Run("phase model override still beats workflow default model", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_model
		env.executor.wf = &workflow.Workflow{
			ID:           "test-workflow",
			DefaultModel: "sonnet",
		}

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{
			ModelOverride: "opus", // Phase override should win
		}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		assert.Equal(t, "opus", model)
	})

	t.Run("falls through to agent when workflow has no default model", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Save an agent with model
		testAgent := &db.Agent{
			ID:    "impl-executor",
			Name:  "Implementation Executor",
			Model: "haiku",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		// Set workflow WITHOUT default_model
		env.executor.wf = &workflow.Workflow{
			ID:           "test-workflow",
			DefaultModel: "", // Empty - should fall through
		}

		tmpl := &db.PhaseTemplate{
			ID:      "implement",
			AgentID: "impl-executor",
		}
		phase := &db.WorkflowPhase{}

		model := env.executor.resolvePhaseModel(tmpl, phase)

		// Should fall through to agent model
		assert.Equal(t, "haiku", model)
	})
}

func TestShouldUseThinking_WorkflowDefaultThinking(t *testing.T) {
	t.Run("uses workflow default_thinking when no phase override", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_thinking=true
		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultThinking: true,
		}

		// Phase without thinking enabled in template (implement phase normally has thinking=false)
		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		// Should use workflow default_thinking=true
		assert.True(t, result)
	})

	t.Run("workflow default_thinking beats template default", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_thinking=true
		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultThinking: true,
		}

		// Phase template with thinking explicitly disabled
		thinkingDisabled := false
		tmpl := &db.PhaseTemplate{
			ID:              "implement",
			ThinkingEnabled: &thinkingDisabled,
		}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		// Workflow default_thinking should win over template default
		assert.True(t, result)
	})

	t.Run("phase thinking override still beats workflow default_thinking", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_thinking=true
		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultThinking: true,
		}

		tmpl := &db.PhaseTemplate{ID: "implement"}
		thinkingOverride := false
		phase := &db.WorkflowPhase{
			ThinkingOverride: &thinkingOverride, // Phase override should win
		}

		result := env.executor.shouldUseThinking(tmpl, phase)

		// Phase override=false should beat workflow default_thinking=true
		assert.False(t, result)
	})

	t.Run("falls through to template when workflow default_thinking is false", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_thinking=false (not set, zero value)
		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultThinking: false, // Zero value - should fall through
		}

		// Phase template with thinking explicitly enabled
		thinkingEnabled := true
		tmpl := &db.PhaseTemplate{
			ID:              "implement",
			ThinkingEnabled: &thinkingEnabled,
		}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		// Should fall through to template default
		assert.True(t, result)
	})

	t.Run("falls through to phase-specific defaults when workflow default_thinking is false", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_thinking=false
		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultThinking: false,
		}

		// spec phase defaults to thinking=true via hardcoded fallback
		tmpl := &db.PhaseTemplate{ID: "spec"}
		phase := &db.WorkflowPhase{}

		result := env.executor.shouldUseThinking(tmpl, phase)

		// Should fall through to spec phase default (true)
		assert.True(t, result)
	})
}

func TestResolvePhaseProvider(t *testing.T) {
	t.Run("defaults to claude when nothing configured", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "claude", provider)
	})

	t.Run("phase provider override takes precedence", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement", Provider: "claude"}
		phase := &db.WorkflowPhase{ProviderOverride: "codex"}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("extracts provider from model override as fallback", func(t *testing.T) {
		cfg := config.Default()
		cfg.Provider = "" // Clear config provider so model tuple fallback is exercised
		env := setupTestExecutor(t, cfg)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{ModelOverride: "codex:gpt-5"}

		// Model tuple is a fallback — only used when no explicit provider is set
		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("uses template provider", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		tmpl := &db.PhaseTemplate{ID: "implement", Provider: "codex"}
		phase := &db.WorkflowPhase{}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("uses workflow default provider", func(t *testing.T) {
		env := setupTestExecutor(t, nil)
		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultProvider: "codex",
		}

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("uses agent provider", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		testAgent := &db.Agent{
			ID:       "impl-executor",
			Name:     "Implementation Executor",
			Provider: "codex",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{ID: "implement", AgentID: "impl-executor"}
		phase := &db.WorkflowPhase{}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("uses run-level provider override", func(t *testing.T) {
		env := setupTestExecutor(t, nil)
		env.executor.runProvider = "codex"

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("run-level provider is highest priority", func(t *testing.T) {
		cfg := config.Default()
		cfg.Provider = "codex"
		env := setupTestExecutor(t, cfg)
		env.executor.runProvider = "codex"

		testAgent := &db.Agent{
			ID:       "impl-executor",
			Name:     "Implementation Executor",
			Provider: "claude",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		tmpl := &db.PhaseTemplate{ID: "implement", AgentID: "impl-executor"}
		phase := &db.WorkflowPhase{}

		// Run-level override wins over everything
		provider := env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove run-level — agent provider wins over config
		env.executor.runProvider = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "claude", provider)
	})

	t.Run("uses config provider", func(t *testing.T) {
		cfg := config.Default()
		cfg.Provider = "codex"
		env := setupTestExecutor(t, cfg)

		tmpl := &db.PhaseTemplate{ID: "implement"}
		phase := &db.WorkflowPhase{}

		provider := env.executor.resolvePhaseProvider(tmpl, phase)

		assert.Equal(t, "codex", provider)
	})

	t.Run("full priority chain: run-level > phase override > workflow > template > agent > config > model tuple", func(t *testing.T) {
		cfg := config.Default()
		cfg.Provider = "codex"
		env := setupTestExecutor(t, cfg)

		testAgent := &db.Agent{
			ID:       "impl-executor",
			Name:     "Implementation Executor",
			Provider: "codex",
		}
		require.NoError(t, env.projectDB.SaveAgent(testAgent))

		env.executor.wf = &workflow.Workflow{
			ID:              "test-workflow",
			DefaultProvider: "codex",
		}
		env.executor.runProvider = "codex"

		tmpl := &db.PhaseTemplate{ID: "implement", AgentID: "impl-executor", Provider: "codex"}
		phase := &db.WorkflowPhase{
			ProviderOverride: "claude",
			ModelOverride:    "codex:gpt-5", // model tuple fallback (lowest tier)
		}

		// Run-level override wins over everything
		provider := env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove run-level — phase override wins
		env.executor.runProvider = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "claude", provider)

		// Remove phase override — workflow default wins
		phase.ProviderOverride = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove workflow default — template wins
		env.executor.wf.DefaultProvider = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove template provider — agent wins
		tmpl.Provider = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove agent provider — config wins
		testAgent.Provider = ""
		require.NoError(t, env.projectDB.SaveAgent(testAgent))
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove config provider — model tuple fallback wins
		env.executor.orcConfig.Provider = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "codex", provider)

		// Remove model override — defaults to claude
		phase.ModelOverride = ""
		provider = env.executor.resolvePhaseProvider(tmpl, phase)
		assert.Equal(t, "claude", provider)
	})
}
