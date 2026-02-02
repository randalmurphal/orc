/**
 * TDD Tests for TaskDetailsModal Component (Step 2 of workflow-first task creation)
 *
 * This test suite is written BEFORE implementation per TDD methodology.
 * Tests will fail until TaskDetailsModal is properly implemented.
 *
 * Success Criteria Coverage:
 * - SC-1: Component displays selected workflow with change button
 * - SC-2: Title field is required and has autofocus
 * - SC-3: Description field is optional
 * - SC-4: Advanced section is collapsible with all fields
 * - SC-5: Form validation prevents submission with empty title
 * - SC-6: Back button returns to workflow picker
 * - SC-7: Create button creates task and closes modal
 * - SC-8: Create & Run button creates task and starts execution
 * - SC-9: Change workflow button returns to Step 1
 * - SC-10: Component handles form state correctly
 * - SC-11: Component handles API errors for task creation
 * - SC-12: Component loads initiatives for dropdown
 * - SC-13: Component handles loading states
 */

import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { TaskDetailsModal } from './TaskDetailsModal';
import * as taskClient from '@/lib/client';
import * as stores from '@/stores';
import { TaskWeight, TaskCategory, TaskPriority, TaskQueue } from '@/gen/orc/v1/task_pb';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';

// Mock the client
vi.mock('@/lib/client', () => ({
	taskClient: {
		createTask: vi.fn(),
		runTask: vi.fn(),
	}
}));

// Mock stores
vi.mock('@/stores', () => ({
	useCurrentProjectId: vi.fn(),
	useInitiatives: vi.fn(),
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	}
}));

// Mock UI components
vi.mock('./Modal', () => ({
	Modal: ({ children, open, title, onClose }: any) => (
		<div data-testid="modal" data-open={open} data-title={title}>
			{open && (
				<>
					<div data-testid="modal-title">{title}</div>
					<button data-testid="modal-close" onClick={onClose}>×</button>
					{children}
				</>
			)}
		</div>
	)
}));

vi.mock('@/components/ui/Button', () => ({
	Button: ({ children, onClick, disabled, loading, variant, type, ...props }: any) => (
		<button
			onClick={onClick}
			disabled={disabled}
			data-loading={loading}
			data-variant={variant}
			type={type}
			{...props}
		>
			{loading ? 'Loading...' : children}
		</button>
	)
}));

vi.mock('@/components/ui/Icon', () => ({
	Icon: ({ name, size }: any) => (
		<span data-testid="icon" data-name={name} data-size={size}>
			{name}
		</span>
	)
}));

// Mock Radix UI Select
vi.mock('@radix-ui/react-select', () => ({
	Root: ({ children, value, onValueChange, open }: any) => (
		<div data-testid="select-root" data-value={value} data-open={open}>
			<div onClick={() => onValueChange?.(value)}>{children}</div>
		</div>
	),
	Trigger: ({ children, className, disabled, ...props }: any) => (
		<button
			className={className}
			disabled={disabled}
			data-testid="select-trigger"
			{...props}
		>
			{children}
		</button>
	),
	Value: ({ children }: any) => <div data-testid="select-value">{children}</div>,
	Icon: ({ children }: any) => <div data-testid="select-icon">{children}</div>,
	Portal: ({ children }: any) => <div data-testid="select-portal">{children}</div>,
	Content: ({ children, className }: any) => (
		<div className={className} data-testid="select-content">{children}</div>
	),
	Viewport: ({ children, className }: any) => (
		<div className={className} data-testid="select-viewport">{children}</div>
	),
	Item: ({ children, value, className }: any) => (
		<div
			className={className}
			data-testid="select-item"
			data-value={value}
		>
			{children}
		</div>
	),
	ItemText: ({ children }: any) => <span>{children}</span>,
	Separator: ({ className }: any) => <div className={className} data-testid="select-separator" />
}));

// Sample workflow data (from Step 1)
const sampleWorkflow = {
	id: 'implement-small',
	name: 'Implement (Small)',
	description: 'Small implementation workflow',
	isBuiltin: true,
	phaseCount: 3
};

// Sample initiatives
const sampleInitiatives = [
	{
		id: 'INIT-001',
		title: 'Test Initiative',
		status: InitiativeStatus.ACTIVE,
		vision: 'Test vision',
		createdAt: new Date(),
		updatedAt: new Date()
	},
	{
		id: 'INIT-002',
		title: 'Another Initiative',
		status: InitiativeStatus.DRAFT,
		vision: 'Another vision',
		createdAt: new Date(),
		updatedAt: new Date()
	}
];

describe('TaskDetailsModal (Step 2: Details)', () => {
	const defaultProps = {
		open: true,
		selectedWorkflow: sampleWorkflow,
		onClose: vi.fn(),
		onBack: vi.fn(),
		onTaskCreated: vi.fn()
	};

	beforeEach(() => {
		vi.clearAllMocks();
		(stores.useCurrentProjectId as any).mockReturnValue('test-project-id');
		(stores.useInitiatives as any).mockReturnValue(sampleInitiatives);
	});

	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('Component Rendering', () => {
		it('SC-1: displays selected workflow with change button', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			expect(screen.getByText('New Task')).toBeInTheDocument();
			expect(screen.getByText('Implement (Small)')).toBeInTheDocument();

			const changeButton = screen.getByRole('button', { name: /change/i });
			expect(changeButton).toBeInTheDocument();
		});

		it('SC-2: renders title field as required with autofocus', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			expect(titleInput).toBeInTheDocument();
			expect(titleInput).toBeRequired();
			expect(titleInput).toHaveFocus();
		});

		it('SC-3: renders optional description field', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			const descriptionTextarea = screen.getByLabelText(/description/i);
			expect(descriptionTextarea).toBeInTheDocument();
			expect(descriptionTextarea).not.toBeRequired();
			expect(descriptionTextarea.tagName.toLowerCase()).toBe('textarea');
		});

		it('SC-4: renders advanced section as collapsible with all fields', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			// Advanced section should exist and be collapsible
			const advancedSection = screen.getByText(/advanced/i);
			expect(advancedSection).toBeInTheDocument();

			// Initially collapsed, click to expand
			fireEvent.click(advancedSection);

			// Check all advanced fields are present
			expect(screen.getByLabelText(/category/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/priority/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/queue/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/initiative/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/target branch/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/branch name/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/pr draft/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/pr labels/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/pr reviewers/i)).toBeInTheDocument();
		});

		it('renders action buttons correctly', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			expect(screen.getByRole('button', { name: /back/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /^create$/i })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /create & run/i })).toBeInTheDocument();
		});
	});

	describe('Form State Management', () => {
		it('SC-10: resets form state when modal opens', () => {
			const { rerender } = render(<TaskDetailsModal {...defaultProps} open={false} />);

			// Modal closed initially
			expect(screen.queryByText('New Task')).not.toBeInTheDocument();

			// Open modal
			rerender(<TaskDetailsModal {...defaultProps} open={true} />);

			const titleInput = screen.getByLabelText(/title/i) as HTMLInputElement;
			const descriptionTextarea = screen.getByLabelText(/description/i) as HTMLTextAreaElement;

			expect(titleInput.value).toBe('');
			expect(descriptionTextarea.value).toBe('');
		});

		it('SC-10: preserves form state during editing', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const descriptionTextarea = screen.getByLabelText(/description/i);

			await user.type(titleInput, 'Test Task Title');
			await user.type(descriptionTextarea, 'Test description');

			expect(titleInput).toHaveValue('Test Task Title');
			expect(descriptionTextarea).toHaveValue('Test description');
		});

		it('SC-12: loads and displays initiatives in dropdown', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			// Expand advanced section
			const advancedSection = screen.getByText(/advanced/i);
			fireEvent.click(advancedSection);

			// Check initiatives are loaded
			const initiativeSelect = screen.getByLabelText(/initiative/i);
			expect(initiativeSelect).toBeInTheDocument();

			// Should have "None" option plus sample initiatives
			expect(screen.getByText('None')).toBeInTheDocument();
			expect(screen.getByText('Test Initiative')).toBeInTheDocument();
			expect(screen.getByText('Another Initiative')).toBeInTheDocument();
		});
	});

	describe('Form Validation', () => {
		it('SC-5: prevents submission with empty title', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const createButton = screen.getByRole('button', { name: /^create$/i });

			// Try to submit without title
			await user.click(createButton);

			// Should not call API
			expect(taskClient.taskClient.createTask).not.toHaveBeenCalled();

			// Should show validation error or disable button
			expect(createButton).toBeDisabled();
		});

		it('allows submission with valid title', async () => {
			const user = userEvent.setup();
			const mockCreateResponse = { task: { id: 'TASK-001', title: 'Test Task' } };
			(taskClient.taskClient.createTask as any).mockResolvedValue(mockCreateResponse);

			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createButton = screen.getByRole('button', { name: /^create$/i });

			await user.type(titleInput, 'Valid Task Title');

			// Button should be enabled with valid title
			expect(createButton).not.toBeDisabled();

			await user.click(createButton);

			// Should call API
			await waitFor(() => {
				expect(taskClient.taskClient.createTask).toHaveBeenCalledWith({
					projectId: 'test-project-id',
					title: 'Valid Task Title',
					workflowId: 'implement-small',
					weight: TaskWeight.SMALL, // Should derive from workflow
					category: TaskCategory.FEATURE,
					priority: TaskPriority.NORMAL,
					queue: TaskQueue.ACTIVE,
					description: '',
					initiativeId: undefined,
					targetBranch: '',
					blockedBy: [],
					relatedTo: [],
					metadata: {},
					branchName: '',
					prDraft: undefined,
					prLabels: [],
					prReviewers: [],
					prLabelsSet: false,
					prReviewersSet: false
				});
			});
		});
	});

	describe('Navigation Actions', () => {
		it('SC-6: back button returns to workflow picker', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const backButton = screen.getByRole('button', { name: /back/i });
			await user.click(backButton);

			expect(defaultProps.onBack).toHaveBeenCalled();
		});

		it('SC-9: change workflow button returns to Step 1', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const changeButton = screen.getByRole('button', { name: /change/i });
			await user.click(changeButton);

			expect(defaultProps.onBack).toHaveBeenCalled();
		});

		it('closes modal on close button click', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const closeButton = screen.getByTestId('modal-close');
			await user.click(closeButton);

			expect(defaultProps.onClose).toHaveBeenCalled();
		});
	});

	describe('Task Creation', () => {
		beforeEach(() => {
			const mockCreateResponse = { task: { id: 'TASK-001', title: 'Test Task' } };
			(taskClient.taskClient.createTask as any).mockResolvedValue(mockCreateResponse);
			(taskClient.taskClient.runTask as any).mockResolvedValue({ task: { id: 'TASK-001' } });
		});

		it('SC-7: Create button creates task and closes modal', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createButton = screen.getByRole('button', { name: /^create$/i });

			await user.type(titleInput, 'New Task');
			await user.click(createButton);

			await waitFor(() => {
				expect(taskClient.taskClient.createTask).toHaveBeenCalled();
				expect(defaultProps.onTaskCreated).toHaveBeenCalledWith(expect.objectContaining({ id: 'TASK-001' }), false);
			});
		});

		it('SC-8: Create & Run button creates task and starts execution', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createAndRunButton = screen.getByRole('button', { name: /create & run/i });

			await user.type(titleInput, 'New Task');
			await user.click(createAndRunButton);

			await waitFor(() => {
				expect(taskClient.taskClient.createTask).toHaveBeenCalled();
				expect(taskClient.taskClient.runTask).toHaveBeenCalledWith({
					projectId: 'test-project-id',
					taskId: 'TASK-001'
				});
				expect(defaultProps.onTaskCreated).toHaveBeenCalledWith(expect.objectContaining({ id: 'TASK-001' }), true);
			});
		});

		it('creates task with all form field data', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			// Fill basic fields
			const titleInput = screen.getByLabelText(/title/i);
			const descriptionTextarea = screen.getByLabelText(/description/i);

			await user.type(titleInput, 'Complete Task Title');
			await user.type(descriptionTextarea, 'Detailed description');

			// Expand advanced section
			const advancedSection = screen.getByText(/advanced/i);
			fireEvent.click(advancedSection);

			// Fill advanced fields
			const categorySelect = screen.getByLabelText(/category/i);
			fireEvent.change(categorySelect, { target: { value: String(TaskCategory.BUG) } });

			const prioritySelect = screen.getByLabelText(/priority/i);
			fireEvent.change(prioritySelect, { target: { value: String(TaskPriority.HIGH) } });

			const queueSelect = screen.getByLabelText(/queue/i);
			fireEvent.change(queueSelect, { target: { value: String(TaskQueue.BACKLOG) } });

			const targetBranchInput = screen.getByLabelText(/target branch/i);
			await user.type(targetBranchInput, 'develop');

			const branchNameInput = screen.getByLabelText(/branch name/i);
			await user.type(branchNameInput, 'custom-branch');

			const prDraftCheckbox = screen.getByLabelText(/pr draft/i);
			await user.click(prDraftCheckbox);

			const prLabelsInput = screen.getByLabelText(/pr labels/i);
			await user.type(prLabelsInput, 'bug,urgent');

			const prReviewersInput = screen.getByLabelText(/pr reviewers/i);
			await user.type(prReviewersInput, 'reviewer1,reviewer2');

			const createButton = screen.getByRole('button', { name: /^create$/i });
			await user.click(createButton);

			await waitFor(() => {
				expect(taskClient.taskClient.createTask).toHaveBeenCalledWith({
					projectId: 'test-project-id',
					title: 'Complete Task Title',
					description: 'Detailed description',
					workflowId: 'implement-small',
					weight: TaskWeight.SMALL,
					category: TaskCategory.BUG,
					priority: TaskPriority.HIGH,
					queue: TaskQueue.BACKLOG,
					initiativeId: undefined,
					targetBranch: 'develop',
					blockedBy: [],
					relatedTo: [],
					metadata: {},
					branchName: 'custom-branch',
					prDraft: true,
					prLabels: ['bug', 'urgent'],
					prReviewers: ['reviewer1', 'reviewer2'],
					prLabelsSet: true,
					prReviewersSet: true
				});
			});
		});
	});

	describe('Error Handling', () => {
		it('SC-11: handles API error for task creation', async () => {
			const user = userEvent.setup();
			const mockError = new Error('Network error');
			(taskClient.taskClient.createTask as any).mockRejectedValue(mockError);

			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createButton = screen.getByRole('button', { name: /^create$/i });

			await user.type(titleInput, 'Test Task');
			await user.click(createButton);

			await waitFor(() => {
				expect(stores.toast.error).toHaveBeenCalledWith('Network error');
			});

			// Should not close modal on error
			expect(defaultProps.onClose).not.toHaveBeenCalled();
			expect(defaultProps.onTaskCreated).not.toHaveBeenCalled();
		});

		it('SC-11: handles API error for run task', async () => {
			const user = userEvent.setup();
			const mockCreateResponse = { task: { id: 'TASK-001', title: 'Test Task' } };
			const mockRunError = new Error('Run failed');

			(taskClient.taskClient.createTask as any).mockResolvedValue(mockCreateResponse);
			(taskClient.taskClient.runTask as any).mockRejectedValue(mockRunError);

			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createAndRunButton = screen.getByRole('button', { name: /create & run/i });

			await user.type(titleInput, 'Test Task');
			await user.click(createAndRunButton);

			await waitFor(() => {
				expect(stores.toast.error).toHaveBeenCalledWith('Run failed');
			});

			// Task should still be created successfully
			expect(defaultProps.onTaskCreated).toHaveBeenCalledWith(expect.objectContaining({ id: 'TASK-001' }), false);
		});
	});

	describe('Loading States', () => {
		it('SC-13: shows loading state during task creation', async () => {
			const user = userEvent.setup();
			let resolveCreate: (value: any) => void;
			const createPromise = new Promise(resolve => { resolveCreate = resolve; });
			(taskClient.taskClient.createTask as any).mockImplementation(() => createPromise);

			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createButton = screen.getByRole('button', { name: /^create$/i });

			await user.type(titleInput, 'Test Task');
			await user.click(createButton);

			// Should show loading state
			expect(createButton).toHaveAttribute('data-loading', 'true');
			expect(createButton).toBeDisabled();
			expect(screen.getByText('Creating...')).toBeInTheDocument();

			// Resolve promise
			resolveCreate!({ task: { id: 'TASK-001' } });

			await waitFor(() => {
				expect(createButton).not.toHaveAttribute('data-loading', 'true');
			});
		});

		it('SC-13: shows loading state during create and run', async () => {
			const user = userEvent.setup();
			let resolveRun: (value: any) => void;
			const runPromise = new Promise(resolve => { resolveRun = resolve; });

			(taskClient.taskClient.createTask as any).mockResolvedValue({ task: { id: 'TASK-001' } });
			(taskClient.taskClient.runTask as any).mockImplementation(() => runPromise);

			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createAndRunButton = screen.getByRole('button', { name: /create & run/i });

			await user.type(titleInput, 'Test Task');
			await user.click(createAndRunButton);

			// Should show loading state
			await waitFor(() => {
				expect(createAndRunButton).toHaveAttribute('data-loading', 'true');
				expect(screen.getByText('Creating & Running...')).toBeInTheDocument();
			});

			// Resolve promise
			resolveRun!({ task: { id: 'TASK-001' } });
		});
	});

	describe('Advanced Section Toggle', () => {
		it('toggles advanced section visibility', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			const advancedToggle = screen.getByText(/advanced/i);

			// Initially collapsed - advanced fields not visible
			expect(screen.queryByLabelText(/category/i)).not.toBeInTheDocument();

			// Click to expand
			await user.click(advancedToggle);

			// Advanced fields should be visible
			expect(screen.getByLabelText(/category/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/priority/i)).toBeInTheDocument();

			// Click to collapse
			await user.click(advancedToggle);

			// Advanced fields should be hidden again
			expect(screen.queryByLabelText(/category/i)).not.toBeInTheDocument();
		});
	});

	describe('Modal Integration', () => {
		it('renders in modal with correct title', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			const modal = screen.getByTestId('modal');
			expect(modal).toHaveAttribute('data-open', 'true');
			expect(modal).toHaveAttribute('data-title', 'New Task');
		});

		it('does not render when closed', () => {
			render(<TaskDetailsModal {...defaultProps} open={false} />);

			const modal = screen.getByTestId('modal');
			expect(modal).toHaveAttribute('data-open', 'false');
			expect(screen.queryByText('Implement (Small)')).not.toBeInTheDocument();
		});
	});

	describe('Keyboard Interactions', () => {
		it('supports Enter key to submit form', async () => {
			const user = userEvent.setup();
			const mockCreateResponse = { task: { id: 'TASK-001', title: 'Test Task' } };
			(taskClient.taskClient.createTask as any).mockResolvedValue(mockCreateResponse);

			render(<TaskDetailsModal {...defaultProps} />);

			const titleInput = screen.getByLabelText(/title/i);
			await user.type(titleInput, 'Test Task{enter}');

			await waitFor(() => {
				expect(taskClient.taskClient.createTask).toHaveBeenCalled();
			});
		});

		it('supports Escape key to close modal', async () => {
			const user = userEvent.setup();
			render(<TaskDetailsModal {...defaultProps} />);

			await user.keyboard('{Escape}');

			expect(defaultProps.onClose).toHaveBeenCalled();
		});
	});

	describe('Form Field Defaults', () => {
		it('uses correct default values for all fields', () => {
			render(<TaskDetailsModal {...defaultProps} />);

			// Expand advanced section to check defaults
			const advancedSection = screen.getByText(/advanced/i);
			fireEvent.click(advancedSection);

			const categorySelect = screen.getByLabelText(/category/i) as HTMLSelectElement;
			const prioritySelect = screen.getByLabelText(/priority/i) as HTMLSelectElement;
			const queueSelect = screen.getByLabelText(/queue/i) as HTMLSelectElement;

			expect(categorySelect.value).toBe(String(TaskCategory.FEATURE));
			expect(prioritySelect.value).toBe(String(TaskPriority.NORMAL));
			expect(queueSelect.value).toBe(String(TaskQueue.ACTIVE));
		});

		it('derives weight from selected workflow', async () => {
			const user = userEvent.setup();
			const mockCreateResponse = { task: { id: 'TASK-001', title: 'Test Task' } };
			(taskClient.taskClient.createTask as any).mockResolvedValue(mockCreateResponse);

			// Test with large workflow
			const largeWorkflow = {
				...sampleWorkflow,
				id: 'implement-large',
				name: 'Implement (Large)'
			};

			render(<TaskDetailsModal {...defaultProps} selectedWorkflow={largeWorkflow} />);

			const titleInput = screen.getByLabelText(/title/i);
			const createButton = screen.getByRole('button', { name: /^create$/i });

			await user.type(titleInput, 'Test Task');
			await user.click(createButton);

			await waitFor(() => {
				expect(taskClient.taskClient.createTask).toHaveBeenCalledWith(
					expect.objectContaining({
						weight: TaskWeight.LARGE
					})
				);
			});
		});
	});
});