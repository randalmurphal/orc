// conflict_resolver.go provides automatic conflict resolution via Claude.
// Used during workflow completion and finalize to resolve merge conflicts
// before PR creation.
package executor

import (
	"context"
	"fmt"
	"text/template"
	"log/slog"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/templates"
)

// ConflictResolver resolves merge conflicts using Claude.
type ConflictResolver struct {
	gitOps       *git.Git
	claudePath   string
	codexPath    string
	workingDir   string
	backend      storage.Backend
	logger       *slog.Logger
	turnExecutor TurnExecutor // For testing injection
}

// ConflictResolverOption configures ConflictResolver.
type ConflictResolverOption func(*ConflictResolver)

// WithConflictGitOps sets the git operations interface.
func WithConflictGitOps(g *git.Git) ConflictResolverOption {
	return func(r *ConflictResolver) { r.gitOps = g }
}

// WithConflictClaudePath sets the path to Claude CLI.
func WithConflictClaudePath(path string) ConflictResolverOption {
	return func(r *ConflictResolver) { r.claudePath = path }
}

// WithConflictCodexPath sets the path to Codex CLI.
func WithConflictCodexPath(path string) ConflictResolverOption {
	return func(r *ConflictResolver) { r.codexPath = path }
}

// WithConflictWorkingDir sets the working directory for Claude.
func WithConflictWorkingDir(dir string) ConflictResolverOption {
	return func(r *ConflictResolver) { r.workingDir = dir }
}

// WithConflictBackend sets the storage backend for transcripts.
func WithConflictBackend(b storage.Backend) ConflictResolverOption {
	return func(r *ConflictResolver) { r.backend = b }
}

// WithConflictLogger sets the logger.
func WithConflictLogger(l *slog.Logger) ConflictResolverOption {
	return func(r *ConflictResolver) { r.logger = l }
}

// WithConflictTurnExecutor sets a custom turn executor (for testing).
func WithConflictTurnExecutor(te TurnExecutor) ConflictResolverOption {
	return func(r *ConflictResolver) { r.turnExecutor = te }
}

// NewConflictResolver creates a new conflict resolver.
func NewConflictResolver(opts ...ConflictResolverOption) *ConflictResolver {
	r := &ConflictResolver{
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ConflictResolutionResult contains the outcome of conflict resolution.
type ConflictResolutionResult struct {
	Resolved      bool     // Whether all conflicts were resolved
	ResolvedFiles []string // Files that were successfully resolved
	FailedFiles   []string // Files that could not be resolved
	Error         error    // Error if resolution failed
}

// Resolve attempts to resolve merge conflicts using Claude.
// It reads conflicted files, spawns Claude with a resolution prompt,
// and verifies that conflicts are resolved.
func (r *ConflictResolver) Resolve(
	ctx context.Context,
	t *orcv1.Task,
	conflictFiles []string,
	cfg config.SyncConfig,
) (*ConflictResolutionResult, error) {
	result := &ConflictResolutionResult{}

	if r.gitOps == nil {
		return result, fmt.Errorf("git operations not available")
	}

	if len(conflictFiles) == 0 {
		result.Resolved = true
		return result, nil
	}

	// Build the conflict resolution prompt
	prompt, err := r.buildPrompt(t, conflictFiles)
	if err != nil {
		return result, fmt.Errorf("build prompt: %w", err)
	}

	// Resolve model with default
	model := cfg.ResolveModel
	if model == "" {
		model = "sonnet"
	}

	// Max attempts with default
	maxAttempts := cfg.MaxResolveAttempts
	if maxAttempts <= 0 {
		maxAttempts = 2
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r.logger.Info("attempting conflict resolution",
			"task", t.Id,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"files", len(conflictFiles),
		)

		// Create or use injected turn executor
		var turnExec TurnExecutor
		sessionID := fmt.Sprintf("%s-conflict-resolve-%d", t.Id, attempt)
		if r.turnExecutor != nil {
			turnExec = r.turnExecutor
		} else {
			turnExec = NewTurnExecutor(TurnExecutorConfig{
				Provider:   "claude", // Conflict resolution always uses claude for now
				ClaudePath: r.claudePath,
				CodexPath:  r.codexPath,
				Model:      model,
				WorkingDir: r.workingDir,
				SessionID:  sessionID,
				MaxTurns:   5,
				Backend:    r.backend,
				TaskID:     t.Id,
				Logger:     r.logger,
			})
		}

		// Execute conflict resolution
		_, execErr := turnExec.ExecuteTurn(ctx, prompt)
		if execErr != nil {
			r.logger.Warn("conflict resolution turn failed",
				"attempt", attempt,
				"error", execErr,
			)
			if attempt == maxAttempts {
				result.Error = fmt.Errorf("conflict resolution failed after %d attempts: %w", attempt, execErr)
				return result, result.Error
			}
			continue
		}

		// Check if conflicts are resolved
		unmerged, gitErr := r.gitOps.Context().RunGit("diff", "--name-only", "--diff-filter=U")
		if gitErr != nil {
			return result, fmt.Errorf("check unmerged files: %w", gitErr)
		}

		if strings.TrimSpace(unmerged) == "" {
			// All conflicts resolved
			r.logger.Info("conflicts resolved successfully",
				"attempt", attempt,
				"files", len(conflictFiles),
			)
			result.Resolved = true
			result.ResolvedFiles = conflictFiles
			return result, nil
		}

		// Some conflicts remain
		remaining := strings.Split(strings.TrimSpace(unmerged), "\n")
		result.FailedFiles = remaining

		r.logger.Warn("conflicts remain after resolution attempt",
			"attempt", attempt,
			"remaining", len(remaining),
			"files", remaining,
		)

		// Update prompt for next attempt with remaining files
		var promptErr error
		prompt, promptErr = r.buildPrompt(t, remaining)
		if promptErr != nil {
			r.logger.Warn("failed to rebuild prompt for remaining files, using previous prompt", "error", promptErr)
		}
	}

	// All attempts exhausted
	result.Error = fmt.Errorf("conflict resolution incomplete after %d attempts: %d files still unmerged",
		maxAttempts, len(result.FailedFiles))
	return result, result.Error
}

// buildPrompt creates the conflict resolution prompt from the template.
func (r *ConflictResolver) buildPrompt(t *orcv1.Task, conflictFiles []string) (string, error) {
	// Load the template
	tmplContent, err := templates.Prompts.ReadFile("prompts/conflict_resolution.md")
	if err != nil {
		return "", fmt.Errorf("read conflict_resolution template: %w", err)
	}

	tmpl, err := template.New("conflict_resolution").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	// Build template data
	data := map[string]any{
		"TaskID":        t.Id,
		"TaskTitle":     t.Title,
		"ConflictFiles": conflictFiles,
	}

	// Add task description if available for context
	if t.Description != nil && *t.Description != "" {
		data["TaskDescription"] = *t.Description
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// StageAndContinueRebase stages resolved files and continues the rebase.
// Call this after Resolve() succeeds to complete the rebase.
func (r *ConflictResolver) StageAndContinueRebase(ctx context.Context, resolvedFiles []string) error {
	if r.gitOps == nil {
		return fmt.Errorf("git operations not available")
	}

	// Stage all resolved files
	for _, file := range resolvedFiles {
		if _, err := r.gitOps.Context().RunGit("add", file); err != nil {
			r.logger.Warn("failed to stage resolved file", "file", file, "error", err)
			// Continue trying other files
		}
	}

	// Continue the rebase
	_, err := r.gitOps.Context().RunGit("rebase", "--continue")
	if err != nil {
		// Check if there are more conflicts
		unmerged, _ := r.gitOps.Context().RunGit("diff", "--name-only", "--diff-filter=U")
		if strings.TrimSpace(unmerged) != "" {
			return fmt.Errorf("rebase continue failed, more conflicts exist: %w", err)
		}
		// Might just be a "nothing to commit" situation
		r.logger.Debug("rebase continue returned error but no conflicts", "error", err)
	}

	return nil
}
