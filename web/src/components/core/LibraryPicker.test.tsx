/**
 * TDD Tests for LibraryPicker
 *
 * Tests for TASK-669: Phase template claude_config editor with collapsible sections
 *
 * Success Criteria Coverage:
 * - SC-4: LibraryPicker for Hooks fetches from configClient.listHooks, groups by event type,
 *         supports multi-select
 * - SC-5: LibraryPicker for Skills fetches from configClient.listSkills, shows name + description,
 *         supports multi-select
 * - SC-6: LibraryPicker for MCP Servers fetches from mcpClient.listMCPServers, shows server name +
 *         command preview, supports multi-select
 *
 * Failure Modes:
 * - configClient.listHooks() fails → inline error message
 * - Empty list → "No X configured" message
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { LibraryPicker } from './LibraryPicker';
import {
	createMockHook,
	createMockSkill,
	createMockMCPServerInfo,
} from '@/test/factories';
import type { Hook } from '@/gen/orc/v1/config_pb';
import type { Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';

// Mock browser APIs for Radix
beforeAll(() => {
	Element.prototype.scrollIntoView = vi.fn();
	Element.prototype.hasPointerCapture = vi.fn().mockReturnValue(false);
	Element.prototype.setPointerCapture = vi.fn();
	Element.prototype.releasePointerCapture = vi.fn();
	global.ResizeObserver = vi.fn().mockImplementation(() => ({
		observe: vi.fn(),
		unobserve: vi.fn(),
		disconnect: vi.fn(),
	}));
});

describe('LibraryPicker', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-4: Hooks - fetch, group by event type, multi-select', () => {
		const mockHooks: Hook[] = [
			createMockHook({ name: 'pre-tool-guard', eventType: 'PreToolUse', content: '#!/bin/bash\nguard.sh' }),
			createMockHook({ name: 'post-tool-log', eventType: 'PostToolUse', content: '#!/bin/bash\nlog.sh' }),
			createMockHook({ name: 'on-stop-cleanup', eventType: 'Stop', content: '#!/bin/bash\ncleanup.sh' }),
			createMockHook({ name: 'notify-slack', eventType: 'PostToolUse', content: '#!/bin/bash\nslack.sh' }),
		];

		it('displays hooks grouped by event type', async () => {
			const onSelectionChange = vi.fn();

			render(
				<LibraryPicker
					type="hooks"
					items={mockHooks}
					selectedNames={[]}
					onSelectionChange={onSelectionChange}
				/>
			);

			// All hook names should be visible
			expect(screen.getByText('pre-tool-guard')).toBeInTheDocument();
			expect(screen.getByText('post-tool-log')).toBeInTheDocument();
			expect(screen.getByText('on-stop-cleanup')).toBeInTheDocument();
			expect(screen.getByText('notify-slack')).toBeInTheDocument();
		});

		it('shows group headers for event types', async () => {
			render(
				<LibraryPicker
					type="hooks"
					items={mockHooks}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
				/>
			);

			// Group headers should appear for each event type
			expect(screen.getByText(/PreToolUse/i)).toBeInTheDocument();
			expect(screen.getByText(/PostToolUse/i)).toBeInTheDocument();
			// AMEND-001: /Stop/i also matches hook name "on-stop-cleanup", use anchored regex
			expect(screen.getByText(/^Stop$/i)).toBeInTheDocument();
		});

		it('supports multi-select - toggling hooks on', async () => {
			const user = userEvent.setup();
			const onSelectionChange = vi.fn();

			render(
				<LibraryPicker
					type="hooks"
					items={mockHooks}
					selectedNames={[]}
					onSelectionChange={onSelectionChange}
				/>
			);

			// Click to select a hook
			await user.click(screen.getByText('pre-tool-guard'));

			expect(onSelectionChange).toHaveBeenCalledWith(
				expect.arrayContaining(['pre-tool-guard'])
			);
		});

		it('shows selected hooks with visual indicator', () => {
			render(
				<LibraryPicker
					type="hooks"
					items={mockHooks}
					selectedNames={['pre-tool-guard', 'notify-slack']}
					onSelectionChange={vi.fn()}
				/>
			);

			// Selected items should have a visual indicator (checked state)
			// We test by finding the items and checking their selected state
			const preToolGuard = screen.getByText('pre-tool-guard').closest('[data-selected]');
			expect(preToolGuard).toBeTruthy();
		});

		it('supports multi-select - toggling hooks off', async () => {
			const user = userEvent.setup();
			const onSelectionChange = vi.fn();

			render(
				<LibraryPicker
					type="hooks"
					items={mockHooks}
					selectedNames={['pre-tool-guard']}
					onSelectionChange={onSelectionChange}
				/>
			);

			// Click to deselect
			await user.click(screen.getByText('pre-tool-guard'));

			expect(onSelectionChange).toHaveBeenCalledWith(
				expect.not.arrayContaining(['pre-tool-guard'])
			);
		});
	});

	describe('SC-5: Skills - name + description, multi-select', () => {
		const mockSkills: Skill[] = [
			createMockSkill({ name: 'python-style', description: 'Python coding standards' }),
			createMockSkill({ name: 'tdd', description: 'Test-driven development workflow' }),
			createMockSkill({ name: 'debugging', description: 'Systematic debugging approach' }),
		];

		it('displays skills with name and description', () => {
			render(
				<LibraryPicker
					type="skills"
					items={mockSkills}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
				/>
			);

			expect(screen.getByText('python-style')).toBeInTheDocument();
			expect(screen.getByText('Python coding standards')).toBeInTheDocument();
			expect(screen.getByText('tdd')).toBeInTheDocument();
			expect(screen.getByText('Test-driven development workflow')).toBeInTheDocument();
			expect(screen.getByText('debugging')).toBeInTheDocument();
		});

		it('supports multi-select for skills', async () => {
			const user = userEvent.setup();
			const onSelectionChange = vi.fn();

			render(
				<LibraryPicker
					type="skills"
					items={mockSkills}
					selectedNames={['python-style']}
					onSelectionChange={onSelectionChange}
				/>
			);

			// Select another skill
			await user.click(screen.getByText('tdd'));

			expect(onSelectionChange).toHaveBeenCalledWith(
				expect.arrayContaining(['python-style', 'tdd'])
			);
		});
	});

	describe('SC-6: MCP Servers - name + command preview, multi-select', () => {
		const mockServers: MCPServerInfo[] = [
			createMockMCPServerInfo({ name: 'filesystem', command: 'npx @modelcontextprotocol/server-filesystem /home' }),
			createMockMCPServerInfo({ name: 'database', command: 'npx @modelcontextprotocol/server-postgres' }),
		];

		it('displays servers with name and truncated command', () => {
			render(
				<LibraryPicker
					type="mcpServers"
					items={mockServers}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
				/>
			);

			expect(screen.getByText('filesystem')).toBeInTheDocument();
			expect(screen.getByText('database')).toBeInTheDocument();
			// Command preview should be visible (possibly truncated)
			expect(screen.getByText(/npx @modelcontextprotocol\/server-filesystem/)).toBeInTheDocument();
		});

		it('supports multi-select for MCP servers', async () => {
			const user = userEvent.setup();
			const onSelectionChange = vi.fn();

			render(
				<LibraryPicker
					type="mcpServers"
					items={mockServers}
					selectedNames={[]}
					onSelectionChange={onSelectionChange}
				/>
			);

			await user.click(screen.getByText('filesystem'));

			expect(onSelectionChange).toHaveBeenCalledWith(
				expect.arrayContaining(['filesystem'])
			);
		});
	});

	describe('Failure modes: API errors and empty states', () => {
		it('shows "No hooks configured" when hooks list is empty', () => {
			render(
				<LibraryPicker
					type="hooks"
					items={[]}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
				/>
			);

			expect(screen.getByText(/no hooks configured/i)).toBeInTheDocument();
		});

		it('shows "No skills configured" when skills list is empty', () => {
			render(
				<LibraryPicker
					type="skills"
					items={[]}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
				/>
			);

			expect(screen.getByText(/no skills configured/i)).toBeInTheDocument();
		});

		it('shows "No MCP servers configured" when servers list is empty', () => {
			render(
				<LibraryPicker
					type="mcpServers"
					items={[]}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
				/>
			);

			expect(screen.getByText(/no mcp servers configured/i)).toBeInTheDocument();
		});

		it('shows error message when error prop is set', () => {
			render(
				<LibraryPicker
					type="hooks"
					items={[]}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
					error="Failed to load hooks"
				/>
			);

			expect(screen.getByText('Failed to load hooks')).toBeInTheDocument();
		});

		it('shows loading state when loading prop is true', () => {
			render(
				<LibraryPicker
					type="hooks"
					items={[]}
					selectedNames={[]}
					onSelectionChange={vi.fn()}
					loading
				/>
			);

			// Should show some loading indicator
			expect(screen.getByText(/loading/i)).toBeInTheDocument();
		});
	});
});
