/**
 * TDD Tests for EditWorkflowModal - completion_action field
 *
 * Tests for TASK-680: Add completion_action to workflow model
 *
 * Success Criteria Coverage:
 * - SC-7e: EditWorkflowModal shows current completion_action value
 * - SC-7f: User can change completion_action via select
 * - SC-7g: Update request includes completion_action
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { EditWorkflowModal } from './EditWorkflowModal';
import {
	createMockWorkflow,
	createMockWorkflowWithDetails,
	createMockPhaseTemplate,
	createMockGetWorkflowResponse,
	createMockListPhaseTemplatesResponse,
	createMockUpdateWorkflowResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	workflowClient: {
		getWorkflow: vi.fn(),
		updateWorkflow: vi.fn(),
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

// Mock browser APIs
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
const mockPhaseTemplates = [
	createMockPhaseTemplate({ id: 'spec', name: 'Spec', isBuiltin: true }),
	createMockPhaseTemplate({ id: 'implement', name: 'Implement', isBuiltin: true }),
];

describe('EditWorkflowModal - completion_action field', () => {
	const mockOnClose = vi.fn();
	const mockOnUpdated = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();

		// Default mock for listPhaseTemplates
		vi.mocked(workflowClient.listPhaseTemplates).mockResolvedValue(
			createMockListPhaseTemplatesResponse(mockPhaseTemplates)
		);
	});

	afterEach(() => {
		cleanup();
	});

	it('SC-7e: displays current completion_action value', async () => {
		const workflowWithDetails = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
				completionAction: 'pr', // Current value is "pr"
			}),
			phases: [],
			variables: [],
		});

		vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
			createMockGetWorkflowResponse(workflowWithDetails)
		);

		render(
			<EditWorkflowModal
				workflowId="test-workflow"
				open={true}
				onClose={mockOnClose}
				onUpdated={mockOnUpdated}
			/>
		);

		await waitFor(() => {
			const select = screen.getByLabelText(/completion action/i);
			expect(select).toHaveValue('pr');
		});
	});

	it('SC-7e: displays empty value when completion_action is inherit', async () => {
		const workflowWithDetails = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
				completionAction: '', // Inherit
			}),
			phases: [],
			variables: [],
		});

		vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
			createMockGetWorkflowResponse(workflowWithDetails)
		);

		render(
			<EditWorkflowModal
				workflowId="test-workflow"
				open={true}
				onClose={mockOnClose}
				onUpdated={mockOnUpdated}
			/>
		);

		await waitFor(() => {
			const select = screen.getByLabelText(/completion action/i);
			expect(select).toHaveValue('');
		});
	});

	it('SC-7f: user can change completion_action from pr to commit', async () => {
		const user = userEvent.setup();

		const workflowWithDetails = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
				completionAction: 'pr',
			}),
			phases: [],
			variables: [],
		});

		vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
			createMockGetWorkflowResponse(workflowWithDetails)
		);
		vi.mocked(workflowClient.updateWorkflow).mockResolvedValue(
			createMockUpdateWorkflowResponse(createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				completionAction: 'commit',
			}))
		);

		render(
			<EditWorkflowModal
				workflowId="test-workflow"
				open={true}
				onClose={mockOnClose}
				onUpdated={mockOnUpdated}
			/>
		);

		await waitFor(() => {
			expect(screen.getByLabelText(/completion action/i)).toBeInTheDocument();
		});

		const select = screen.getByLabelText(/completion action/i);
		await user.selectOptions(select, 'commit');

		// Save button should save the new value
		await user.click(screen.getByRole('button', { name: /save/i }));

		await waitFor(() => {
			expect(workflowClient.updateWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					completionAction: 'commit',
				})
			);
		});
	});

	it('SC-7g: update includes completion_action=none when changed', async () => {
		const user = userEvent.setup();

		const workflowWithDetails = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
				completionAction: 'pr',
			}),
			phases: [],
			variables: [],
		});

		vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
			createMockGetWorkflowResponse(workflowWithDetails)
		);
		vi.mocked(workflowClient.updateWorkflow).mockResolvedValue(
			createMockUpdateWorkflowResponse(createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				completionAction: 'none',
			}))
		);

		render(
			<EditWorkflowModal
				workflowId="test-workflow"
				open={true}
				onClose={mockOnClose}
				onUpdated={mockOnUpdated}
			/>
		);

		await waitFor(() => {
			expect(screen.getByLabelText(/completion action/i)).toBeInTheDocument();
		});

		const select = screen.getByLabelText(/completion action/i);
		await user.selectOptions(select, 'none');

		await user.click(screen.getByRole('button', { name: /save/i }));

		await waitFor(() => {
			expect(workflowClient.updateWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					completionAction: 'none',
				})
			);
		});
	});

	it('SC-7g: update includes empty completion_action when set to inherit', async () => {
		const user = userEvent.setup();

		const workflowWithDetails = createMockWorkflowWithDetails({
			workflow: createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				isBuiltin: false,
				completionAction: 'pr', // Currently set to pr
			}),
			phases: [],
			variables: [],
		});

		vi.mocked(workflowClient.getWorkflow).mockResolvedValue(
			createMockGetWorkflowResponse(workflowWithDetails)
		);
		vi.mocked(workflowClient.updateWorkflow).mockResolvedValue(
			createMockUpdateWorkflowResponse(createMockWorkflow({
				id: 'test-workflow',
				name: 'Test',
				completionAction: '',
			}))
		);

		render(
			<EditWorkflowModal
				workflowId="test-workflow"
				open={true}
				onClose={mockOnClose}
				onUpdated={mockOnUpdated}
			/>
		);

		await waitFor(() => {
			expect(screen.getByLabelText(/completion action/i)).toBeInTheDocument();
		});

		// Change to inherit (empty)
		const select = screen.getByLabelText(/completion action/i);
		await user.selectOptions(select, ''); // Select empty option

		await user.click(screen.getByRole('button', { name: /save/i }));

		await waitFor(() => {
			expect(workflowClient.updateWorkflow).toHaveBeenCalledWith(
				expect.objectContaining({
					completionAction: '',
				})
			);
		});
	});
});
