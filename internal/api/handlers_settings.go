package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// handleGetSettings returns merged settings (global + project).
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadSettings(s.getProjectRoot())
	if err != nil {
		// Return empty settings on error
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleGetGlobalSettings returns global-only settings from ~/.claude/settings.json.
func (s *Server) handleGetGlobalSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadGlobalSettings()
	if err != nil {
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleGetProjectSettings returns project-only settings.
func (s *Server) handleGetProjectSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleUpdateSettings saves settings to either project or global scope.
// Query parameter ?scope=global saves to ~/.claude/settings.json
// Otherwise saves to {projectRoot}/.claude/settings.json
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings claudeconfig.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	scope := r.URL.Query().Get("scope")

	if scope == "global" {
		if err := claudeconfig.SaveGlobalSettings(&settings); err != nil {
			s.jsonError(w, fmt.Sprintf("failed to save global settings: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		if err := claudeconfig.SaveProjectSettings(s.getProjectRoot(), &settings); err != nil {
			s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
			return
		}
	}

	s.jsonResponse(w, settings)
}

// SettingsHierarchyResponse contains settings from each level with source tracking.
type SettingsHierarchyResponse struct {
	Merged  *claudeconfig.Settings        `json:"merged"`
	Global  *SettingsLevel                `json:"global"`
	Project *SettingsLevel                `json:"project"`
	Sources map[string]SettingsSourceInfo `json:"sources"`
}

// SettingsLevel represents settings from a specific level.
type SettingsLevel struct {
	Path     string                 `json:"path"`
	Settings *claudeconfig.Settings `json:"settings,omitempty"`
}

// SettingsSourceInfo indicates which level a setting came from.
type SettingsSourceInfo struct {
	Source string `json:"source"` // "global", "project", or "default"
	Path   string `json:"path,omitempty"`
}

// handleGetSettingsHierarchy returns settings with source information.
func (s *Server) handleGetSettingsHierarchy(w http.ResponseWriter, r *http.Request) {
	projectRoot := s.getProjectRoot()

	// Load settings from each level
	globalSettings, _ := claudeconfig.LoadGlobalSettings()
	projectSettings, _ := claudeconfig.LoadProjectSettings(projectRoot)
	mergedSettings, _ := claudeconfig.LoadSettings(projectRoot)

	// Determine global path
	globalPath, _ := claudeconfig.GlobalSettingsPath()

	// Determine project path
	projectPath := projectRoot + "/.claude/settings.json"

	// Build sources map by comparing merged values to each level
	sources := make(map[string]SettingsSourceInfo)

	// Check env settings
	if mergedSettings != nil && mergedSettings.Env != nil {
		for key := range mergedSettings.Env {
			source := determineSettingSource(key, "env", globalSettings, projectSettings, globalPath, projectPath)
			sources["env."+key] = source
		}
	}

	// Check hooks
	if mergedSettings != nil && mergedSettings.Hooks != nil {
		for event := range mergedSettings.Hooks {
			source := determineHookSource(event, globalSettings, projectSettings, globalPath, projectPath)
			sources["hooks."+event] = source
		}
	}

	// Check permissions (allow/deny lists)
	if mergedSettings != nil && mergedSettings.Permissions != nil {
		if len(mergedSettings.Permissions.Allow) > 0 {
			source := "default"
			path := ""
			if projectSettings != nil && projectSettings.Permissions != nil && len(projectSettings.Permissions.Allow) > 0 {
				source = "project"
				path = projectPath
			} else if globalSettings != nil && globalSettings.Permissions != nil && len(globalSettings.Permissions.Allow) > 0 {
				source = "global"
				path = globalPath
			}
			sources["permissions.allow"] = SettingsSourceInfo{Source: source, Path: path}
		}

		if len(mergedSettings.Permissions.Deny) > 0 {
			source := "default"
			path := ""
			if projectSettings != nil && projectSettings.Permissions != nil && len(projectSettings.Permissions.Deny) > 0 {
				source = "project"
				path = projectPath
			} else if globalSettings != nil && globalSettings.Permissions != nil && len(globalSettings.Permissions.Deny) > 0 {
				source = "global"
				path = globalPath
			}
			sources["permissions.deny"] = SettingsSourceInfo{Source: source, Path: path}
		}
	}

	// Check statusLine
	if mergedSettings != nil && mergedSettings.StatusLine != nil {
		source := "default"
		path := ""
		if projectSettings != nil && projectSettings.StatusLine != nil {
			source = "project"
			path = projectPath
		} else if globalSettings != nil && globalSettings.StatusLine != nil {
			source = "global"
			path = globalPath
		}
		sources["statusLine"] = SettingsSourceInfo{Source: source, Path: path}
	}

	response := SettingsHierarchyResponse{
		Merged: mergedSettings,
		Global: &SettingsLevel{
			Path:     globalPath,
			Settings: globalSettings,
		},
		Project: &SettingsLevel{
			Path:     projectPath,
			Settings: projectSettings,
		},
		Sources: sources,
	}

	s.jsonResponse(w, response)
}

// determineSettingSource determines which level a setting came from.
func determineSettingSource(key, settingType string, global, project *claudeconfig.Settings, globalPath, projectPath string) SettingsSourceInfo {
	if settingType == "env" {
		if project != nil && project.Env != nil {
			if _, ok := project.Env[key]; ok {
				return SettingsSourceInfo{Source: "project", Path: projectPath}
			}
		}
		if global != nil && global.Env != nil {
			if _, ok := global.Env[key]; ok {
				return SettingsSourceInfo{Source: "global", Path: globalPath}
			}
		}
	}
	return SettingsSourceInfo{Source: "default"}
}

// determineHookSource determines which level a hook came from.
func determineHookSource(event string, global, project *claudeconfig.Settings, globalPath, projectPath string) SettingsSourceInfo {
	if project != nil && project.Hooks != nil {
		if _, ok := project.Hooks[event]; ok {
			return SettingsSourceInfo{Source: "project", Path: projectPath}
		}
	}
	if global != nil && global.Hooks != nil {
		if _, ok := global.Hooks[event]; ok {
			return SettingsSourceInfo{Source: "global", Path: globalPath}
		}
	}
	return SettingsSourceInfo{Source: "default"}
}
