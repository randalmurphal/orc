// Package skills provides Claude Code skills management.
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a Claude Code skill.
type Skill struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Prompt      string `yaml:"prompt" json:"prompt"`
}

// SkillInfo contains summary information about a skill.
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Service manages Claude Code skills.
type Service struct {
	claudeDir string
}

// NewService creates a new skills service.
func NewService(claudeDir string) *Service {
	return &Service{claudeDir: claudeDir}
}

// DefaultService creates a service using the default .claude directory.
func DefaultService() *Service {
	return NewService(".claude")
}

// skillsDir returns the path to the skills directory.
func (s *Service) skillsDir() string {
	return filepath.Join(s.claudeDir, "skills")
}

// skillPath returns the path to a specific skill file.
func (s *Service) skillPath(name string) string {
	return filepath.Join(s.skillsDir(), name+".yaml")
}

// List returns all skills.
func (s *Service) List() ([]SkillInfo, error) {
	skillsDir := s.skillsDir()
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SkillInfo{}, nil
		}
		return nil, fmt.Errorf("read skills directory: %w", err)
	}

	var skills []SkillInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Support both .yaml and .yml extensions
		name := entry.Name()
		var skillName string
		if strings.HasSuffix(name, ".yaml") {
			skillName = strings.TrimSuffix(name, ".yaml")
		} else if strings.HasSuffix(name, ".yml") {
			skillName = strings.TrimSuffix(name, ".yml")
		} else {
			continue
		}
		skill, err := s.Get(skillName)
		if err != nil {
			continue // Skip invalid skills
		}

		skills = append(skills, SkillInfo{
			Name:        skill.Name,
			Description: skill.Description,
		})
	}

	// Sort by name
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})

	// Ensure we return an empty array, not null
	if skills == nil {
		skills = []SkillInfo{}
	}

	return skills, nil
}

// Get returns a specific skill by name.
func (s *Service) Get(name string) (*Skill, error) {
	// Try .yaml first, then .yml
	path := s.skillPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try .yml
			path = filepath.Join(s.skillsDir(), name+".yml")
			data, err = os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, fmt.Errorf("skill not found: %s", name)
				}
				return nil, fmt.Errorf("read skill: %w", err)
			}
		} else {
			return nil, fmt.Errorf("read skill: %w", err)
		}
	}

	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return nil, fmt.Errorf("parse skill: %w", err)
	}

	// Ensure name matches filename
	if skill.Name == "" {
		skill.Name = name
	}

	return &skill, nil
}

// Create creates a new skill.
func (s *Service) Create(skill Skill) error {
	if skill.Name == "" {
		return fmt.Errorf("skill name is required")
	}

	if skill.Prompt == "" {
		return fmt.Errorf("skill prompt is required")
	}

	// Check if skill already exists
	if s.Exists(skill.Name) {
		return fmt.Errorf("skill already exists: %s", skill.Name)
	}

	return s.save(skill)
}

// Update updates an existing skill.
func (s *Service) Update(name string, skill Skill) error {
	// Verify skill exists
	if !s.Exists(name) {
		return fmt.Errorf("skill not found: %s", name)
	}

	// If name changed, delete old file
	if skill.Name != "" && skill.Name != name {
		if err := s.Delete(name); err != nil {
			return fmt.Errorf("remove old skill: %w", err)
		}
	} else {
		skill.Name = name
	}

	return s.save(skill)
}

// Delete deletes a skill.
func (s *Service) Delete(name string) error {
	// Try .yaml first
	path := s.skillPath(name)
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Try .yml
			path = filepath.Join(s.skillsDir(), name+".yml")
			err = os.Remove(path)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("skill not found: %s", name)
				}
				return fmt.Errorf("delete skill: %w", err)
			}
			return nil
		}
		return fmt.Errorf("delete skill: %w", err)
	}
	return nil
}

// Exists checks if a skill exists.
func (s *Service) Exists(name string) bool {
	// Check both extensions
	path := s.skillPath(name)
	if _, err := os.Stat(path); err == nil {
		return true
	}
	path = filepath.Join(s.skillsDir(), name+".yml")
	_, err := os.Stat(path)
	return err == nil
}

// save writes a skill to disk.
func (s *Service) save(skill Skill) error {
	// Ensure skills directory exists
	if err := os.MkdirAll(s.skillsDir(), 0755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	data, err := yaml.Marshal(skill)
	if err != nil {
		return fmt.Errorf("marshal skill: %w", err)
	}

	path := s.skillPath(skill.Name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write skill: %w", err)
	}

	return nil
}
