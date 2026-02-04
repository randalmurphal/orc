/**
 * TDD Tests for AgentsPalette component
 *
 * Tests written BEFORE implementation per TDD methodology.
 * These tests define the expected behavior for TASK-725:
 * "Add agents section to editor left palette"
 *
 * Success Criteria from spec:
 * - SC-1: Agents section appears in left palette below Workflow Settings
 * - SC-2: Agents are fetched from API on component mount
 * - SC-3: Loading state is displayed while agents are fetching
 * - SC-4: Built-in and custom agents are displayed in separate groups
 * - SC-5: Each agent card shows icon, name, and truncated description
 * - SC-6: Clicking agent with no phase selected shows agent details in inspector
 * - SC-7: Clicking agent with phase selected assigns agent to that phase
 * - SC-8: Section is collapsible
 */

import { render, screen, waitFor, fireEvent, within } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { AgentsPalette } from './AgentsPalette';
import type { Agent } from '@/gen/orc/v1/config_pb';

// Mock the config client
vi.mock('@/lib/client', () => ({
	configClient: {
		listAgents: vi.fn(),
	},
}));

// Mock the workflow editor store for selection state
vi.mock('@/stores/workflowEditorStore', () => ({
	useWorkflowEditorStore: vi.fn((selector) => {
		// Allow tests to control selection state via mockSelectedNodeId
		const state = {
			selectedNodeId: mockSelectedNodeId,
			selectNode: mockSelectNode,
		};
		return selector(state);
	}),
}));

// Import mocked modules for test setup
import { configClient } from '@/lib/client';

// Test control variables
let mockSelectedNodeId: string | null = null;
const mockSelectNode = vi.fn();
const mockOnAgentClick = vi.fn();
const mockOnAgentAssign = vi.fn();

// Helper to create mock agent data
function createMockAgent(overrides: Partial<Agent> = {}): Agent {
	return {
		id: 'test-agent-1',
		name: 'Test Agent',
		description: 'A test agent for unit testing purposes',
		model: 'claude-sonnet-4',
		isBuiltin: false,
		scope: 0,
		skillRefs: [],
		createdAt: undefined,
		updatedAt: undefined,
		...overrides,
	} as Agent;
}

describe('AgentsPalette', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockSelectedNodeId = null;
	});

	afterEach(() => {
		vi.resetAllMocks();
	});

	// ─── SC-2: Agents are fetched from API on component mount ─────────────────
	describe('API Integration (SC-2)', () => {
		it('fetches agents from API on mount', async () => {
			const mockAgents = [createMockAgent()];
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: mockAgents });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalledTimes(1);
				expect(configClient.listAgents).toHaveBeenCalledWith({});
			});
		});

		it('handles API errors gracefully', async () => {
			vi.mocked(configClient.listAgents).mockRejectedValue(new Error('Network error'));

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load agents/i)).toBeInTheDocument();
			});
		});

		it('shows retry button on error', async () => {
			vi.mocked(configClient.listAgents).mockRejectedValue(new Error('Network error'));

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const retryButton = screen.getByRole('button', { name: /retry/i });
				expect(retryButton).toBeInTheDocument();
			});
		});
	});

	// ─── SC-3: Loading state is displayed while agents are fetching ───────────
	describe('Loading State (SC-3)', () => {
		it('shows loading indicator while fetching agents', async () => {
			// Create a promise that doesn't resolve immediately
			let resolvePromise: (value: { agents: Agent[] }) => void;
			const pendingPromise = new Promise<{ agents: Agent[] }>((resolve) => {
				resolvePromise = resolve;
			});
			vi.mocked(configClient.listAgents).mockReturnValue(pendingPromise);

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			// Should show loading state
			expect(screen.getByText(/loading agents/i)).toBeInTheDocument();

			// Resolve the promise
			resolvePromise!({ agents: [createMockAgent()] });

			// Wait for loading to complete
			await waitFor(() => {
				expect(screen.queryByText(/loading agents/i)).not.toBeInTheDocument();
			});
		});

		it('has accessible loading state with aria-busy', async () => {
			let resolvePromise: (value: { agents: Agent[] }) => void;
			const pendingPromise = new Promise<{ agents: Agent[] }>((resolve) => {
				resolvePromise = resolve;
			});
			vi.mocked(configClient.listAgents).mockReturnValue(pendingPromise);

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			const palette = screen.getByTestId('agents-palette');
			expect(palette).toHaveAttribute('aria-busy', 'true');

			resolvePromise!({ agents: [] });

			await waitFor(() => {
				expect(palette).toHaveAttribute('aria-busy', 'false');
			});
		});
	});

	// ─── SC-4: Built-in and custom agents are displayed in separate groups ────
	describe('Agent Grouping (SC-4)', () => {
		it('displays built-in agents in a separate group', async () => {
			const builtinAgent = createMockAgent({
				id: 'builtin-1',
				name: 'Code Reviewer',
				isBuiltin: true,
			});
			const customAgent = createMockAgent({
				id: 'custom-1',
				name: 'My Personal Agent',
				isBuiltin: false,
			});
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [builtinAgent, customAgent],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/built-in/i)).toBeInTheDocument();
				expect(screen.getByText(/custom/i)).toBeInTheDocument();
			});
		});

		it('groups built-in agents under Built-in header', async () => {
			const builtinAgent1 = createMockAgent({
				id: 'builtin-1',
				name: 'Code Reviewer',
				isBuiltin: true,
			});
			const builtinAgent2 = createMockAgent({
				id: 'builtin-2',
				name: 'Test Writer',
				isBuiltin: true,
			});
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [builtinAgent1, builtinAgent2],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const builtinSection = screen.getByTestId('agents-group-builtin');
				expect(within(builtinSection).getByText('Code Reviewer')).toBeInTheDocument();
				expect(within(builtinSection).getByText('Test Writer')).toBeInTheDocument();
			});
		});

		it('groups custom agents under Custom header', async () => {
			const customAgent1 = createMockAgent({
				id: 'custom-1',
				name: 'My Agent 1',
				isBuiltin: false,
			});
			const customAgent2 = createMockAgent({
				id: 'custom-2',
				name: 'My Agent 2',
				isBuiltin: false,
			});
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [customAgent1, customAgent2],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const customSection = screen.getByTestId('agents-group-custom');
				expect(within(customSection).getByText('My Agent 1')).toBeInTheDocument();
				expect(within(customSection).getByText('My Agent 2')).toBeInTheDocument();
			});
		});

		it('hides empty group headers when no agents of that type', async () => {
			// Only custom agents, no built-in
			const customAgent = createMockAgent({
				id: 'custom-1',
				name: 'My Custom Agent',
				isBuiltin: false,
			});
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [customAgent],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.queryByTestId('agents-group-builtin')).not.toBeInTheDocument();
				expect(screen.getByTestId('agents-group-custom')).toBeInTheDocument();
			});
		});

		it('shows empty state when no agents exist', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/no agents available/i)).toBeInTheDocument();
			});
		});
	});

	// ─── SC-5: Each agent card shows icon, name, and truncated description ────
	describe('Agent Card Display (SC-5)', () => {
		it('displays agent name', async () => {
			const agent = createMockAgent({ name: 'Code Reviewer Agent' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText('Code Reviewer Agent')).toBeInTheDocument();
			});
		});

		it('displays agent description', async () => {
			const agent = createMockAgent({ description: 'Reviews code for bugs' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText('Reviews code for bugs')).toBeInTheDocument();
			});
		});

		it('truncates long descriptions', async () => {
			const longDescription =
				'This is a very long description that should be truncated because it exceeds the maximum allowed length for display in the compact agent card view within the palette';
			const agent = createMockAgent({ description: longDescription });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				// The description element should have truncation styling
				const descEl = screen.getByTestId(`agent-description-${agent.id}`);
				expect(descEl).toHaveClass('truncated');
			});
		});

		it('displays agent icon', async () => {
			const agent = createMockAgent({ id: 'agent-1', name: 'Test Agent' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const agentCard = screen.getByTestId('agent-card-agent-1');
				const icon = within(agentCard).getByTestId('agent-icon');
				expect(icon).toBeInTheDocument();
			});
		});

		it('handles missing description gracefully', async () => {
			const agent = createMockAgent({ description: '' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				// Should render without description, no crash
				expect(screen.getByText(agent.name)).toBeInTheDocument();
			});
		});
	});

	// ─── SC-6: Click agent with no phase selected → show details in inspector ─
	describe('Agent Click - No Phase Selected (SC-6)', () => {
		it('calls onAgentClick when clicking agent with no phase selected', async () => {
			mockSelectedNodeId = null; // No phase selected
			const agent = createMockAgent({ id: 'agent-1', name: 'Test Agent' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			const agentCard = screen.getByTestId('agent-card-agent-1');
			fireEvent.click(agentCard);

			expect(mockOnAgentClick).toHaveBeenCalledTimes(1);
			expect(mockOnAgentClick).toHaveBeenCalledWith(agent);
		});

		it('does not call onAgentAssign when no phase is selected', async () => {
			mockSelectedNodeId = null;
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByTestId('agent-card-agent-1'));

			expect(mockOnAgentAssign).not.toHaveBeenCalled();
		});
	});

	// ─── SC-7: Click agent with phase selected → assign agent to phase ────────
	describe('Agent Click - Phase Selected (SC-7)', () => {
		it('calls onAgentAssign when clicking agent with phase selected', async () => {
			mockSelectedNodeId = 'phase-node-1'; // Phase is selected
			const agent = createMockAgent({ id: 'agent-1', name: 'Test Agent' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
					selectedNodeId="phase-node-1"
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByTestId('agent-card-agent-1'));

			expect(mockOnAgentAssign).toHaveBeenCalledTimes(1);
			expect(mockOnAgentAssign).toHaveBeenCalledWith(agent);
		});

		it('does not call onAgentClick when phase is selected', async () => {
			mockSelectedNodeId = 'phase-node-1';
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
					selectedNodeId="phase-node-1"
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			fireEvent.click(screen.getByTestId('agent-card-agent-1'));

			expect(mockOnAgentClick).not.toHaveBeenCalled();
		});

		it('shows visual indicator that clicking will assign agent when phase is selected', async () => {
			mockSelectedNodeId = 'phase-node-1';
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
					selectedNodeId="phase-node-1"
				/>
			);

			await waitFor(() => {
				const agentCard = screen.getByTestId('agent-card-agent-1');
				// Should have a class or aria-label indicating it can be assigned
				expect(agentCard).toHaveAttribute('aria-label', expect.stringMatching(/assign/i));
			});
		});
	});

	// ─── SC-8: Section is collapsible ─────────────────────────────────────────
	describe('Collapsible Section (SC-8)', () => {
		it('renders as a collapsible section', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [createMockAgent()],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const header = screen.getByRole('button', { name: /agents/i });
				expect(header).toHaveAttribute('aria-expanded');
			});
		});

		it('starts expanded by default', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [createMockAgent()],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const header = screen.getByRole('button', { name: /agents/i });
				expect(header).toHaveAttribute('aria-expanded', 'true');
			});
		});

		it('collapses when header is clicked', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [createMockAgent({ name: 'Visible Agent' })],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText('Visible Agent')).toBeInTheDocument();
			});

			const header = screen.getByRole('button', { name: /agents/i });
			fireEvent.click(header);

			await waitFor(() => {
				expect(header).toHaveAttribute('aria-expanded', 'false');
			});

			// Agent list should be hidden
			expect(screen.queryByText('Visible Agent')).not.toBeVisible();
		});

		it('expands when collapsed header is clicked', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [createMockAgent({ name: 'Hidden Agent' })],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
					defaultCollapsed={true}
				/>
			);

			await waitFor(() => {
				const header = screen.getByRole('button', { name: /agents/i });
				expect(header).toHaveAttribute('aria-expanded', 'false');
			});

			const header = screen.getByRole('button', { name: /agents/i });
			fireEvent.click(header);

			await waitFor(() => {
				expect(header).toHaveAttribute('aria-expanded', 'true');
				expect(screen.getByText('Hidden Agent')).toBeVisible();
			});
		});

		it('shows chevron indicator matching expanded state', async () => {
			vi.mocked(configClient.listAgents).mockResolvedValue({
				agents: [createMockAgent()],
			});

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				// Should show expanded chevron (▾ or similar)
				const chevron = screen.getByTestId('agents-chevron');
				expect(chevron).toHaveTextContent(/▾|▼/);
			});

			// Click to collapse
			fireEvent.click(screen.getByRole('button', { name: /agents/i }));

			await waitFor(() => {
				// Should show collapsed chevron (▸ or similar)
				const chevron = screen.getByTestId('agents-chevron');
				expect(chevron).toHaveTextContent(/▸|▶/);
			});
		});
	});

	// ─── Keyboard Accessibility ───────────────────────────────────────────────
	describe('Keyboard Accessibility', () => {
		it('agent cards are focusable', async () => {
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				const agentCard = screen.getByTestId('agent-card-agent-1');
				expect(agentCard).toHaveAttribute('tabIndex', '0');
			});
		});

		it('activates agent on Enter key', async () => {
			mockSelectedNodeId = null;
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			const agentCard = screen.getByTestId('agent-card-agent-1');
			agentCard.focus();
			fireEvent.keyDown(agentCard, { key: 'Enter' });

			expect(mockOnAgentClick).toHaveBeenCalledWith(agent);
		});

		it('activates agent on Space key', async () => {
			mockSelectedNodeId = null;
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			const agentCard = screen.getByTestId('agent-card-agent-1');
			agentCard.focus();
			fireEvent.keyDown(agentCard, { key: ' ' });

			expect(mockOnAgentClick).toHaveBeenCalledWith(agent);
		});
	});

	// ─── Read-Only Mode ───────────────────────────────────────────────────────
	describe('Read-Only Mode', () => {
		it('disables interaction when readOnly is true', async () => {
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
					readOnly={true}
				/>
			);

			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			const agentCard = screen.getByTestId('agent-card-agent-1');
			fireEvent.click(agentCard);

			// Should not trigger any callbacks in read-only mode
			expect(mockOnAgentClick).not.toHaveBeenCalled();
			expect(mockOnAgentAssign).not.toHaveBeenCalled();
		});

		it('shows visual indication of read-only state', async () => {
			const agent = createMockAgent({ id: 'agent-1' });
			vi.mocked(configClient.listAgents).mockResolvedValue({ agents: [agent] });

			render(
				<AgentsPalette
					onAgentClick={mockOnAgentClick}
					onAgentAssign={mockOnAgentAssign}
					readOnly={true}
				/>
			);

			await waitFor(() => {
				const agentCard = screen.getByTestId('agent-card-agent-1');
				expect(agentCard).toHaveClass('readonly');
			});
		});
	});
});
