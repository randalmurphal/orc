/**
 * Shared utilities for parsing, serializing, and merging claude_config JSON.
 *
 * Used by:
 * - PhaseListEditor (edit dialog with inherited/override distinction)
 * - PhaseInspector (read-only effective config summary)
 * - EditPhaseTemplateModal (template-level claude_config editing)
 */

export interface ClaudeConfigState {
	hooks: string[];
	skillRefs: string[];
	mcpServers: string[];
	allowedTools: string[];
	disallowedTools: string[];
	env: Record<string, string>;
	extra: Record<string, unknown>;
}

const DEFAULTS: ClaudeConfigState = {
	hooks: [],
	skillRefs: [],
	mcpServers: [],
	allowedTools: [],
	disallowedTools: [],
	env: {},
	extra: {},
};

/**
 * Extract hook script names from Claude Code hook config format.
 * Looks for {{hook:id}} patterns in hook commands.
 * Falls back to event type labels when no script references found.
 *
 * Example input: {"Stop": [{"hooks": [{"type": "command", "command": "bash {{hook:orc-verify-completion}}"}]}]}
 * Example output: ["orc-verify-completion"]
 */
function extractHookRefsFromConfig(hooks: Record<string, unknown>): string[] {
	const refs = new Set<string>();
	const hookRefPattern = /\{\{hook:([^}]+)\}\}/g;

	for (const [eventType, matchers] of Object.entries(hooks)) {
		if (!Array.isArray(matchers)) continue;
		let foundRef = false;
		for (const matcher of matchers) {
			if (!matcher || typeof matcher !== 'object') continue;
			const hookEntries = (matcher as Record<string, unknown>).hooks;
			if (!Array.isArray(hookEntries)) continue;
			for (const entry of hookEntries) {
				if (!entry || typeof entry !== 'object') continue;
				const command = (entry as Record<string, unknown>).command;
				if (typeof command === 'string') {
					let match;
					while ((match = hookRefPattern.exec(command)) !== null) {
						refs.add(match[1]);
						foundRef = true;
					}
				}
			}
		}
		if (!foundRef) {
			refs.add(`${eventType} hook`);
		}
	}

	return [...refs];
}

/** Parse claude_config JSON string into structured state.
 * Handles both simple name arrays (user overrides) and Claude Code event map format (templates). */
export function parseClaudeConfig(configStr: string | undefined): ClaudeConfigState {
	if (!configStr) return { ...DEFAULTS, env: {}, extra: {} };

	try {
		const parsed = JSON.parse(configStr);
		if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
			return { ...DEFAULTS, env: {}, extra: {} };
		}

		const {
			hooks,
			skill_refs,
			mcp_servers,
			allowed_tools,
			disallowed_tools,
			env,
			...rest
		} = parsed;

		// Parse hooks — supports both array format (hook names from UI)
		// and object format (Claude Code event map from templates)
		let parsedHooks: string[] = [];
		if (Array.isArray(hooks)) {
			parsedHooks = hooks;
		} else if (hooks && typeof hooks === 'object') {
			parsedHooks = extractHookRefsFromConfig(hooks as Record<string, unknown>);
		}

		return {
			hooks: parsedHooks,
			skillRefs: Array.isArray(skill_refs) ? skill_refs : [],
			mcpServers: mcp_servers && typeof mcp_servers === 'object' && !Array.isArray(mcp_servers)
				? Object.keys(mcp_servers)
				: [],
			allowedTools: Array.isArray(allowed_tools) ? allowed_tools : [],
			disallowedTools: Array.isArray(disallowed_tools) ? disallowed_tools : [],
			env: env && typeof env === 'object' && !Array.isArray(env) ? env : {},
			extra: rest,
		};
	} catch {
		return { ...DEFAULTS, env: {}, extra: {} };
	}
}

/** Serialize structured state back to claude_config JSON string. */
export function serializeClaudeConfig(state: ClaudeConfigState): string {
	const config: Record<string, unknown> = { ...state.extra };

	if (state.hooks.length > 0) config.hooks = state.hooks;
	if (state.skillRefs.length > 0) config.skill_refs = state.skillRefs;
	if (state.mcpServers.length > 0) {
		const servers: Record<string, unknown> = {};
		for (const name of state.mcpServers) {
			servers[name] = {};
		}
		config.mcp_servers = servers;
	}
	if (state.allowedTools.length > 0) config.allowed_tools = state.allowedTools;
	if (state.disallowedTools.length > 0) config.disallowed_tools = state.disallowedTools;
	if (Object.keys(state.env).length > 0) config.env = state.env;

	return JSON.stringify(config);
}

/** Merge template claude_config with override claude_config.
 * Arrays are unioned (deduplicated). Env vars are merged (override wins on collision). */
export function mergeClaudeConfigs(
	templateStr: string | undefined,
	overrideStr: string | undefined,
): ClaudeConfigState {
	const template = parseClaudeConfig(templateStr);
	const override = parseClaudeConfig(overrideStr);

	return {
		hooks: [...new Set([...template.hooks, ...override.hooks])],
		skillRefs: [...new Set([...template.skillRefs, ...override.skillRefs])],
		mcpServers: [...new Set([...template.mcpServers, ...override.mcpServers])],
		allowedTools: [...new Set([...template.allowedTools, ...override.allowedTools])],
		disallowedTools: [...new Set([...template.disallowedTools, ...override.disallowedTools])],
		env: { ...template.env, ...override.env },
		extra: { ...template.extra, ...override.extra },
	};
}
