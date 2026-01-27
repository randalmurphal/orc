/**
 * TDD Tests for TASK-554: Standardize button usage
 *
 * These tests verify that raw <button> elements have been replaced with
 * the Button component from @/components/ui/Button.tsx.
 *
 * Success Criteria Coverage:
 * - SC-1: TaskHeader.tsx uses Button component for all 5 buttons
 * - SC-2: CommentsTab.tsx uses Button component for all 5 buttons
 * - SC-3: NewTaskModal.tsx uses Button with correct variants (Cancel=secondary, Create=primary)
 * - SC-4: All icon-only buttons use Button with iconOnly prop
 * - SC-5: All raw buttons replaced (static analysis - manual verification)
 * - SC-6: No visual regressions (manual verification)
 * - SC-7: Loading states work on buttons that previously had them
 */

import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { TaskHeader } from '@/components/task-detail/TaskHeader';
import { CommentsTab } from '@/components/task-detail/CommentsTab';
import { NewTaskModal } from '@/components/overlays/NewTaskModal';
import { TooltipProvider } from '@/components/ui/Tooltip';
import { TaskStatus, TaskWeight, TaskCategory, TaskPriority, TaskQueue } from '@/gen/orc/v1/task_pb';
import { createMockTask, createTimestamp, createMockWorkflow, createMockListWorkflowsResponse, createMockCreateTaskResponse, createMockTaskComment, createMockListCommentsResponse } from '@/test/factories';

// Mock router
const mockNavigate = vi.fn();
vi.mock('react-router-dom', async () => {
	const actual = await vi.importActual('react-router-dom');
	return {
		...actual,
		useNavigate: () => mockNavigate,
	};
});

// Mock API clients
vi.mock('@/lib/client', () => ({
	taskClient: {
		runTask: vi.fn(),
		pauseTask: vi.fn(),
		resumeTask: vi.fn(),
		deleteTask: vi.fn(),
		listComments: vi.fn(),
		createComment: vi.fn(),
		updateComment: vi.fn(),
		deleteComment: vi.fn(),
		createTask: vi.fn(),
	},
	workflowClient: {
		listWorkflows: vi.fn(),
	},
}));

vi.mock('@/stores', () => ({
	getInitiativeBadgeTitle: () => null,
	useInitiatives: () => [],
	useCurrentProjectId: () => 'test-project',
}));

vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

import { taskClient, workflowClient } from '@/lib/client';

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

/**
 * Helper to check if an element uses the Button component
 * Button component renders <button> with specific CSS classes
 */
function expectButtonComponent(element: HTMLElement) {
	expect(element.tagName).toBe('BUTTON');
	// Button component applies 'btn' base class and variant classes
	expect(element).toHaveClass('btn');
}

/**
 * Helper to check if a button has the icon-only mode
 */
function expectIconOnlyButton(element: HTMLElement) {
	expectButtonComponent(element);
	expect(element).toHaveClass('btn-icon-only');
	// Icon-only buttons MUST have aria-label for accessibility
	expect(element).toHaveAttribute('aria-label');
}

describe('TASK-554: Button Standardization', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		// Set default mock for listComments
		vi.mocked(taskClient.listComments).mockResolvedValue(createMockListCommentsResponse([]));
	});

	describe('SC-1: TaskHeader.tsx Button Standardization', () => {
		const createTestTask = (overrides = {}) =>
			createMockTask({
				id: 'TASK-001',
				title: 'Test Task',
				description: 'Test description',
				status: TaskStatus.CREATED,
				weight: TaskWeight.SMALL,
				branch: 'orc/TASK-001',
				priority: TaskPriority.NORMAL,
				category: TaskCategory.FEATURE,
				queue: TaskQueue.ACTIVE,
				createdAt: createTimestamp('2024-01-01T00:00:00Z'),
				updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
				...overrides,
			});

		const renderTaskHeader = (taskOverrides = {}) => {
			return render(
				<TooltipProvider delayDuration={0}>
					<MemoryRouter>
						<TaskHeader
							task={createTestTask(taskOverrides)}
							onTaskUpdate={vi.fn()}
							onTaskDelete={vi.fn()}
						/>
					</MemoryRouter>
				</TooltipProvider>
			);
		};

		it('back button uses Button component with iconOnly prop', () => {
			renderTaskHeader();

			// Find back button by its aria-label or title
			const backBtn = screen.getByRole('button', { name: /go back/i });
			expectIconOnlyButton(backBtn);
			expect(backBtn).toHaveClass('btn-ghost'); // Back buttons should be ghost variant
		});

		it('run button uses Button component with primary variant', () => {
			renderTaskHeader({ status: TaskStatus.CREATED });

			const runBtn = screen.getByRole('button', { name: /run/i });
			expectButtonComponent(runBtn);
			expect(runBtn).toHaveClass('btn-primary');
		});

		it('pause button uses Button component when task is running', () => {
			renderTaskHeader({ status: TaskStatus.RUNNING });

			const pauseBtn = screen.getByRole('button', { name: /pause/i });
			expectButtonComponent(pauseBtn);
			// Pause button should have a specific variant - could be secondary or custom class
			expect(pauseBtn.className).toMatch(/btn-(secondary|warning)/);
		});

		it('resume button uses Button component when task is paused', () => {
			renderTaskHeader({ status: TaskStatus.PAUSED });

			const resumeBtn = screen.getByRole('button', { name: /resume/i });
			expectButtonComponent(resumeBtn);
			expect(resumeBtn).toHaveClass('btn-primary');
		});

		it('edit button uses Button component with iconOnly and ghost variant', () => {
			renderTaskHeader();

			const editBtn = screen.getByRole('button', { name: /edit/i });
			expectIconOnlyButton(editBtn);
			expect(editBtn).toHaveClass('btn-ghost');
		});

		it('delete button uses Button component with iconOnly and danger variant', () => {
			renderTaskHeader();

			const deleteBtn = screen.getByRole('button', { name: /delete/i });
			expectIconOnlyButton(deleteBtn);
			expect(deleteBtn).toHaveClass('btn-danger');
		});

		it('shows loading spinner using Button loading state', async () => {
			const user = userEvent.setup();
			vi.mocked(taskClient.runTask).mockImplementation(
				() => new Promise((resolve) => setTimeout(resolve, 100))
			);

			renderTaskHeader({ status: TaskStatus.CREATED });

			const runBtn = screen.getByRole('button', { name: /run/i });
			await user.click(runBtn);

			// Button should show loading state
			await waitFor(() => {
				// Button component adds btn-loading class and aria-busy when loading
				const loadingBtn = screen.getByRole('button', { name: /run/i });
				expect(loadingBtn).toHaveClass('btn-loading');
				expect(loadingBtn).toHaveAttribute('aria-busy', 'true');
			});
		});

		it('delete confirmation buttons use Button component with correct variants', async () => {
			const user = userEvent.setup();
			renderTaskHeader();

			// Click delete to open confirmation
			const deleteBtn = screen.getByRole('button', { name: /delete/i });
			await user.click(deleteBtn);

			// Modal should appear with Cancel and Delete buttons
			await waitFor(() => {
				const cancelBtn = screen.getByRole('button', { name: /cancel/i });
				const confirmDeleteBtn = screen.getByRole('button', { name: /^delete$/i });

				expectButtonComponent(cancelBtn);
				expect(cancelBtn).toHaveClass('btn-secondary');

				expectButtonComponent(confirmDeleteBtn);
				expect(confirmDeleteBtn).toHaveClass('btn-danger');
			});
		});
	});

	describe('SC-2: CommentsTab.tsx Button Standardization', () => {
		const renderCommentsTab = () => {
			return render(
				<TooltipProvider delayDuration={0}>
					<CommentsTab taskId="TASK-001" phases={['spec', 'implement', 'review']} />
				</TooltipProvider>
			);
		};

		it('Add Comment button uses Button component', async () => {
			vi.mocked(taskClient.listComments).mockResolvedValue(createMockListCommentsResponse([]));

			renderCommentsTab();

			await waitFor(() => {
				const addBtn = screen.getByRole('button', { name: /add comment/i });
				expectButtonComponent(addBtn);
				expect(addBtn).toHaveClass('btn-primary');
			});
		});

		it('close form button uses Button component with iconOnly', async () => {
			const user = userEvent.setup();
			vi.mocked(taskClient.listComments).mockResolvedValue(createMockListCommentsResponse([]));

			renderCommentsTab();

			// Open the comment form
			await waitFor(async () => {
				const addBtn = screen.getByRole('button', { name: /add comment/i });
				await user.click(addBtn);
			});

			// Close button should appear (aria-label="Close" for icon-only close button)
			// AMEND-001: Changed from /cancel/i to /close/i since "Close" is semantically
			// correct for an X icon button and avoids conflict with the Cancel text button
			await waitFor(() => {
				const closeBtn = screen.getByRole('button', { name: /close/i });
				expectIconOnlyButton(closeBtn);
			});
		});

		it('filter buttons use Button component with ghost variant', async () => {
			vi.mocked(taskClient.listComments).mockResolvedValue(
				createMockListCommentsResponse([
					createMockTaskComment({ id: 'comment-1' }),
				])
			);

			renderCommentsTab();

			// Wait for comments to load and filter bar to appear
			await waitFor(() => {
				const allFilter = screen.getByRole('button', { name: /all/i });
				expectButtonComponent(allFilter);
				expect(allFilter).toHaveClass('btn-ghost');
			});
		});

		it('edit comment button uses Button component with iconOnly', async () => {
			vi.mocked(taskClient.listComments).mockResolvedValue(
				createMockListCommentsResponse([
					createMockTaskComment({ id: 'comment-1' }),
				])
			);

			renderCommentsTab();

			await waitFor(() => {
				const editBtn = screen.getByRole('button', { name: /edit/i });
				expectIconOnlyButton(editBtn);
				expect(editBtn).toHaveClass('btn-ghost');
			});
		});

		it('delete comment button uses Button component with iconOnly and danger variant', async () => {
			vi.mocked(taskClient.listComments).mockResolvedValue(
				createMockListCommentsResponse([
					createMockTaskComment({ id: 'comment-1' }),
				])
			);

			renderCommentsTab();

			await waitFor(() => {
				const deleteBtn = screen.getByRole('button', { name: /delete/i });
				expectIconOnlyButton(deleteBtn);
				expect(deleteBtn).toHaveClass('btn-danger');
			});
		});

		it('form Cancel button uses Button component with secondary variant', async () => {
			const user = userEvent.setup();
			vi.mocked(taskClient.listComments).mockResolvedValue(createMockListCommentsResponse([]));

			renderCommentsTab();

			// Open form
			await waitFor(async () => {
				const addBtn = screen.getByRole('button', { name: /add comment/i });
				await user.click(addBtn);
			});

			await waitFor(() => {
				// Find Cancel button in form (not close icon button)
				const buttons = screen.getAllByRole('button');
				const cancelBtn = buttons.find(
					(btn) => btn.textContent?.toLowerCase().includes('cancel') && !btn.hasAttribute('aria-label')
				);
				expect(cancelBtn).toBeDefined();
				expectButtonComponent(cancelBtn!);
				expect(cancelBtn).toHaveClass('btn-secondary');
			});
		});

		it('form Submit button uses Button component with primary variant', async () => {
			const user = userEvent.setup();
			vi.mocked(taskClient.listComments).mockResolvedValue(createMockListCommentsResponse([]));

			renderCommentsTab();

			// Open form
			await waitFor(async () => {
				const addBtn = screen.getByRole('button', { name: /add comment/i });
				await user.click(addBtn);
			});

			await waitFor(() => {
				const submitBtn = screen.getByRole('button', { name: /add comment$/i });
				expectButtonComponent(submitBtn);
				expect(submitBtn).toHaveClass('btn-primary');
			});
		});
	});

	describe('SC-3: NewTaskModal.tsx Button Standardization', () => {
		const mockWorkflows = [
			createMockWorkflow({ id: 'small', name: 'Small', isBuiltin: true }),
			createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true }),
			createMockWorkflow({ id: 'large', name: 'Large', isBuiltin: true }),
		];

		beforeEach(() => {
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse(mockWorkflows)
			);
		});

		const renderNewTaskModal = () => {
			return render(
				<NewTaskModal open={true} onClose={vi.fn()} onCreate={vi.fn()} />
			);
		};

		it('Cancel button uses Button component with secondary variant', async () => {
			renderNewTaskModal();

			await waitFor(() => {
				const cancelBtn = screen.getByRole('button', { name: /cancel/i });
				// Button component MUST apply base 'btn' class - raw buttons only have variant classes
				expectButtonComponent(cancelBtn);
				// Should NOT have custom cancel-btn class when using Button component
				expect(cancelBtn).not.toHaveClass('cancel-btn');
				expect(cancelBtn).toHaveClass('btn-secondary');
			});
		});

		it('Create Task button uses Button component with primary variant', async () => {
			renderNewTaskModal();

			await waitFor(() => {
				const createBtn = screen.getByRole('button', { name: /create task/i });
				// Button component MUST apply base 'btn' class - raw buttons only have variant classes
				expectButtonComponent(createBtn);
				// Should NOT have custom save-btn class when using Button component
				expect(createBtn).not.toHaveClass('save-btn');
				expect(createBtn).toHaveClass('btn-primary');
			});
		});

		it('Create Task button shows loading state during save', async () => {
			const user = userEvent.setup();
			const mockTask = createMockTask({ id: 'TASK-001', title: 'Test' });
			vi.mocked(taskClient.createTask).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(createMockCreateTaskResponse(mockTask)), 100))
			);

			renderNewTaskModal();

			await waitFor(async () => {
				const titleInput = screen.getByLabelText(/title/i);
				await user.type(titleInput, 'Test Task');
			});

			const createBtn = screen.getByRole('button', { name: /create task/i });
			await user.click(createBtn);

			// Button should show loading state
			await waitFor(() => {
				expect(createBtn).toHaveClass('btn-loading');
				expect(createBtn).toHaveAttribute('aria-busy', 'true');
			});
		});

		it('Retry button (on workflow error) uses Button component', async () => {
			vi.mocked(workflowClient.listWorkflows).mockRejectedValue(new Error('Network error'));

			renderNewTaskModal();

			await waitFor(() => {
				const retryBtn = screen.getByRole('button', { name: /retry/i });
				expectButtonComponent(retryBtn);
			});
		});
	});

	describe('SC-4: Icon-only buttons have iconOnly prop and aria-label', () => {
		it('all icon-only buttons in TaskHeader have aria-label and use Button component', () => {
			render(
				<TooltipProvider delayDuration={0}>
					<MemoryRouter>
						<TaskHeader
							task={createMockTask({
								id: 'TASK-001',
								title: 'Test',
								status: TaskStatus.CREATED,
								weight: TaskWeight.SMALL,
								createdAt: createTimestamp('2024-01-01T00:00:00Z'),
								updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
							})}
							onTaskUpdate={vi.fn()}
							onTaskDelete={vi.fn()}
						/>
					</MemoryRouter>
				</TooltipProvider>
			);

			// Find all buttons with btn-icon-only class (from Button component)
			const buttons = screen.getAllByRole('button');
			const iconOnlyButtons = buttons.filter((btn) => btn.classList.contains('btn-icon-only'));

			// There should be at least 3 icon-only buttons: back, edit, delete
			expect(iconOnlyButtons.length).toBeGreaterThanOrEqual(3);

			// Each icon-only button must have aria-label
			iconOnlyButtons.forEach((btn) => {
				expect(btn).toHaveAttribute('aria-label');
				expect(btn.getAttribute('aria-label')).not.toBe('');
			});
		});

		it('icon-only buttons have btn-icon-only class', async () => {
			vi.mocked(taskClient.listComments).mockResolvedValue(
				createMockListCommentsResponse([
					createMockTaskComment({ id: 'comment-1', content: 'Test', author: 'User' }),
				])
			);

			render(
				<TooltipProvider delayDuration={0}>
					<CommentsTab taskId="TASK-001" />
				</TooltipProvider>
			);

			await waitFor(() => {
				// Edit and delete buttons should be icon-only
				const editBtn = screen.getByRole('button', { name: /edit/i });
				const deleteBtn = screen.getByRole('button', { name: /delete/i });

				expect(editBtn).toHaveClass('btn-icon-only');
				expect(deleteBtn).toHaveClass('btn-icon-only');
			});
		});
	});

	describe('SC-7: Loading states work correctly', () => {
		it('TaskHeader action buttons use Button component and disable during loading', async () => {
			const user = userEvent.setup();
			vi.mocked(taskClient.runTask).mockImplementation(
				() => new Promise((resolve) => setTimeout(resolve, 500))
			);

			render(
				<TooltipProvider delayDuration={0}>
					<MemoryRouter>
						<TaskHeader
							task={createMockTask({
								id: 'TASK-001',
								title: 'Test',
								status: TaskStatus.CREATED,
								weight: TaskWeight.SMALL,
								createdAt: createTimestamp('2024-01-01T00:00:00Z'),
								updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
							})}
							onTaskUpdate={vi.fn()}
							onTaskDelete={vi.fn()}
						/>
					</MemoryRouter>
				</TooltipProvider>
			);

			const runBtn = screen.getByRole('button', { name: /run/i });
			// Must use Button component
			expectButtonComponent(runBtn);
			await user.click(runBtn);

			// Button should be disabled during loading and show loading state
			await waitFor(() => {
				expect(runBtn).toBeDisabled();
				expect(runBtn).toHaveClass('btn-loading');
			});
		});

		it('NewTaskModal Create button shows spinner during save', async () => {
			const user = userEvent.setup();
			const mockTask = createMockTask({ id: 'TASK-001', title: 'Test' });
			vi.mocked(taskClient.createTask).mockImplementation(
				() => new Promise((resolve) => setTimeout(() => resolve(createMockCreateTaskResponse(mockTask)), 200))
			);
			vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
				createMockListWorkflowsResponse([createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true })])
			);

			render(<NewTaskModal open={true} onClose={vi.fn()} onCreate={vi.fn()} />);

			await waitFor(async () => {
				const titleInput = screen.getByLabelText(/title/i);
				await user.type(titleInput, 'Test Task');
			});

			const createBtn = screen.getByRole('button', { name: /create task/i });
			await user.click(createBtn);

			// Spinner should appear (Button component renders .btn-spinner when loading)
			await waitFor(() => {
				const spinner = createBtn.querySelector('.btn-spinner');
				expect(spinner).toBeInTheDocument();
			});
		});

		it('CommentForm submit button shows loading during submission', async () => {
			const user = userEvent.setup();
			vi.mocked(taskClient.listComments).mockResolvedValue(createMockListCommentsResponse([]));
			vi.mocked(taskClient.createComment).mockImplementation(
				() => new Promise((resolve) => setTimeout(resolve, 200))
			);

			render(
				<TooltipProvider delayDuration={0}>
					<CommentsTab taskId="TASK-001" />
				</TooltipProvider>
			);

			// Open form and fill it
			await waitFor(async () => {
				const addBtn = screen.getByRole('button', { name: /add comment/i });
				await user.click(addBtn);
			});

			await waitFor(async () => {
				const textarea = screen.getByLabelText(/comment/i);
				await user.type(textarea, 'Test comment content');
			});

			// Submit
			const submitBtn = screen.getByRole('button', { name: /add comment$/i });
			await user.click(submitBtn);

			// Button should show loading state
			await waitFor(() => {
				expect(submitBtn).toHaveClass('btn-loading');
				expect(submitBtn).toBeDisabled();
			});
		});
	});
});

/**
 * Tests for button variant consistency
 * Ensures buttons use appropriate variants for their semantic meaning
 */
describe('Button Variant Consistency', () => {
	it('destructive actions use Button component with danger variant', () => {
		render(
			<TooltipProvider delayDuration={0}>
				<MemoryRouter>
					<TaskHeader
						task={createMockTask({
							id: 'TASK-001',
							title: 'Test',
							status: TaskStatus.CREATED,
							weight: TaskWeight.SMALL,
							createdAt: createTimestamp('2024-01-01T00:00:00Z'),
							updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
						})}
						onTaskUpdate={vi.fn()}
						onTaskDelete={vi.fn()}
					/>
				</MemoryRouter>
			</TooltipProvider>
		);

		const deleteBtn = screen.getByRole('button', { name: /delete/i });
		// Must use Button component
		expectButtonComponent(deleteBtn);
		expect(deleteBtn).toHaveClass('btn-danger');
	});

	it('primary actions use Button component with primary variant', async () => {
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse([createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true })])
		);

		render(<NewTaskModal open={true} onClose={vi.fn()} onCreate={vi.fn()} />);

		await waitFor(() => {
			const createBtn = screen.getByRole('button', { name: /create task/i });
			// Must use Button component
			expectButtonComponent(createBtn);
			expect(createBtn).toHaveClass('btn-primary');
		});
	});

	it('cancel/dismiss actions use Button component with secondary variant', async () => {
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse([createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true })])
		);

		render(<NewTaskModal open={true} onClose={vi.fn()} onCreate={vi.fn()} />);

		await waitFor(() => {
			const cancelBtn = screen.getByRole('button', { name: /cancel/i });
			// Must use Button component
			expectButtonComponent(cancelBtn);
			expect(cancelBtn).toHaveClass('btn-secondary');
		});
	});
});

/**
 * Accessibility tests for standardized buttons
 */
describe('Button Accessibility', () => {
	it('all buttons use Button component and are keyboard accessible', () => {
		render(
			<TooltipProvider delayDuration={0}>
				<MemoryRouter>
					<TaskHeader
						task={createMockTask({
							id: 'TASK-001',
							title: 'Test',
							status: TaskStatus.CREATED,
							weight: TaskWeight.SMALL,
							createdAt: createTimestamp('2024-01-01T00:00:00Z'),
							updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
						})}
						onTaskUpdate={vi.fn()}
						onTaskDelete={vi.fn()}
					/>
				</MemoryRouter>
			</TooltipProvider>
		);

		const buttons = screen.getAllByRole('button');
		// All interactive buttons should use Button component (have 'btn' class)
		// Exclude special cases like export dropdown trigger
		const actionButtons = buttons.filter((btn) =>
			btn.title === 'Go back' ||
			btn.title === 'Run task' ||
			btn.title === 'Edit task' ||
			btn.title === 'Delete task'
		);

		// All action buttons must use Button component
		actionButtons.forEach((btn) => {
			expectButtonComponent(btn);
		});

		buttons.forEach((btn) => {
			// All buttons should be focusable (no tabIndex=-1 unless disabled)
			if (!btn.hasAttribute('disabled')) {
				expect(btn.tabIndex).not.toBe(-1);
			}
		});
	});

	it('loading buttons have aria-busy attribute', async () => {
		const user = userEvent.setup();
		vi.mocked(taskClient.runTask).mockImplementation(
			() => new Promise((resolve) => setTimeout(resolve, 100))
		);

		render(
			<TooltipProvider delayDuration={0}>
				<MemoryRouter>
					<TaskHeader
						task={createMockTask({
							id: 'TASK-001',
							title: 'Test',
							status: TaskStatus.CREATED,
							weight: TaskWeight.SMALL,
							createdAt: createTimestamp('2024-01-01T00:00:00Z'),
							updatedAt: createTimestamp('2024-01-01T00:00:00Z'),
						})}
						onTaskUpdate={vi.fn()}
						onTaskDelete={vi.fn()}
					/>
				</MemoryRouter>
			</TooltipProvider>
		);

		const runBtn = screen.getByRole('button', { name: /run/i });
		await user.click(runBtn);

		await waitFor(() => {
			expect(runBtn).toHaveAttribute('aria-busy', 'true');
		});
	});

	it('disabled buttons have aria-disabled attribute (Button component)', async () => {
		vi.mocked(workflowClient.listWorkflows).mockResolvedValue(
			createMockListWorkflowsResponse([createMockWorkflow({ id: 'medium', name: 'Medium', isBuiltin: true })])
		);

		render(<NewTaskModal open={true} onClose={vi.fn()} onCreate={vi.fn()} />);

		await waitFor(() => {
			// Create button should be disabled when title is empty
			const createBtn = screen.getByRole('button', { name: /create task/i });
			// Must use Button component (has 'btn' base class)
			expectButtonComponent(createBtn);
			expect(createBtn).toBeDisabled();
			expect(createBtn).toHaveAttribute('aria-disabled', 'true');
		});
	});
});
