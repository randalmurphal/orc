// Package executor provides conflict resolution for orc.
// This module provides shared conflict resolution logic used by both the
// finalize phase and normal completion flow.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/task"
)

// ConflictResolver handles conflict resolution for task branches.
// It first tries auto-resolution for known patterns (CLAUDE.md knowledge tables),
// then falls back to Claude-assisted resolution for remaining conflicts.
type ConflictResolver struct {
	gitSvc     *git.Git
	manager    session.SessionManager
	logger     *slog.Logger
	config     config.FinalizeConfig
	workingDir string

	// Model settings for conflict resolution
	model    string
	thinking bool
}

// ConflictResolverOption configures a ConflictResolver.
type ConflictResolverOption func(*ConflictResolver)

// WithResolverGitSvc sets the git service.
func WithResolverGitSvc(svc *git.Git) ConflictResolverOption {
	return func(r *ConflictResolver) { r.gitSvc = svc }
}

// WithResolverSessionManager sets the session manager.
func WithResolverSessionManager(mgr session.SessionManager) ConflictResolverOption {
	return func(r *ConflictResolver) { r.manager = mgr }
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

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(opts ...ConflictResolverOption) *ConflictResolver {
	r := &ConflictResolver{
		logger: slog.Default(),
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
	if r.manager == nil {
		r.logger.Debug("session manager not available, skipping Claude-assisted resolution")
		return result, nil
	}

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
	prompt := r.buildConflictResolutionPrompt(t, conflictFiles)

	// Inject "ultrathink" for extended thinking mode
	if r.thinking {
		prompt = "ultrathink\n\n" + prompt
		r.logger.Debug("extended thinking enabled for conflict resolution", "task", t.ID)
	}

	// Create session for conflict resolution
	adapterOpts := SessionAdapterOptions{
		SessionID:   fmt.Sprintf("%s-conflict-resolution", t.ID),
		Model:       r.model,
		Workdir:     r.workingDir,
		MaxTurns:    5, // Limited turns for conflict resolution
		Persistence: false,
	}

	adapter, err := NewSessionAdapter(ctx, r.manager, adapterOpts)
	if err != nil {
		return false, fmt.Errorf("create conflict resolution session: %w", err)
	}
	defer func() { _ = adapter.Close() }()

	// Execute conflict resolution
	turnResult, err := adapter.ExecuteTurn(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("conflict resolution turn: %w", err)
	}

	// Check if Claude indicated success
	if turnResult.Status == PhaseStatusComplete {
		// Verify no unmerged files remain
		ctx := r.gitSvc.Context()
		unmerged, _ := ctx.RunGit("diff", "--name-only", "--diff-filter=U")
		if strings.TrimSpace(unmerged) == "" {
			// All conflicts resolved, commit the merge
			_, commitErr := ctx.RunGit("commit", "--no-edit")
			return commitErr == nil, commitErr
		}
	}

	return false, fmt.Errorf("conflict resolution incomplete")
}

// buildConflictResolutionPrompt creates the prompt for conflict resolution.
func (r *ConflictResolver) buildConflictResolutionPrompt(t *task.Task, conflictFiles []string) string {
	cfg := r.config.ConflictResolution
	var sb strings.Builder

	sb.WriteString("# Conflict Resolution Task\n\n")
	sb.WriteString("You are resolving merge conflicts for task: ")
	sb.WriteString(t.ID)
	sb.WriteString(" - ")
	sb.WriteString(t.Title)
	sb.WriteString("\n\n")

	sb.WriteString("## Conflicted Files\n\n")
	for _, f := range conflictFiles {
		sb.WriteString("- `")
		sb.WriteString(f)
		sb.WriteString("`\n")
	}

	sb.WriteString("\n## Conflict Resolution Rules\n\n")
	sb.WriteString("**CRITICAL - You MUST follow these rules:**\n\n")
	sb.WriteString("1. **NEVER remove features** - Both your changes AND upstream changes must be preserved\n")
	sb.WriteString("2. **Merge intentions, not text** - Understand what each side was trying to accomplish\n")
	sb.WriteString("3. **Prefer additive resolution** - If in doubt, keep both implementations\n")
	sb.WriteString("4. **Test after every file** - Don't batch conflict resolutions\n\n")

	sb.WriteString("## Prohibited Resolutions\n\n")
	sb.WriteString("- **NEVER**: Just take \"ours\" or \"theirs\" without understanding\n")
	sb.WriteString("- **NEVER**: Remove upstream features to fix conflicts\n")
	sb.WriteString("- **NEVER**: Remove your features to fix conflicts\n")
	sb.WriteString("- **NEVER**: Comment out conflicting code\n\n")

	// Add custom instructions if provided
	if cfg.Instructions != "" {
		sb.WriteString("## Additional Instructions\n\n")
		sb.WriteString(cfg.Instructions)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. For each conflicted file, read and understand both sides of the conflict\n")
	sb.WriteString("2. Resolve the conflict by merging both changes appropriately\n")
	sb.WriteString("3. Stage the resolved file with `git add <file>`\n")
	sb.WriteString("4. After all files are resolved, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "complete", "summary": "Resolved X conflicts in files A, B, C"}`)
	sb.WriteString("\n\nIf you cannot resolve a conflict, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "blocked", "reason": "[explanation]"}`)
	sb.WriteString("\n")

	return sb.String()
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
