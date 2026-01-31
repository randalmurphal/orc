// Package api provides the Connect RPC and REST API server for orc.
// This file implements export/import/scan handlers for hooks and skills.
package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// resolveDestinationDir returns the base directory for the given scope.
func (s *configServer) resolveDestinationDir(projectID string, scope orcv1.SettingsScope) (string, error) {
	switch scope {
	case orcv1.SettingsScope_SETTINGS_SCOPE_PROJECT:
		workDir, err := s.getWorkDir(projectID)
		if err != nil {
			return "", err
		}
		return workDir, nil
	case orcv1.SettingsScope_SETTINGS_SCOPE_GLOBAL:
		if s.testHomeDir != "" {
			return s.testHomeDir, nil
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		return homeDir, nil
	default:
		return "", fmt.Errorf("destination must be PROJECT or GLOBAL")
	}
}

// ExportHooks exports hook scripts from GlobalDB to .claude/hooks/ directory.
func (s *configServer) ExportHooks(
	ctx context.Context,
	req *connect.Request[orcv1.ExportHooksRequest],
) (*connect.Response[orcv1.ExportHooksResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	if len(req.Msg.HookIds) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one hook ID required"))
	}

	baseDir, err := s.resolveDestinationDir(req.Msg.ProjectId, req.Msg.Destination)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	hooksDir := filepath.Join(baseDir, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create hooks directory: %w", err))
	}

	var writtenPaths []string
	for _, hookID := range req.Msg.HookIds {
		hs, err := s.globalDB.GetHookScript(hookID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get hook script %s: %w", hookID, err))
		}
		if hs == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("hook %s not found", hookID))
		}

		// Validate hook name for path traversal
		if strings.Contains(hs.Name, "..") || strings.Contains(hs.Name, "/") || strings.Contains(hs.Name, "\\") {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid hook name %q: must not contain path separators", hs.Name))
		}

		filePath := filepath.Join(hooksDir, hs.Name)
		if err := os.WriteFile(filePath, []byte(hs.Content), 0755); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write hook file %s: %w", filePath, err))
		}
		writtenPaths = append(writtenPaths, filePath)
	}

	return connect.NewResponse(&orcv1.ExportHooksResponse{
		WrittenPaths: writtenPaths,
	}), nil
}

// ExportSkills exports skills from GlobalDB to .claude/skills/ directory.
func (s *configServer) ExportSkills(
	ctx context.Context,
	req *connect.Request[orcv1.ExportSkillsRequest],
) (*connect.Response[orcv1.ExportSkillsResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	if len(req.Msg.SkillIds) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one skill ID required"))
	}

	// Validate skill IDs for path traversal before doing any work
	for _, skillID := range req.Msg.SkillIds {
		if strings.Contains(skillID, "..") || strings.Contains(skillID, "/") || strings.Contains(skillID, "\\") {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid skill ID %q: must not contain path separators", skillID))
		}
	}

	baseDir, err := s.resolveDestinationDir(req.Msg.ProjectId, req.Msg.Destination)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var writtenPaths []string
	for _, skillID := range req.Msg.SkillIds {
		sk, err := s.globalDB.GetSkill(skillID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get skill %s: %w", skillID, err))
		}
		if sk == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("skill %s not found", skillID))
		}

		// Validate skill name for path traversal
		if strings.Contains(sk.Name, "..") || strings.Contains(sk.Name, "/") || strings.Contains(sk.Name, "\\") {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid skill name %q: must not contain path separators", sk.Name))
		}

		// Use skill Name as directory name (not ID) to match the convention
		skillDir := filepath.Join(baseDir, ".claude", "skills", sk.Name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create skill directory %s: %w", skillDir, err))
		}

		// Write SKILL.md
		skillMdPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillMdPath, []byte(sk.Content), 0644); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write SKILL.md: %w", err))
		}
		writtenPaths = append(writtenPaths, skillMdPath)

		// Write supporting files
		for filename, content := range sk.SupportingFiles {
			if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("invalid supporting filename %q in skill %q: must not contain path separators", filename, sk.Name))
			}
			filePath := filepath.Join(skillDir, filename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("write supporting file %s: %w", filename, err))
			}
			writtenPaths = append(writtenPaths, filePath)
		}
	}

	return connect.NewResponse(&orcv1.ExportSkillsResponse{
		WrittenPaths: writtenPaths,
	}), nil
}

// ScanClaudeDir scans .claude/ directories for hooks and skills not in GlobalDB.
func (s *configServer) ScanClaudeDir(
	ctx context.Context,
	req *connect.Request[orcv1.ScanClaudeDirRequest],
) (*connect.Response[orcv1.ScanClaudeDirResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	baseDir, err := s.resolveDestinationDir(req.Msg.ProjectId, req.Msg.Source)
	if err != nil {
		// For scan, default to project if unspecified
		if req.Msg.Source == orcv1.SettingsScope_SETTINGS_SCOPE_UNSPECIFIED {
			baseDir = s.workDir
		} else {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}

	var items []*orcv1.DiscoveredItem

	// Scan hooks
	hookItems, err := s.scanHooks(baseDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("scan hooks: %w", err))
	}
	items = append(items, hookItems...)

	// Scan skills
	skillItems, err := s.scanSkills(baseDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("scan skills: %w", err))
	}
	items = append(items, skillItems...)

	return connect.NewResponse(&orcv1.ScanClaudeDirResponse{
		Items: items,
	}), nil
}

const scanPreviewMaxBytes = 10240 // 10KB for scan preview truncation

// scanHooks scans .claude/hooks/ for script files.
func (s *configServer) scanHooks(baseDir string) ([]*orcv1.DiscoveredItem, error) {
	hooksDir := filepath.Join(baseDir, ".claude", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read hooks directory: %w", err)
	}

	// Build name->hook map from GlobalDB for comparison
	existingHooks, err := s.globalDB.ListHookScripts()
	if err != nil {
		return nil, fmt.Errorf("list hook scripts: %w", err)
	}
	hookByName := make(map[string]*db.HookScript)
	for _, hs := range existingHooks {
		hookByName[hs.Name] = hs
	}

	var items []*orcv1.DiscoveredItem
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(hooksDir, entry.Name())
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Skip binary files (containing null bytes)
		if bytes.ContainsRune(content, 0) {
			continue
		}

		// Derive name: strip .sh extension if present
		name := strings.TrimSuffix(entry.Name(), ".sh")

		contentStr := string(content)

		// Check against GlobalDB
		existing, found := hookByName[name]
		if found && existing.Content == contentStr {
			// Already synced, skip
			continue
		}

		status := "new"
		if found {
			status = "modified"
		}

		// Truncate content for preview if too large
		preview := contentStr
		if len(preview) > scanPreviewMaxBytes {
			preview = preview[:scanPreviewMaxBytes]
		}

		items = append(items, &orcv1.DiscoveredItem{
			Name:     name,
			Content:  preview,
			ItemType: "hook",
			Status:   status,
		})
	}

	return items, nil
}

// scanSkills scans .claude/skills/ for skill directories containing SKILL.md.
func (s *configServer) scanSkills(baseDir string) ([]*orcv1.DiscoveredItem, error) {
	skillsDir := filepath.Join(baseDir, ".claude", "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read skills directory: %w", err)
	}

	// Build name->skill map from GlobalDB for comparison
	existingSkills, err := s.globalDB.ListSkills()
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	skillByName := make(map[string]*db.Skill)
	for _, sk := range existingSkills {
		skillByName[sk.Name] = sk
	}

	var items []*orcv1.DiscoveredItem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(skillsDir, entry.Name())
		skillMdPath := filepath.Join(skillDir, "SKILL.md")

		// Must have SKILL.md
		skillContent, err := os.ReadFile(skillMdPath)
		if err != nil {
			continue // No SKILL.md, skip
		}

		name := entry.Name()
		contentStr := string(skillContent)

		// Read supporting files (everything except SKILL.md)
		supportingFiles := make(map[string]string)
		subEntries, err := os.ReadDir(skillDir)
		if err == nil {
			for _, sub := range subEntries {
				if sub.IsDir() || sub.Name() == "SKILL.md" {
					continue
				}
				subContent, err := os.ReadFile(filepath.Join(skillDir, sub.Name()))
				if err == nil {
					supportingFiles[sub.Name()] = string(subContent)
				}
			}
		}

		// Check against GlobalDB
		existing, found := skillByName[name]
		if found && existing.Content == contentStr {
			// Already synced, skip
			continue
		}

		status := "new"
		if found {
			status = "modified"
		}

		items = append(items, &orcv1.DiscoveredItem{
			Name:            name,
			Content:         contentStr,
			ItemType:        "skill",
			Status:          status,
			SupportingFiles: supportingFiles,
		})
	}

	return items, nil
}

// ImportHooks creates GlobalDB entries from discovered hook files.
func (s *configServer) ImportHooks(
	ctx context.Context,
	req *connect.Request[orcv1.ImportHooksRequest],
) (*connect.Response[orcv1.ImportHooksResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	// Validate all items first
	existingHooks, err := s.globalDB.ListHookScripts()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list hook scripts: %w", err))
	}
	hookByName := make(map[string]bool)
	for _, hs := range existingHooks {
		hookByName[hs.Name] = true
	}

	for _, item := range req.Msg.Items {
		if item.Content == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("hook %q has empty content", item.Name))
		}
		if hookByName[item.Name] {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("hook with name %q already exists in library", item.Name))
		}
	}

	var imported []*orcv1.Hook
	for _, item := range req.Msg.Items {
		id := fmt.Sprintf("hook-%d", time.Now().UnixNano())
		hs := &db.HookScript{
			ID:        id,
			Name:      item.Name,
			EventType: item.EventType,
			Content:   item.Content,
			IsBuiltin: false,
		}

		if err := s.globalDB.SaveHookScript(hs); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save hook %s: %w", item.Name, err))
		}

		imported = append(imported, hookScriptToProto(hs))
	}

	return connect.NewResponse(&orcv1.ImportHooksResponse{
		Imported: imported,
	}), nil
}

// ImportSkills creates GlobalDB entries from discovered skill directories.
func (s *configServer) ImportSkills(
	ctx context.Context,
	req *connect.Request[orcv1.ImportSkillsRequest],
) (*connect.Response[orcv1.ImportSkillsResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	// Validate all items first
	existingSkills, err := s.globalDB.ListSkills()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list skills: %w", err))
	}
	skillByName := make(map[string]bool)
	for _, sk := range existingSkills {
		skillByName[sk.Name] = true
	}

	for _, item := range req.Msg.Items {
		if skillByName[item.Name] {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("skill with name %q already exists in library", item.Name))
		}
	}

	var imported []*orcv1.Skill
	for _, item := range req.Msg.Items {
		id := fmt.Sprintf("skill-%d", time.Now().UnixNano())
		sk := &db.Skill{
			ID:              id,
			Name:            item.Name,
			Content:         item.Content,
			SupportingFiles: item.SupportingFiles,
			IsBuiltin:       false,
		}

		if err := s.globalDB.SaveSkill(sk); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save skill %s: %w", item.Name, err))
		}

		imported = append(imported, dbSkillToProto(sk))
	}

	return connect.NewResponse(&orcv1.ImportSkillsResponse{
		Imported: imported,
	}), nil
}
