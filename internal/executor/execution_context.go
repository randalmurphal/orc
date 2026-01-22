// Package executor provides the execution engine for orc.
// This file centralizes execution context building to ensure all executors
// use the same logic for template variables, model resolution, and prompt rendering.
package executor

import (
	"log/slog"
	"os"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ExecutionContext holds everything needed to run a phase.
// It centralizes the context-building that was previously duplicated
// across TrivialExecutor, StandardExecutor, and FullExecutor.
type ExecutionContext struct {
	// Task and phase info
	Task       *task.Task
	Phase      *plan.Phase
	State      *state.State
	PromptText string

	// Model settings
	ModelSetting config.PhaseModelSetting

	// Paths
	WorkingDir    string
	MCPConfigPath string
	TaskDir       string

	// Session handling
	SessionID string
	IsResume  bool

	// Spec content for validation (loaded from database)
	SpecContent string

	// Template variables (for reference/debugging)
	TemplateVars TemplateVars
}

// ExecutionContextConfig holds the configuration needed to build an ExecutionContext.
type ExecutionContextConfig struct {
	// Required
	Task    *task.Task
	Phase   *plan.Phase
	State   *state.State
	Backend storage.Backend

	// Execution settings
	WorkingDir      string
	MCPConfigPath   string
	TaskDir         string
	ExecutorConfig  ExecutorConfig
	OrcConfig       *config.Config
	ResumeSessionID string

	// Logging
	Logger *slog.Logger
}

// BuildExecutionContext creates a fully-populated execution context for any executor.
// This is the single source of truth for:
// - Template loading & rendering
// - Spec loading from database
// - Review context loading
// - Initiative context
// - Automation context
// - UI testing context
// - Ultrathink injection
// - Model resolution
func BuildExecutionContext(cfg ExecutionContextConfig) (*ExecutionContext, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	t := cfg.Task
	p := cfg.Phase
	s := cfg.State

	// Generate session ID: {task_id}-{phase_id}
	// If resuming, use the stored session ID instead
	sessionID := t.ID + "-" + p.ID
	isResume := cfg.ResumeSessionID != ""
	if isResume {
		sessionID = cfg.ResumeSessionID
		logger.Info("resuming from previous session", "session_id", sessionID)
	}

	// Resolve model settings for this phase and weight
	modelSetting := cfg.ExecutorConfig.ResolveModelSetting(string(t.Weight), p.ID)

	// Load and render initial prompt using shared template module
	tmpl, err := LoadPromptTemplate(p)
	if err != nil {
		return nil, err
	}

	// Build template variables
	vars := BuildTemplateVars(t, p, s, 0, LoadRetryContextForPhase(s))

	// Load spec content from database (specs are not stored as file artifacts)
	vars = vars.WithSpecFromDatabase(cfg.Backend, t.ID)

	// Load phase artifact content from database (design, tdd_write, breakdown, research)
	vars = vars.WithArtifactsFromDatabase(cfg.Backend, t.ID)

	// Load review context for review phases (round 2+ needs prior findings)
	if p.ID == "review" {
		round := 1
		if cfg.OrcConfig != nil {
			// Check if this is a subsequent review round based on state
			if s != nil && s.Phases != nil {
				if ps, ok := s.Phases["review"]; ok && ps.Status == "completed" {
					round = 2 // Re-running review means it's round 2
				}
			}
		}
		vars = vars.WithReviewContext(cfg.Backend, t.ID, round)
	}

	// Add testing configuration (coverage threshold)
	if cfg.OrcConfig != nil {
		vars.CoverageThreshold = cfg.OrcConfig.Testing.CoverageThreshold
	}

	// Add worktree context for template rendering
	if cfg.WorkingDir != "" {
		vars.WorktreePath = cfg.WorkingDir
		vars.TaskBranch = t.Branch
		vars.TargetBranch = ResolveTargetBranchForTask(t, cfg.Backend, cfg.OrcConfig)
	}

	// Add UI testing context if task requires it
	if t.RequiresUITesting {
		if cfg.WorkingDir == "" {
			logger.Warn("workingDir not set for UI testing - skipping UI testing context",
				"task", t.ID, "phase", p.ID)
		} else {
			// Set up screenshot directory in task test-results
			screenshotDir := task.ScreenshotsPath(cfg.WorkingDir, t.ID)
			if err := os.MkdirAll(screenshotDir, 0755); err != nil {
				logger.Warn("failed to create screenshot directory", "error", err)
			}

			vars = vars.WithUITestingContext(UITestingContext{
				RequiresUITesting: true,
				ScreenshotDir:     screenshotDir,
				TestResults:       loadPriorContent(task.TaskDir(t.ID), s, "test"),
			})

			logger.Info("UI testing enabled",
				"task", t.ID,
				"phase", p.ID,
				"screenshot_dir", screenshotDir,
			)
		}
	}

	// Add initiative context if task belongs to an initiative
	if initCtx := LoadInitiativeContext(t, cfg.Backend); initCtx != nil {
		vars = vars.WithInitiativeContext(*initCtx)
		logger.Info("initiative context injected",
			"task", t.ID,
			"initiative", initCtx.ID,
			"has_vision", initCtx.Vision != "",
			"decision_count", len(initCtx.Decisions),
		)
	}

	// Add automation context if this is an automation task (AUTO-XXX)
	if t.IsAutomation {
		if cfg.WorkingDir == "" {
			logger.Warn("workingDir not set for automation context - skipping automation context",
				"task", t.ID, "phase", p.ID)
		} else if autoCtx := LoadAutomationContext(t, cfg.Backend, cfg.WorkingDir); autoCtx != nil {
			vars = vars.WithAutomationContext(*autoCtx)
			logger.Info("automation context injected",
				"task", t.ID,
				"has_recent_tasks", autoCtx.RecentCompletedTasks != "",
				"has_changed_files", autoCtx.RecentChangedFiles != "",
			)
		}
	}

	// Build prompt text
	var promptText string
	if isResume {
		// Use continuation prompt when resuming (Claude already has full context)
		promptText = BuildContinuationPrompt(s, p.ID)
		logger.Info("using continuation prompt for resume", "task", t.ID, "phase", p.ID)
	} else {
		// Render the full template
		promptText = RenderTemplate(tmpl, vars)
	}

	// Inject "ultrathink" for extended thinking mode (skip for resume - Claude preserves thinking mode)
	// This triggers maximum thinking budget (31,999 tokens) in Claude Code
	if modelSetting.Thinking && !isResume {
		promptText = "ultrathink\n\n" + promptText
		logger.Debug("extended thinking enabled", "task", t.ID, "phase", p.ID)
	}

	// Load spec content for progress/criteria validation (if backend available)
	var specContent string
	if cfg.Backend != nil {
		if content, err := cfg.Backend.LoadSpec(t.ID); err == nil {
			specContent = content
		}
	}

	return &ExecutionContext{
		Task:          t,
		Phase:         p,
		State:         s,
		PromptText:    promptText,
		ModelSetting:  modelSetting,
		WorkingDir:    cfg.WorkingDir,
		MCPConfigPath: cfg.MCPConfigPath,
		TaskDir:       cfg.TaskDir,
		SessionID:     sessionID,
		IsResume:      isResume,
		SpecContent:   specContent,
		TemplateVars:  vars,
	}, nil
}
