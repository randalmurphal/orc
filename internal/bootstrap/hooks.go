package bootstrap

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed hooks/*
var embeddedHooks embed.FS

const (
	// HookDir is the directory where Claude hooks are stored.
	HookDir = ".claude/hooks"

	// OrcStopHook is the name of the orc stop hook.
	OrcStopHook = "orc-stop.sh"

	// TDDDisciplineHook is the name of the TDD discipline hook.
	// It blocks non-test file modifications during tdd_write phase.
	TDDDisciplineHook = "tdd-discipline.sh"
)

// InstallHooks installs orc hooks into the project's .claude/hooks directory.
func InstallHooks(projectDir string) error {
	hooksDir := filepath.Join(projectDir, HookDir)

	// Create hooks directory if it doesn't exist
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	// Install all hooks
	hooks := []string{OrcStopHook, TDDDisciplineHook}
	for _, hookName := range hooks {
		hookPath := filepath.Join(hooksDir, hookName)

		// Read embedded hook
		content, err := embeddedHooks.ReadFile("hooks/" + hookName)
		if err != nil {
			return fmt.Errorf("read embedded hook %s: %w", hookName, err)
		}

		// Write hook file
		if err := os.WriteFile(hookPath, content, 0755); err != nil {
			return fmt.Errorf("write hook file %s: %w", hookName, err)
		}
	}

	return nil
}

