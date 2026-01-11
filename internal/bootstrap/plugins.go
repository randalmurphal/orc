package bootstrap

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed plugins/*
var embeddedPlugins embed.FS

const (
	// PluginDir is the directory where Claude plugins are stored.
	PluginDir = ".claude/plugins"

	// OrcPluginName is the name of the orc plugin.
	OrcPluginName = "orc"
)

// InstallPlugins installs orc plugins into the project's .claude/plugins directory.
func InstallPlugins(projectDir string) error {
	pluginsDir := filepath.Join(projectDir, PluginDir)

	// Install orc plugin
	if err := installPlugin(pluginsDir, OrcPluginName); err != nil {
		return fmt.Errorf("install orc plugin: %w", err)
	}

	return nil
}

// installPlugin copies an embedded plugin to the plugins directory.
func installPlugin(pluginsDir, pluginName string) error {
	srcDir := "plugins/" + pluginName
	dstDir := filepath.Join(pluginsDir, pluginName)

	// Walk the embedded plugin files
	err := fs.WalkDir(embeddedPlugins, srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from srcDir
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("rel path: %w", err)
		}

		dstPath := filepath.Join(dstDir, relPath)

		if d.IsDir() {
			// Create directory
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return fmt.Errorf("create directory %s: %w", dstPath, err)
			}
			return nil
		}

		// Read file content
		content, err := embeddedPlugins.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded file %s: %w", path, err)
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("create parent directory: %w", err)
		}

		// Write file
		if err := os.WriteFile(dstPath, content, 0644); err != nil {
			return fmt.Errorf("write file %s: %w", dstPath, err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walk embedded plugin: %w", err)
	}

	return nil
}

// PluginsInstalled checks if orc plugins are already installed.
func PluginsInstalled(projectDir string) bool {
	pluginPath := filepath.Join(projectDir, PluginDir, OrcPluginName, ".claude-plugin", "plugin.json")
	_, err := os.Stat(pluginPath)
	return err == nil
}
