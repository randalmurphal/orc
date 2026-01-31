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

/** Parse claude_config JSON string into structured state. */
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

		return {
			hooks: Array.isArray(hooks) ? hooks : [],
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
