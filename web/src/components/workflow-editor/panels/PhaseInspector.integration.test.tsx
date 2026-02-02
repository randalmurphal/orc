import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseInspector } from './PhaseInspector';
import { workflowClient, configClient, mcpClient } from '@/lib/client';

// Mock all client dependencies
vi.mock('@/lib/client', () => ({
	workflowClient: {
		updatePhase: vi.fn(),
	},
	configClient: {
		listAgents: vi.fn(),
		listHooks: vi.fn(),
		listSkills: vi.fn(),
	},
	mcpClient: {
		listMCPServers: vi.fn(),
	},
}));

describe('PhaseInspector - Integration Tests (TDD)', () => {
	const mockUser = userEvent.setup();

	const mockPhase = {
		id: 1,
		sequence: 1,
		phaseTemplateId: 'spec',
		template: {
			id: 'spec',
			name: 'Specification',
			isBuiltin: false,
			agentId: 'default-agent',
			model: 'claude-sonnet-4',
			maxIterations: 3,
			inputVariables: ['TASK_ID'],
			promptSource: 'template',
			promptContent: 'Write a spec',
			gateType: 0,
		},
		agentOverride: undefined,
		modelOverride: undefined,
		maxIterationsOverride: 3,
		subAgentsOverride: [],
		claudeConfigOverride: undefined,
	};

	const mockWorkflowDetails = {
		workflow: {
			id: 'test-workflow',
			name: 'Test Workflow',
			isBuiltin: false,
		},
		phases: [mockPhase],
		variables: [
			{ id: '1', name: 'TASK_ID', sourceType: 'STATIC', value: 'TASK-001' },
		],
	};

	beforeEach(() => {
		vi.clearAllMocks();
		(workflowClient.updatePhase as any).mockResolvedValue({});
		(configClient.listAgents as any).mockResolvedValue({
			agents: [{ id: '1', name: 'test-agent', model: 'claude-sonnet-4' }],
		});
		(configClient.listHooks as any).mockResolvedValue({ hooks: [] });
		(configClient.listSkills as any).mockResolvedValue({ skills: [] });
		(mcpClient.listMCPServers as any).mockResolvedValue({ servers: [] });
	});

	// Integration test for workflow editor canvas communication
	describe('Workflow Editor Canvas Integration', () => {
		it('calls onWorkflowRefresh when phase changes need canvas update', async () => {
			const mockOnWorkflowRefresh = vi.fn();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
					onWorkflowRefresh={mockOnWorkflowRefresh}
				/>
			);

			// Change phase name (should trigger canvas update)
			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.clear(nameInput);
			await mockUser.type(nameInput, 'Updated Spec');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalled();
				expect(mockOnWorkflowRefresh).toHaveBeenCalled();
			});
		});

		it('updates phase selection when onDeletePhase is called', async () => {
			const mockOnDeletePhase = vi.fn();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
					onDeletePhase={mockOnDeletePhase}
				/>
			);

			// Navigate to advanced section to find delete button
			const advancedHeader = screen.getByRole('button', { name: /advanced/i });
			await mockUser.click(advancedHeader);

			const deleteButton = screen.getByRole('button', { name: /remove phase/i });
			await mockUser.click(deleteButton);

			expect(mockOnDeletePhase).toHaveBeenCalled();
		});
	});

	// Integration test for API communication
	describe('API Integration', () => {
		it('makes correct updatePhase API call with all changed fields', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Wait for agents to load
			await waitFor(() => {
				expect(screen.getByLabelText(/executor/i)).toBeInTheDocument();
			});

			// Change multiple fields
			const executorSelect = screen.getByLabelText(/executor/i);
			const modelSelect = screen.getByLabelText(/model/i);
			const iterationsInput = screen.getByLabelText(/max iterations/i);

			await mockUser.selectOptions(executorSelect, 'test-agent');
			await mockUser.selectOptions(modelSelect, 'claude-opus-4');
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '5');

			// Blur to trigger save
			fireEvent.blur(iterationsInput);

			// Verify correct API call structure
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith({
					workflowId: 'test-workflow',
					phaseId: 1,
					agentOverride: 'test-agent',
					modelOverride: 'claude-opus-4',
					maxIterationsOverride: 5,
				});
			});
		});

		it('handles UpdatePhaseRequest protobuf message format correctly', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Enable artifact production to test complex field
			const dataFlowHeader = screen.getByRole('button', { name: /data flow/i });
			await mockUser.click(dataFlowHeader);

			const artifactToggle = screen.getByLabelText(/produces artifact/i);
			await mockUser.click(artifactToggle);

			const artifactTypeSelect = await screen.findByLabelText(/artifact type/i);
			await mockUser.selectOptions(artifactTypeSelect, 'spec');

			// Should use correct protobuf field names
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'test-workflow',
						phaseId: 1,
						producesArtifact: true,
						artifactType: 'spec',
					})
				);
			});
		});

		it('handles API errors gracefully without breaking UI state', async () => {
			(workflowClient.updatePhase as any).mockRejectedValue(
				new Error('Network timeout')
			);

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const nameInput = screen.getByDisplayValue('Specification');
			await mockUser.type(nameInput, ' Updated');
			fireEvent.blur(nameInput);

			await waitFor(() => {
				// Should show error but UI remains functional
				expect(screen.getByText(/network timeout/i)).toBeInTheDocument();
				expect(nameInput).not.toBeDisabled();
			});

			// Should still be able to interact with other parts of UI
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader);
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();
		});
	});

	// Integration test for PromptEditor component
	describe('PromptEditor Integration', () => {
		it('delegates prompt editing to existing PromptEditor component', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const promptHeader = screen.getByRole('button', { name: /prompt/i });
			await mockUser.click(promptHeader);

			// Should render PromptEditor with correct props
			const promptEditor = screen.getByTestId('prompt-editor');
			expect(promptEditor).toBeInTheDocument();
			expect(promptEditor).toHaveAttribute('data-phase-template-id', 'spec');
			expect(promptEditor).toHaveAttribute('data-prompt-source', 'template');
		});

		it('maintains prompt editing functionality from current implementation', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			const promptHeader = screen.getByRole('button', { name: /prompt/i });
			await mockUser.click(promptHeader);

			// Switch to custom prompt source
			const customButton = screen.getByRole('button', { name: 'Custom' });
			await mockUser.click(customButton);

			// Should show monaco editor (from existing PromptEditor)
			await waitFor(() => {
				expect(screen.getByTestId('monaco-editor')).toBeInTheDocument();
			});
		});
	});

	// Integration test for state management
	describe('State Management Integration', () => {
		it('maintains section state across component re-renders', async () => {
			const { rerender } = render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Expand multiple sections
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			const envHeader = screen.getByRole('button', { name: /environment/i });

			await mockUser.click(subAgentsHeader);
			await mockUser.click(envHeader);

			// Verify sections are expanded
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();
			expect(screen.getByTestId('environment-content')).toBeVisible();

			// Re-render with same props
			rerender(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// State should persist
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();
			expect(screen.getByTestId('environment-content')).toBeVisible();
		});

		it('integrates with workflow editor store for selected phase changes', async () => {
			const { rerender } = render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Expand section
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader);
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();

			// Change to different phase
			const differentPhase = {
				...mockPhase,
				id: 2,
				phaseTemplateId: 'implement',
				template: {
					...mockPhase.template,
					name: 'Implementation',
				},
			};

			rerender(
				<PhaseInspector
					phase={differentPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should show different phase data but maintain section states
			expect(screen.getByDisplayValue('Implementation')).toBeInTheDocument();
			expect(screen.getByTestId('sub-agents-content')).toBeVisible();
		});
	});

	// Integration test for accessibility
	describe('Accessibility Integration', () => {
		it('maintains proper ARIA labels and keyboard navigation', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// All collapsible sections should have proper ARIA attributes
			const sections = screen.getAllByRole('button', {
				name: /^(sub-agents|prompt|data flow|environment|advanced)$/i,
			});

			sections.forEach((section) => {
				expect(section).toHaveAttribute('aria-expanded');
				expect(section).toHaveAttribute('aria-controls');
			});

			// Form controls should have proper labels
			expect(screen.getByLabelText(/executor/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/model/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/max iterations/i)).toBeInTheDocument();
		});

		it('supports keyboard navigation between sections', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Should be able to navigate sections with keyboard
			const firstSection = screen.getByRole('button', { name: /sub-agents/i });
			firstSection.focus();

			fireEvent.keyDown(firstSection, { key: 'Tab' });

			// Next focusable element should be the prompt section
			const promptSection = screen.getByRole('button', { name: /prompt/i });
			expect(document.activeElement).toBe(promptSection);
		});
	});

	// Integration test for performance
	describe('Performance Integration', () => {
		it('does not re-fetch data unnecessarily on section expand/collapse', async () => {
			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Wait for initial load
			await waitFor(() => {
				expect(configClient.listAgents).toHaveBeenCalledTimes(1);
			});

			// Expand/collapse sections
			const subAgentsHeader = screen.getByRole('button', { name: /sub-agents/i });
			await mockUser.click(subAgentsHeader); // expand
			await mockUser.click(subAgentsHeader); // collapse

			// Should not trigger additional API calls
			expect(configClient.listAgents).toHaveBeenCalledTimes(1);
		});

		it('batches API calls when multiple fields change simultaneously', async () => {
			vi.useFakeTimers();

			render(
				<PhaseInspector
					phase={mockPhase}
					workflowDetails={mockWorkflowDetails}
					readOnly={false}
				/>
			);

			// Change multiple fields rapidly
			const nameInput = screen.getByDisplayValue('Specification');
			const iterationsInput = screen.getByLabelText(/max iterations/i);

			await mockUser.type(nameInput, ' Updated');
			await mockUser.clear(iterationsInput);
			await mockUser.type(iterationsInput, '5');

			// Complete debounce
			vi.advanceTimersByTime(500);

			// Should make single batched API call
			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledTimes(1);
			});

			vi.useRealTimers();
		});
	});
});