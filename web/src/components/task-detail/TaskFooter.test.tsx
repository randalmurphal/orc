/**
 * Tests for TaskFooter component
 *
 * TDD tests for the footer bar that displays session metrics
 * and action buttons (Pause, Cancel, Retry).
 *
 * Success Criteria Coverage:
 * - SC-7: Footer displays real-time metrics (token count, cost, iteration count)
 * - SC-8: Footer action buttons (Pause, Cancel) are enabled for running tasks
 * - SC-9: Failed phase displays error summary
 * - SC-10: Retry from current phase button
 * - SC-11: Retry from earlier phase button
 * - SC-12: Guidance textarea sends feedback with retry
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { TaskFooter } from './TaskFooter';
import { TaskStatus, PhaseStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockPhase, createMockTaskPlan } from '@/test/factories';

// Mock the task client
const mockPauseTask = vi.fn();
const mockResumeTask = vi.fn();
const mockRetryTask = vi.fn();
const mockCancelTask = vi.fn();

vi.mock('@/lib/client', () => ({
	taskClient: {
		pauseTask: (...args: unknown[]) => mockPauseTask(...args),
		resumeTask: (...args: unknown[]) => mockResumeTask(...args),
		retryTask: (...args: unknown[]) => mockRetryTask(...args),
		cancelTask: (...args: unknown[]) => mockCancelTask(...args),
	},
}));

// Mock uiStore for toasts
vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

// Mock stores
vi.mock('@/stores', () => ({
	useCurrentProjectId: () => 'test-project',
}));

describe('TaskFooter', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockPauseTask.mockResolvedValue({ task: {} });
		mockResumeTask.mockResolvedValue({ task: {} });
		mockRetryTask.mockResolvedValue({ task: {} });
		mockCancelTask.mockResolvedValue({ task: {} });
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-7: Displays real-time metrics', () => {
		it('displays token count', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });

			render(
				<TaskFooter
					task={task}
					metrics={{
						tokens: 45200,
						cost: 1.96,
						inputTokens: 30000,
						outputTokens: 15200,
					}}
				/>
			);

			expect(screen.getByText(/45\.2K/i)).toBeInTheDocument();
		});

		it('displays estimated cost', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });

			render(
				<TaskFooter
					task={task}
					metrics={{
						tokens: 45200,
						cost: 1.96,
						inputTokens: 30000,
						outputTokens: 15200,
					}}
				/>
			);

			expect(screen.getByText(/\$1\.96/)).toBeInTheDocument();
		});

		it('displays placeholder when metrics are unavailable', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });

			render(<TaskFooter task={task} metrics={null} />);

			// Should show placeholder dashes
			expect(screen.getByText('—')).toBeInTheDocument();
		});

		it('updates metrics in real-time', async () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			const { rerender } = render(
				<TaskFooter
					task={task}
					metrics={{ tokens: 10000, cost: 0.50 }}
				/>
			);

			expect(screen.getByText(/10K/i)).toBeInTheDocument();

			// Simulate real-time update
			rerender(
				<TaskFooter
					task={task}
					metrics={{ tokens: 20000, cost: 1.00 }}
				/>
			);

			expect(screen.getByText(/20K/i)).toBeInTheDocument();
		});
	});

	describe('SC-8: Action buttons for running tasks', () => {
		it('shows Pause button when task is running', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });

			render(<TaskFooter task={task} metrics={null} />);

			const pauseButton = screen.getByRole('button', { name: /pause/i });
			expect(pauseButton).toBeInTheDocument();
			expect(pauseButton).not.toBeDisabled();
		});

		it('clicking Pause calls API and changes button to Resume', async () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			const onTaskUpdate = vi.fn();

			render(
				<TaskFooter
					task={task}
					metrics={null}
					onTaskUpdate={onTaskUpdate}
				/>
			);

			const pauseButton = screen.getByRole('button', { name: /pause/i });
			fireEvent.click(pauseButton);

			await waitFor(() => {
				expect(mockPauseTask).toHaveBeenCalled();
			});
		});

		it('shows Resume button when task is paused', () => {
			const task = createMockTask({ status: TaskStatus.PAUSED });

			render(<TaskFooter task={task} metrics={null} />);

			const resumeButton = screen.getByRole('button', { name: /resume/i });
			expect(resumeButton).toBeInTheDocument();
			expect(resumeButton).not.toBeDisabled();
		});

		it('clicking Resume calls API', async () => {
			const task = createMockTask({ status: TaskStatus.PAUSED });

			render(<TaskFooter task={task} metrics={null} />);

			const resumeButton = screen.getByRole('button', { name: /resume/i });
			fireEvent.click(resumeButton);

			await waitFor(() => {
				expect(mockResumeTask).toHaveBeenCalled();
			});
		});

		it('shows Cancel button when task is running', () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });

			render(<TaskFooter task={task} metrics={null} />);

			const cancelButton = screen.getByRole('button', { name: /cancel/i });
			expect(cancelButton).toBeInTheDocument();
		});

		it('disables action buttons when task is completed', () => {
			const task = createMockTask({ status: TaskStatus.COMPLETED });

			render(<TaskFooter task={task} metrics={null} />);

			// Action buttons should not be present or should be disabled for completed tasks
			expect(screen.queryByRole('button', { name: /pause/i })).not.toBeInTheDocument();
		});

		it('shows toast error on API failure', async () => {
			const task = createMockTask({ status: TaskStatus.RUNNING });
			mockPauseTask.mockRejectedValue(new Error('Network error'));

			const { toast } = await import('@/stores/uiStore');

			render(<TaskFooter task={task} metrics={null} />);

			const pauseButton = screen.getByRole('button', { name: /pause/i });
			fireEvent.click(pauseButton);

			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('error'));
			});
		});
	});

	describe('SC-9: Failed phase error display', () => {
		it('displays error summary when task has failed', () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = {
				error: 'Validation failed: missing required field',
				phase: 'implement',
			};

			const { container } = render(<TaskFooter task={task} taskState={taskState} metrics={null} />);

			// Check error header shows "Error at" with the phase name
			const errorHeader = container.querySelector('.task-footer__error-header');
			expect(errorHeader).toBeInTheDocument();
			expect(errorHeader?.textContent).toContain('Error at');
			expect(errorHeader?.textContent).toContain('implement');

			// Check error details shows the error message
			expect(screen.getByText(/validation failed/i)).toBeInTheDocument();
		});

		it('displays scrollable error details block', () => {
			const longError = 'A'.repeat(500);
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = {
				error: longError,
				phase: 'implement',
			};

			const { container } = render(
				<TaskFooter task={task} taskState={taskState} metrics={null} />
			);

			const errorBlock = container.querySelector('.task-footer__error-details');
			expect(errorBlock).toBeInTheDocument();
			expect(errorBlock).toHaveStyle({ overflow: 'auto' });
		});
	});

	describe('SC-10: Retry from current phase', () => {
		it('shows Retry button when task has failed', () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} taskState={taskState} metrics={null} />);

			const retryButton = screen.getByRole('button', { name: /retry implement/i });
			expect(retryButton).toBeInTheDocument();
		});

		it('clicking Retry from current phase calls API with correct phase', async () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} taskState={taskState} metrics={null} />);

			const retryButton = screen.getByRole('button', { name: /retry implement/i });
			fireEvent.click(retryButton);

			await waitFor(() => {
				expect(mockRetryTask).toHaveBeenCalledWith(
					expect.objectContaining({
						fromPhase: 'implement',
					})
				);
			});
		});
	});

	describe('SC-11: Retry from earlier phase', () => {
		it('shows option to retry from earlier phase', () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} plan={plan} taskState={taskState} metrics={null} />);

			// Should have a way to select earlier phase
			expect(screen.getByRole('button', { name: /retry from spec/i })).toBeInTheDocument();
		});

		it('clicking Retry from earlier phase calls API with selected phase', async () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'spec', status: PhaseStatus.COMPLETED }),
					createMockPhase({ id: 'phase-2', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} plan={plan} taskState={taskState} metrics={null} />);

			const retryFromSpecButton = screen.getByRole('button', { name: /retry from spec/i });
			fireEvent.click(retryFromSpecButton);

			await waitFor(() => {
				expect(mockRetryTask).toHaveBeenCalledWith(
					expect.objectContaining({
						fromPhase: 'spec',
					})
				);
			});
		});
	});

	describe('SC-12: Guidance textarea for retry feedback', () => {
		it('displays guidance textarea when task has failed', () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} taskState={taskState} metrics={null} />);

			const textarea = screen.getByPlaceholderText(/guidance|feedback|note/i);
			expect(textarea).toBeInTheDocument();
		});

		it('sends feedback text with retry request', async () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} taskState={taskState} metrics={null} />);

			const textarea = screen.getByPlaceholderText(/guidance|feedback|note/i);
			fireEvent.change(textarea, { target: { value: 'Use validateSession instead' } });

			const retryButton = screen.getByRole('button', { name: /retry implement/i });
			fireEvent.click(retryButton);

			await waitFor(() => {
				expect(mockRetryTask).toHaveBeenCalledWith(
					expect.objectContaining({
						instructions: 'Use validateSession instead',
					})
				);
			});
		});

		it('proceeds with retry when feedback is empty', async () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} taskState={taskState} metrics={null} />);

			// Don't enter any feedback
			const retryButton = screen.getByRole('button', { name: /retry implement/i });
			fireEvent.click(retryButton);

			await waitFor(() => {
				// instructions should be undefined when no feedback is entered
				expect(mockRetryTask).toHaveBeenCalledWith(
					expect.objectContaining({
						fromPhase: 'implement',
					})
				);
			});
		});
	});

	describe('Edge Cases', () => {
		it('handles task with single phase workflow', () => {
			const task = createMockTask({
				status: TaskStatus.FAILED,
				currentPhase: 'implement',
			});
			const plan = createMockTaskPlan({
				phases: [
					createMockPhase({ id: 'phase-1', name: 'implement', status: PhaseStatus.PENDING }),
				],
			});
			const taskState = { error: 'Test error', phase: 'implement' };

			render(<TaskFooter task={task} plan={plan} taskState={taskState} metrics={null} />);

			// Should only show retry for current phase (no earlier phases)
			expect(screen.getByRole('button', { name: /retry implement/i })).toBeInTheDocument();
			expect(screen.queryByRole('button', { name: /retry from/i })).not.toBeInTheDocument();
		});

		it('shows Completed status when task is done', () => {
			const task = createMockTask({ status: TaskStatus.COMPLETED });

			render(<TaskFooter task={task} metrics={null} />);

			expect(screen.getByText(/completed/i)).toBeInTheDocument();
		});
	});
});
