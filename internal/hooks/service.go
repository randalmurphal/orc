// Package hooks provides Claude Code hooks management.
package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// HookType represents the type of hook trigger.
type HookType string

const (
	HookPreTool      HookType = "pre:tool"
	HookPostTool     HookType = "post:tool"
	HookPreCommand   HookType = "pre:command"
	HookPostCommand  HookType = "post:command"
	HookPromptSubmit HookType = "prompt:submit"
)

// Hook represents a Claude Code hook configuration.
type Hook struct {
	Name     string   `json:"name"`
	Type     HookType `json:"type"`
	Pattern  string   `json:"pattern,omitempty"` // Tool/command pattern to match
	Command  string   `json:"command"`           // Command to execute
	Timeout  int      `json:"timeout,omitempty"` // Timeout in seconds
	Disabled bool     `json:"disabled,omitempty"`
}

// HookInfo contains summary information about a hook.
type HookInfo struct {
	Name     string   `json:"name"`
	Type     HookType `json:"type"`
	Pattern  string   `json:"pattern,omitempty"`
	Disabled bool     `json:"disabled"`
}

// isValidHookType checks if the given type is a valid hook type.
func isValidHookType(t HookType) bool {
	switch t {
	case HookPreTool, HookPostTool, HookPreCommand, HookPostCommand, HookPromptSubmit:
		return true
	default:
		return false
	}
}

// Service manages Claude Code hooks.
type Service struct {
	claudeDir string
}

// NewService creates a new hooks service.
func NewService(claudeDir string) *Service {
	return &Service{claudeDir: claudeDir}
}

// DefaultService creates a service using the default .claude directory.
func DefaultService() *Service {
	return NewService(".claude")
}

// hooksDir returns the path to the hooks directory.
func (s *Service) hooksDir() string {
	return filepath.Join(s.claudeDir, "hooks")
}

// hookPath returns the path to a specific hook file.
func (s *Service) hookPath(name string) string {
	return filepath.Join(s.hooksDir(), name+".json")
}

// List returns all hooks.
func (s *Service) List() ([]HookInfo, error) {
	hooksDir := s.hooksDir()
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []HookInfo{}, nil
		}
		return nil, fmt.Errorf("read hooks directory: %w", err)
	}

	var hooks []HookInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		hook, err := s.Get(name)
		if err != nil {
			continue // Skip invalid hooks
		}

		hooks = append(hooks, HookInfo{
			Name:     hook.Name,
			Type:     hook.Type,
			Pattern:  hook.Pattern,
			Disabled: hook.Disabled,
		})
	}

	// Sort by name
	sort.Slice(hooks, func(i, j int) bool {
		return hooks[i].Name < hooks[j].Name
	})

	// Ensure we return an empty array, not null
	if hooks == nil {
		hooks = []HookInfo{}
	}

	return hooks, nil
}

// Get returns a specific hook by name.
func (s *Service) Get(name string) (*Hook, error) {
	path := s.hookPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("hook not found: %s", name)
		}
		return nil, fmt.Errorf("read hook: %w", err)
	}

	var hook Hook
	if err := json.Unmarshal(data, &hook); err != nil {
		return nil, fmt.Errorf("parse hook: %w", err)
	}

	// Ensure name matches filename
	hook.Name = name

	return &hook, nil
}

// Create creates a new hook.
func (s *Service) Create(hook Hook) error {
	if hook.Name == "" {
		return fmt.Errorf("hook name is required")
	}

	if hook.Type == "" {
		return fmt.Errorf("hook type is required")
	}

	if !isValidHookType(hook.Type) {
		return fmt.Errorf("invalid hook type: %s", hook.Type)
	}

	if hook.Command == "" {
		return fmt.Errorf("hook command is required")
	}

	// Check if hook already exists
	path := s.hookPath(hook.Name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("hook already exists: %s", hook.Name)
	}

	return s.save(hook)
}

// Update updates an existing hook.
func (s *Service) Update(name string, hook Hook) error {
	// Verify hook exists
	path := s.hookPath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("hook not found: %s", name)
	}

	// Validate required fields
	if hook.Type != "" && !isValidHookType(hook.Type) {
		return fmt.Errorf("invalid hook type: %s", hook.Type)
	}

	// If name changed, save new file first, then delete old (safe rename)
	if hook.Name != "" && hook.Name != name {
		// Save new hook first
		if err := s.save(hook); err != nil {
			return fmt.Errorf("save new hook: %w", err)
		}
		// Then delete old file
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove old hook: %w", err)
		}
		return nil
	}

	hook.Name = name
	return s.save(hook)
}

// Delete deletes a hook.
func (s *Service) Delete(name string) error {
	path := s.hookPath(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("hook not found: %s", name)
		}
		return fmt.Errorf("delete hook: %w", err)
	}
	return nil
}

// Exists checks if a hook exists.
func (s *Service) Exists(name string) bool {
	path := s.hookPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// save writes a hook to disk.
func (s *Service) save(hook Hook) error {
	// Ensure hooks directory exists
	if err := os.MkdirAll(s.hooksDir(), 0755); err != nil {
		return fmt.Errorf("create hooks directory: %w", err)
	}

	data, err := json.MarshalIndent(hook, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hook: %w", err)
	}

	path := s.hookPath(hook.Name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write hook: %w", err)
	}

	return nil
}

// GetHookTypes returns all valid hook types.
func GetHookTypes() []HookType {
	return []HookType{
		HookPreTool,
		HookPostTool,
		HookPreCommand,
		HookPostCommand,
		HookPromptSubmit,
	}
}
