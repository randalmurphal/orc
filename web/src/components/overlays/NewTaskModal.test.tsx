/**
 * TDD Tests for NewTaskModal workflow selector
 *
 * Tests for TASK-536: Add workflow selector to New Task modal
 *
 * Success Criteria Coverage:
 * - SC-1: New Task modal displays workflow selector dropdown
 * - SC-2: Creating task with workflow saves workflow_id (via mock verification)
 * - SC-3: Default workflow selection matches task weight
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { NewTaskModal } from './NewTaskModal';
import { TaskWeight } from '@/gen/orc/v1/task_pb';
import {
	createMockWorkflow,
	createMockListWorkflowsResponse,
	createMockTask,
	createMockCreateTaskResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	taskClient: {
		createTask: vi.fn(),
	},
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
		success: vi.fn(),
		error: vi.fn(),
	},
}));

// Create mock workflows using factory
const mockWorkflows = [
	createMockWorkflow({ id: 'trivial', name: 'Trivial', isBuiltin: true, description: 'For one-liner fixes' }),
	createMockWorkflow({ id: 'small', name: 'Small', isBuiltin: true, description: 'For bug fixes' }),
	createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true, description: 'For features' }),
	createMockWorkflow({ id: 'large', name: 'Large', isBuiltin: true, description: 'For complex features' }),
	createMockWorkflow({ id: 'custom-workflow', name: 'Custom Workflow', isBuiltin: false, description: 'User defined' }),
];

// Import mocked modules for assertions
import { taskClient, workflowClient } from '@/lib/client';
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
});

describe('NewTaskModal - Workflow Selector', () => {
	const mockOnClose = vi.fn();
	const mockOnCreate = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		// Setup workflow list mock to return workflows
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse(mockWorkflows)
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-1: Workflow selector display', () => {
		it('should display workflow selector dropdown when modal is open', async () => {
			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			// Wait for workflows to load
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Workflow selector should exist
			const workflowLabel = screen.getByLabelText(/workflow/i);
			expect(workflowLabel).toBeInTheDocument();
		});

		it('should load workflows on mount with includeBuiltin: true', async () => {
			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
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

		it('should display all available workflows including builtin and custom', async () => {
			const user = userEvent.setup();

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			// Wait for workflows to load
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Open the workflow dropdown
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			// Should show all workflows
			await waitFor(() => {
				expect(screen.getByRole('option', { name: /small/i })).toBeInTheDocument();
				expect(screen.getByRole('option', { name: /medium/i })).toBeInTheDocument();
				expect(screen.getByRole('option', { name: /large/i })).toBeInTheDocument();
				expect(screen.getByRole('option', { name: /custom workflow/i })).toBeInTheDocument();
			});
		});

		it('should show error state when workflows fail to load', async () => {
			vi.mocked(workflowClient.listWorkflows).mockRejectedValue(
				new Error('Network error')
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				// Should show error message in the workflow selector area
				expect(screen.getByText(/failed to load workflows/i)).toBeInTheDocument();
			});
		});

		it('should show "No workflows available" when API returns empty list', async () => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse([])
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/no workflows available/i)).toBeInTheDocument();
			});
		});
	});

	describe('SC-3: Default workflow selection matches weight', () => {
		it('should auto-select "medium" workflow when weight is medium (default)', async () => {
			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Default weight is medium, so workflow should default to "medium"
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/medium/i);
		});

		it('should auto-select "small" workflow when weight changed to small', async () => {
			const user = userEvent.setup();

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Change weight to small
			const weightSelect = screen.getByLabelText(/weight/i);
			await user.selectOptions(weightSelect, String(TaskWeight.SMALL));

			// Workflow should auto-update to small
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/small/i);
		});

		it('should auto-select "large" workflow when weight changed to large', async () => {
			const user = userEvent.setup();

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Change weight to large
			const weightSelect = screen.getByLabelText(/weight/i);
			await user.selectOptions(weightSelect, String(TaskWeight.LARGE));

			// Workflow should auto-update to large
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/large/i);
		});

		it('should not have default workflow when weight is trivial', async () => {
			const user = userEvent.setup();

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Change weight to trivial
			const weightSelect = screen.getByLabelText(/weight/i);
			await user.selectOptions(weightSelect, String(TaskWeight.TRIVIAL));

			// Workflow should show none/placeholder for trivial tasks
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			// Trivial tasks don't need a workflow - should show "None" or be empty
			expect(workflowTrigger).toHaveTextContent(/none|trivial/i);
		});

		it('should preserve manual workflow selection when weight changes', async () => {
			const user = userEvent.setup();

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Manually select custom workflow
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			const customOption = await screen.findByRole('option', { name: /custom workflow/i });
			await user.click(customOption);

			// Now change weight - workflow should NOT auto-change because manual selection
			const weightSelect = screen.getByLabelText(/weight/i);
			await user.selectOptions(weightSelect, String(TaskWeight.SMALL));

			// Custom workflow should still be selected
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/custom workflow/i);
		});
	});

	describe('SC-2: Task creation with workflow_id', () => {
		it('should include workflowId in create task request', async () => {
			const user = userEvent.setup();

			const mockTask = createMockTask({ id: 'TASK-001', title: 'Test Task', workflowId: 'medium' });
			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(mockTask)
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Fill in title
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Test Task');

			// Submit the form
			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			// Verify createTask was called with workflowId
			await waitFor(() => {
				expect(taskClient.createTask).toHaveBeenCalledWith(
					expect.objectContaining({
						title: 'Test Task',
						workflowId: 'medium', // Default workflow for medium weight
					})
				);
			});
		});

		it('should send selected workflow when manually changed', async () => {
			const user = userEvent.setup();

			const mockTask = createMockTask({ id: 'TASK-001', title: 'Test Task', workflowId: 'custom-workflow' });
			vi.mocked(taskClient.createTask).mockResolvedValue(
				createMockCreateTaskResponse(mockTask)
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Fill in title
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Test Task');

			// Change workflow to custom
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			const customOption = await screen.findByRole('option', { name: /custom workflow/i });
			await user.click(customOption);

			// Submit
			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(taskClient.createTask).toHaveBeenCalledWith(
					expect.objectContaining({
						workflowId: 'custom-workflow',
					})
				);
			});
		});

		it('should show error toast on invalid workflow selection', async () => {
			const user = userEvent.setup();

			vi.mocked(taskClient.createTask).mockRejectedValue(
				new Error('Invalid workflow selected')
			);

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Fill in title
			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Test Task');

			// Submit
			const createButton = screen.getByRole('button', { name: /create task/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith('Invalid workflow selected');
			});
		});
	});

	describe('Edge Cases', () => {
		it('should handle workflow with very long name (truncate with ellipsis via CSS)', async () => {
			const longNameWorkflow = createMockWorkflow({
				id: 'long-name',
				name: 'This is an extremely long workflow name that should be truncated in the dropdown display',
				isBuiltin: false,
				description: 'Test',
			});

			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse([...mockWorkflows, longNameWorkflow])
			);

			const user = userEvent.setup();

			render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Open dropdown
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			// Long name workflow should be in the list
			const longOption = await screen.findByRole('option', { name: /extremely long/i });
			expect(longOption).toBeInTheDocument();
		});

		it('should reset workflow selection when modal reopens', async () => {
			const { rerender } = render(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Close modal
			rerender(
				<NewTaskModal
					open={false}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			// Clear mock
			vi.mocked(workflowClient.listWorkflows).mockClear();

			// Reopen modal
			rerender(
				<NewTaskModal
					open={true}
					onClose={mockOnClose}
					onCreate={mockOnCreate}
				/>
			);

			// Should reload workflows and reset to default
			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Default workflow should be medium (matching default weight)
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/medium/i);
		});
	});
});
