import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { vi, beforeEach, describe, it, expect, afterEach } from 'vitest';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { WorkflowEditorPage } from './WorkflowEditorPage';
import { workflowClient, configClient, mcpClient } from '@/lib/client';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import type { Agent } from '@/gen/orc/v1/config_pb';
import { useWorkflowEditorStore } from '@/stores/workflowEditorStore';

// Mock agents for testing
const mockAgents: Agent[] = [
	{
		id: 'agent-1',
		name: 'TestAgent',
		description: 'A test agent',
		isBuiltin: false,
		model: 'sonnet',
		scope: 1,
	} as unknown as Agent,
	{
		id: 'agent-2',
		name: 'BuiltinAgent',
		description: 'A built-in agent',
		isBuiltin: true,
		model: 'opus',
		scope: 1,
	} as unknown as Agent,
];

// Mock the client
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getWorkflow: vi.fn(),
		updateWorkflow: vi.fn(),
		listPhaseTemplates: vi.fn(),
		listWorkflowRuns: vi.fn(),
		updatePhase: vi.fn(),
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

const mockWorkflowDetails: WorkflowWithDetails = {
	workflow: {
		id: 'test-workflow',
		name: 'Test Workflow',
		description: 'A test workflow',
		defaultModel: 'sonnet',
		defaultThinking: true,
		completionAction: 'pr',
		targetBranch: 'main',
		isBuiltin: false,
		basedOn: '',
		createdAt: undefined,
		updatedAt: undefined,
	} as WorkflowWithDetails['workflow'],
	phases: [],
	variables: [],
} as unknown as WorkflowWithDetails;

const mockBuiltinWorkflowDetails: WorkflowWithDetails = {
	workflow: {
		...mockWorkflowDetails.workflow!,
		id: 'builtin-workflow',
		name: 'Built-in Workflow',
		isBuiltin: true,
	} as WorkflowWithDetails['workflow'],
	phases: [],
	variables: [],
} as unknown as WorkflowWithDetails;

describe('WorkflowEditorPage Integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Reset the store state between tests
		useWorkflowEditorStore.getState().reset();

		// Mock successful API responses by default
		(workflowClient.getWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({ workflow: mockWorkflowDetails });
		(workflowClient.listPhaseTemplates as ReturnType<typeof vi.fn>).mockResolvedValue({ templates: [], sources: {} });
		(workflowClient.listWorkflowRuns as ReturnType<typeof vi.fn>).mockResolvedValue({ runs: [] });
		(configClient.listAgents as ReturnType<typeof vi.fn>).mockResolvedValue({ agents: mockAgents });
		(configClient.listHooks as ReturnType<typeof vi.fn>).mockResolvedValue({ hooks: [] });
		(configClient.listSkills as ReturnType<typeof vi.fn>).mockResolvedValue({ skills: [] });
		(mcpClient.listMCPServers as ReturnType<typeof vi.fn>).mockResolvedValue({ servers: [] });
		(mcpClient.getMCPServer as ReturnType<typeof vi.fn>).mockResolvedValue({ server: undefined });
	});

	afterEach(() => {
		// Clean up store state after each test
		useWorkflowEditorStore.getState().reset();
	});

	// SC-6: Integration with Existing Editor
	describe('Left Palette Integration', () => {
		it('renders workflow settings in left palette alongside phase templates', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Should have the left palette with both sections
			expect(screen.getByTestId('left-palette')).toBeInTheDocument();
			expect(screen.getByTestId('workflow-settings-panel')).toBeInTheDocument();
			expect(screen.getByTestId('phase-template-palette')).toBeInTheDocument();
		});

		it('preserves existing phase template drag and drop functionality', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Phase template palette should still be present and functional
			const phaseTemplatePalette = screen.getByTestId('phase-template-palette');
			expect(phaseTemplatePalette).toBeInTheDocument();
			expect(phaseTemplatePalette).toHaveAttribute('data-preserves-drag-drop', 'true');
		});

		it('maintains existing canvas and inspector layout', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Should maintain the existing three-column layout
			expect(screen.getByTestId('workflow-editor-canvas')).toBeInTheDocument();
			expect(screen.getByTestId('left-palette')).toBeInTheDocument();

			// Inspector panels should still work (when nodes/edges selected)
			const editorBody = screen.getByTestId('workflow-editor-body');
			expect(editorBody).toBeInTheDocument();
		});
	});

	// SC-5: Settings Persistence through Editor
	describe('Settings Persistence Integration', () => {
		it('updates workflow settings and refreshes editor state', async () => {
			const updatedWorkflow = {
				...mockWorkflowDetails.workflow!,
				name: 'Updated Workflow Name',
			};

			// Mock the update call
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({ workflow: updatedWorkflow });

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Find and update the name field in workflow settings
			const nameInput = screen.getByLabelText('Name');
			fireEvent.change(nameInput, { target: { value: 'Updated Workflow Name' } });
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith({
					id: 'test-workflow',
					name: 'Updated Workflow Name',
				});
			});
		});

		it('handles workflow settings update errors gracefully', async () => {
			(workflowClient.updateWorkflow as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('Update failed'));

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			const nameInput = screen.getByLabelText('Name');
			fireEvent.change(nameInput, { target: { value: 'New Name' } });
			fireEvent.blur(nameInput);

			// Should show error message and not break the editor
			await waitFor(() => {
				expect(screen.getByText(/Failed to update workflow/)).toBeInTheDocument();
			});

			// Editor should still be functional
			expect(screen.getByTestId('workflow-editor-canvas')).toBeInTheDocument();
		});
	});

	// SC-1: Read-only behavior for builtin workflows
	describe('Builtin Workflow Handling', () => {
		it('shows read-only workflow settings for builtin workflows', async () => {
			(workflowClient.getWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({ workflow: mockBuiltinWorkflowDetails });

			render(
				<MemoryRouter initialEntries={['/workflows/builtin-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Built-in Workflow')).toBeInTheDocument();
			});

			// Settings should be read-only (both settings panel and phase palette show this message)
			const cloneMessages = screen.getAllByText('Clone to customize');
			expect(cloneMessages.length).toBeGreaterThan(0);
			expect(screen.getByLabelText('Name')).toBeDisabled();

			// Built-in badge should be shown (header and/or settings panel)
			const builtinBadges = screen.getAllByText('Built-in');
			expect(builtinBadges.length).toBeGreaterThan(0);

			// Clone button should be present
			expect(screen.getByText('Clone')).toBeInTheDocument();
		});

		it('maintains existing clone workflow functionality', async () => {
			(workflowClient.getWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({ workflow: mockBuiltinWorkflowDetails });

			render(
				<MemoryRouter initialEntries={['/workflows/builtin-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Built-in Workflow')).toBeInTheDocument();
			});

			// Clone button should still work (existing functionality)
			const cloneButton = screen.getByText('Clone');
			fireEvent.click(cloneButton);

			// Should open clone modal (existing functionality)
			expect(screen.getByTestId('clone-workflow-modal')).toBeInTheDocument();
		});
	});

	// Performance and layout tests
	describe('Performance and Layout', () => {
		it('does not affect editor loading performance', async () => {
			const startTime = Date.now();

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			const loadTime = Date.now() - startTime;
			// Should load within reasonable time (< 1 second in test environment)
			expect(loadTime).toBeLessThan(1000);
		});

		it('maintains responsive layout with settings panel', async () => {
			// Mock narrow viewport
			Object.defineProperty(window, 'innerWidth', {
				writable: true,
				configurable: true,
				value: 768,
			});

			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			const leftPalette = screen.getByTestId('left-palette');
			expect(leftPalette).toHaveClass('left-palette');
		});
	});

	// TASK-725: Agent Integration Tests
	// These tests verify the full integration from clicking an agent in the palette
	// through to showing the AgentInspector panel.
	describe('Agent Integration (TASK-725)', () => {
		it('renders AgentsPalette in the left palette', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Verify AgentsPalette is rendered within the left palette
			expect(screen.getByTestId('left-palette')).toBeInTheDocument();
			expect(screen.getByTestId('agents-palette')).toBeInTheDocument();
		});

		it('opens AgentInspector when clicking an agent in the palette', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Wait for agents to load (palette has count badge when loaded)
			await waitFor(() => {
				expect(screen.getByTestId('agents-palette')).toBeInTheDocument();
			});

			// Wait for agents to appear in the palette
			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			// Click the agent card (div with role="button")
			const agentCard = screen.getByTestId('agent-card-agent-1');
			await act(async () => {
				fireEvent.click(agentCard);
			});

			// Wait for AgentInspector to appear
			await waitFor(() => {
				expect(screen.getByTestId('agent-inspector')).toBeInTheDocument();
			});

			// Verify agent details are shown in the inspector
			const inspector = screen.getByTestId('agent-inspector');
			expect(inspector).toHaveTextContent('TestAgent');
		});

		it('closes AgentInspector when clicking close button', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Wait for agents to load
			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			// Open the inspector by clicking an agent
			const agentCard = screen.getByTestId('agent-card-agent-1');
			await act(async () => {
				fireEvent.click(agentCard);
			});

			await waitFor(() => {
				expect(screen.getByTestId('agent-inspector')).toBeInTheDocument();
			});

			// Find and click the close button (aria-label is "Close agent inspector")
			const closeButton = screen.getByRole('button', { name: /close agent inspector/i });
			await act(async () => {
				fireEvent.click(closeButton);
			});

			// Verify inspector is closed
			await waitFor(() => {
				expect(screen.queryByTestId('agent-inspector')).not.toBeInTheDocument();
			});
		});

		it('selects agent and updates store when no phase is selected', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Wait for agents to load
			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-1')).toBeInTheDocument();
			});

			// Verify no agent or node is selected initially
			expect(useWorkflowEditorStore.getState().selectedAgent).toBeNull();
			expect(useWorkflowEditorStore.getState().selectedNodeId).toBeNull();

			// Click an agent card (with no phase selected)
			const agentCard = screen.getByTestId('agent-card-agent-1');
			await act(async () => {
				fireEvent.click(agentCard);
			});

			// Verify: agent is now selected in the store
			await waitFor(() => {
				expect(useWorkflowEditorStore.getState().selectedAgent?.id).toBe('agent-1');
			});
		});

		it('shows correct agent details in AgentInspector', async () => {
			render(
				<MemoryRouter initialEntries={['/workflows/test-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Test Workflow')).toBeInTheDocument();
			});

			// Wait for agents to load
			await waitFor(() => {
				expect(screen.getByTestId('agent-card-agent-2')).toBeInTheDocument();
			});

			// Click the builtin agent card
			const agentCard = screen.getByTestId('agent-card-agent-2');
			await act(async () => {
				fireEvent.click(agentCard);
			});

			await waitFor(() => {
				expect(screen.getByTestId('agent-inspector')).toBeInTheDocument();
			});

			// Verify agent details are shown correctly in the inspector
			const inspector = screen.getByTestId('agent-inspector');
			expect(inspector).toHaveTextContent('BuiltinAgent');
			expect(inspector).toHaveTextContent('A built-in agent');
			// The agent badge shows "Built-in" for builtin agents
			expect(inspector).toHaveTextContent('Built-in');
		});

		it('passes readOnly to AgentsPalette based on workflow.isBuiltin', async () => {
			(workflowClient.getWorkflow as ReturnType<typeof vi.fn>).mockResolvedValue({
				workflow: mockBuiltinWorkflowDetails,
			});

			render(
				<MemoryRouter initialEntries={['/workflows/builtin-workflow']}>
					<Routes>
						<Route path="/workflows/:id" element={<WorkflowEditorPage />} />
					</Routes>
				</MemoryRouter>
			);

			await waitFor(() => {
				expect(screen.getByText('Built-in Workflow')).toBeInTheDocument();
			});

			// AgentsPalette should receive readOnly=true for builtin workflows
			const agentsPalette = screen.getByTestId('agents-palette');
			expect(agentsPalette).toHaveAttribute('data-readonly', 'true');
		});
	});
});
