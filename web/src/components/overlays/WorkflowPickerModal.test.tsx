/**
 * TDD Tests for WorkflowPickerModal
 *
 * Tests for TASK-731: Implement workflow-first task creation modal (Step 1: Picker)
 *
 * Success Criteria Coverage:
 * - SC-1: Workflow picker displays available workflows as selectable cards
 * - SC-2: Default workflow is visually indicated and pre-selected
 * - SC-3: Selected workflow is tracked in component state
 * - SC-4: Workflow cards show workflow name and phase count
 * - SC-5: Can proceed to step 2 with selected workflow
 * - SC-6: Can cancel workflow selection process
 * - SC-7: Weight dropdown is completely removed (workflow replaces weight)
 * - SC-8: Error states are handled (failed to load workflows, empty list)
 * - SC-9: Workflow selection is required to proceed
 * - SC-10: Built-in workflows are sorted before custom workflows
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkflowPickerModal } from './WorkflowPickerModal';
import {
	createMockWorkflow,
	createMockListWorkflowsResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		listWorkflows: vi.fn(),
	},
}));

// Mock the stores
vi.mock('@/stores', () => ({
	useCurrentProjectId: () => 'test-project',
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		error: vi.fn(),
	},
}));

// Create mock workflows using factory
const mockWorkflows = [
	createMockWorkflow({
		id: 'implement-small',
		name: 'Implement (Small)',
		isBuiltin: true,
		description: 'For bug fixes and small features'
	}),
	createMockWorkflow({
		id: 'implement-medium',
		name: 'Implement (Medium)',
		isBuiltin: true,
		description: 'For standard features requiring spec and review'
	}),
	createMockWorkflow({
		id: 'implement-large',
		name: 'Implement (Large)',
		isBuiltin: true,
		description: 'For complex features requiring TDD and breakdown'
	}),
	createMockWorkflow({
		id: 'review-only',
		name: 'Review Only',
		isBuiltin: true,
		description: 'For code review without implementation'
	}),
	createMockWorkflow({
		id: 'custom-workflow',
		name: 'Custom Workflow',
		isBuiltin: false,
		description: 'User-defined workflow'
	}),
];

// Mock workflow phase counts (from ListWorkflowsResponse)
const mockPhaseCounts = {
	'implement-small': 3,
	'implement-medium': 5,
	'implement-large': 6,
	'review-only': 1,
	'custom-workflow': 4,
};

// Import mocked modules for assertions
import { workflowClient } from '@/lib/client';

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

describe('WorkflowPickerModal - Workflow-First Task Creation', () => {
	const mockOnClose = vi.fn();
	const mockOnSelectWorkflow = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		// Setup workflow list mock to return workflows with phase counts
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse(mockWorkflows, mockPhaseCounts)
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Workflow picker displays available workflows as selectable cards', () => {
		it('should display all available workflows as cards when modal is open', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			// Wait for workflows to load
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show all workflows as cards
			expect(screen.getByText('Implement (Small)')).toBeInTheDocument();
			expect(screen.getByText('Implement (Medium)')).toBeInTheDocument();
			expect(screen.getByText('Implement (Large)')).toBeInTheDocument();
			expect(screen.getByText('Review Only')).toBeInTheDocument();
			expect(screen.getByText('Custom Workflow')).toBeInTheDocument();
		});

		it('should load workflows on mount with includeBuiltin: true', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalledWith(
					expect.objectContaining({
						includeBuiltin: true,
					})
				);
			});
		});

		it('should display workflow cards as interactive buttons', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Workflow cards should be clickable buttons
			const smallCard = screen.getByRole('button', { name: /implement \(small\)/i });
			expect(smallCard).toBeInTheDocument();
			expect(smallCard).toBeEnabled();

			const mediumCard = screen.getByRole('button', { name: /implement \(medium\)/i });
			expect(mediumCard).toBeInTheDocument();
			expect(mediumCard).toBeEnabled();
		});
	});

	describe('SC-2: Default workflow is visually indicated and pre-selected', () => {
		it('should visually indicate the default workflow with a star', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-medium"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Default workflow should show a star indicator
			const defaultCard = screen.getByRole('button', { name: /implement \(medium\)/i });
			expect(defaultCard).toHaveTextContent('★');
		});

		it('should pre-select the default workflow', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-medium"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Default workflow should have selected styling
			const defaultCard = screen.getByRole('button', { name: /implement \(medium\)/i });
			expect(defaultCard).toHaveClass('selected');
		});

		it('should work without default workflow (no pre-selection)', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// No workflow should be pre-selected
			const cards = screen.getAllByRole('button', { name: /implement|review|custom/i });
			cards.forEach(card => {
				expect(card).not.toHaveClass('selected');
			});
		});
	});

	describe('SC-3: Selected workflow is tracked in component state', () => {
		it('should update selection when clicking a workflow card', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Click on a workflow card
			const largeCard = screen.getByRole('button', { name: /implement \(large\)/i });
			await user.click(largeCard);

			// Card should now be selected
			expect(largeCard).toHaveClass('selected');
		});

		it('should allow changing selection between workflows', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-small"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Initially small is selected
			const smallCard = screen.getByRole('button', { name: /implement \(small\)/i });
			const mediumCard = screen.getByRole('button', { name: /implement \(medium\)/i });

			expect(smallCard).toHaveClass('selected');
			expect(mediumCard).not.toHaveClass('selected');

			// Click medium workflow
			await user.click(mediumCard);

			// Selection should change
			expect(smallCard).not.toHaveClass('selected');
			expect(mediumCard).toHaveClass('selected');
		});
	});

	describe('SC-4: Workflow cards show workflow name and phase count', () => {
		it('should display workflow name on each card', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show workflow names
			expect(screen.getByText('Implement (Small)')).toBeInTheDocument();
			expect(screen.getByText('Implement (Medium)')).toBeInTheDocument();
			expect(screen.getByText('Implement (Large)')).toBeInTheDocument();
			expect(screen.getByText('Review Only')).toBeInTheDocument();
			expect(screen.getByText('Custom Workflow')).toBeInTheDocument();
		});

		it('should display phase count for each workflow', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show phase counts
			expect(screen.getByText('3 phases')).toBeInTheDocument(); // implement-small
			expect(screen.getByText('5 phases')).toBeInTheDocument(); // implement-medium
			expect(screen.getByText('6 phases')).toBeInTheDocument(); // implement-large
			expect(screen.getByText('1 phase')).toBeInTheDocument();  // review-only
			expect(screen.getByText('4 phases')).toBeInTheDocument(); // custom-workflow
		});

		it('should handle singular vs plural phase count text correctly', async () => {
			// Test workflow with exactly 1 phase
			const singlePhaseWorkflow = createMockWorkflow({
				id: 'single-phase',
				name: 'Single Phase',
				isBuiltin: true
			});

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse([singlePhaseWorkflow], { 'single-phase': 1 })
			);

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show singular "phase" not "phases"
			expect(screen.getByText('1 phase')).toBeInTheDocument();
			expect(screen.queryByText('1 phases')).not.toBeInTheDocument();
		});
	});

	describe('SC-5: Can proceed to step 2 with selected workflow', () => {
		it('should enable Next button when workflow is selected', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Next button should be disabled initially
			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).toBeDisabled();

			// Select a workflow
			const smallCard = screen.getByRole('button', { name: /implement \(small\)/i });
			await user.click(smallCard);

			// Next button should now be enabled
			expect(nextButton).toBeEnabled();
		});

		it('should call onSelectWorkflow when Next is clicked with selection', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-medium"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Click Next with default selection
			const nextButton = screen.getByRole('button', { name: /next/i });
			await user.click(nextButton);

			// Should call onSelectWorkflow with the selected workflow
			expect(mockOnSelectWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					id: 'implement-medium',
					name: 'Implement (Medium)',
				})
			);
		});

		it('should pass phase count in selected workflow data', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-large"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Click Next
			const nextButton = screen.getByRole('button', { name: /next/i });
			await user.click(nextButton);

			// Should include phase count in the callback
			expect(mockOnSelectWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					id: 'implement-large',
					phaseCount: 6,
				})
			);
		});
	});

	describe('SC-6: Can cancel workflow selection process', () => {
		it('should display Cancel button', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			expect(cancelButton).toBeInTheDocument();
			expect(cancelButton).toBeEnabled();
		});

		it('should call onClose when Cancel is clicked', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			await user.click(cancelButton);

			expect(mockOnClose).toHaveBeenCalled();
		});

		it('should call onClose when modal overlay is clicked', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Use Escape key to close modal (equivalent to overlay click)
			await user.keyboard('[Escape]');
			expect(mockOnClose).toHaveBeenCalled();
		});
	});

	describe('SC-7: Weight dropdown is completely removed', () => {
		it('should not display any weight-related UI elements', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should not have weight-related form controls or labels
			expect(screen.queryByText(/weight/i)).not.toBeInTheDocument();
			expect(screen.queryByText(/trivial/i)).not.toBeInTheDocument();
			expect(screen.queryByLabelText(/weight/i)).not.toBeInTheDocument();

			// Should not have weight dropdown or select elements
			expect(screen.queryByRole('combobox', { name: /weight/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('listbox', { name: /weight/i })).not.toBeInTheDocument();

			// Should not have weight-related buttons (excluding workflow names)
			expect(screen.queryByRole('button', { name: /^trivial$/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /^small$/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /^medium$/i })).not.toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /^large$/i })).not.toBeInTheDocument();
		});

		it('should have workflow selection as the primary interaction', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should have workflow cards as main UI
			const workflowCards = screen.getAllByRole('button', { name: /implement|review|custom/i });
			expect(workflowCards.length).toBeGreaterThan(0);

			// Should have clear heading indicating workflow selection
			expect(screen.getByText(/choose.*workflow/i)).toBeInTheDocument();
		});
	});

	describe('SC-8: Error states are handled', () => {
		it('should show error state when workflows fail to load', async () => {
			vi.mocked(workflowClient.listWorkflows).mockRejectedValue(
				new Error('Network error')
			);

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				// Should show error message
				expect(screen.getByText(/failed to load workflows/i)).toBeInTheDocument();
			});

			// Should show retry button
			const retryButton = screen.getByRole('button', { name: /retry/i });
			expect(retryButton).toBeInTheDocument();
		});

		it('should allow retry when workflow loading fails', async () => {
			const user = userEvent.setup();

			vi.mocked(workflowClient.listWorkflows)
				.mockRejectedValueOnce(new Error('Network error'))
				.mockResolvedValueOnce(createMockListWorkflowsResponse(mockWorkflows, mockPhaseCounts));

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load workflows/i)).toBeInTheDocument();
			});

			// Click retry
			const retryButton = screen.getByRole('button', { name: /retry/i });
			await user.click(retryButton);

			// Should load workflows successfully
			await waitFor(() => {
				expect(screen.getByText('Implement (Small)')).toBeInTheDocument();
			});
		});

		it('should show empty state when no workflows are available', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse([], {})
			);

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/no workflows available/i)).toBeInTheDocument();
			});

			// Next button should be disabled in empty state
			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).toBeDisabled();
		});

		it('should show loading state while workflows are being fetched', async () => {
			// Create a promise that we can control
			let resolvePromise: (value: any) => void;
			const loadingPromise = new Promise((resolve) => {
				resolvePromise = resolve;
			});

			vi.mocked(workflowClient.listWorkflows).mockReturnValue(loadingPromise as any);

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			// Should show loading indicator
			expect(screen.getByText(/loading/i)).toBeInTheDocument();

			// Resolve the promise
			resolvePromise!(createMockListWorkflowsResponse(mockWorkflows, mockPhaseCounts));

			await waitFor(() => {
				expect(screen.getByText('Implement (Small)')).toBeInTheDocument();
			});
		});
	});

	describe('SC-9: Workflow selection is required to proceed', () => {
		it('should keep Next button disabled when no workflow is selected', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Next button should be disabled when no selection
			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).toBeDisabled();
		});

		it('should show helper text indicating selection is required', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show instruction text
			expect(screen.getByText(/select a workflow to continue/i)).toBeInTheDocument();
		});
	});

	describe('SC-10: Built-in workflows are sorted before custom workflows', () => {
		it('should display built-in workflows before custom workflows', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Get all workflow card buttons in order
			const workflowCards = screen.getAllByRole('button', { name: /implement|review|custom/i });

			// Built-in workflows should appear first
			expect(workflowCards[0]).toHaveTextContent(/implement \(small\)/i);
			expect(workflowCards[1]).toHaveTextContent(/implement \(medium\)/i);
			expect(workflowCards[2]).toHaveTextContent(/implement \(large\)/i);
			expect(workflowCards[3]).toHaveTextContent(/review only/i);

			// Custom workflow should be last
			expect(workflowCards[4]).toHaveTextContent(/custom workflow/i);
		});

		it('should show built-in indicator on built-in workflows', async () => {
			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Built-in workflows should have built-in indicator
			const smallCard = screen.getByRole('button', { name: /implement \(small\)/i });
			expect(smallCard).toHaveTextContent(/built-in/i);

			// Custom workflow should not have built-in indicator
			const customCard = screen.getByRole('button', { name: /custom workflow/i });
			expect(customCard).not.toHaveTextContent(/built-in/i);
		});
	});

	describe('Integration: Full workflow picker flow', () => {
		it('should complete full selection flow successfully', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			// Wait for load
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Select workflow
			const mediumCard = screen.getByRole('button', { name: /implement \(medium\)/i });
			await user.click(mediumCard);

			// Verify selection
			expect(mediumCard).toHaveClass('selected');

			// Proceed to next step
			const nextButton = screen.getByRole('button', { name: /next/i });
			expect(nextButton).toBeEnabled();
			await user.click(nextButton);

			// Verify callback
			expect(mockOnSelectWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					id: 'implement-medium',
					name: 'Implement (Medium)',
					phaseCount: 5,
				})
			);
		});

		it('should handle keyboard navigation between workflow cards', async () => {
			const user = userEvent.setup();

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Tab to first workflow card
			await user.tab();
			const firstCard = screen.getByRole('button', { name: /implement \(small\)/i });
			expect(firstCard).toHaveFocus();

			// Use arrow keys to navigate
			await user.keyboard('[ArrowDown]');
			const secondCard = screen.getByRole('button', { name: /implement \(medium\)/i });
			expect(secondCard).toHaveFocus();

			// Select with Enter
			await user.keyboard('[Enter]');
			expect(secondCard).toHaveClass('selected');
		});
	});

	describe('Edge Cases', () => {
		it('should handle workflow with very long name', async () => {
			const longNameWorkflow = createMockWorkflow({
				id: 'long-name',
				name: 'This is an extremely long workflow name that should be handled gracefully in the UI',
				isBuiltin: false,
				description: 'Test workflow with long name',
			});

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse([longNameWorkflow], { 'long-name': 2 })
			);

			render(
				<WorkflowPickerModal
					open={true}
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Long name should be displayed (may be truncated with CSS)
			const longCard = screen.getByRole('button', { name: /extremely long/i });
			expect(longCard).toBeInTheDocument();
		});

		it('should reset selection when modal reopens', async () => {
			const { rerender } = render(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-medium"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Close modal
			rerender(
				<WorkflowPickerModal
					open={false}
					defaultWorkflowId="implement-medium"
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			// Clear mock and reopen
			vi.mocked(workflowClient.listWorkflows).mockClear();

			rerender(
				<WorkflowPickerModal
					open={true}
					defaultWorkflowId="implement-small" // Different default
					onClose={mockOnClose}
					onSelectWorkflow={mockOnSelectWorkflow}
				/>
			);

			// Should reload workflows and reset to new default
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			const newDefaultCard = screen.getByRole('button', { name: /implement \(small\)/i });
			expect(newDefaultCard).toHaveClass('selected');
		});
	});
});