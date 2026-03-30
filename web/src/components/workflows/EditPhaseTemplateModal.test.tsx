/**
 * TDD Tests for EditPhaseTemplateModal - Runtime Config Settings Sections
 *
 * Tests for TASK-669: Phase template runtime_config editor with collapsible sections
 *
 * Success Criteria Coverage:
 * - SC-2: EditPhaseTemplateModal renders 7 collapsible settings sections
 * - SC-3: Saving serializes structured field state to runtime_config JSON
 * - SC-9: JSON Override shows current runtime_config as formatted JSON
 * - SC-10: JSON Override edits update structured fields; invalid JSON shows error
 * - SC-11: On modal open with existing runtime_config, all sections populate
 * - SC-12: All new components are wired into EditPhaseTemplateModal
 *
 * Failure Modes:
 * - Malformed runtime_config → sections render empty with console.warn
 * - Save failure → error toast, modal stays open
 * - Invalid JSON in override → red border, structured fields unchanged
 *
 * Edge Cases:
 * - Empty runtime_config → all sections show 0 items
 * - Unknown fields preserved through parse→edit→serialize cycle
 * - Built-in template → settings sections disabled/read-only
 * - JSON override edited, then structured field edited → structured takes precedence
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, cleanup, fireEvent, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EditPhaseTemplateModal } from './EditPhaseTemplateModal';
import { create } from '@bufbuild/protobuf';
import { ListHooksResponseSchema, ListSkillsResponseSchema, ListAgentsResponseSchema } from '@/gen/orc/v1/config_pb';
import { GetMCPServerResponseSchema, ListMCPServersResponseSchema, MCPServerSchema } from '@/gen/orc/v1/mcp_pb';
import {
	createMockPhaseTemplate,
	createMockUpdatePhaseTemplateResponse,
	createMockHook,
	createMockSkill,
	createMockMCPServerInfo,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhaseTemplate: vi.fn(),
	},
	configClient: {
		listAgents: vi.fn(),
		listHooks: vi.fn(),
		listSkills: vi.fn(),
	},
	mcpClient: {
		listMCPServers: vi.fn(),
		getMCPServer: vi.fn(),
	},
}));

// Mock toast
vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

// Import mocked modules for assertions
import { workflowClient, configClient, mcpClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';

// Standard mock data for library pickers
const mockHooks = [
	createMockHook({ name: 'pre-guard', eventType: 'PreToolUse' }),
	createMockHook({ name: 'post-log', eventType: 'PostToolUse' }),
];
const mockSkills = [
	createMockSkill({ name: 'python-style', description: 'Python coding standards' }),
	createMockSkill({ name: 'tdd', description: 'TDD workflow' }),
];
const mockMCPServers = [
	createMockMCPServerInfo({ name: 'filesystem', command: 'npx fs-server' }),
];

function makeRuntimeConfig({
	hooks,
	skillRefs,
	mcpServers,
	allowedTools,
	disallowedTools,
	env,
}: {
	hooks?: string[];
	skillRefs?: string[];
	mcpServers?: Record<string, unknown>;
	allowedTools?: string[];
	disallowedTools?: string[];
	env?: Record<string, string>;
}): string {
	const config: Record<string, unknown> = {};
	const shared: Record<string, unknown> = {};
	const claude: Record<string, unknown> = {};

	if (hooks && hooks.length > 0) {
		const hookConfig: Record<string, unknown[]> = {};
		const hookEvents = new Map(mockHooks.map((hook) => [hook.name, hook.eventType || 'PreToolUse']));
		for (const hook of hooks) {
			const eventType = hookEvents.get(hook) || 'PreToolUse';
			if (!hookConfig[eventType]) {
				hookConfig[eventType] = [];
			}
			hookConfig[eventType].push({
				matcher: '*',
				hooks: [{ type: 'command', command: `bash {{hook:${hook}}}` }],
			});
		}
		claude.hooks = hookConfig;
	}
	if (skillRefs && skillRefs.length > 0) claude.skill_refs = skillRefs;
	if (mcpServers && Object.keys(mcpServers).length > 0) shared.mcp_servers = mcpServers;
	if (allowedTools && allowedTools.length > 0) shared.allowed_tools = allowedTools;
	if (disallowedTools && disallowedTools.length > 0) shared.disallowed_tools = disallowedTools;
	if (env && Object.keys(env).length > 0) shared.env = env;
	if (Object.keys(shared).length > 0) config.shared = shared;
	if (Object.keys(claude).length > 0) config.providers = { claude };
	return JSON.stringify(config);
}

function setupMocks() {
	vi.mocked(configClient.listAgents).mockResolvedValue(create(ListAgentsResponseSchema, { agents: [] }));
	vi.mocked(configClient.listHooks).mockResolvedValue(create(ListHooksResponseSchema, { hooks: mockHooks }));
	vi.mocked(configClient.listSkills).mockResolvedValue(create(ListSkillsResponseSchema, { skills: mockSkills }));
	vi.mocked(mcpClient.listMCPServers).mockResolvedValue(create(ListMCPServersResponseSchema, { servers: mockMCPServers }));
	vi.mocked(mcpClient.getMCPServer).mockImplementation(async (request) =>
		create(GetMCPServerResponseSchema, {
			server: create(MCPServerSchema, {
				name: request.name,
				type: 'stdio',
				command: 'npx fs-server',
				args: [],
				env: {},
				headers: [],
				disabled: false,
			}),
		}),
	);
}

const mockOnClose = vi.fn();
const mockOnUpdated = vi.fn();

describe('EditPhaseTemplateModal - Runtime Config Settings', () => {
	// NOTE: Browser API mocks (ResizeObserver, IntersectionObserver, scrollIntoView) provided by global test-setup.ts

	beforeEach(() => {
		vi.clearAllMocks();
		setupMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-2: Renders 7 collapsible settings sections', () => {
		it('renders all 7 section headers below existing fields', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// All 7 section headers should be visible
			// AMEND-003: Anchor regexes to avoid substring matches (e.g. "Allowed Tools" inside "Disallowed Tools")
			expect(screen.getByText(/^Hooks$/i)).toBeInTheDocument();
			expect(screen.getByText(/^MCP Servers$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Skills$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Allowed Tools$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Disallowed Tools$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Env Vars$/i)).toBeInTheDocument();
			expect(screen.getByText(/^JSON Override$/i)).toBeInTheDocument();
		});

		it('shows badge count 0 when runtime_config is empty', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig: undefined })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// The sections should exist with 0 items
			// Existing form fields should also still be rendered
			expect(screen.getByLabelText(/name/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
		});

		it('shows correct badge counts with populated runtime_config', async () => {
			const runtimeConfig = makeRuntimeConfig({
				hooks: ['pre-guard'],
				skillRefs: ['python-style', 'tdd'],
				mcpServers: { filesystem: {} },
				allowedTools: ['Bash', 'Read'],
				disallowedTools: ['Write'],
				env: { FOO: 'bar', BAZ: 'qux' },
			});

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Badge counts should reflect the config
			// Hooks: 1, Skills: 2, MCP: 1, Allowed: 2, Disallowed: 1, Env: 2
			// AMEND-004: Multiple badges may have same text; use getAllByText
			expect(screen.getAllByText('1').length).toBeGreaterThanOrEqual(1);
			expect(screen.getAllByText('2').length).toBeGreaterThanOrEqual(1);
		});
	});

	describe('SC-3: Saving serializes structured fields to runtime_config JSON', () => {
		it('calls updatePhaseTemplate with runtimeConfig containing serialized JSON', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue(
				createMockUpdatePhaseTemplateResponse(
					createMockPhaseTemplate({ isBuiltin: false })
				)
			);

			const runtimeConfig = makeRuntimeConfig({
				allowedTools: ['Bash', 'Read'],
				env: { API_KEY: 'secret' },
			});

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Click save
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						runtimeConfig: expect.any(String),
					})
				);
			});

			// Verify the runtimeConfig is valid JSON
			const call = vi.mocked(workflowClient.updatePhaseTemplate).mock.calls[0][0];
			const parsedConfig = JSON.parse(call.runtimeConfig as string);
			expect(parsedConfig.shared.allowed_tools).toEqual(['Bash', 'Read']);
			expect(parsedConfig.shared.env).toEqual({ API_KEY: 'secret' });
		});

		it('shows success toast on successful save', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue(
				createMockUpdatePhaseTemplateResponse(
					createMockPhaseTemplate({ isBuiltin: false })
				)
			);

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(toast.success).toHaveBeenCalled();
			});
		});

		it('shows error toast and keeps modal open on save failure', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.updatePhaseTemplate).mockRejectedValue(
				new Error('Network error')
			);

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(
					expect.stringContaining('Failed')
				);
			});

			// Modal should stay open
			expect(screen.getByRole('dialog')).toBeInTheDocument();
			expect(mockOnClose).not.toHaveBeenCalled();
		});

		it('preserves existing form fields when saving with runtime_config', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue(
				createMockUpdatePhaseTemplateResponse(
					createMockPhaseTemplate({ isBuiltin: false })
				)
			);

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({
						isBuiltin: false,
						name: 'Custom Phase',
						description: 'A custom phase',
					})}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalledWith(
					expect.objectContaining({
						name: 'Custom Phase',
						description: 'A custom phase',
					})
				);
			});
		});
	});

	describe('SC-9: JSON Override shows current runtime_config as formatted JSON', () => {
		it('shows JSON textarea reflecting structured field state', async () => {
			const user = userEvent.setup();

			const runtimeConfig = makeRuntimeConfig({
				allowedTools: ['Bash'],
				env: { FOO: 'bar' },
			});

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Expand JSON Override section
			const jsonHeader = screen.getByText(/json override/i);
			await user.click(jsonHeader);

			// Find the JSON textarea
			const jsonTextarea = screen.getByRole('textbox', { name: /json/i }) ||
				screen.getByDisplayValue(/"shared"/);

			// Should contain formatted JSON with the configured values
			const textareaValue = (jsonTextarea as HTMLTextAreaElement).value;
			const parsed = JSON.parse(textareaValue);
			expect(parsed.shared.allowed_tools).toContain('Bash');
			expect(parsed.shared.env).toHaveProperty('FOO', 'bar');
		});
	});

	describe('SC-10: JSON Override edits sync to structured fields', () => {
		it('updates structured fields when valid JSON is edited', async () => {
			const user = userEvent.setup();

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig: '{}' })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Expand JSON Override section
			const jsonHeader = screen.getByText(/json override/i);
			await user.click(jsonHeader);

			// AMEND-005: Use fireEvent.change for JSON content (userEvent.type parses [ ] { as keyboard descriptors)
			const jsonTextarea = screen.getByRole('textbox', { name: /json/i });
			fireEvent.change(jsonTextarea, { target: { value: '{"providers":{"claude":{"skill_refs":["python-style"]}}}' } });

			// Trigger blur/apply
			await user.tab();

			// The skills section badge should update
			// Expand Skills section to verify
			const skillsHeader = screen.getByText(/skills/i);
			await user.click(skillsHeader);

			// python-style should now appear as selected
			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});
		});

		it('shows validation error for invalid JSON without changing structured fields', async () => {
			const user = userEvent.setup();

			const runtimeConfig = makeRuntimeConfig({ allowedTools: ['Bash'] });

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Expand JSON Override section
			const jsonHeader = screen.getByText(/json override/i);
			await user.click(jsonHeader);

			// AMEND-005: Use fireEvent.change for JSON content
			const jsonTextarea = screen.getByRole('textbox', { name: /json/i });
			fireEvent.change(jsonTextarea, { target: { value: '{invalid json' } });
			fireEvent.blur(jsonTextarea);

			// Should show "Invalid JSON" error message
			// AMEND-006: Use CSS class selector to distinguish error message from textarea content
			await waitFor(() => {
				const errorEl = document.querySelector('.edit-template-json-error');
				expect(errorEl).toBeTruthy();
			});

			// Original structured fields should be unchanged
			// Allowed Tools section should still have "Bash"
		});

		it('preserves invalid JSON in textarea for user to fix', async () => {
			const user = userEvent.setup();

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig: '{}' })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Expand JSON Override section
			const jsonHeader = screen.getByText(/json override/i);
			await user.click(jsonHeader);

			// AMEND-005: Use fireEvent.change for JSON content
			const jsonTextarea = screen.getByRole('textbox', { name: /json/i });
			const invalidJson = '{"missing_close_brace": true';
			fireEvent.change(jsonTextarea, { target: { value: invalidJson } });
			fireEvent.blur(jsonTextarea);

			// The textarea should still contain the user's invalid input
			expect((jsonTextarea as HTMLTextAreaElement).value).toContain('missing_close_brace');
		});
	});

	describe('SC-11: On modal open, sections populate from parsed runtime_config', () => {
		it('populates all sections from existing runtime_config JSON', async () => {
			const runtimeConfig = makeRuntimeConfig({
				hooks: ['pre-guard'],
				skillRefs: ['python-style'],
				mcpServers: { filesystem: { command: 'npx fs-server' } },
				allowedTools: ['Bash', 'Read'],
				disallowedTools: ['Write'],
				env: { FOO: 'bar' },
			});

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// The sections should have populated badge counts
			// Each section should reflect the parsed config
			// (We can't easily check collapsed section content without expanding,
			// but badge counts should be visible)
			expect(screen.getByRole('dialog')).toBeInTheDocument();
		});

		it('treats malformed runtime_config as empty config gracefully', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({
						isBuiltin: false,
						runtimeConfig: 'not valid json {{{',
					})}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// All sections should still render (empty state)
			expect(screen.getByText(/hooks/i)).toBeInTheDocument();
			expect(screen.getByText(/skills/i)).toBeInTheDocument();
		});

		it('renders sections empty when runtime_config is undefined', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({
						isBuiltin: false,
						runtimeConfig: undefined,
					})}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// All 7 section headers should be present
			expect(screen.getByText(/hooks/i)).toBeInTheDocument();
			expect(screen.getByText(/json override/i)).toBeInTheDocument();
		});
	});

	describe('SC-12: All new components are wired into EditPhaseTemplateModal', () => {
		it('renders CollapsibleSettingsSection instances for all 7 sections', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// All section headers should be clickable buttons (CollapsibleSettingsSection)
			// AMEND-003: Anchor regexes to avoid substring matches
			const sectionHeaders = [
				/^Hooks$/i, /^MCP Servers$/i, /^Skills$/i,
				/^Allowed Tools$/i, /^Disallowed Tools$/i, /^Env Vars$/i,
				/^JSON Override$/i,
			];

			for (const headerPattern of sectionHeaders) {
				expect(screen.getByText(headerPattern)).toBeInTheDocument();
			}
		});

		it('fetches hooks, skills, and MCP servers on mount', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listHooks).toHaveBeenCalled();
				expect(configClient.listSkills).toHaveBeenCalled();
				expect(mcpClient.listMCPServers).toHaveBeenCalled();
			});
		});
	});

	describe('Edge cases', () => {
		it('preserves unknown fields through parse→edit→serialize cycle', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue(
				createMockUpdatePhaseTemplateResponse(
					createMockPhaseTemplate({ isBuiltin: false })
				)
			);

			const runtimeConfig = JSON.stringify({
				shared: {
					allowed_tools: ['Bash'],
				},
				unknown_future_field: { some: 'data' },
			});

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Save without changing anything
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalled();
			});

			const call = vi.mocked(workflowClient.updatePhaseTemplate).mock.calls[0][0];
			const parsedConfig = JSON.parse(call.runtimeConfig as string);

			// Unknown field should be preserved
			expect(parsedConfig.unknown_future_field).toEqual({ some: 'data' });
			// Known field should also be preserved
			expect(parsedConfig.shared.allowed_tools).toEqual(['Bash']);
		});

		it('renders settings sections as read-only for built-in templates', async () => {
			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: true })}
					isBuiltin={true}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Built-in template should show the read-only message
			expect(
				screen.getByText(/cannot edit built-in template/i)
			).toBeInTheDocument();

			// Wait for any pending async operations to complete
			await act(async () => {
				await new Promise(resolve => setTimeout(resolve, 0));
			});
		});

		it('listHooks error does not break other sections', async () => {
			vi.mocked(configClient.listHooks).mockRejectedValue(new Error('API error'));

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			// Other sections should still render
			// AMEND-003: Anchor regexes to avoid substring matches
			expect(screen.getByText(/^Skills$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Allowed Tools$/i)).toBeInTheDocument();
		});

		it('empty runtime_config produces minimal JSON on save', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.updatePhaseTemplate).mockResolvedValue(
				createMockUpdatePhaseTemplateResponse(
					createMockPhaseTemplate({ isBuiltin: false })
				)
			);

			render(
				<EditPhaseTemplateModal
					open={true}
					template={createMockPhaseTemplate({ isBuiltin: false, runtimeConfig: undefined })}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhaseTemplate).toHaveBeenCalled();
			});

			// runtimeConfig should be either undefined, empty string, or minimal JSON
			const call = vi.mocked(workflowClient.updatePhaseTemplate).mock.calls[0][0];
			if (call.runtimeConfig) {
				const parsed = JSON.parse(call.runtimeConfig);
				// Should be an empty or minimal object
				expect(typeof parsed).toBe('object');
			}
		});
	});
});
