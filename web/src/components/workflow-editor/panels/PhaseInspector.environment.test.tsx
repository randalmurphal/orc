/**
 * TDD Tests for PhaseInspector Environment Section + Sub-Agents Drag Reorder
 *
 * Tests for TASK-773: Complete TASK-726 phase inspector (drag-reorder + library pickers)
 *
 * Success Criteria Coverage:
 * - SC-1: Sub-agents can be reordered via drag-and-drop - autoSave called with new order
 * - SC-2: EnvironmentSection displays LibraryPicker for MCP Servers - selections save via autoSave
 * - SC-3: EnvironmentSection displays LibraryPicker for Skills - selections save via autoSave
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { create } from '@bufbuild/protobuf';
import {
	createMockWorkflowPhase,
	createMockPhaseTemplate,
	createMockHook,
	createMockSkill,
	createMockMCPServerInfo,
	createMockWorkflowWithDetails,
	createMockWorkflow,
} from '@/test/factories';
import type { WorkflowPhase, WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { Skill } from '@/gen/orc/v1/config_pb';
import type { MCPServerInfo } from '@/gen/orc/v1/mcp_pb';
import { AgentSchema, type Agent } from '@/gen/orc/v1/config_pb';

// NOTE: Browser API mocks (ResizeObserver, etc.) provided by global test-setup.ts

/** Create a mock Agent */
function createMockAgent(overrides: Partial<Agent> = {}): Agent {
	const base = create(AgentSchema, {
		name: 'test-agent',
		description: 'Test agent description',
	});
	return Object.assign(base, overrides);
}

// Mock clients used by PhaseInspector
// PhaseInspector uses configClient.listAgents (not agentClient)
const mockUpdatePhase = vi.fn().mockResolvedValue({ phase: {} });
const mockListAgents = vi.fn().mockResolvedValue({ agents: [] });
const mockListHooks = vi.fn().mockResolvedValue({ hooks: [] });
const mockListSkills = vi.fn().mockResolvedValue({ skills: [] });
const mockListMCPServers = vi.fn().mockResolvedValue({ servers: [] });
const mockGetMCPServer = vi.fn();
const phaseInspectorLibraryData = {
	agents: [] as Agent[],
	hooks: [] as unknown[],
	skills: [] as Skill[],
	mcpServers: [] as MCPServerInfo[],
	agentsLoading: false,
	hooksLoading: false,
	skillsLoading: false,
	mcpLoading: false,
};

vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhase: (...args: unknown[]) => mockUpdatePhase(...args),
	},
	configClient: {
		listAgents: (...args: unknown[]) => mockListAgents(...args),
		listHooks: (...args: unknown[]) => mockListHooks(...args),
		listSkills: (...args: unknown[]) => mockListSkills(...args),
	},
	mcpClient: {
		listMCPServers: (...args: unknown[]) => mockListMCPServers(...args),
		getMCPServer: (...args: unknown[]) => mockGetMCPServer(...args),
	},
}));

vi.mock('./phase-inspector/hooks', () => ({
	useMobileViewport: () => false,
}));

vi.mock('@/hooks/useLibraryData', () => ({
	useLibraryData: () => phaseInspectorLibraryData,
}));

// Import after mocks are set up
import { PhaseInspector } from './PhaseInspector';

/** Helper to expand a collapsible section by clicking its header button */
async function expandSection(sectionName: string) {
	// Find the button with the section name and click it
	const button = screen.getByRole('button', { name: new RegExp(sectionName, 'i') });
	await userEvent.click(button);
}

describe('TASK-773: PhaseInspector Enhancements', () => {
	const defaultWorkflowDetails: WorkflowWithDetails = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'test-workflow', name: 'Test', isBuiltin: false }),
		phases: [],
		variables: [],
	});

	beforeEach(() => {
		vi.clearAllMocks();
		phaseInspectorLibraryData.agents = [];
		phaseInspectorLibraryData.hooks = [];
		phaseInspectorLibraryData.skills = [];
		phaseInspectorLibraryData.mcpServers = [];
		phaseInspectorLibraryData.agentsLoading = false;
		phaseInspectorLibraryData.hooksLoading = false;
		phaseInspectorLibraryData.skillsLoading = false;
		phaseInspectorLibraryData.mcpLoading = false;
		mockGetMCPServer.mockImplementation(async (request: { name?: string }) => ({
			server: request.name === 'filesystem'
				? { name: 'filesystem', type: 'stdio', command: 'npx @mcp/server-fs', args: [], env: {}, headers: {}, disabled: false }
				: request.name === 'database'
					? { name: 'database', type: 'stdio', command: 'npx @mcp/server-pg', args: [], env: {}, headers: {}, disabled: false }
					: undefined,
		}));
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Sub-agents drag-to-reorder', () => {
		const mockAgents: Agent[] = [
			createMockAgent({ name: 'agent-1', description: 'First agent' }),
			createMockAgent({ name: 'agent-2', description: 'Second agent' }),
			createMockAgent({ name: 'agent-3', description: 'Third agent' }),
		];

		const createPhaseWithSubAgents = (subAgents: string[]): WorkflowPhase => {
			return createMockWorkflowPhase({
				subAgentsOverride: subAgents,
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
		};

		it('reorders sub-agents when item is dragged to new position', async () => {
			const phase = createPhaseWithSubAgents(['agent-1', 'agent-2', 'agent-3']);
			phaseInspectorLibraryData.agents = mockAgents;

			const onWorkflowRefresh = vi.fn();

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={onWorkflowRefresh}
				/>
			);

			// Wait for agents to load and expand sub-agents section
			await expandSection('Sub-Agents');

			// Wait for agent items to appear
			await waitFor(() => {
				expect(screen.getByText('agent-1')).toBeInTheDocument();
			});

			// Get the draggable items by their data-testid
			const agent1Handle = screen.getByTestId('drag-handle-agent-1');
			const agent3Handle = screen.getByTestId('drag-handle-agent-3');

			// Simulate drag-drop: drag agent-3 before agent-1
			fireEvent.dragStart(agent3Handle);
			fireEvent.dragOver(agent1Handle);
			fireEvent.drop(agent1Handle);
			fireEvent.dragEnd(agent3Handle);

			// Verify updatePhase was called with reordered array
			await waitFor(() => {
				expect(mockUpdatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						subAgentsOverride: ['agent-3', 'agent-1', 'agent-2'],
					})
				);
			});
		});

		it('drag handles have data-testid for each sub-agent in editable mode', async () => {
			const phase = createPhaseWithSubAgents(['agent-1', 'agent-2']);
			phaseInspectorLibraryData.agents = mockAgents;

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand sub-agents section
			await expandSection('Sub-Agents');

			await waitFor(() => {
				expect(screen.getByText('agent-1')).toBeInTheDocument();
			});

			// Each agent should have a visible drag handle with proper test id
			expect(screen.getByTestId('drag-handle-agent-1')).toBeInTheDocument();
			expect(screen.getByTestId('drag-handle-agent-2')).toBeInTheDocument();
		});
	});

	describe('SC-2: EnvironmentSection MCP Servers LibraryPicker', () => {
		const mockMCPServers: MCPServerInfo[] = [
			createMockMCPServerInfo({ name: 'filesystem', command: 'npx @mcp/server-fs' }),
			createMockMCPServerInfo({ name: 'database', command: 'npx @mcp/server-pg' }),
		];

		it('displays LibraryPicker for MCP Servers in Environment section', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
			phaseInspectorLibraryData.mcpServers = mockMCPServers;

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Environment section
			await expandSection('Environment');

			// Wait for MCP servers to load - they should appear as selectable items
			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			expect(screen.getByText('database')).toBeInTheDocument();
			// Should NOT show placeholder anymore
			expect(
				screen.queryByText('MCP servers, skills, and hooks will be shown here')
			).not.toBeInTheDocument();
		});

		it('selecting MCP server calls autoSave with updated runtimeConfigOverride', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
			phaseInspectorLibraryData.mcpServers = mockMCPServers;

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Environment section
			await expandSection('Environment');

			await waitFor(() => {
				expect(screen.getByText('filesystem')).toBeInTheDocument();
			});

			// Click to select filesystem server
			await userEvent.click(screen.getByText('filesystem'));

			// Verify updatePhase was called with runtimeConfigOverride containing mcpServers
			await waitFor(() => {
				expect(mockUpdatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						runtimeConfigOverride: expect.stringContaining('filesystem'),
					})
				);
			});
		});
	});

	describe('SC-3: EnvironmentSection Skills LibraryPicker', () => {
		const mockSkills: Skill[] = [
			createMockSkill({ name: 'python-style', description: 'Python coding standards' }),
			createMockSkill({ name: 'tdd', description: 'Test-driven development' }),
		];

		it('displays LibraryPicker for Skills in Environment section', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
			phaseInspectorLibraryData.skills = mockSkills;

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Environment section
			await expandSection('Environment');

			// Wait for skills to load
			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			expect(screen.getByText('tdd')).toBeInTheDocument();
		});

		it('selecting skill calls autoSave with updated runtimeConfigOverride', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
			phaseInspectorLibraryData.skills = mockSkills;

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Environment section
			await expandSection('Environment');

			await waitFor(() => {
				expect(screen.getByText('python-style')).toBeInTheDocument();
			});

			// Click to select python-style skill
			await userEvent.click(screen.getByText('python-style'));

			// Verify updatePhase was called with runtimeConfigOverride containing skillRefs
			await waitFor(() => {
				expect(mockUpdatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						runtimeConfigOverride: expect.stringContaining('python-style'),
					})
				);
			});
		});
	});

	describe('Environment section placeholder removal', () => {
		it('shows placeholder when no library data available', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
			// All library mocks return empty arrays (default)

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Environment section
			await expandSection('Environment');

			// With no data, the empty state should show
			await waitFor(() => {
				expect(screen.getByText('None configured')).toBeInTheDocument();
			});
		});

		it('replaces placeholder with LibraryPickers when data is loaded', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});
			phaseInspectorLibraryData.hooks = [createMockHook({ name: 'test-hook', eventType: 'PreToolUse' })];

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{
						...defaultWorkflowDetails,
						phases: [phase],
					}}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Environment section
			await expandSection('Environment');

			// Wait for hooks to load
			await waitFor(() => {
				expect(screen.getByText('test-hook')).toBeInTheDocument();
			});

			// Placeholder should be gone
			expect(
				screen.queryByText('MCP servers, skills, and hooks will be shown here')
			).not.toBeInTheDocument();
		});
	});
});
