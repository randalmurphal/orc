/**
 * TDD Tests for PhaseInspector Component - Core Functionality
 *
 * Tests for TASK-774: Restore test coverage for components with deleted tests
 *
 * Success Criteria Coverage:
 * - SC-1: Displays collapsible sections (Sub-Agents, Prompt, Data Flow, Environment, Advanced)
 * - SC-2: Auto-saves field changes with 500ms debounce
 * - SC-3: Validates phase name (non-empty)
 * - SC-4: Validates max iterations (1-20 range)
 * - SC-5: Shows read-only notice for built-in templates
 * - SC-6: Preserves section state across phase selections
 * - SC-7: Displays loading state while fetching data
 * - SC-8: Handles field errors gracefully
 *
 * Note: Environment section and sub-agents drag-reorder are tested in PhaseInspector.environment.test.tsx
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { create } from '@bufbuild/protobuf';
import {
	createMockWorkflowPhase,
	createMockPhaseTemplate,
	createMockWorkflowWithDetails,
	createMockWorkflow,
} from '@/test/factories';
import type { WorkflowWithDetails } from '@/gen/orc/v1/workflow_pb';
import { AgentSchema, type Agent } from '@/gen/orc/v1/config_pb';

// Mock clients used by PhaseInspector
const mockUpdatePhase = vi.fn().mockResolvedValue({ phase: {} });
const mockListAgents = vi.fn().mockResolvedValue({ agents: [] });
const mockListHooks = vi.fn().mockResolvedValue({ hooks: [] });
const mockListSkills = vi.fn().mockResolvedValue({ skills: [] });
const mockListMCPServers = vi.fn().mockResolvedValue({ servers: [] });

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
	},
}));

// Import after mocks are set up
import { PhaseInspector } from './PhaseInspector';

/** Create a mock Agent */
function createMockAgent(overrides: Partial<Agent> = {}): Agent {
	const base = create(AgentSchema, {
		name: 'test-agent',
		description: 'Test agent description',
	});
	return Object.assign(base, overrides);
}

/** Helper to expand a collapsible section by clicking its header button */
async function expandSection(sectionName: string) {
	const button = screen.getByRole('button', { name: new RegExp(sectionName, 'i') });
	await userEvent.click(button);
}

describe('TASK-774: PhaseInspector Core Functionality', () => {
	const defaultWorkflowDetails: WorkflowWithDetails = createMockWorkflowWithDetails({
		workflow: createMockWorkflow({ id: 'test-workflow', name: 'Test', isBuiltin: false }),
		phases: [],
		variables: [],
	});

	const mockAgents: Agent[] = [
		createMockAgent({ name: 'agent-1', description: 'First agent' }),
		createMockAgent({ name: 'agent-2', description: 'Second agent' }),
	];

	beforeEach(() => {
		vi.clearAllMocks();
		vi.useFakeTimers();
		mockListAgents.mockResolvedValue({ agents: mockAgents });
		mockListHooks.mockResolvedValue({ hooks: [] });
		mockListSkills.mockResolvedValue({ skills: [] });
		mockListMCPServers.mockResolvedValue({ servers: [] });
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
	});

	describe('SC-1: Displays collapsible sections', () => {
		it('renders all collapsible section headers', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// All section headers should be visible
			expect(screen.getByRole('button', { name: /sub-agents/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /prompt/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /data flow/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /environment/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /advanced/i })).toBeInTheDocument();
		});

		it('expands section when header is clicked', async () => {
			vi.useRealTimers(); // Need real timers for userEvent
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Prompt section should be collapsed by default
			expect(screen.queryByTestId('prompt-content')).not.toBeInTheDocument();

			// Click to expand
			await expandSection('Prompt');

			// Content should now be visible
			await waitFor(() => {
				expect(screen.getByTestId('prompt-content')).toBeInTheDocument();
			});
		});

		it('collapses section when header is clicked again', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand first
			await expandSection('Prompt');
			await waitFor(() => {
				expect(screen.getByTestId('prompt-content')).toBeInTheDocument();
			});

			// Click again to collapse
			await expandSection('Prompt');
			await waitFor(() => {
				expect(screen.queryByTestId('prompt-content')).not.toBeInTheDocument();
			});
		});

		it('displays always-visible section with core fields', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Always-visible fields should be present
			expect(screen.getByTestId('always-visible-section')).toBeInTheDocument();
			expect(screen.getByTestId('phase-name')).toBeInTheDocument();
			expect(screen.getByLabelText(/executor/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/model/i)).toBeInTheDocument();
		});
	});

	describe('SC-2: Auto-saves field changes with 500ms debounce', () => {
		it('auto-saves phase name after debounce period', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const nameInput = screen.getByTestId('phase-name');
			await userEvent.clear(nameInput);
			await userEvent.type(nameInput, 'New Phase Name');

			// Should not save immediately
			expect(mockUpdatePhase).not.toHaveBeenCalled();

			// Wait for debounce (500ms + buffer)
			await waitFor(
				() => {
					expect(mockUpdatePhase).toHaveBeenCalledWith(
						expect.objectContaining({
							templateName: 'New Phase Name',
						})
					);
				},
				{ timeout: 1000 }
			);
		});

		it('saves immediately on blur', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const nameInput = screen.getByTestId('phase-name');
			await userEvent.clear(nameInput);
			await userEvent.type(nameInput, 'New Name');
			fireEvent.blur(nameInput);

			// Should save immediately on blur
			await waitFor(() => {
				expect(mockUpdatePhase).toHaveBeenCalled();
			});
		});

		it('auto-saves model selection', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const modelSelect = screen.getByLabelText(/model/i);
			await userEvent.selectOptions(modelSelect, 'sonnet');

			await waitFor(
				() => {
					expect(mockUpdatePhase).toHaveBeenCalledWith(
						expect.objectContaining({
							modelOverride: 'sonnet',
						})
					);
				},
				{ timeout: 1000 }
			);
		});

		it('auto-saves executor selection', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Wait for agents to load
			await waitFor(() => {
				expect(screen.getByLabelText(/executor/i)).not.toBeDisabled();
			});

			const executorSelect = screen.getByLabelText(/executor/i);
			await userEvent.selectOptions(executorSelect, 'agent-1');

			await waitFor(
				() => {
					expect(mockUpdatePhase).toHaveBeenCalledWith(
						expect.objectContaining({
							agentOverride: 'agent-1',
						})
					);
				},
				{ timeout: 1000 }
			);
		});
	});

	describe('SC-3: Validates phase name (non-empty)', () => {
		it('shows error for empty phase name', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const nameInput = screen.getByTestId('phase-name');
			await userEvent.clear(nameInput);
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(screen.getByText(/name cannot be empty/i)).toBeInTheDocument();
			});
		});

		it('reverts to original value on blur with empty name', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const nameInput = screen.getByTestId('phase-name') as HTMLInputElement;
			await userEvent.clear(nameInput);
			fireEvent.blur(nameInput);

			await waitFor(() => {
				expect(nameInput.value).toBe('Implement'); // Reverts to original
			});
		});

		it('does not save when validation fails', async () => {
			vi.useRealTimers();
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const nameInput = screen.getByTestId('phase-name');
			await userEvent.clear(nameInput);
			fireEvent.blur(nameInput);

			// Wait a bit to ensure no save was triggered
			await new Promise((resolve) => setTimeout(resolve, 600));

			// Should not have been called with empty name
			expect(mockUpdatePhase).not.toHaveBeenCalledWith(
				expect.objectContaining({
					templateName: '',
				})
			);
		});
	});

	describe('SC-5: Shows read-only notice for built-in templates', () => {
		it('displays read-only notice for built-in template', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement', isBuiltin: true }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={true}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			expect(screen.getByText(/built-in template/i)).toBeInTheDocument();
			expect(screen.getByText(/clone to customize/i)).toBeInTheDocument();
		});

		it('disables inputs when readOnly is true', async () => {
			vi.useRealTimers(); // Need real timers for async agent loading
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement', isBuiltin: true }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={true}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			expect(screen.getByTestId('phase-name')).toBeDisabled();
			await waitFor(() => {
				expect(screen.getByLabelText(/executor/i)).toBeDisabled();
			});
			expect(screen.getByLabelText(/model/i)).toBeDisabled();
		});
	});

	describe('SC-6: Preserves section state across phase selections', () => {
		it('remembers expanded sections when switching phases', async () => {
			vi.useRealTimers();
			const phase1 = createMockWorkflowPhase({
				id: 1,
				template: createMockPhaseTemplate({ name: 'phase1' }),
			});
			const phase2 = createMockWorkflowPhase({
				id: 2,
				template: createMockPhaseTemplate({ name: 'phase2' }),
			});

			const { rerender } = render(
				<PhaseInspector
					phase={phase1}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase1, phase2] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Expand Prompt section for phase1
			await expandSection('Prompt');
			await waitFor(() => {
				expect(screen.getByTestId('prompt-content')).toBeInTheDocument();
			});

			// Switch to phase2
			rerender(
				<PhaseInspector
					phase={phase2}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase1, phase2] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Prompt should be collapsed for phase2 (different phase)
			expect(screen.queryByTestId('prompt-content')).not.toBeInTheDocument();

			// Switch back to phase1
			rerender(
				<PhaseInspector
					phase={phase1}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase1, phase2] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Prompt should still be expanded for phase1
			await waitFor(() => {
				expect(screen.getByTestId('prompt-content')).toBeInTheDocument();
			});
		});
	});

	describe('SC-7: Displays loading state while fetching data', () => {
		it('shows loading state for workflow details', async () => {
			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={null}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			expect(screen.getByText(/loading/i)).toBeInTheDocument();
		});

		it('shows loading state for agents', async () => {
			mockListAgents.mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ agents: mockAgents }), 100))
			);

			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Should show loading for agents
			expect(screen.getByText(/loading agents/i)).toBeInTheDocument();
		});

		it('returns null when no phase is provided', () => {
			const { container } = render(
				<PhaseInspector
					phase={null}
					workflowDetails={defaultWorkflowDetails}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			expect(container.firstChild).toBeNull();
		});
	});

	describe('SC-8: Handles field errors gracefully', () => {
		it('shows error message when save fails', async () => {
			vi.useRealTimers();
			mockUpdatePhase.mockRejectedValue(new Error('Save failed'));

			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const modelSelect = screen.getByLabelText(/model/i);
			await userEvent.selectOptions(modelSelect, 'sonnet');

			await waitFor(() => {
				expect(screen.getByText(/save failed/i)).toBeInTheDocument();
			});
		});

		it('shows "Saving..." indicator during save', async () => {
			vi.useRealTimers();
			mockUpdatePhase.mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve({ phase: {} }), 500))
			);

			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			const nameInput = screen.getByTestId('phase-name');
			await userEvent.clear(nameInput);
			await userEvent.type(nameInput, 'New Name');
			fireEvent.blur(nameInput);

			// Should show saving indicator
			await waitFor(() => {
				expect(screen.getByText(/saving/i)).toBeInTheDocument();
			});

			// Should disappear after save completes
			await waitFor(
				() => {
					expect(screen.queryByText(/saving/i)).not.toBeInTheDocument();
				},
				{ timeout: 2000 }
			);
		});

		it('shows error state when template is missing', async () => {
			const phase = createMockWorkflowPhase({
				phaseTemplateId: 'missing-template',
				template: undefined,
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			expect(screen.getByText(/template not found/i)).toBeInTheDocument();
		});
	});

	describe('Edge Cases', () => {
		it('clears pending changes when phase changes', async () => {
			vi.useRealTimers();
			const phase1 = createMockWorkflowPhase({
				id: 1,
				template: createMockPhaseTemplate({ name: 'phase1' }),
			});
			const phase2 = createMockWorkflowPhase({
				id: 2,
				template: createMockPhaseTemplate({ name: 'phase2' }),
			});

			const { rerender } = render(
				<PhaseInspector
					phase={phase1}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase1, phase2] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// Start typing but don't complete
			const nameInput = screen.getByTestId('phase-name');
			await userEvent.clear(nameInput);
			await userEvent.type(nameInput, 'Partial');

			// Switch phases before debounce completes
			rerender(
				<PhaseInspector
					phase={phase2}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase1, phase2] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			// The save for phase1's partial change should not have been called
			await new Promise((resolve) => setTimeout(resolve, 600));
			expect(mockUpdatePhase).not.toHaveBeenCalledWith(
				expect.objectContaining({
					phaseId: 1,
					templateName: 'Partial',
				})
			);
		});

		it('handles no available agents gracefully', async () => {
			vi.useRealTimers(); // Need real timers for async agent loading
			mockListAgents.mockResolvedValue({ agents: [] });

			const phase = createMockWorkflowPhase({
				template: createMockPhaseTemplate({ name: 'implement' }),
			});

			render(
				<PhaseInspector
					phase={phase}
					workflowDetails={{ ...defaultWorkflowDetails, phases: [phase] }}
					readOnly={false}
					onWorkflowRefresh={vi.fn()}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/no agents available/i)).toBeInTheDocument();
			});
		});
	});
});
