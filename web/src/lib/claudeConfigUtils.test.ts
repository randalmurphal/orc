/**
 * TDD Tests for claude_config utility functions
 *
 * Tests for TASK-670: Workflow phase override editor with inherited/override distinction
 *
 * These utility functions provide:
 * - parseClaudeConfig: Parse JSON string into structured state
 * - serializeClaudeConfig: Serialize structured state back to JSON
 * - mergeClaudeConfigs: Merge template config with override config
 *
 * Success Criteria Coverage:
 * - SC-3: serializeClaudeConfig correctly produces JSON for save
 * - SC-10: mergeClaudeConfigs correctly merges template + override
 */

import { describe, it, expect } from 'vitest';
import {
	parseClaudeConfig,
	serializeClaudeConfig,
	mergeClaudeConfigs,
} from './claudeConfigUtils';

// ─── parseClaudeConfig ───────────────────────────────────────────────────────

describe('parseClaudeConfig', () => {
	it('returns defaults for undefined input', () => {
		const result = parseClaudeConfig(undefined);

		expect(result.hooks).toEqual([]);
		expect(result.skillRefs).toEqual([]);
		expect(result.mcpServers).toEqual([]);
		expect(result.allowedTools).toEqual([]);
		expect(result.disallowedTools).toEqual([]);
		expect(result.env).toEqual({});
		expect(result.extra).toEqual({});
	});

	it('returns defaults for empty string', () => {
		const result = parseClaudeConfig('');

		expect(result.hooks).toEqual([]);
		expect(result.env).toEqual({});
	});

	it('parses hooks array', () => {
		const result = parseClaudeConfig('{"hooks": ["lint-hook", "format-hook"]}');

		expect(result.hooks).toEqual(['lint-hook', 'format-hook']);
	});

	it('parses skill_refs with snake_case to camelCase mapping', () => {
		const result = parseClaudeConfig('{"skill_refs": ["my-skill", "other-skill"]}');

		expect(result.skillRefs).toEqual(['my-skill', 'other-skill']);
	});

	it('parses mcp_servers as object keys', () => {
		const result = parseClaudeConfig('{"mcp_servers": {"server-a": {}, "server-b": {"url": "http://test"}}}');

		expect(result.mcpServers).toEqual(['server-a', 'server-b']);
	});

	it('parses allowed_tools and disallowed_tools', () => {
		const result = parseClaudeConfig('{"allowed_tools": ["Bash", "Read"], "disallowed_tools": ["Write"]}');

		expect(result.allowedTools).toEqual(['Bash', 'Read']);
		expect(result.disallowedTools).toEqual(['Write']);
	});

	it('parses env vars', () => {
		const result = parseClaudeConfig('{"env": {"NODE_ENV": "test", "DEBUG": "true"}}');

		expect(result.env).toEqual({ NODE_ENV: 'test', DEBUG: 'true' });
	});

	it('preserves empty string env var values', () => {
		const result = parseClaudeConfig('{"env": {"EMPTY": ""}}');

		expect(result.env).toEqual({ EMPTY: '' });
	});

	it('collects unknown keys into extra', () => {
		const result = parseClaudeConfig('{"hooks": ["h1"], "custom_field": 42}');

		expect(result.hooks).toEqual(['h1']);
		expect(result.extra).toEqual({ custom_field: 42 });
	});

	it('returns defaults for invalid JSON', () => {
		const result = parseClaudeConfig('not valid json');

		expect(result.hooks).toEqual([]);
		expect(result.env).toEqual({});
	});

	it('returns defaults for non-object JSON', () => {
		const result = parseClaudeConfig('"just a string"');

		expect(result.hooks).toEqual([]);
	});

	it('handles non-array hooks gracefully', () => {
		const result = parseClaudeConfig('{"hooks": "not-an-array"}');

		expect(result.hooks).toEqual([]);
	});

	it('handles non-object env gracefully', () => {
		const result = parseClaudeConfig('{"env": "not-an-object"}');

		expect(result.env).toEqual({});
	});

	it('parses full config with all fields', () => {
		const config = {
			hooks: ['hook-1'],
			skill_refs: ['skill-1'],
			mcp_servers: { 'mcp-1': {} },
			allowed_tools: ['Bash'],
			disallowed_tools: ['Write'],
			env: { KEY: 'value' },
		};
		const result = parseClaudeConfig(JSON.stringify(config));

		expect(result.hooks).toEqual(['hook-1']);
		expect(result.skillRefs).toEqual(['skill-1']);
		expect(result.mcpServers).toEqual(['mcp-1']);
		expect(result.allowedTools).toEqual(['Bash']);
		expect(result.disallowedTools).toEqual(['Write']);
		expect(result.env).toEqual({ KEY: 'value' });
	});
});

// ─── serializeClaudeConfig ───────────────────────────────────────────────────

describe('serializeClaudeConfig', () => {
	it('returns empty JSON for empty state', () => {
		const result = serializeClaudeConfig({
			hooks: [],
			skillRefs: [],
			mcpServers: [],
			allowedTools: [],
			disallowedTools: [],
			env: {},
			extra: {},
		});

		const parsed = JSON.parse(result);
		expect(Object.keys(parsed)).toHaveLength(0);
	});

	it('serializes hooks', () => {
		const result = serializeClaudeConfig({
			hooks: ['my-hook'],
			skillRefs: [],
			mcpServers: [],
			allowedTools: [],
			disallowedTools: [],
			env: {},
			extra: {},
		});

		const parsed = JSON.parse(result);
		expect(parsed.hooks).toEqual(['my-hook']);
	});

	it('serializes skill_refs with camelCase to snake_case mapping', () => {
		const result = serializeClaudeConfig({
			hooks: [],
			skillRefs: ['skill-1'],
			mcpServers: [],
			allowedTools: [],
			disallowedTools: [],
			env: {},
			extra: {},
		});

		const parsed = JSON.parse(result);
		expect(parsed.skill_refs).toEqual(['skill-1']);
		expect(parsed.skillRefs).toBeUndefined();
	});

	it('serializes allowed_tools and disallowed_tools', () => {
		const result = serializeClaudeConfig({
			hooks: [],
			skillRefs: [],
			mcpServers: [],
			allowedTools: ['Bash', 'Read'],
			disallowedTools: ['Write'],
			env: {},
			extra: {},
		});

		const parsed = JSON.parse(result);
		expect(parsed.allowed_tools).toEqual(['Bash', 'Read']);
		expect(parsed.disallowed_tools).toEqual(['Write']);
	});

	it('serializes env vars', () => {
		const result = serializeClaudeConfig({
			hooks: [],
			skillRefs: [],
			mcpServers: [],
			allowedTools: [],
			disallowedTools: [],
			env: { NODE_ENV: 'test' },
			extra: {},
		});

		const parsed = JSON.parse(result);
		expect(parsed.env).toEqual({ NODE_ENV: 'test' });
	});

	it('omits empty arrays and objects from output', () => {
		const result = serializeClaudeConfig({
			hooks: [],
			skillRefs: [],
			mcpServers: [],
			allowedTools: [],
			disallowedTools: [],
			env: {},
			extra: {},
		});

		const parsed = JSON.parse(result);
		expect(parsed.hooks).toBeUndefined();
		expect(parsed.skill_refs).toBeUndefined();
		expect(parsed.env).toBeUndefined();
	});

	it('preserves extra fields from original config', () => {
		const result = serializeClaudeConfig({
			hooks: [],
			skillRefs: [],
			mcpServers: [],
			allowedTools: [],
			disallowedTools: [],
			env: {},
			extra: { custom_field: 42 },
		});

		const parsed = JSON.parse(result);
		expect(parsed.custom_field).toBe(42);
	});
});

// ─── mergeClaudeConfigs (SC-10) ──────────────────────────────────────────────

describe('mergeClaudeConfigs', () => {
	it('returns template config when override is undefined', () => {
		const result = mergeClaudeConfigs(
			'{"hooks": ["template-hook"], "env": {"A": "1"}}',
			undefined,
		);

		expect(result.hooks).toEqual(['template-hook']);
		expect(result.env).toEqual({ A: '1' });
	});

	it('returns override config when template is undefined', () => {
		const result = mergeClaudeConfigs(
			undefined,
			'{"hooks": ["override-hook"]}',
		);

		expect(result.hooks).toEqual(['override-hook']);
	});

	it('returns defaults when both are undefined', () => {
		const result = mergeClaudeConfigs(undefined, undefined);

		expect(result.hooks).toEqual([]);
		expect(result.env).toEqual({});
	});

	it('unions hooks from template and override', () => {
		const result = mergeClaudeConfigs(
			'{"hooks": ["lint-hook"]}',
			'{"hooks": ["my-hook"]}',
		);

		expect(result.hooks).toContain('lint-hook');
		expect(result.hooks).toContain('my-hook');
		expect(result.hooks).toHaveLength(2);
	});

	it('unions skill_refs from template and override', () => {
		const result = mergeClaudeConfigs(
			'{"skill_refs": ["skill-a"]}',
			'{"skill_refs": ["skill-b"]}',
		);

		expect(result.skillRefs).toContain('skill-a');
		expect(result.skillRefs).toContain('skill-b');
	});

	it('unions mcp_servers from template and override', () => {
		const result = mergeClaudeConfigs(
			'{"mcp_servers": {"server-a": {}}}',
			'{"mcp_servers": {"server-b": {}}}',
		);

		expect(result.mcpServers).toContain('server-a');
		expect(result.mcpServers).toContain('server-b');
	});

	it('unions allowed_tools from template and override', () => {
		const result = mergeClaudeConfigs(
			'{"allowed_tools": ["Bash"]}',
			'{"allowed_tools": ["Read"]}',
		);

		expect(result.allowedTools).toContain('Bash');
		expect(result.allowedTools).toContain('Read');
	});

	it('unions disallowed_tools from template and override', () => {
		const result = mergeClaudeConfigs(
			'{"disallowed_tools": ["Write"]}',
			'{"disallowed_tools": ["Edit"]}',
		);

		expect(result.disallowedTools).toContain('Write');
		expect(result.disallowedTools).toContain('Edit');
	});

	it('merges env vars with override winning on key collision (BDD-4)', () => {
		const result = mergeClaudeConfigs(
			'{"env": {"A": "1", "B": "2"}}',
			'{"env": {"B": "3", "C": "4"}}',
		);

		expect(result.env).toEqual({ A: '1', B: '3', C: '4' });
	});

	it('handles template with env but no override env', () => {
		const result = mergeClaudeConfigs(
			'{"env": {"A": "1"}}',
			'{"hooks": ["h1"]}',
		);

		expect(result.env).toEqual({ A: '1' });
		expect(result.hooks).toContain('h1');
	});

	it('handles override with env but no template env', () => {
		const result = mergeClaudeConfigs(
			'{"hooks": ["h1"]}',
			'{"env": {"B": "2"}}',
		);

		expect(result.env).toEqual({ B: '2' });
		expect(result.hooks).toContain('h1');
	});

	it('does not duplicate when template and override have same hook', () => {
		const result = mergeClaudeConfigs(
			'{"hooks": ["shared-hook"]}',
			'{"hooks": ["shared-hook"]}',
		);

		// Union should deduplicate
		const uniqueHooks = [...new Set(result.hooks)];
		expect(uniqueHooks).toHaveLength(1);
		expect(uniqueHooks[0]).toBe('shared-hook');
	});

	it('handles empty override sections (all cleared)', () => {
		const result = mergeClaudeConfigs(
			'{"hooks": ["template-hook"], "env": {"A": "1"}}',
			'{}',
		);

		// Template values should remain since override has no sections
		expect(result.hooks).toEqual(['template-hook']);
		expect(result.env).toEqual({ A: '1' });
	});
});
