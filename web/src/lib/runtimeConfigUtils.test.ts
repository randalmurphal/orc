import { describe, expect, it } from 'vitest';
import {
	mergeRuntimeConfigs,
	parseRuntimeConfig,
	serializeRuntimeConfig,
	type RuntimeConfigState,
} from './runtimeConfigUtils';

function state(overrides: Partial<RuntimeConfigState> = {}): RuntimeConfigState {
	return {
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
		...overrides,
	};
}

const hookDefinitions = [
	{ name: 'lint-hook', eventType: 'PreToolUse' },
	{ name: 'review-hook', eventType: 'PostToolUse' },
];

describe('parseRuntimeConfig', () => {
	it('returns defaults for empty input', () => {
		expect(parseRuntimeConfig(undefined)).toEqual(state());
		expect(parseRuntimeConfig('')).toEqual(state());
	});

	it('parses nested shared and providers.claude fields', () => {
		const result = parseRuntimeConfig(JSON.stringify({
			shared: {
				mcp_servers: {
					filesystem: { command: 'npx', args: ['-y', '@modelcontextprotocol/server-filesystem'] },
				},
				allowed_tools: ['Bash', 'Read'],
				disallowed_tools: ['Write'],
				env: { NODE_ENV: 'test' },
			},
			providers: {
				claude: {
					hooks: {
						PreToolUse: [{
							matcher: '*',
							hooks: [{ type: 'command', command: 'bash {{hook:lint-hook}}' }],
						}],
					},
					skill_refs: ['python-style'],
				},
			},
		}));

		expect(result).toEqual(state({
			hooks: ['lint-hook'],
			skillRefs: ['python-style'],
			mcpServers: ['filesystem'],
			allowedTools: ['Bash', 'Read'],
			disallowedTools: ['Write'],
			env: { NODE_ENV: 'test' },
			mcpServerData: {
				filesystem: { command: 'npx', args: ['-y', '@modelcontextprotocol/server-filesystem'] },
			},
			hookConfig: {
				PreToolUse: [{
					matcher: '*',
					hooks: [{ type: 'command', command: 'bash {{hook:lint-hook}}' }],
				}],
			},
			hookEventTypes: {
				'lint-hook': 'PreToolUse',
			},
		}));
	});

	it('extracts hook refs from Claude hook event maps', () => {
		const result = parseRuntimeConfig(JSON.stringify({
			providers: {
				claude: {
					hooks: {
						Stop: [{
							hooks: [{ type: 'command', command: 'bash {{hook:orc-verify-completion}}' }],
						}],
						PreToolUse: [{
							hooks: [{ type: 'command', command: 'bash {{hook:orc-tdd-discipline}}' }],
						}],
					},
				},
			},
		}));

		expect(result.hooks).toEqual(['orc-verify-completion', 'orc-tdd-discipline']);
		expect(result.hookEventTypes).toEqual({
			'orc-verify-completion': 'Stop',
			'orc-tdd-discipline': 'PreToolUse',
		});
	});

	it('preserves unknown nested fields in extra', () => {
		const result = parseRuntimeConfig(JSON.stringify({
			tag: 'custom',
			shared: {
				add_dirs: ['/tmp/project'],
			},
			providers: {
				codex: {
					reasoning_effort: 'high',
				},
				claude: {
					system_prompt_file: 'review.md',
				},
			},
		}));

		expect(result.extra).toEqual({
			tag: 'custom',
			shared: { add_dirs: ['/tmp/project'] },
			providers: {
				codex: { reasoning_effort: 'high' },
				claude: { system_prompt_file: 'review.md' },
			},
		});
	});
});

describe('serializeRuntimeConfig', () => {
	it('serializes runtime config state to nested shared/providers shape', () => {
		const result = JSON.parse(serializeRuntimeConfig(state({
			hooks: ['lint-hook'],
			skillRefs: ['python-style'],
			mcpServers: ['filesystem'],
			allowedTools: ['Bash'],
			disallowedTools: ['Write'],
			env: { NODE_ENV: 'test' },
			mcpServerData: {
				filesystem: { command: 'npx', args: ['server-filesystem'] },
			},
		}), { hookDefinitions }));

		expect(result).toEqual({
			shared: {
				mcp_servers: {
					filesystem: { command: 'npx', args: ['server-filesystem'] },
				},
				allowed_tools: ['Bash'],
				disallowed_tools: ['Write'],
				env: { NODE_ENV: 'test' },
			},
			providers: {
				claude: {
					hooks: {
						PreToolUse: [{
							matcher: '*',
							hooks: [{ type: 'command', command: 'bash {{hook:lint-hook}}' }],
						}],
					},
					skill_refs: ['python-style'],
				},
			},
		});
	});

	it('preserves unknown nested fields on serialize', () => {
		const result = JSON.parse(serializeRuntimeConfig(state({
			hooks: ['lint-hook'],
			extra: {
				tag: 'custom',
				shared: { add_dirs: ['/tmp/project'] },
				providers: {
					codex: { reasoning_effort: 'high' },
					claude: { system_prompt_file: 'review.md' },
				},
			},
		}), { hookDefinitions }));

		expect(result).toEqual({
			tag: 'custom',
			shared: { add_dirs: ['/tmp/project'] },
			providers: {
				codex: { reasoning_effort: 'high' },
				claude: {
					system_prompt_file: 'review.md',
					hooks: {
						PreToolUse: [{
							matcher: '*',
							hooks: [{ type: 'command', command: 'bash {{hook:lint-hook}}' }],
						}],
					},
				},
			},
		});
	});

	it('returns empty object for empty state', () => {
		expect(JSON.parse(serializeRuntimeConfig(state()))).toEqual({});
	});
});

describe('mergeRuntimeConfigs', () => {
	it('merges shared settings, claude settings, and MCP server data', () => {
		const merged = mergeRuntimeConfigs(
			JSON.stringify({
				shared: {
					mcp_servers: { filesystem: { command: 'npx fs' } },
					allowed_tools: ['Read'],
					env: { A: '1', B: '2' },
				},
				providers: {
					claude: {
						hooks: {
							PreToolUse: [{
								hooks: [{ type: 'command', command: 'bash {{hook:lint-hook}}' }],
							}],
						},
						skill_refs: ['python-style'],
					},
				},
			}),
			JSON.stringify({
				shared: {
					mcp_servers: { github: { command: 'npx gh' } },
					allowed_tools: ['Bash'],
					disallowed_tools: ['Write'],
					env: { B: '3', C: '4' },
				},
				providers: {
					claude: {
						hooks: {
							PostToolUse: [{
								hooks: [{ type: 'command', command: 'bash {{hook:review-hook}}' }],
							}],
						},
						skill_refs: ['tdd'],
					},
				},
			}),
		);

		expect(merged).toEqual(state({
			hooks: ['lint-hook', 'review-hook'],
			skillRefs: ['python-style', 'tdd'],
			mcpServers: ['filesystem', 'github'],
			allowedTools: ['Read', 'Bash'],
			disallowedTools: ['Write'],
			env: { A: '1', B: '3', C: '4' },
			mcpServerData: {
				filesystem: { command: 'npx fs' },
				github: { command: 'npx gh' },
			},
			hookEventTypes: {
				'lint-hook': 'PreToolUse',
				'review-hook': 'PostToolUse',
			},
			hookConfig: {
				PostToolUse: [{
					hooks: [{ type: 'command', command: 'bash {{hook:review-hook}}' }],
				}],
			},
		}));
	});

	it('preserves unknown extras with override precedence', () => {
		const merged = mergeRuntimeConfigs(
			JSON.stringify({
				shared: { add_dirs: ['/a'] },
				providers: {
					codex: { reasoning_effort: 'medium' },
					claude: { system_prompt_file: 'base.md' },
				},
			}),
			JSON.stringify({
				shared: { add_dirs: ['/b'] },
				providers: {
					codex: { reasoning_effort: 'high' },
					claude: { system_prompt_file: 'override.md' },
				},
			}),
		);

		expect(merged.extra).toEqual({
			shared: { add_dirs: ['/b'] },
			providers: {
				codex: { reasoning_effort: 'high' },
				claude: { system_prompt_file: 'override.md' },
			},
		});
	});
});
