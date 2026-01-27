/**
 * TDD Tests for PhaseListEditor
 *
 * Tests for TASK-585: UI: Implement workflow editing - add/edit/remove phases
 *
 * This component is a sub-component of EditWorkflowModal that handles:
 * - Phase list display with sequence numbers
 * - Add phase functionality
 * - Edit phase overrides
 * - Remove phase
 * - Reorder phases
 *
 * Success Criteria Coverage:
 * - SC-3: User can add a phase template to workflow
 * - SC-5: User can edit phase overrides (model, thinking, gate, iterations)
 * - SC-6: User can remove a phase from workflow
 * - SC-7: User can reorder phases using up/down buttons
 * - SC-8: Phases display in sequence order with visual indicators
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { PhaseListEditor } from './PhaseListEditor';
import { GateType } from '@/gen/orc/v1/workflow_pb';
import {
	createMockWorkflowPhase,
	createMockPhaseTemplate,
} from '@/test/factories';
import type { WorkflowPhase, PhaseTemplate } from '@/gen/orc/v1/workflow_pb';

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
	window.confirm = vi.fn().mockReturnValue(true);
});

// Create mock phase templates
const mockPhaseTemplates: PhaseTemplate[] = [
	createMockPhaseTemplate({ id: 'spec', name: 'Spec', description: 'Write specification', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'implement', name: 'Implement', description: 'Implement the feature', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'review', name: 'Review', description: 'Review the code', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'tdd_write', name: 'TDD Write', description: 'Write tests first', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'custom-phase', name: 'Custom Phase', description: 'User-defined phase', isBuiltin: false }),
];

// Create mock phases
function createMockPhases(): WorkflowPhase[] {
	return [
		createMockWorkflowPhase({ id: 1, workflowId: 'test-workflow', phaseTemplateId: 'spec', sequence: 1 }),
		createMockWorkflowPhase({ id: 2, workflowId: 'test-workflow', phaseTemplateId: 'implement', sequence: 2 }),
		createMockWorkflowPhase({ id: 3, workflowId: 'test-workflow', phaseTemplateId: 'review', sequence: 3 }),
	];
}

describe('PhaseListEditor', () => {
	const mockOnAddPhase = vi.fn();
	const mockOnUpdatePhase = vi.fn();
	const mockOnRemovePhase = vi.fn();
	const mockOnReorderPhase = vi.fn();

	const defaultProps = {
		workflowId: 'test-workflow',
		phases: createMockPhases(),
		phaseTemplates: mockPhaseTemplates,
		loading: false,
		onAddPhase: mockOnAddPhase,
		onUpdatePhase: mockOnUpdatePhase,
		onRemovePhase: mockOnRemovePhase,
		onReorderPhase: mockOnReorderPhase,
	};

	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-8: Phase list display', () => {
		it('should display all phases', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(phaseItems).toHaveLength(3);
		});

		it('should display phases in sequence order', () => {
			// Create phases in random order
			const phases = [
				createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 3 }),
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 2 }),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(phaseItems[0]).toHaveTextContent(/spec/i);
			expect(phaseItems[1]).toHaveTextContent(/implement/i);
			expect(phaseItems[2]).toHaveTextContent(/review/i);
		});

		it('should display sequence number badge for each phase', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(within(phaseItems[0]).getByText('1')).toBeInTheDocument();
			expect(within(phaseItems[1]).getByText('2')).toBeInTheDocument();
			expect(within(phaseItems[2]).getByText('3')).toBeInTheDocument();
		});

		it('should display phase template name', () => {
			render(<PhaseListEditor {...defaultProps} />);

			expect(screen.getByText(/spec/i)).toBeInTheDocument();
			expect(screen.getByText(/implement/i)).toBeInTheDocument();
			expect(screen.getByText(/review/i)).toBeInTheDocument();
		});

		it('should display override badges when phase has overrides', () => {
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					modelOverride: 'opus',
					gateTypeOverride: GateType.HUMAN,
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			expect(within(phaseItem).getByText(/opus/i)).toBeInTheDocument();
		});

		it('should show empty state when no phases', () => {
			render(<PhaseListEditor {...defaultProps} phases={[]} />);

			expect(screen.getByText(/no phases|add your first phase/i)).toBeInTheDocument();
		});

		it('should handle gaps in sequence numbers', () => {
			const phases = [
				createMockWorkflowPhase({ id: 1, phaseTemplateId: 'spec', sequence: 1 }),
				createMockWorkflowPhase({ id: 2, phaseTemplateId: 'implement', sequence: 5 }),
				createMockWorkflowPhase({ id: 3, phaseTemplateId: 'review', sequence: 10 }),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			// Should display all phases in order despite gaps
			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(phaseItems).toHaveLength(3);
		});
	});

	describe('SC-3: Add phase', () => {
		it('should show Add Phase button', () => {
			render(<PhaseListEditor {...defaultProps} />);

			expect(screen.getByRole('button', { name: /add phase/i })).toBeInTheDocument();
		});

		it('should open add phase dialog when button clicked', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			// Dialog should show template selector
			expect(await screen.findByLabelText(/phase template/i)).toBeInTheDocument();
		});

		it('should show all available templates in dropdown', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			// All templates should be shown
			expect(await screen.findByRole('option', { name: /spec/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /implement/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /review/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /tdd write/i })).toBeInTheDocument();
			expect(screen.getByRole('option', { name: /custom phase/i })).toBeInTheDocument();
		});

		it('should show template descriptions in dropdown', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			// Descriptions should be visible
			expect(screen.getByText(/write specification/i)).toBeInTheDocument();
			expect(screen.getByText(/implement the feature/i)).toBeInTheDocument();
		});

		it('should call onAddPhase with selected template when confirmed', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			const customOption = await screen.findByRole('option', { name: /custom phase/i });
			await user.click(customOption);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			await user.click(confirmButton);

			expect(mockOnAddPhase).toHaveBeenCalledWith(
				expect.objectContaining({
					phaseTemplateId: 'custom-phase',
					sequence: 4, // Next after existing 3
				})
			);
		});

		it('should close dialog after adding phase', async () => {
			const user = userEvent.setup();
			mockOnAddPhase.mockResolvedValue(undefined);
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			const customOption = await screen.findByRole('option', { name: /custom phase/i });
			await user.click(customOption);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			await user.click(confirmButton);

			await waitFor(() => {
				expect(screen.queryByLabelText(/phase template/i)).not.toBeInTheDocument();
			});
		});

		it('should cancel adding phase when cancel button clicked', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await user.click(cancelButton);

			expect(mockOnAddPhase).not.toHaveBeenCalled();
			expect(screen.queryByLabelText(/phase template/i)).not.toBeInTheDocument();
		});

		it('should disable Add button when no template selected', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			expect(confirmButton).toBeDisabled();
		});

		it('should allow adding duplicate template', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			const templateSelect = await screen.findByLabelText(/phase template/i);
			await user.click(templateSelect);

			// spec already exists, should still be selectable
			const specOption = await screen.findByRole('option', { name: /spec/i });
			await user.click(specOption);

			const confirmButton = screen.getByRole('button', { name: /^add$/i });
			expect(confirmButton).not.toBeDisabled();
		});
	});

	describe('SC-5: Edit phase overrides', () => {
		it('should show edit button for each phase', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			phaseItems.forEach((item) => {
				expect(within(item).getByRole('button', { name: /edit/i })).toBeInTheDocument();
			});
		});

		it('should open edit dialog when edit button clicked', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Edit dialog should appear with override fields
			expect(await screen.findByLabelText(/model/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/thinking/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/gate/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/max iterations/i)).toBeInTheDocument();
		});

		it('should pre-fill edit dialog with current overrides', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					modelOverride: 'opus',
					maxIterationsOverride: 5,
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			await waitFor(() => {
				const modelSelect = screen.getByLabelText(/model/i);
				expect(modelSelect).toHaveTextContent(/opus/i);
			});

			const iterationsInput = screen.getByLabelText(/max iterations/i);
			expect(iterationsInput).toHaveValue(5);
		});

		it('should call onUpdatePhase with new overrides when saved', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Change model
			const modelSelect = await screen.findByLabelText(/model/i);
			await user.click(modelSelect);
			const opusOption = await screen.findByRole('option', { name: /opus/i });
			await user.click(opusOption);

			// Save
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			expect(mockOnUpdatePhase).toHaveBeenCalledWith(
				1, // phase ID
				expect.objectContaining({
					modelOverride: 'opus',
				})
			);
		});

		it('should allow editing thinking override', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			const thinkingCheckbox = await screen.findByLabelText(/thinking/i);
			await user.click(thinkingCheckbox);

			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			expect(mockOnUpdatePhase).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					thinkingOverride: true,
				})
			);
		});

		it('should allow editing gate type override', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			const gateSelect = await screen.findByLabelText(/gate/i);
			await user.click(gateSelect);
			const humanOption = await screen.findByRole('option', { name: /human/i });
			await user.click(humanOption);

			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			expect(mockOnUpdatePhase).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					gateTypeOverride: GateType.HUMAN,
				})
			);
		});

		it('should allow editing max iterations override', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			const iterationsInput = await screen.findByLabelText(/max iterations/i);
			await user.clear(iterationsInput);
			await user.type(iterationsInput, '10');

			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			expect(mockOnUpdatePhase).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					maxIterationsOverride: 10,
				})
			);
		});

		it('should close edit dialog when cancelled', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await user.click(cancelButton);

			expect(mockOnUpdatePhase).not.toHaveBeenCalled();
			expect(screen.queryByLabelText(/model/i)).not.toBeInTheDocument();
		});

		it('should allow clearing overrides', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					modelOverride: 'opus',
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Select "inherit" or "none" option to clear override
			const modelSelect = await screen.findByLabelText(/model/i);
			await user.click(modelSelect);
			const inheritOption = await screen.findByRole('option', { name: /inherit|default|none/i });
			await user.click(inheritOption);

			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			expect(mockOnUpdatePhase).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					modelOverride: undefined,
				})
			);
		});
	});

	describe('SC-6: Remove phase', () => {
		it('should show delete button for each phase', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			phaseItems.forEach((item) => {
				expect(within(item).getByRole('button', { name: /delete|remove/i })).toBeInTheDocument();
			});
		});

		it('should show confirmation dialog when delete clicked', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			expect(window.confirm).toHaveBeenCalled();
		});

		it('should call onRemovePhase when deletion confirmed', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			expect(mockOnRemovePhase).toHaveBeenCalledWith(1); // phase ID
		});

		it('should not call onRemovePhase when deletion cancelled', async () => {
			const user = userEvent.setup();
			vi.mocked(window.confirm).mockReturnValueOnce(false);

			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const deleteButton = within(phaseItems[0]).getByRole('button', { name: /delete|remove/i });
			await user.click(deleteButton);

			expect(mockOnRemovePhase).not.toHaveBeenCalled();
		});
	});

	describe('SC-7: Reorder phases', () => {
		it('should show move up/down buttons for phases', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			// At least middle phase should have both buttons
			expect(within(phaseItems[1]).getByRole('button', { name: /move up/i })).toBeInTheDocument();
			expect(within(phaseItems[1]).getByRole('button', { name: /move down/i })).toBeInTheDocument();
		});

		it('should disable move up button on first phase', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const upButton = within(phaseItems[0]).getByRole('button', { name: /move up/i });
			expect(upButton).toBeDisabled();
		});

		it('should disable move down button on last phase', () => {
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const downButton = within(phaseItems[2]).getByRole('button', { name: /move down/i });
			expect(downButton).toBeDisabled();
		});

		it('should call onReorderPhase when moving up', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const upButton = within(phaseItems[1]).getByRole('button', { name: /move up/i });
			await user.click(upButton);

			// Should swap phases 1 and 2
			expect(mockOnReorderPhase).toHaveBeenCalledWith(2, 'up');
		});

		it('should call onReorderPhase when moving down', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const downButton = within(phaseItems[1]).getByRole('button', { name: /move down/i });
			await user.click(downButton);

			// Should swap phases 2 and 3
			expect(mockOnReorderPhase).toHaveBeenCalledWith(2, 'down');
		});

		it('should not show reorder buttons when only one phase', () => {
			const phases = [createMockWorkflowPhase({ id: 1, phaseTemplateId: 'implement', sequence: 1 })];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			// Both buttons should be disabled or not shown
			const upButton = within(phaseItem).queryByRole('button', { name: /move up/i });
			const downButton = within(phaseItem).queryByRole('button', { name: /move down/i });

			if (upButton) expect(upButton).toBeDisabled();
			if (downButton) expect(downButton).toBeDisabled();
		});
	});

	describe('Loading state', () => {
		it('should show loading indicator when loading', () => {
			render(<PhaseListEditor {...defaultProps} loading={true} />);

			expect(screen.getByText(/loading/i)).toBeInTheDocument();
		});

		it('should disable add button when loading', () => {
			render(<PhaseListEditor {...defaultProps} loading={true} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			expect(addButton).toBeDisabled();
		});

		it('should disable edit buttons when loading', () => {
			render(<PhaseListEditor {...defaultProps} loading={true} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			phaseItems.forEach((item) => {
				const editButton = within(item).getByRole('button', { name: /edit/i });
				expect(editButton).toBeDisabled();
			});
		});

		it('should disable delete buttons when loading', () => {
			render(<PhaseListEditor {...defaultProps} loading={true} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			phaseItems.forEach((item) => {
				const deleteButton = within(item).getByRole('button', { name: /delete|remove/i });
				expect(deleteButton).toBeDisabled();
			});
		});

		it('should disable reorder buttons when loading', () => {
			render(<PhaseListEditor {...defaultProps} loading={true} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const upButton = within(phaseItems[1]).getByRole('button', { name: /move up/i });
			const downButton = within(phaseItems[1]).getByRole('button', { name: /move down/i });
			expect(upButton).toBeDisabled();
			expect(downButton).toBeDisabled();
		});
	});

	describe('Edge cases', () => {
		it('should handle many phases (10+)', () => {
			const phases = Array.from({ length: 12 }, (_, i) =>
				createMockWorkflowPhase({
					id: i + 1,
					phaseTemplateId: `phase-${i + 1}`,
					sequence: i + 1,
				})
			);

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			expect(phaseItems).toHaveLength(12);
		});

		it('should show appropriate text for phase without matching template', () => {
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'unknown-template',
					sequence: 1,
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			// Should show template ID or "unknown"
			expect(phaseItem).toHaveTextContent(/unknown-template|unknown/i);
		});

		it('should handle empty template list', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} phaseTemplates={[]} />);

			const addButton = screen.getByRole('button', { name: /add phase/i });
			await user.click(addButton);

			// Should show empty state or message
			expect(screen.getByText(/no templates available|no phases available/i)).toBeInTheDocument();
		});

		it('should handle phase with all overrides set', () => {
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

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			// Should display override badges
			expect(within(phaseItem).getByText(/opus/i)).toBeInTheDocument();
		});
	});
});
