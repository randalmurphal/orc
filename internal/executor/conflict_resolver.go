// Package executor provides conflict resolution for orc.
// This module provides shared conflict resolution logic used by both the
// finalize phase and normal completion flow.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"text/template"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/templates"
)

// ConflictResolver handles conflict resolution for task branches.
// It first tries auto-resolution for known patterns (CLAUDE.md knowledge tables),
// then falls back to Claude-assisted resolution for remaining conflicts.
type ConflictResolver struct {
	gitSvc     *git.Git
	logger     *slog.Logger
	config     config.FinalizeConfig
	workingDir string

	// Model settings for conflict resolution
	model    string
	thinking bool

	// ClaudeCLI settings
	claudePath    string
	mcpConfigPath string

	// turnExecutor allows injection of a mock for testing
	turnExecutor TurnExecutor
}

// ConflictResolverOption configures a ConflictResolver.
type ConflictResolverOption func(*ConflictResolver)

// WithResolverGitSvc sets the git service.
func WithResolverGitSvc(svc *git.Git) ConflictResolverOption {
	return func(r *ConflictResolver) { r.gitSvc = svc }
}

// WithResolverClaudePath sets the path to claude binary.
func WithResolverClaudePath(path string) ConflictResolverOption {
	return func(r *ConflictResolver) { r.claudePath = path }
}

// WithResolverMCPConfig sets the MCP config path.
func WithResolverMCPConfig(path string) ConflictResolverOption {
	return func(r *ConflictResolver) { r.mcpConfigPath = path }
}

// WithResolverLogger sets the logger.
func WithResolverLogger(l *slog.Logger) ConflictResolverOption {
	return func(r *ConflictResolver) { r.logger = l }
}

// WithResolverConfig sets the finalize configuration.
func WithResolverConfig(cfg config.FinalizeConfig) ConflictResolverOption {
	return func(r *ConflictResolver) { r.config = cfg }
}

// WithResolverWorkingDir sets the working directory.
func WithResolverWorkingDir(dir string) ConflictResolverOption {
	return func(r *ConflictResolver) { r.workingDir = dir }
}

// WithResolverModel sets the model and thinking mode.
func WithResolverModel(model string, thinking bool) ConflictResolverOption {
	return func(r *ConflictResolver) {
		r.model = model
		r.thinking = thinking
	}
}

// WithResolverTurnExecutor sets a TurnExecutor for testing.
func WithResolverTurnExecutor(te TurnExecutor) ConflictResolverOption {
	return func(r *ConflictResolver) { r.turnExecutor = te }
}

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(opts ...ConflictResolverOption) *ConflictResolver {
	r := &ConflictResolver{
		claudePath: "claude", // Default claude binary path
		logger:     slog.Default(),
		config: config.FinalizeConfig{
			ConflictResolution: config.ConflictResolutionConfig{
				Enabled: true,
			},
		},
		model: "sonnet", // Default model for conflict resolution
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// ResolutionResult contains the outcome of a conflict resolution attempt.
type ResolutionResult struct {
	// Resolved indicates if all conflicts were resolved successfully
	Resolved bool

	// AutoResolved lists files that were auto-resolved (CLAUDE.md patterns)
	AutoResolved []string

	// ClaudeResolved lists files that Claude helped resolve
	ClaudeResolved []string

	// Unresolved lists files that could not be resolved
	Unresolved []string

	// ResolutionLogs contains detailed logs from the resolution process
	ResolutionLogs []string
}

// Resolve attempts to resolve merge conflicts in the given files.
// Returns a ResolutionResult indicating what was resolved.
func (r *ConflictResolver) Resolve(ctx context.Context, t *task.Task, conflictFiles []string) (*ResolutionResult, error) {
	result := &ResolutionResult{
		Unresolved: conflictFiles, // Start with all files unresolved
	}

	if !r.config.ConflictResolution.Enabled || len(conflictFiles) == 0 {
		return result, nil
	}

	if r.gitSvc == nil {
		return result, fmt.Errorf("git service not available")
	}

	// Step 1: Try auto-resolution for known patterns (CLAUDE.md knowledge tables)
	autoResolved, remaining, autoLogs := r.gitSvc.AutoResolveConflicts(conflictFiles, r.logger)
	result.AutoResolved = autoResolved
	result.ResolutionLogs = append(result.ResolutionLogs, autoLogs...)

	if len(autoResolved) > 0 {
		r.logger.Info("auto-resolved conflicts",
			"files", autoResolved,
			"remaining", remaining,
		)
	}

	// If all conflicts were auto-resolved, we're done
	if len(remaining) == 0 {
		result.Resolved = true
		result.Unresolved = nil
		return result, nil
	}

	result.Unresolved = remaining

	// Step 2: Use Claude to resolve remaining conflicts

	claudeResolved, claudeErr := r.resolveWithClaude(ctx, t, remaining)
	if claudeErr != nil {
		r.logger.Warn("Claude-assisted conflict resolution failed", "error", claudeErr)
		return result, claudeErr
	}

	if claudeResolved {
		result.ClaudeResolved = remaining
		result.Unresolved = nil
		result.Resolved = true
	}

	return result, nil
}

// resolveWithClaude uses Claude to resolve conflicts.
func (r *ConflictResolver) resolveWithClaude(ctx context.Context, t *task.Task, conflictFiles []string) (bool, error) {
	// Build conflict resolution prompt
	prompt, err := r.buildConflictResolutionPrompt(t, conflictFiles)
	if err != nil {
		return false, fmt.Errorf("build conflict resolution prompt: %w", err)
	}

	// Inject "ultrathink" for extended thinking mode
	if r.thinking {
		prompt = "ultrathink\n\n" + prompt
		r.logger.Debug("extended thinking enabled for conflict resolution", "task", t.ID)
	}

	// Use injected turnExecutor if available, otherwise create ClaudeExecutor
	var turnExec TurnExecutor
	if r.turnExecutor != nil {
		turnExec = r.turnExecutor
	} else {
		claudeOpts := []ClaudeExecutorOption{
			WithClaudePath(r.claudePath),
			WithClaudeWorkdir(r.workingDir),
			WithClaudeModel(r.model),
			WithClaudeMaxTurns(5), // Limited turns for conflict resolution
			WithClaudeLogger(r.logger),
		}
		if r.mcpConfigPath != "" {
			claudeOpts = append(claudeOpts, WithClaudeMCPConfig(r.mcpConfigPath))
		}
		turnExec = NewClaudeExecutor(claudeOpts...)
	}

	// Execute conflict resolution without schema - we verify success by checking git status
	_, err = turnExec.ExecuteTurnWithoutSchema(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("conflict resolution turn: %w", err)
	}

	// Verify no unmerged files remain - this is the real success check
	gitCtx := r.gitSvc.Context()
	unmerged, _ := gitCtx.RunGit("diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(unmerged) == "" {
		// All conflicts resolved, commit the merge
		_, commitErr := gitCtx.RunGit("commit", "--no-edit")
		return commitErr == nil, commitErr
	}

	return false, fmt.Errorf("conflict resolution incomplete: unmerged files remain")
}

// buildConflictResolutionPrompt creates the prompt for conflict resolution.
func (r *ConflictResolver) buildConflictResolutionPrompt(t *task.Task, conflictFiles []string) (string, error) {
	// Load template from centralized templates
	tmplContent, err := templates.Prompts.ReadFile("prompts/conflict_resolution.md")
	if err != nil {
		return "", fmt.Errorf("read conflict resolution template: %w", err)
	}

	tmpl, err := template.New("conflict_resolution").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse conflict resolution template: %w", err)
	}

	data := map[string]any{
		"TaskID":        t.ID,
		"TaskTitle":     t.Title,
		"ConflictFiles": conflictFiles,
		"Instructions":  r.config.ConflictResolution.Instructions,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute conflict resolution template: %w", err)
	}

	return buf.String(), nil
}

// BranchLock provides per-branch serialization for conflict resolution.
// This ensures only one conflict resolution agent operates on a target branch at a time.
type BranchLock struct {
	mu     sync.Mutex
	active map[string]chan struct{} // branch → done channel
}

// NewBranchLock creates a new branch lock.
func NewBranchLock() *BranchLock {
	return &BranchLock{
		active: make(map[string]chan struct{}),
	}
}

// Acquire obtains a lock for the given branch.
// If another operation is in progress on the branch, it blocks until complete.
// Returns a release function that MUST be called when done.
func (l *BranchLock) Acquire(branch string) func() {
	l.mu.Lock()

	// Wait for any existing operation on this branch
	if done, exists := l.active[branch]; exists {
		l.mu.Unlock()
		<-done // Wait for completion
		l.mu.Lock()
	}

	// Create a new done channel for this operation
	done := make(chan struct{})
	l.active[branch] = done

	l.mu.Unlock()

	// Return release function
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		close(done)
		delete(l.active, branch)
	}
}

// TryAcquire attempts to obtain a lock for the given branch without blocking.
// Returns (release function, true) if lock acquired, (nil, false) if busy.
func (l *BranchLock) TryAcquire(branch string) (func(), bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if branch is busy
	if _, exists := l.active[branch]; exists {
		return nil, false
	}

	// Create a new done channel for this operation
	done := make(chan struct{})
	l.active[branch] = done

	// Return release function
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		close(done)
		delete(l.active, branch)
	}, true
}
