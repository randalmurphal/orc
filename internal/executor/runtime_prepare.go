package executor

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/randalmurphal/orc/internal/db"
)

// HookScriptGetter retrieves hook scripts by ID.
type HookScriptGetter interface {
	GetHookScript(id string) (*db.HookScript, error)
}

// SkillGetter retrieves skills by ID.
type SkillGetter interface {
	GetSkill(id string) (*db.Skill, error)
}

var hookRefPattern = regexp.MustCompile(`\{\{hook:([^}]+)\}\}`)

func PreparePhaseRuntime(
	ctx context.Context,
	provider string,
	worktreePath string,
	phaseCfg *PhaseRuntimeConfig,
	baseCfg *WorktreeBaseConfig,
	hsGetter HookScriptGetter,
	sGetter SkillGetter,
) (*llmkit.PreparedRuntime, error) {
	if worktreePath == "" {
		return nil, nil
	}

	cfg := phaseCfg
	if cfg == nil {
		cfg = &PhaseRuntimeConfig{}
	} else {
		clone := *cfg
		cfg = &clone
	}

	if len(baseCfg.AdditionalEnv) > 0 {
		if cfg.Shared.Env == nil {
			cfg.Shared.Env = make(map[string]string)
		}
		for k, v := range baseCfg.AdditionalEnv {
			cfg.Shared.Env[k] = v
		}
	}

	assets, err := buildRuntimeAssets(provider, worktreePath, cfg, baseCfg, hsGetter, sGetter)
	if err != nil {
		return nil, err
	}

	prepared, err := llmkit.PrepareRuntime(ctx, llmkit.PrepareRequest{
		Provider:       provider,
		WorkDir:        worktreePath,
		RuntimeConfig:  cfg.ToLLMKit(),
		Assets:         assets,
		Tag:            baseCfg.TaskID,
		RecoverOrphans: true,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare runtime: %w", err)
	}
	return prepared, nil
}

func buildRuntimeAssets(
	provider string,
	worktreePath string,
	cfg *PhaseRuntimeConfig,
	baseCfg *WorktreeBaseConfig,
	hsGetter HookScriptGetter,
	sGetter SkillGetter,
) (*llmkit.RuntimeAssets, error) {
	if provider != ProviderClaude {
		return nil, nil
	}

	if cfg.Providers.Claude == nil {
		cfg.Providers.Claude = &llmkit.ClaudeRuntimeConfig{}
	}

	assets := &llmkit.RuntimeAssets{
		Skills:      map[string]llmkit.SkillAsset{},
		HookScripts: map[string]string{},
	}

	if hsGetter != nil && baseCfg != nil {
		isolationScript, err := hsGetter.GetHookScript("orc-worktree-isolation")
		if err != nil {
			return nil, fmt.Errorf("get isolation hook script: %w", err)
		}
		if isolationScript != nil {
			const filename = "orc-worktree-isolation.py"
			assets.HookScripts[filename] = isolationScript.Content
			cfg.Providers.Claude.Hooks = appendRuntimeHook(
				cfg.Providers.Claude.Hooks,
				"PreToolUse",
				llmkit.HookMatcher{
					Matcher: "Edit|Write|Read|Glob|Grep|MultiEdit",
					Hooks: []llmkit.HookEntry{{
						Type: "command",
						Command: fmt.Sprintf(
							`ORC_WORKTREE_PATH="%s" ORC_MAIN_REPO_PATH="%s" ORC_TASK_ID="%s" python3 "%s"`,
							baseCfg.WorktreePath,
							baseCfg.MainRepoPath,
							baseCfg.TaskID,
							filepath.Join(worktreePath, ".claude", "hooks", filename),
						),
					}},
				},
			)
		}
	}

	if hsGetter != nil {
		for _, hookID := range collectHookScriptIDs(cfg) {
			hs, err := hsGetter.GetHookScript(hookID)
			if err != nil {
				return nil, fmt.Errorf("get hook script %s: %w", hookID, err)
			}
			if hs == nil {
				return nil, fmt.Errorf("hook script %q not found in database", hookID)
			}
			assets.HookScripts[hookID] = hs.Content
		}
	}

	if sGetter != nil && cfg.Providers.Claude != nil {
		for _, skillID := range cfg.Providers.Claude.SkillRefs {
			skill, err := sGetter.GetSkill(skillID)
			if err != nil {
				return nil, fmt.Errorf("get skill %s: %w", skillID, err)
			}
			if skill == nil {
				return nil, fmt.Errorf("skill %q not found in database", skillID)
			}
			assets.Skills[skillID] = llmkit.SkillAsset{
				Name:            skill.Name,
				Description:     skill.Description,
				Content:         skill.Content,
				SupportingFiles: skill.SupportingFiles,
			}
		}
	}

	if len(assets.Skills) == 0 {
		assets.Skills = nil
	}
	if len(assets.HookScripts) == 0 {
		assets.HookScripts = nil
	}
	return assets, nil
}

func appendRuntimeHook(
	hooks map[string][]llmkit.HookMatcher,
	event string,
	matcher llmkit.HookMatcher,
) map[string][]llmkit.HookMatcher {
	if hooks == nil {
		hooks = make(map[string][]llmkit.HookMatcher)
	}
	hooks[event] = append(hooks[event], matcher)
	return hooks
}

func collectHookScriptIDs(cfg *PhaseRuntimeConfig) []string {
	if cfg == nil || cfg.Providers.Claude == nil {
		return nil
	}

	seen := make(map[string]bool)
	var ids []string
	for _, matchers := range cfg.Providers.Claude.Hooks {
		for _, matcher := range matchers {
			for _, hook := range matcher.Hooks {
				matches := hookRefPattern.FindAllStringSubmatch(hook.Command, -1)
				for _, match := range matches {
					id := match[1]
					if !seen[id] {
						seen[id] = true
						ids = append(ids, id)
					}
				}
			}
		}
	}
	return ids
}
