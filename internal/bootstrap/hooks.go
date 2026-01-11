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
)

// InstallHooks installs orc hooks into the project's .claude/hooks directory.
func InstallHooks(projectDir string) error {
	hooksDir := filepath.Join(projectDir, HookDir)

	// Create hooks directory if it doesn't exist
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	// Install orc stop hook
	hookPath := filepath.Join(hooksDir, OrcStopHook)

	// Read embedded hook
	content, err := embeddedHooks.ReadFile("hooks/" + OrcStopHook)
	if err != nil {
		return fmt.Errorf("read embedded hook: %w", err)
	}

	// Write hook file
	if err := os.WriteFile(hookPath, content, 0755); err != nil {
		return fmt.Errorf("write hook file: %w", err)
	}

	return nil
}

// HooksInstalled checks if orc hooks are already installed.
func HooksInstalled(projectDir string) bool {
	hookPath := filepath.Join(projectDir, HookDir, OrcStopHook)
	_, err := os.Stat(hookPath)
	return err == nil
}
