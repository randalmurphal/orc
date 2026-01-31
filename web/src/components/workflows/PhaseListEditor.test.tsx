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

	// ─── TASK-670: Claude Config Override Sections ─────────────────────────────

	describe('SC-1: Claude config collapsible sections in edit dialog', () => {
		it('should render 7 collapsible claude_config sections in edit dialog', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// 7 collapsible sections should be visible below the existing 4 overrides
			expect(await screen.findByText(/hooks/i)).toBeInTheDocument();
			expect(screen.getByText(/mcp servers/i)).toBeInTheDocument();
			expect(screen.getByText(/skills/i)).toBeInTheDocument();
			expect(screen.getByText(/^Allowed Tools$/i)).toBeInTheDocument();
			expect(screen.getByText(/^Disallowed Tools$/i)).toBeInTheDocument();
			expect(screen.getByText(/env vars/i)).toBeInTheDocument();
			expect(screen.getByText(/json override/i)).toBeInTheDocument();
		});

		it('should render sections as collapsible (initially collapsed)', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Sections should use CollapsibleSettingsSection component
			// which has a data-testid or specific class pattern
			const sections = await screen.findAllByTestId(/collapsible-section/);
			expect(sections.length).toBeGreaterThanOrEqual(7);
		});
	});

	describe('SC-2: Editor types for claude_config sections', () => {
		it('should render LibraryPicker for hooks section', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Should show a library picker for hooks
			expect(screen.getByTestId(/library-picker-hooks|hooks-picker/)).toBeInTheDocument();
		});

		it('should render TagInput for allowed tools section', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand allowed tools section
			const toolsHeader = await screen.findByText(/^Allowed Tools$/i);
			await user.click(toolsHeader);

			// Should show a tag input for tools
			expect(screen.getByTestId(/tag-input-allowed-tools|allowed-tools-input/)).toBeInTheDocument();
		});

		it('should render KeyValueEditor for env vars section', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand env vars section
			const envHeader = await screen.findByText(/env vars/i);
			await user.click(envHeader);

			// Should show a key-value editor for env vars
			expect(screen.getByTestId(/key-value-editor-env|env-editor/)).toBeInTheDocument();
		});

		it('should render textarea for JSON override section', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand JSON override section
			const jsonHeader = await screen.findByText(/json override/i);
			await user.click(jsonHeader);

			// Should show a textarea for raw JSON editing
			expect(screen.getByRole('textbox', { name: /json override/i })).toBeInTheDocument();
		});
	});

	describe('SC-3: Save serializes claudeConfigOverride', () => {
		it('should include claudeConfigOverride in onUpdatePhase call when overrides added (BDD-2)', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section and add a hook override
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Add a hook (interact with the library picker)
			const hooksPicker = screen.getByTestId(/library-picker-hooks|hooks-picker/);
			expect(hooksPicker).toBeInTheDocument();
			// The specific interaction depends on LibraryPicker API,
			// but we simulate selecting a hook item
			const addHookButton = within(hooksPicker).getByRole('button', { name: /add|select/i });
			await user.click(addHookButton);

			// Save
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			expect(mockOnUpdatePhase).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					claudeConfigOverride: expect.stringContaining('hooks'),
				}),
			);
		});

		it('should omit claudeConfigOverride when no claude_config sections changed', async () => {
			const user = userEvent.setup();
			render(<PhaseListEditor {...defaultProps} />);

			const phaseItems = screen.getAllByTestId(/phase-item/);
			const editButton = within(phaseItems[0]).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Save without changing any claude_config sections
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			// claudeConfigOverride should be undefined or empty string when no overrides
			const call = mockOnUpdatePhase.mock.calls[0];
			const overrides = call[1];
			expect(
				overrides.claudeConfigOverride === undefined ||
				overrides.claudeConfigOverride === '' ||
				overrides.claudeConfigOverride === '{}'
			).toBe(true);
		});

		it('should produce valid JSON in claudeConfigOverride', async () => {
			const user = userEvent.setup();
			// Create a phase that already has claude_config_override
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks":["existing-hook"],"allowed_tools":["Bash"]}',
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Save with existing overrides
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			const call = mockOnUpdatePhase.mock.calls[0];
			const overrides = call[1];
			if (overrides.claudeConfigOverride) {
				// Should be valid JSON
				expect(() => JSON.parse(overrides.claudeConfigOverride)).not.toThrow();
			}
		});
	});

	describe('SC-5: Inherited vs override visual distinction (BDD-1)', () => {
		it('should show inherited items dimmed with "inherited" badge', async () => {
			const user = userEvent.setup();
			// Phase with template that has claude_config
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["lint-hook"], "env": {"NODE_ENV": "test"}}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Inherited hooks should show with inherited styling
			const inheritedItem = screen.getByText('lint-hook');
			expect(inheritedItem.closest('[class*="inherited"]')).toBeTruthy();

			// Should have "inherited" badge
			expect(screen.getByText(/inherited/i)).toBeInTheDocument();
		});

		it('should show override items with "override" badge', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["lint-hook"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Override hook should have override styling
			const overrideItem = screen.getByText('my-hook');
			expect(overrideItem.closest('[class*="override"]')).toBeTruthy();
		});

		it('should show all items as override when template has no claude_config', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						// No claudeConfig on template
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// No inherited badge should appear
			expect(screen.queryByText(/inherited/i)).not.toBeInTheDocument();
		});
	});

	describe('SC-6: Section badge counts with inherited/override breakdown', () => {
		it('should show badge count with inherited/override breakdown', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["lint-hook"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Hooks section badge should show "2 — 1 inherited, 1 override"
			expect(screen.getByText(/2.*1 inherited.*1 override/i)).toBeInTheDocument();
		});

		it('should show "0" badge when section is empty', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						// No claude_config
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Look for a section badge showing 0
			// The hooks section header should show "0" or similar
			const hooksHeader = await screen.findByText(/hooks/i);
			const section = hooksHeader.closest('[data-testid*="collapsible-section"]');
			expect(section).toBeTruthy();
			expect(within(section! as HTMLElement).getByText(/\b0\b/)).toBeInTheDocument();
		});

		it('should show all inherited when no overrides exist', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["hook-a", "hook-b"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Should show "2 inherited" (no override count)
			expect(screen.getByText(/2 inherited/i)).toBeInTheDocument();
		});

		it('should show all override when no inherited exist', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["hook-a", "hook-b"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						// No claudeConfig on template
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Should show "2 override" (no inherited count)
			expect(screen.getByText(/2 override/i)).toBeInTheDocument();
		});
	});

	describe('SC-7: Clear override button per section (BDD-3)', () => {
		it('should show clear override button when section has overrides', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["lint-hook"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Clear override button should be visible
			expect(screen.getByRole('button', { name: /clear override/i })).toBeInTheDocument();
		});

		it('should reset override items when clear override clicked', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["lint-hook"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Verify override hook exists
			expect(screen.getByText('my-hook')).toBeInTheDocument();

			// Click clear override
			const clearButton = screen.getByRole('button', { name: /clear override/i });
			await user.click(clearButton);

			// Override hook should be removed, only inherited remains
			expect(screen.queryByText('my-hook')).not.toBeInTheDocument();
			expect(screen.getByText('lint-hook')).toBeInTheDocument();
		});

		it('should exclude cleared section from save JSON', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"], "allowed_tools": ["Bash"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section and clear
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			const clearButton = screen.getByRole('button', { name: /clear override/i });
			await user.click(clearButton);

			// Save
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			const call = mockOnUpdatePhase.mock.calls[0];
			const overrides = call[1];

			// The saved JSON should not contain hooks key, but should still have allowed_tools
			if (overrides.claudeConfigOverride) {
				const parsed = JSON.parse(overrides.claudeConfigOverride);
				expect(parsed.hooks).toBeUndefined();
				expect(parsed.allowed_tools).toEqual(['Bash']);
			}
		});

		it('should hide or disable clear button when section has no overrides', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					// No claude_config_override
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["lint-hook"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Clear button should be disabled or not present
			const clearButton = screen.queryByRole('button', { name: /clear override/i });
			if (clearButton) {
				expect(clearButton).toBeDisabled();
			} else {
				expect(clearButton).toBeNull();
			}
		});
	});

	describe('SC-8: PhaseOverrides interface extension', () => {
		it('should propagate claudeConfigOverride through onUpdatePhase', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"env": {"KEY": "value"}}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Save with existing overrides
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			// The second argument should have claudeConfigOverride as a string field
			const call = mockOnUpdatePhase.mock.calls[0];
			const overrides = call[1];
			expect('claudeConfigOverride' in overrides).toBe(true);
			expect(typeof overrides.claudeConfigOverride === 'string' || overrides.claudeConfigOverride === undefined).toBe(true);
		});
	});

	describe('Failure modes: Claude config sections', () => {
		it('should handle template with no claude_config gracefully', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						// No claudeConfig
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// All sections should render without error, showing 0 inherited items
			expect(await screen.findByText(/hooks/i)).toBeInTheDocument();
			expect(screen.getByText(/env vars/i)).toBeInTheDocument();
		});

		it('should handle phase with no template nested data', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					// No template field
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Should render as override-only mode
			expect(await screen.findByText(/hooks/i)).toBeInTheDocument();
		});

		it('should clear claudeConfigOverride when all sections cleared (save with empty overrides)', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["my-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Clear the hooks override
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);
			const clearButton = screen.getByRole('button', { name: /clear override/i });
			await user.click(clearButton);

			// Save
			const saveButton = screen.getByRole('button', { name: /save phase/i });
			await user.click(saveButton);

			const call = mockOnUpdatePhase.mock.calls[0];
			const overrides = call[1];
			// claudeConfigOverride should be empty/undefined when all cleared
			expect(
				overrides.claudeConfigOverride === undefined ||
				overrides.claudeConfigOverride === '' ||
				overrides.claudeConfigOverride === '{}'
			).toBe(true);
		});
	});

	describe('Edge cases: Claude config sections', () => {
		it('should handle duplicate hook in template and override', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["shared-hook"]}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
						claudeConfig: '{"hooks": ["shared-hook"]}',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);

			// Should show as inherited (template wins for display)
			const hookItems = screen.getAllByText('shared-hook');
			expect(hookItems).toHaveLength(1); // Should not duplicate
		});

		it('should pre-fill edit dialog with existing claude_config_override', async () => {
			const user = userEvent.setup();
			const phases = [
				createMockWorkflowPhase({
					id: 1,
					phaseTemplateId: 'implement',
					sequence: 1,
					claudeConfigOverride: '{"hooks": ["existing-hook"], "env": {"MY_VAR": "my-value"}}',
					template: createMockPhaseTemplate({
						id: 'implement',
						name: 'Implement',
					}),
				}),
			];

			render(<PhaseListEditor {...defaultProps} phases={phases} />);

			const phaseItem = screen.getByTestId(/phase-item/);
			const editButton = within(phaseItem).getByRole('button', { name: /edit/i });
			await user.click(editButton);

			// Expand hooks section - should see existing override hook
			const hooksHeader = await screen.findByText(/hooks/i);
			await user.click(hooksHeader);
			expect(screen.getByText('existing-hook')).toBeInTheDocument();

			// Expand env vars section - should see existing override env var
			const envHeader = screen.getByText(/env vars/i);
			await user.click(envHeader);
			expect(screen.getByText('MY_VAR')).toBeInTheDocument();
		});
	});
});
