/**
 * TDD Tests for TaskEditModal workflow selector
 *
 * Tests for TASK-536: Add workflow selector to Edit Task modal
 *
 * Success Criteria Coverage:
 * - SC-4: Edit Task modal displays workflow selector with current workflow pre-selected
 * - SC-6: Backend UpdateTask handler processes workflow_id changes (via mock verification)
 */

import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TaskEditModal } from './TaskEditModal';
import {
	type Task,
	TaskStatus,
	TaskWeight,
	TaskCategory,
	TaskPriority,
	TaskQueue,
} from '@/gen/orc/v1/task_pb';
import {
	createMockTask,
	createTimestamp,
	createMockWorkflow,
	createMockListWorkflowsResponse,
	createMockUpdateTaskResponse,
} from '@/test/factories';

// Mock the client module
vi.mock('@/lib/client', () => ({
	taskClient: {
		updateTask: vi.fn(),
	},
	workflowClient: {
		listWorkflows: vi.fn(),
	},
}));

// Mock stores
vi.mock('@/stores', () => ({
	useInitiatives: () => [],
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

// Helper to create a task with workflow
function createTaskWithWorkflow(workflowId?: string): Task {
	return createMockTask({
		id: 'TASK-001',
		title: 'Test Task',
		description: 'A test task',
		weight: TaskWeight.MEDIUM,
		status: TaskStatus.CREATED,
		category: TaskCategory.FEATURE,
		priority: TaskPriority.NORMAL,
		queue: TaskQueue.ACTIVE,
		workflowId: workflowId,
		createdAt: createTimestamp('2024-01-01T00:00:00Z'),
		updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
	});
}

describe('TaskEditModal - Workflow Selector', () => {
	const mockOnClose = vi.fn();
	const mockOnUpdate = vi.fn();

	beforeEach(() => {
		vi.clearAllMocks();
		// Setup workflow list mock
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse(mockWorkflows)
		);
	});

	afterEach(() => {
		cleanup();
	});

	describe('SC-4: Workflow selector with current workflow pre-selected', () => {
		it('should display workflow selector in edit modal', async () => {
			const task = createTaskWithWorkflow('medium');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Workflow selector should exist
			const workflowLabel = screen.getByLabelText(/workflow/i);
			expect(workflowLabel).toBeInTheDocument();
		});

		it('should pre-select current task workflow', async () => {
			const task = createTaskWithWorkflow('medium');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Workflow selector should show "Medium" as selected
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/medium/i);
		});

		it('should show "None" when task has no workflow', async () => {
			const task = createTaskWithWorkflow(undefined); // No workflow

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show "None" option selected
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/none/i);
		});

		it('should show "Unknown workflow" when task has deleted workflow', async () => {
			// Task has a workflow that no longer exists
			const task = createTaskWithWorkflow('deleted-workflow');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should indicate unknown/deleted workflow
			const workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/unknown|deleted-workflow/i);
		});

		it('should update pre-selected workflow when task prop changes', async () => {
			const task1 = createTaskWithWorkflow('small');
			const task2 = createTaskWithWorkflow('large');

			const { rerender } = render(
				<TaskEditModal
					open={true}
					task={task1}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Should show "Small"
			let workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/small/i);

			// Rerender with different task
			rerender(
				<TaskEditModal
					open={true}
					task={task2}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			// Should now show "Large"
			workflowTrigger = screen.getByLabelText(/workflow/i);
			expect(workflowTrigger).toHaveTextContent(/large/i);
		});
	});

	describe('SC-6: Workflow changes persisted via UpdateTask', () => {
		it('should include workflowId in update request when changed', async () => {
			const user = userEvent.setup();
			const task = createTaskWithWorkflow('small');

			const updatedTask = createMockTask({ ...task, workflowId: 'large' });
			vi.mocked(taskClient.updateTask).mockResolvedValue(
				createMockUpdateTaskResponse(updatedTask)
			);

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Change workflow to large
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			const largeOption = await screen.findByRole('option', { name: /large/i });
			await user.click(largeOption);

			// Save changes
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(taskClient.updateTask).toHaveBeenCalledWith(
					expect.objectContaining({
						taskId: 'TASK-001',
						projectId: 'test-project',
						workflowId: 'large',
					})
				);
			});
		});

		it('should not include workflowId when unchanged', async () => {
			const user = userEvent.setup();
			const task = createTaskWithWorkflow('medium');

			vi.mocked(taskClient.updateTask).mockResolvedValue(
				createMockUpdateTaskResponse(task)
			);

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Just change title, not workflow
			const titleInput = screen.getByLabelText(/title/i);
			await user.clear(titleInput);
			await user.type(titleInput, 'Updated Title');

			// Save changes
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			// workflowId should either be undefined or the same as original
			await waitFor(() => {
				expect(taskClient.updateTask).toHaveBeenCalled();
				const callArg = vi.mocked(taskClient.updateTask).mock.calls[0][0];
				// If workflow wasn't changed, it should either not be included or be the same
				expect(callArg.workflowId).toBeOneOf([undefined, 'medium']);
			});
		});

		it('should allow setting workflow to None', async () => {
			const user = userEvent.setup();
			const task = createTaskWithWorkflow('medium');

			const updatedTask = createMockTask({ ...task, workflowId: undefined });
			vi.mocked(taskClient.updateTask).mockResolvedValue(
				createMockUpdateTaskResponse(updatedTask)
			);

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Change workflow to None
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			const noneOption = await screen.findByRole('option', { name: /none/i });
			await user.click(noneOption);

			// Save
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(taskClient.updateTask).toHaveBeenCalledWith(
					expect.objectContaining({
						taskId: 'TASK-001',
						projectId: 'test-project',
						workflowId: undefined,
					})
				);
			});
		});

		it('should show error toast when update fails', async () => {
			const user = userEvent.setup();
			const task = createTaskWithWorkflow('small');

			vi.mocked(taskClient.updateTask).mockRejectedValue(
				new Error('Failed to update task')
			);

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Change workflow
			const workflowSelect = screen.getByLabelText(/workflow/i);
			await user.click(workflowSelect);

			const largeOption = await screen.findByRole('option', { name: /large/i });
			await user.click(largeOption);

			// Save
			const saveButton = screen.getByRole('button', { name: /save/i });
			await user.click(saveButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith('Failed to update task');
			});
		});
	});

	describe('Error Handling', () => {
		it('should show error when workflow API fails to load', async () => {
			vi.mocked(workflowClient.listWorkflows).mockRejectedValue(
				new Error('Network error')
			);

			const task = createTaskWithWorkflow('medium');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load workflows/i)).toBeInTheDocument();
			});
		});

		it('should allow retry when workflows fail to load', async () => {
			// First call fails
			vi.mocked(workflowClient.listWorkflows).mockRejectedValueOnce(
				new Error('Network error')
			);
			// Second call succeeds
			vi.mocked(workflowClient.listWorkflows).mockResolvedValueOnce(
				createMockListWorkflowsResponse(mockWorkflows)
			);

			const task = createTaskWithWorkflow('medium');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(screen.getByText(/failed to load workflows/i)).toBeInTheDocument();
			});

			// Click retry button
			const retryButton = screen.getByRole('button', { name: /retry/i });
			await userEvent.click(retryButton);

			// Should reload successfully
			await waitFor(() => {
				const workflowTrigger = screen.getByLabelText(/workflow/i);
				expect(workflowTrigger).toHaveTextContent(/medium/i);
			});
		});
	});

	describe('Preservation Requirements', () => {
		it('should not affect initiative selector functionality', async () => {
			// Regression test: initiative selector must still work
			const task = createTaskWithWorkflow('medium');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Initiative selector should still exist
			const initiativeLabel = screen.getByLabelText(/initiative/i);
			expect(initiativeLabel).toBeInTheDocument();
		});

		it('should position workflow selector appropriately in the form', async () => {
			const task = createTaskWithWorkflow('medium');

			render(
				<TaskEditModal
					open={true}
					task={task}
					onClose={mockOnClose}
					onUpdate={mockOnUpdate}
				/>
			);

			await waitFor(() => {
				expect(workflowClient.listWorkflows).toHaveBeenCalled();
			});

			// Workflow should be in the form, and accessible
			const workflowGroup = screen.getByLabelText(/workflow/i).closest('.form-group');
			expect(workflowGroup).toBeInTheDocument();

			// Should have hint text
			const hint = workflowGroup?.querySelector('.form-hint');
			expect(hint).toBeInTheDocument();
		});
	});
});

// Custom matcher for checking one of multiple values
expect.extend({
	toBeOneOf(received, expected: any[]) {
		const pass = expected.includes(received);
		return {
			pass,
			message: () =>
				pass
					? `expected ${received} not to be one of ${expected.join(', ')}`
					: `expected ${received} to be one of ${expected.join(', ')}`,
		};
	},
});

declare module 'vitest' {
	interface Assertion<T = any> {
		toBeOneOf(expected: T[]): void;
	}
}
