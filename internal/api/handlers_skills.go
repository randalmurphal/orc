// Package api provides the REST API and SSE server for orc.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// handleListSkills returns all discovered skills.
// Supports ?scope=global to list from ~/.claude/skills/ instead of project.
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	var claudeDir string
	if scope == "global" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
			return
		}
		claudeDir = filepath.Join(homeDir, ".claude")
	} else {
		claudeDir = filepath.Join(s.getProjectRoot(), ".claude")
	}

	skills, err := claudeconfig.DiscoverSkills(claudeDir)
	if err != nil {
		// No skills directory is OK - return empty list
		s.jsonResponse(w, []claudeconfig.SkillInfo{})
		return
	}

	// Convert to SkillInfo for listing
	infos := make([]claudeconfig.SkillInfo, 0, len(skills))
	for _, skill := range skills {
		infos = append(infos, claudeconfig.SkillInfo{
			Name:        skill.Name,
			Description: skill.Description,
			Path:        skill.Path,
		})
	}

	s.jsonResponse(w, infos)
}

// handleGetSkill returns a specific skill by name.
// Supports ?scope=global to get from ~/.claude/skills/ instead of project.
func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	var skillPath string
	if scope == "global" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
			return
		}
		skillPath = filepath.Join(homeDir, ".claude", "skills", name, "SKILL.md")
	} else {
		skillPath = filepath.Join(s.getProjectRoot(), ".claude", "skills", name, "SKILL.md")
	}

	skill, err := claudeconfig.ParseSkillMD(skillPath)
	if err != nil {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, skill)
}

// handleCreateSkill creates a new skill in SKILL.md format.
// Supports ?scope=global to create in ~/.claude/skills/ instead of project.
func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	var skill claudeconfig.Skill
	if err := json.NewDecoder(r.Body).Decode(&skill); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if skill.Name == "" {
		s.jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	scope := r.URL.Query().Get("scope")
	var skillDir string
	if scope == "global" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
			return
		}
		skillDir = filepath.Join(homeDir, ".claude", "skills", skill.Name)
	} else {
		skillDir = filepath.Join(s.getProjectRoot(), ".claude", "skills", skill.Name)
	}

	// WriteSkillMD creates SKILL.md inside the given directory
	if err := claudeconfig.WriteSkillMD(&skill, skillDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, skill)
}

// handleUpdateSkill updates an existing skill.
// Supports ?scope=global to update in ~/.claude/skills/ instead of project.
func (s *Server) handleUpdateSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	var baseDir string
	if scope == "global" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
			return
		}
		baseDir = filepath.Join(homeDir, ".claude", "skills")
	} else {
		baseDir = filepath.Join(s.getProjectRoot(), ".claude", "skills")
	}

	skillDir := filepath.Join(baseDir, name)

	// Check if skill exists
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); os.IsNotExist(err) {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	var skill claudeconfig.Skill
	if err := json.NewDecoder(r.Body).Decode(&skill); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// If name changed, we need to rename the directory
	if skill.Name != "" && skill.Name != name {
		newDir := filepath.Join(baseDir, skill.Name)
		if err := os.Rename(skillDir, newDir); err != nil {
			s.jsonError(w, fmt.Sprintf("failed to rename skill: %v", err), http.StatusInternalServerError)
			return
		}
		skillDir = newDir
	} else {
		skill.Name = name
	}

	// Write the updated skill to the skill-specific directory
	if err := claudeconfig.WriteSkillMD(&skill, skillDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to update skill: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, skill)
}

// handleDeleteSkill deletes a skill directory.
// Supports ?scope=global to delete from ~/.claude/skills/ instead of project.
func (s *Server) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	scope := r.URL.Query().Get("scope")

	var skillDir string
	if scope == "global" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
			return
		}
		skillDir = filepath.Join(homeDir, ".claude", "skills", name)
	} else {
		skillDir = filepath.Join(s.getProjectRoot(), ".claude", "skills", name)
	}

	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	if err := os.RemoveAll(skillDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to delete skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
