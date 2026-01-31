package setup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// SpawnerOptions configures the Claude spawner.
type SpawnerOptions struct {
	// WorkDir is the working directory for Claude
	WorkDir string

	// Model is the Claude model to use
	Model string

	// ClaudePath is the path to the claude binary (default: "claude")
	ClaudePath string

	// DangerouslySkipPermissions skips permission prompts
	DangerouslySkipPermissions bool
}

// Spawner manages spawning Claude processes.
type Spawner struct {
	opts SpawnerOptions
}

// NewSpawner creates a new Claude spawner.
func NewSpawner(opts SpawnerOptions) *Spawner {
	if opts.ClaudePath == "" {
		opts.ClaudePath = "claude"
	}
	return &Spawner{opts: opts}
}

// RunInteractive spawns an interactive Claude session with the given prompt.
// This connects Claude's stdin/stdout/stderr to the terminal for interactive use.
func (s *Spawner) RunInteractive(ctx context.Context, prompt string) error {
	args := []string{
		prompt,
	}

	if s.opts.Model != "" {
		args = append(args, "--model", s.opts.Model)
	}

	if s.opts.DangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	cmd := exec.CommandContext(ctx, s.opts.ClaudePath, args...)
	cmd.Dir = s.opts.WorkDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("claude exited with error: %w", err)
	}

	return nil
}

