/**
 * Shared utilities for parsing, serializing, and merging runtime_config JSON.
 *
 * Used by:
 * - PhaseListEditor (edit dialog with inherited/override distinction)
 * - PhaseInspector (read-only effective config summary)
 * - EditPhaseTemplateModal (template-level runtime_config editing)
 */

import { create } from '@bufbuild/protobuf';
import { GetMCPServerRequestSchema } from '@/gen/orc/v1/mcp_pb';
import { mcpClient } from '@/lib/client';

/** Fetch a single MCP server's config by name. */
export async function fetchMCPServerConfig(name: string): Promise<Record<string, unknown> | undefined> {
	const response = await mcpClient.getMCPServer(
		create(GetMCPServerRequestSchema, { name }),
	);
	if (!response.server) {
		return undefined;
	}
	return {
		type: response.server.type,
		command: response.server.command,
		args: response.server.args,
		env: response.server.env,
		url: response.server.url,
		headers: response.server.headers,
		disabled: response.server.disabled,
	};
}

export interface RuntimeConfigState {
	hooks: string[];
	skillRefs: string[];
	mcpServers: string[];
	allowedTools: string[];
	disallowedTools: string[];
	env: Record<string, string>;
	mcpServerData?: Record<string, unknown>;
	hookConfig?: Record<string, unknown>;
	hookEventTypes?: Record<string, string>;
	extra: Record<string, unknown>;
}

export interface HookDefinition {
	name: string;
	eventType?: string;
}

export async function hydrateSelectedMCPServers(
	selectedNames: string[],
	currentData: Record<string, unknown>,
	fetchServer: (name: string) => Promise<unknown | undefined>,
): Promise<Record<string, unknown>> {
	if (selectedNames.length === 0) {
		return {};
	}

	const hydrated = { ...currentData };
	for (const name of selectedNames) {
		if (hydrated[name]) {
			continue;
		}
		const server = await fetchServer(name);
		if (server) {
			hydrated[name] = server;
		}
	}
	return hydrated;
}

const DEFAULTS: RuntimeConfigState = {
	hooks: [],
	skillRefs: [],
	mcpServers: [],
	allowedTools: [],
	disallowedTools: [],
	env: {},
	mcpServerData: {},
	hookConfig: {},
	hookEventTypes: {},
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
function extractHookRefsFromConfig(hooks: Record<string, unknown>): {
	names: string[];
	eventTypes: Record<string, string>;
} {
	const refs = new Set<string>();
	const eventTypes: Record<string, string> = {};
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
						eventTypes[match[1]] = eventType;
						foundRef = true;
					}
				}
			}
		}
		if (!foundRef) {
			const fallbackName = `${eventType} hook`;
			refs.add(fallbackName);
			eventTypes[fallbackName] = eventType;
		}
	}

	return { names: [...refs], eventTypes };
}

function asObject(value: unknown): Record<string, unknown> | null {
	if (!value || typeof value !== 'object' || Array.isArray(value)) {
		return null;
	}
	return value as Record<string, unknown>;
}

function sameStringSet(left: string[], right: string[]): boolean {
	if (left.length !== right.length) {
		return false;
	}
	const rightSet = new Set(right);
	return left.every((value) => rightSet.has(value));
}

function buildHookConfig(
	selectedHooks: string[],
	hookEventTypes: Record<string, string>,
	hookDefinitions: HookDefinition[],
): Record<string, unknown> | undefined {
	if (selectedHooks.length === 0) {
		return undefined;
	}

	const eventByHook = { ...hookEventTypes };
	for (const hook of hookDefinitions) {
		if (hook.name && hook.eventType) {
			eventByHook[hook.name] = hook.eventType;
		}
	}

	const grouped: Record<string, Array<{ matcher: string; hooks: Array<{ type: string; command: string }> }>> = {};
	for (const hookName of selectedHooks) {
		const eventType = eventByHook[hookName];
		if (!eventType) {
			return undefined;
		}
		if (!grouped[eventType]) {
			grouped[eventType] = [];
		}
		grouped[eventType].push({
			matcher: '*',
			hooks: [{ type: 'command', command: `bash {{hook:${hookName}}}` }],
		});
	}

	return grouped;
}

/** Parse runtime_config JSON string into structured state.
 * Uses the nested runtime model:
 * - shared: mcp_servers, allowed_tools, disallowed_tools, env
 * - providers.claude: hooks, skill_refs
 */
export function parseRuntimeConfig(configStr: string | undefined): RuntimeConfigState {
	if (!configStr) {
		return { ...DEFAULTS, env: {}, mcpServerData: {}, hookConfig: {}, hookEventTypes: {}, extra: {} };
	}

	try {
		const parsed = JSON.parse(configStr);
		const root = asObject(parsed);
		if (!root) {
			return { ...DEFAULTS, env: {}, mcpServerData: {}, hookConfig: {}, hookEventTypes: {}, extra: {} };
		}

		const { shared: sharedValue, providers: providersValue, ...restRoot } = root;
		const shared = asObject(sharedValue) ?? {};
		const providers = asObject(providersValue) ?? {};
		const claude = asObject(providers.claude) ?? {};
		const { mcp_servers, allowed_tools, disallowed_tools, env, ...restShared } = shared;
		const { hooks, skill_refs, ...restClaude } = claude;
		const { claude: _claudeIgnored, ...restProviders } = providers;

		// Parse hooks — supports both array format (hook names from UI)
		// and object format (Claude Code event map from templates)
		let parsedHooks: string[] = [];
		let hookConfig: Record<string, unknown> | undefined;
		let hookEventTypes: Record<string, string> = {};
		if (Array.isArray(hooks)) {
			parsedHooks = hooks;
		} else if (hooks && typeof hooks === 'object') {
			hookConfig = hooks as Record<string, unknown>;
			const extracted = extractHookRefsFromConfig(hookConfig);
			parsedHooks = extracted.names;
			hookEventTypes = extracted.eventTypes;
		}

		const extra: Record<string, unknown> = { ...restRoot };
		if (Object.keys(restShared).length > 0) {
			extra.shared = restShared;
		}
		if (Object.keys(restProviders).length > 0 || Object.keys(restClaude).length > 0) {
			extra.providers = {
				...restProviders,
				...(Object.keys(restClaude).length > 0 ? { claude: restClaude } : {}),
			};
		}

		const mcpServerData = asObject(mcp_servers) ?? {};

		return {
			hooks: parsedHooks,
			skillRefs: Array.isArray(skill_refs) ? skill_refs : [],
			mcpServers: Object.keys(mcpServerData),
			allowedTools: Array.isArray(allowed_tools) ? allowed_tools : [],
			disallowedTools: Array.isArray(disallowed_tools) ? disallowed_tools : [],
			env: asObject(env) as Record<string, string> ?? {},
			mcpServerData,
			hookConfig,
			hookEventTypes,
			extra,
		};
	} catch {
		return { ...DEFAULTS, env: {}, mcpServerData: {}, hookConfig: {}, hookEventTypes: {}, extra: {} };
	}
}

/** Serialize structured state back to runtime_config JSON string. */
export function serializeRuntimeConfig(
	state: RuntimeConfigState,
	options: { hookDefinitions?: HookDefinition[] } = {},
): string {
	const { shared: extraSharedValue, providers: extraProvidersValue, ...extraRoot } = state.extra;
	const shared = { ...(asObject(extraSharedValue) ?? {}) };
	const extraProviders = asObject(extraProvidersValue) ?? {};
	const { claude: extraClaudeValue, ...providersRest } = extraProviders;
	const claude = { ...(asObject(extraClaudeValue) ?? {}) };
	const config: Record<string, unknown> = { ...extraRoot };

	if (state.hooks.length > 0) {
		const preservedHookConfig = asObject(state.hookConfig);
		const preservedHookNames = preservedHookConfig ? extractHookRefsFromConfig(preservedHookConfig).names : [];
		if (preservedHookConfig && sameStringSet(state.hooks, preservedHookNames)) {
			claude.hooks = preservedHookConfig;
		} else {
			const generatedHooks = buildHookConfig(
				state.hooks,
				state.hookEventTypes ?? {},
				options.hookDefinitions ?? [],
			);
			if (generatedHooks) {
				claude.hooks = generatedHooks;
			} else if (preservedHookConfig) {
				claude.hooks = preservedHookConfig;
			}
		}
	}
	if (state.skillRefs.length > 0) claude.skill_refs = state.skillRefs;
	if (state.mcpServers.length > 0) {
		const servers: Record<string, unknown> = {};
		for (const name of state.mcpServers) {
			servers[name] = state.mcpServerData?.[name] ?? {};
		}
		shared.mcp_servers = servers;
	}
	if (state.allowedTools.length > 0) shared.allowed_tools = state.allowedTools;
	if (state.disallowedTools.length > 0) shared.disallowed_tools = state.disallowedTools;
	if (Object.keys(state.env).length > 0) shared.env = state.env;

	const providers: Record<string, unknown> = { ...providersRest };
	if (Object.keys(claude).length > 0) {
		providers.claude = claude;
	}
	if (Object.keys(shared).length > 0) {
		config.shared = shared;
	}
	if (Object.keys(providers).length > 0) {
		config.providers = providers;
	}

	return JSON.stringify(config);
}

/** Merge template runtime_config with override runtime_config.
 * Arrays are unioned (deduplicated). Env vars are merged (override wins on collision). */
export function mergeRuntimeConfigs(
	templateStr: string | undefined,
	overrideStr: string | undefined,
): RuntimeConfigState {
	const template = parseRuntimeConfig(templateStr);
	const override = parseRuntimeConfig(overrideStr);

	return {
		hooks: [...new Set([...template.hooks, ...override.hooks])],
		skillRefs: [...new Set([...template.skillRefs, ...override.skillRefs])],
		mcpServers: [...new Set([...template.mcpServers, ...override.mcpServers])],
		allowedTools: [...new Set([...template.allowedTools, ...override.allowedTools])],
		disallowedTools: [...new Set([...template.disallowedTools, ...override.disallowedTools])],
		env: { ...template.env, ...override.env },
		mcpServerData: { ...template.mcpServerData, ...override.mcpServerData },
		hookConfig: override.hookConfig ?? template.hookConfig,
		hookEventTypes: { ...template.hookEventTypes, ...override.hookEventTypes },
		extra: { ...template.extra, ...override.extra },
	};
}
