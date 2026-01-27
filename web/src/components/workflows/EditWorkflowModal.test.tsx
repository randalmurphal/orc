/**
 * TDD Tests for EditWorkflowModal
 *
 * Tests for TASK-585: UI: Implement workflow editing - add/edit/remove phases
 *
 * Success Criteria Coverage:
 * - SC-1: User can open Edit Workflow modal from workflow detail panel (custom workflows only)
 * - SC-2: User can save workflow metadata changes
 * - SC-3: User can add a phase template to workflow
 * - SC-4: Phase template selector shows all available templates with descriptions
 * - SC-5: User can edit phase overrides (model, thinking, gate, iterations)
 * - SC-6: User can remove a phase from workflow
 * - SC-7: User can reorder phases using up/down buttons
 * - SC-8: Phases display in sequence order with visual indicators
 * - SC-9: Built-in workflows cannot be edited (read-only)
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EditWorkflowModal } from './EditWorkflowModal';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockWorkflowPhase,
	createMockPhaseTemplate,
	createMockGetWorkflowResponse,
	createMockListPhaseTemplatesResponse,
	createMockUpdateWorkflowResponse,
	createMockAddPhaseResponse,
	createMockUpdatePhaseResponse,
	createMockRemovePhaseResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getWorkflow: vi.fn(),
		updateWorkflow: vi.fn(),
		addPhase: vi.fn(),
		updatePhase: vi.fn(),
		removePhase: vi.fn(),
		listPhaseTemplates: vi.fn(),
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
import { workflowClient } from '@/lib/client';
import { toast } from '@/stores/uiStore';

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
	// Mock window.confirm for delete confirmations
	window.confirm = vi.fn().mockReturnValue(true);
});

// Create mock phase templates
const mockPhaseTemplates = [
	createMockPhaseTemplate({ id: 'spec', name: 'Spec', description: 'Write specification', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'implement', name: 'Implement', description: 'Implement the feature', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'review', name: 'Review', description: 'Review the code', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'custom-phase', name: 'Custom Phase', description: 'User-defined phase', isBuiltin: false }),
];

// Create mock workflow with phases
function createCustomWorkflowWithPhases() {
	const workflow = createMockWorkflow({
		id: 'my-custom-workflow',
		name: 'My Custom Workflow',
		description: 'A custom workflow',
		isBuiltin: false,
		defaultModel: 'sonnet',
		defaultThinking: true,
	});
	const phases = [
		createMockWorkflowPhase({ id: 1, workflowId: 'my-custom-workflow', phaseTemplateId: 'spec', sequence: 1 }),
		createMockWorkflowPhase({ id: 2, workflowId: 'my-custom-workflow', phaseTemplateId: 'implement', sequence: 2 }),
		createMockWorkflowPhase({ id: 3, workflowId: 'my-custom-workflow', phaseTemplateId: 'review', sequence: 3 }),
	];
	return createMockWorkflowWithDetails({ workflow, phases, variables: [] });
}

function createBuiltinWorkflow() {
	return createMockWorkflow({
		id: 'medium',
		name: 'Medium',
		description: 'For features needing thought',
		isBuiltin: true,
	});
}

describe('EditWorkflowModal', () => {
	const mockOnClose = vi.fn();
	const mockOnUpdated = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		// Default mocks
		vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
			createMockListPhaseTemplatesResponse(mockPhaseTemplates)
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Open Edit Workflow modal from workflow detail panel', () => {
		it('should render modal when open is true with custom workflow', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Modal should be visible
			expect(screen.getByRole('dialog')).toBeInTheDocument();
			expect(screen.getByText(/edit workflow/i)).toBeInTheDocument();
		});

		it('should not render modal when open is false', () => {
			const workflowDetails = createCustomWorkflowWithPhases();

			render(
				<EditWorkflowModal
					open={false}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
		});

		it('should load workflow details when modal opens', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalledWith({
					id: 'my-custom-workflow',
				});
			});
		});

		it('should pre-fill form with current workflow metadata', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Check form fields are pre-filled
			expect(screen.getByLabelText(/name/i)).toHaveValue('My Custom Workflow');
			expect(screen.getByLabelText(/description/i)).toHaveValue('A custom workflow');
		});
	});

	describe('SC-2: Save workflow metadata changes', () => {
		it('should call updateWorkflow API with changed metadata', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.updateWorkflow).mockResolvedValue(
				createMockUpdateWorkflowResponse(createMockWorkflow({
					...workflowDetails.workflow!,
					name: 'Updated Name',
				}))
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Change name
			const nameInput = screen.getByLabelText(/name/i);
			await user.clear(nameInput);
			await user.type(nameInput, 'Updated Name');

			// Save
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updateWorkflow).toHaveBeenCalledWith(
					expect.objectContaining({
						id: 'my-custom-workflow',
						name: 'Updated Name',
					})
				);
			});
		});

		it('should show success toast on successful save', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.updateWorkflow).mockResolvedValue(
				createMockUpdateWorkflowResponse(workflowDetails.workflow!)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(toast.success).toHaveBeenCalled();
			});
		});

		it('should show error toast on API failure', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.updateWorkflow).mockRejectedValue(
				new Error('Failed to update workflow')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to update workflow'));
			});
		});

		it('should keep form open on API failure', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.updateWorkflow).mockRejectedValue(
				new Error('Failed to update workflow')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalled();
			});

			// Modal should still be open
			expect(screen.getByRole('dialog')).toBeInTheDocument();
			expect(mockOnClose).not.toHaveBeenCalled();
		});

		it('should call onUpdated callback after successful save', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			const updatedWorkflow = createMockWorkflow({
				...workflowDetails.workflow!,
				name: 'Updated Name',
			});
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.updateWorkflow).mockResolvedValue(
				createMockUpdateWorkflowResponse(updatedWorkflow)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(mockOnUpdated).toHaveBeenCalled();
			});
		});
	});

	describe('SC-3: Add a phase template to workflow', () => {
		it('should show Add Phase button in phases section', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			expect(screen.getByRole('button', { name: /add phase/i })).toBeInTheDocument();
		});

		it('should call addPhase API when adding a phase', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			const newPhase = createMockWorkflowPhase({
				id: 4,
				workflowId: 'my-custom-workflow',
				phaseTemplateId: 'custom-phase',
				sequence: 4,
			});
			vi.mocked(workflowClient.addPhase).mockResolvedValue(
				createMockAddPhaseResponse(newPhase)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Click Add Phase button
			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			// Select a template from dropdown
			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);
			const customOption = await screen.findByRole('option', { name: /custom phase/i });
			await user.click(customOption);

			// Confirm add
			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			await user.click(confirmButton);

			await waitFor(() => {
				expect(workflowClient.addPhase).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'my-custom-workflow',
						phaseTemplateId: 'custom-phase',
						sequence: 4, // Next sequence after existing 3 phases
					})
				);
			});
		});

		it('should update phase list after adding a phase', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			const newPhase = createMockWorkflowPhase({
				id: 4,
				workflowId: 'my-custom-workflow',
				phaseTemplateId: 'custom-phase',
				sequence: 4,
			});
			vi.mocked(workflowClient.addPhase).mockResolvedValue(
				createMockAddPhaseResponse(newPhase)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Initially 3 phases
			const initialPhaseItems = screen.getAllByTestId(/phase-item/);
			expect(initialPhaseItems).toHaveLength(3);

			// Add phase
			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);
			const customOption = await screen.findByRole('option', { name: /custom phase/i });
			await user.click(customOption);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			await user.click(confirmButton);

			await waitFor(() => {
				const phaseItems = screen.getAllByTestId(/phase-item/);
				expect(phaseItems).toHaveLength(4);
			});
		});

		it('should show error toast when adding phase fails', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.addPhase).mockRejectedValue(
				new Error('Failed to add phase')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);
			const customOption = await screen.findByRole('option', { name: /custom phase/i });
			await user.click(customOption);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			await user.click(confirmButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to add phase'));
			});
		});
	});

	describe('SC-4: Phase template selector shows all available templates', () => {
		it('should fetch and display all phase templates', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listPhaseTemplates).toHaveBeenCalledWith({
					includeBuiltin: true,
				});
			});

			// Open add phase dialog
			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			// Open template dropdown
			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			// All templates should be visible
			expect(await screen.findByRole('option', { name: /spec/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /implement/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /review/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /custom phase/i })).toBeInTheDocument();
		});

		it('should show template descriptions in dropdown', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			// Descriptions should be visible
			expect(screen.getByText(/write specification/i)).toBeInTheDocument();
			expect(screen.getByText(/implement the feature/i)).toBeInTheDocument();
		});

		it('should show loading state while fetching templates', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			// Delay template loading
			vi.mocked(workflowClient.listPhaseTemplates).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(createMockListPhaseTemplatesResponse(mockPhaseTemplates)), 100))
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Check for loading indicator
			expect(screen.getByText(/loading/i)).toBeInTheDocument();

			await waitFor(() => {
				expect(screen.queryByText(/loading templates/i)).not.toBeInTheDocument();
			});
		});

		it('should show error state when template fetch fails', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.listPhaseTemplates).mockRejectedValue(
				new Error('Network error')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
			});
		});
	});

	describe('SC-5: Edit phase overrides', () => {
		it('should show edit button for each phase', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Each phase should have edit button
			const phaseItems = screen.getAllByTestId(/phase-item/);
			phaseItems.forEach((item) => {
				expect(within(item).getByRole('button', { name: /edit/i })).toBeInTheDocument();
			});
		});

		it('should call updatePhase API with overrides', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			const updatedPhase = createMockWorkflowPhase({
				...workflowDetails.phases[0],
				modelOverride: 'opus',
				maxIterationsOverride: 5,
			});
			vi.mocked(workflowClient.updatePhase).mockResolvedValue(
				createMockUpdatePhaseResponse(updatedPhase)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Click edit on first phase
			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Edit model override
			const modelSelect = await screen.findByLabelText(/model/i);
			await user.click(modelSelect);
			const opusOption = await screen.findByRole('option', { name: /opus/i });
			await user.click(opusOption);

			// Edit max iterations
			const iterationsInput = screen.getByLabelText(/max iterations/i);
			await user.clear(iterationsInput);
			await user.type(iterationsInput, '5');

			// Save
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'my-custom-workflow',
						phaseId: 1,
						modelOverride: 'opus',
						maxIterationsOverride: 5,
					})
				);
			});
		});

		it('should allow editing thinking override', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			const updatedPhase = createMockWorkflowPhase({
				...workflowDetails.phases[0],
				thinkingOverride: true,
			});
			vi.mocked(workflowClient.updatePhase).mockResolvedValue(
				createMockUpdatePhaseResponse(updatedPhase)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Toggle thinking
			const thinkingCheckbox = await screen.findByLabelText(/thinking/i);
			await user.click(thinkingCheckbox);

			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						thinkingOverride: true,
					})
				);
			});
		});

		it('should allow editing gate type override', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			const updatedPhase = createMockWorkflowPhase({
				...workflowDetails.phases[0],
				gateTypeOverride: GateType.HUMAN,
			});
			vi.mocked(workflowClient.updatePhase).mockResolvedValue(
				createMockUpdatePhaseResponse(updatedPhase)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Change gate type
			const gateSelect = await screen.findByLabelText(/gate/i);
			await user.click(gateSelect);
			const humanOption = await screen.findByRole('option', { name: /human/i });
			await user.click(humanOption);

			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(workflowClient.updatePhase).toHaveBeenCalledWith(
					expect.objectContaining({
						gateTypeOverride: GateType.HUMAN,
					})
				);
			});
		});
	});

	describe('SC-6: Remove a phase from workflow', () => {
		it('should show delete button for each phase', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			phaseItems.forEach((item) => {
				expect(within(item).getByRole('button', { name: /delete|remove/i })).toBeInTheDocument();
			});
		});

		it('should show confirmation dialog before deleting', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			// Confirmation should be shown
			expect(window.confirm).toHaveBeenCalled();
		});

		it('should call removePhase API when confirmed', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.removePhase).mockResolvedValue(
				createMockRemovePhaseResponse(workflowDetails.workflow!)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			await waitFor(() => {
				expect(workflowClient.removePhase).toHaveBeenCalledWith({
					workflowId: 'my-custom-workflow',
					phaseId: 1,
				});
			});
		});

		it('should update phase list after removing a phase', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.removePhase).mockResolvedValue(
				createMockRemovePhaseResponse(workflowDetails.workflow!)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Initially 3 phases
			expect(screen.getAllByTestId(/phase-item/)).toHaveLength(3);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			await waitFor(() => {
				expect(screen.getAllByTestId(/phase-item/)).toHaveLength(2);
			});
		});

		it('should show error toast when removal fails', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.removePhase).mockRejectedValue(
				new Error('Failed to remove phase')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to remove phase'));
			});
		});

		it('should not call API when confirmation is cancelled', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			// Cancel confirmation
			vi.mocked(window.confirm).mockReturnValueOnce(false);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			expect(workflowClient.removePhase).not.toHaveBeenCalled();
		});
	});

	describe('SC-7: Reorder phases using up/down buttons', () => {
		it('should show move up/down buttons for phases', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			// Middle phase should have both up and down
			expect(within(phaseItems[1]).getByRole('button', { name: /move up/i })).toBeInTheDocument();
			expect(within(phaseItems[1]).getByRole('button', { name: /move down/i })).toBeInTheDocument();
		});

		it('should disable move up button on first phase', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const upButton = within(phaseItems[0]).getByRole('button', { name: /move up/i });
			expect(upButton).toBeDisabled();
		});

		it('should disable move down button on last phase', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const downButton = within(phaseItems[2]).getByRole('button', { name: /move down/i });
			expect(downButton).toBeDisabled();
		});

		it('should call updatePhase API to swap sequences when moving up', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.updatePhase).mockResolvedValue(
				createMockUpdatePhaseResponse(createMockWorkflowPhase())
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			// Move second phase up
			const upButton = within(phaseItems[1]).getByRole('button', { name: /move up/i });
			await user.click(upButton);

			await waitFor(() => {
				// Should update sequences for both swapped phases
				expect(workflowClient.updatePhase).toHaveBeenCalled();
			});
		});

		it('should update UI after reordering', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			// Return updated phases with swapped sequences
			vi.mocked(workflowClient.updatePhase)
				.mockResolvedValueOnce(createMockUpdatePhaseResponse(createMockWorkflowPhase({ id: 2, sequence: 1 })))
				.mockResolvedValueOnce(createMockUpdatePhaseResponse(createMockWorkflowPhase({ id: 1, sequence: 2 })));

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Get initial order
			const initialPhaseItems = screen.getAllByTestId(/phase-item/);
			const initialFirstPhaseText = initialPhaseItems[0].textContent;

			// Move second phase up
			const upButton = within(initialPhaseItems[1]).getByRole('button', { name: /move up/i });
			await user.click(upButton);

			await waitFor(() => {
				const updatedPhaseItems = screen.getAllByTestId(/phase-item/);
				// Order should be changed
				expect(updatedPhaseItems[0].textContent).not.toBe(initialFirstPhaseText);
			});
		});

		it('should not make API call when reordering to same position', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Try to move first phase up (should be disabled/no-op)
			const phaseItems = screen.getAllByTestId(/phase-item/);
			const upButton = within(phaseItems[0]).getByRole('button', { name: /move up/i });

			// Button should be disabled, click should not trigger API
			expect(upButton).toBeDisabled();
			await user.click(upButton);

			expect(workflowClient.updatePhase).not.toHaveBeenCalled();
		});
	});

	describe('SC-8: Phases display in sequence order with visual indicators', () => {
		it('should display phases sorted by sequence number', async () => {
			// Create workflow with out-of-order phases
			const workflow = createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
			});
			const phases = [
				createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];
			const workflowDetails = createMockWorkflowWithDetails({ workflow, phases, variables: [] });
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			// Should be in sequence order: spec (1), implement (2), review (3)
			expect(phaseItems[0]).toHaveTextContent(/spec/i);
			expect(phaseItems[1]).toHaveTextContent(/implement/i);
			expect(phaseItems[2]).toHaveTextContent(/review/i);
		});

		it('should display sequence numbers as visual indicators', async () => {
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItems = screen.getAllByTestId(/phase-item/);
			// Each phase should show its sequence number
			expect(within(phaseItems[0]).getByText('1')).toBeInTheDocument();
			expect(within(phaseItems[1]).getByText('2')).toBeInTheDocument();
			expect(within(phaseItems[2]).getByText('3')).toBeInTheDocument();
		});

		it('should handle gaps in sequence numbers gracefully', async () => {
			const workflow = createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
			});
			// Sequence numbers with gaps
			const phases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 5 }), // gap
				createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 10 }), // gap
			];
			const workflowDetails = createMockWorkflowWithDetails({ workflow, phases, variables: [] });
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Should display all 3 phases in order
			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(phaseItems).toHaveLength(3);
			expect(phaseItems[0]).toHaveTextContent(/spec/i);
			expect(phaseItems[1]).toHaveTextContent(/implement/i);
			expect(phaseItems[2]).toHaveTextContent(/review/i);
		});
	});

	describe('SC-9: Built-in workflows cannot be edited', () => {
		it('should not render EditWorkflowModal for built-in workflows', async () => {
			const builtinWorkflow = createBuiltinWorkflow();

			// The component should not render or show a message
			render(
				<EditWorkflowModal
					open={true}
					workflow={builtinWorkflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Should show message that built-in cannot be edited
			expect(screen.getByText(/cannot edit built-in workflow|clone to customize/i)).toBeInTheDocument();
		});

		it('should not call getWorkflow API for built-in workflows', async () => {
			const builtinWorkflow = createBuiltinWorkflow();

			render(
				<EditWorkflowModal
					open={true}
					workflow={builtinWorkflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Should not try to load details for built-in
			expect(workflowClient.getWorkflow).not.toHaveBeenCalled();
		});

		it('should suggest cloning for built-in workflows', async () => {
			const builtinWorkflow = createBuiltinWorkflow();

			render(
				<EditWorkflowModal
					open={true}
					workflow={builtinWorkflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			// Should have a clone suggestion or button
			expect(screen.getByText(/clone/i)).toBeInTheDocument();
		});
	});

	describe('Edge Cases', () => {
		it('should show empty state when workflow has 0 phases', async () => {
			const workflow = createMockWorkflow({
				id: 'empty-workflow',
				name: 'Empty',
				isBuiltin: false,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow,
				phases: [],
				variables: [],
			});
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Should show empty state message
			expect(screen.getByText(/add your first phase|no phases/i)).toBeInTheDocument();
		});

		it('should handle workflow with 10+ phases', async () => {
			const workflow = createMockWorkflow({
				id: 'large-workflow',
				name: 'Large',
				isBuiltin: false,
			});
			const phases = Array.from({ length: 12 }, (_, i) =>
				createMockWorkflowPhase({
					id: i + 1,
					phaseTemplateId: `phase-${i + 1}`,
					sequence: i + 1,
				})
			);
			const workflowDetails = createMockWorkflowWithDetails({
				workflow,
				phases,
				variables: [],
			});
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// All phases should be rendered
			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(phaseItems).toHaveLength(12);
		});

		it('should allow adding duplicate phase template', async () => {
			const user = userEvent.setup();
			const workflowDetails = createCustomWorkflowWithPhases();
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			// spec already exists, but should allow adding another
			const newPhase = createMockWorkflowPhase({
				id: 4,
				workflowId: 'my-custom-workflow',
				phaseTemplateId: 'spec', // duplicate
				sequence: 4,
			});
			vi.mocked(workflowClient.addPhase).mockResolvedValue(
				createMockAddPhaseResponse(newPhase)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflowDetails.workflow!}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);
			const specOption = await screen.findByRole('option', { name: /spec/i });
			await user.click(specOption);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			await user.click(confirmButton);

			await waitFor(() => {
				expect(workflowClient.addPhase).toHaveBeenCalledWith(
					expect.objectContaining({
						phaseTemplateId: 'spec',
					})
				);
			});
		});

		it('should show empty state after removing last phase', async () => {
			const user = userEvent.setup();
			const workflow = createMockWorkflow({
				id: 'single-phase',
				name: 'Single',
				isBuiltin: false,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow,
				phases: [createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 })],
				variables: [],
			});
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);
			vi.mocked(workflowClient.removePhase).mockResolvedValue(
				createMockRemovePhaseResponse(workflow)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			// Remove the only phase
			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			await waitFor(() => {
				expect(screen.getByText(/add your first phase|no phases/i)).toBeInTheDocument();
			});
		});

		it('should display phase overrides as badges', async () => {
			const workflow = createMockWorkflow({
				id: 'override-workflow',
				name: 'Override Test',
				isBuiltin: false,
			});
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					modelOverride: 'opus',
					thinkingOverride: true,
					gateTypeOverride: GateType.HUMAN,
					maxIterationsOverride: 5,
				}),
			];
			const workflowDetails = createMockWorkflowWithDetails({
				workflow,
				phases,
				variables: [],
			});
			vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
				createMockGetWorkflowResponse(workflowDetails)
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalled();
			});

			const phaseItem = screen.getByTestId(/phase-item/);
			// Should show override badges
			expect(within(phaseItem).getByText(/opus/i)).toBeInTheDocument();
		});
	});

	describe('Error Handling', () => {
		it('should show error state when workflow details fail to load', async () => {
			const workflow = createMockWorkflow({
				id: 'error-workflow',
				name: 'Error',
				isBuiltin: false,
			});
			vi.mocked(workflowClient.getWorkflow).mockRejectedValue(
				new Error('Failed to load workflow')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load workflow/i)).toBeInTheDocument();
			});
		});

		it('should provide retry button on load failure', async () => {
			const user = userEvent.setup();
			const workflow = createMockWorkflow({
				id: 'retry-workflow',
				name: 'Retry',
				isBuiltin: false,
			});
			const workflowDetails = createMockWorkflowWithDetails({
				workflow,
				phases: [],
				variables: [],
			});
			// First call fails, second succeeds
			vi.mocked(workflowClient.getWorkflow)
				.mockRejectedValueOnce(new Error('Network error'))
				.mockResolvedValueOnce(createMockGetWorkflowResponse(workflowDetails));

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load/i)).toBeInTheDocument();
			});

			const retryButton = screen.getByRole('button', { name: /retry/i });
			await user.click(retryButton);

			await waitFor(() => {
				expect(workflowClient.getWorkflow).toHaveBeenCalledTimes(2);
			});
		});

		it('should handle network timeout gracefully', async () => {
			const workflow = createMockWorkflow({
				id: 'timeout-workflow',
				name: 'Timeout',
				isBuiltin: false,
			});
			vi.mocked(workflowClient.getWorkflow).mockRejectedValue(
				new Error('Request timed out')
			);

			render(
				<EditWorkflowModal
					open={true}
					workflow={workflow}
					onClose={mockOnClose}
					onUpdated={mockOnUpdated}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/timed out|failed/i)).toBeInTheDocument();
			});
		});
	});
});
