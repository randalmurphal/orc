/**
 * Tests for FeedbackPanel component
 *
 * TDD tests for the feedback panel that allows users to provide feedback
 * to agents during task execution with timing controls.
 *
 * Success Criteria Coverage:
 * - SC-1: Feedback panel displays list of existing feedback for current task
 * - SC-2: Users can create new feedback with different types (GENERAL, INLINE, APPROVAL, DIRECTION)
 * - SC-3: Users can select feedback timing (NOW, WHEN_DONE, MANUAL)
 * - SC-4: Users can add inline comments targeting specific files and lines
 * - SC-5: Users can send all pending feedback to the agent
 * - SC-6: Users can delete specific feedback items
 * - SC-7: NOW timing feedback immediately pauses task execution
 * - SC-8: WHEN_DONE feedback is queued until phase completion
 * - SC-9: Feedback panel shows real-time status updates via WebSocket
 * - SC-10: Form validation prevents invalid feedback submission
 * - SC-11: Error handling for API failures
 * - SC-12: Accessibility compliance (keyboard navigation, ARIA labels)
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, act, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { FeedbackPanel } from './FeedbackPanel';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { createMockTask, createMockFeedback } from '@/test/factories';

// Mock the feedback client
const mockAddFeedback = vi.fn();
const mockListFeedback = vi.fn();
const mockSendFeedback = vi.fn();
const mockDeleteFeedback = vi.fn();

vi.mock('@/lib/client', () => ({
	feedbackClient: {
		addFeedback: (...args: unknown[]) => mockAddFeedback(...args),
		listFeedback: (...args: unknown[]) => mockListFeedback(...args),
		sendFeedback: (...args: unknown[]) => mockSendFeedback(...args),
		deleteFeedback: (...args: unknown[]) => mockDeleteFeedback(...args),
	},
	taskClient: {
		pauseTask: vi.fn(),
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
	useWebSocket: () => ({
		on: vi.fn(),
		off: vi.fn(),
	}),
}));

// Mock task update callback
const mockOnTaskUpdate = vi.fn();

describe('FeedbackPanel', () => {
	const mockTask = createMockTask({
		id: 'TASK-123',
		status: TaskStatus.RUNNING,
	});

	beforeEach(() => {
		vi.clearAllMocks();
		// Default mock implementations
		mockListFeedback.mockResolvedValue({ feedback: [] });
		mockAddFeedback.mockResolvedValue({ feedback: createMockFeedback() });
		mockSendFeedback.mockResolvedValue({ sentCount: 1 });
		mockDeleteFeedback.mockResolvedValue({});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-1: Display existing feedback list', () => {
		it('renders feedback panel with title', () => {
			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			expect(screen.getByRole('heading', { name: /feedback/i })).toBeInTheDocument();
		});

		it('displays list of existing feedback items', async () => {
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					text: 'Please add error handling',
					type: FeedbackType.GENERAL,
					timing: FeedbackTiming.WHEN_DONE,
					received: false,
				}),
				createMockFeedback({
					id: 'feedback-2',
					text: 'LGTM',
					type: FeedbackType.APPROVAL,
					timing: FeedbackTiming.NOW,
					received: true,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByText('Please add error handling')).toBeInTheDocument();
				expect(screen.getByText('LGTM')).toBeInTheDocument();
			});

			expect(mockListFeedback).toHaveBeenCalledWith({
				projectId: 'test-project',
				taskId: 'TASK-123',
				excludeReceived: false,
			});
		});

		it('shows feedback status indicators (received/pending)', async () => {
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					text: 'Pending feedback',
					received: false,
				}),
				createMockFeedback({
					id: 'feedback-2',
					text: 'Received feedback',
					received: true,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByLabelText(/pending feedback/i)).toBeInTheDocument();
				expect(screen.getByLabelText(/received feedback/i)).toBeInTheDocument();
			});
		});
	});

	describe('SC-2: Create feedback with different types', () => {
		it('displays feedback creation form', () => {
			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			expect(screen.getByLabelText(/feedback text/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/feedback type/i)).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /add feedback/i })).toBeInTheDocument();
		});

		it('allows selecting GENERAL feedback type', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'GENERAL');

			expect(typeSelect).toHaveValue('GENERAL');
		});

		it('allows selecting INLINE feedback type', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'INLINE');

			expect(typeSelect).toHaveValue('INLINE');
		});

		it('allows selecting APPROVAL feedback type', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'APPROVAL');

			expect(typeSelect).toHaveValue('APPROVAL');
		});

		it('allows selecting DIRECTION feedback type', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'DIRECTION');

			expect(typeSelect).toHaveValue('DIRECTION');
		});
	});

	describe('SC-3: Select feedback timing', () => {
		it('displays timing options', () => {
			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			expect(screen.getByLabelText(/timing/i)).toBeInTheDocument();
		});

		it('allows selecting NOW timing', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'NOW');

			expect(timingSelect).toHaveValue('NOW');
		});

		it('allows selecting WHEN_DONE timing', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'WHEN_DONE');

			expect(timingSelect).toHaveValue('WHEN_DONE');
		});

		it('allows selecting MANUAL timing', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'MANUAL');

			expect(timingSelect).toHaveValue('MANUAL');
		});
	});

	describe('SC-4: Inline comments with file/line targeting', () => {
		it('shows file and line inputs when INLINE type is selected', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'INLINE');

			expect(screen.getByLabelText(/file path/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/line number/i)).toBeInTheDocument();
		});

		it('hides file and line inputs for non-INLINE types', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'GENERAL');

			expect(screen.queryByLabelText(/file path/i)).not.toBeInTheDocument();
			expect(screen.queryByLabelText(/line number/i)).not.toBeInTheDocument();
		});

		it('creates inline feedback with file and line data', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			// Set up inline feedback
			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'INLINE');

			await user.type(screen.getByLabelText(/feedback text/i), 'Fix this bug');
			await user.type(screen.getByLabelText(/file path/i), 'src/main.go');
			await user.type(screen.getByLabelText(/line number/i), '42');

			await user.click(screen.getByRole('button', { name: /add feedback/i }));

			await waitFor(() => {
				expect(mockAddFeedback).toHaveBeenCalledWith({
					projectId: 'test-project',
					taskId: 'TASK-123',
					type: FeedbackType.INLINE,
					text: 'Fix this bug',
					timing: FeedbackTiming.WHEN_DONE, // default
					file: 'src/main.go',
					line: 42,
				});
			});
		});
	});

	describe('SC-5: Send pending feedback', () => {
		it('displays send button when pending feedback exists', async () => {
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					received: false,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /send pending/i })).toBeInTheDocument();
			});
		});

		it('hides send button when no pending feedback exists', async () => {
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					received: true,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.queryByRole('button', { name: /send pending/i })).not.toBeInTheDocument();
			});
		});

		it('sends all pending feedback when send button is clicked', async () => {
			const user = userEvent.setup();
			const mockFeedback = [
				createMockFeedback({ id: 'feedback-1', received: false }),
				createMockFeedback({ id: 'feedback-2', received: false }),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /send pending/i })).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /send pending/i }));

			await waitFor(() => {
				expect(mockSendFeedback).toHaveBeenCalledWith({
					projectId: 'test-project',
					taskId: 'TASK-123',
				});
			});
		});
	});

	describe('SC-6: Delete specific feedback', () => {
		it('displays delete button for each feedback item', async () => {
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					text: 'Test feedback',
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByLabelText(/delete feedback/i)).toBeInTheDocument();
			});
		});

		it('deletes feedback when delete button is clicked', async () => {
			const user = userEvent.setup();
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					text: 'Test feedback',
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByLabelText(/delete feedback/i)).toBeInTheDocument();
			});

			await user.click(screen.getByLabelText(/delete feedback/i));

			await waitFor(() => {
				expect(mockDeleteFeedback).toHaveBeenCalledWith({
					projectId: 'test-project',
					taskId: 'TASK-123',
					feedbackId: 'feedback-1',
				});
			});
		});
	});

	describe('SC-7: NOW timing pauses task', () => {
		it('shows warning when NOW timing is selected for running task', async () => {
			const user = userEvent.setup();
			const runningTask = createMockTask({
				id: 'TASK-123',
				status: TaskStatus.RUNNING,
			});

			await act(async () => {
				render(<FeedbackPanel task={runningTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'NOW');

			expect(screen.getByText(/will pause the task/i)).toBeInTheDocument();
		});

		it('does not show warning when NOW timing is selected for non-running task', async () => {
			const user = userEvent.setup();
			const completedTask = createMockTask({
				id: 'TASK-123',
				status: TaskStatus.COMPLETED,
			});

			await act(async () => {
				render(<FeedbackPanel task={completedTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'NOW');

			expect(screen.queryByText(/will pause the task/i)).not.toBeInTheDocument();
		});
	});

	describe('SC-8: WHEN_DONE queued feedback', () => {
		it('shows info about WHEN_DONE timing', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'WHEN_DONE');

			expect(screen.getByText(/queued until phase completion/i)).toBeInTheDocument();
		});
	});

	describe('SC-10: Form validation', () => {
		it('prevents submission with empty feedback text', async () => {
			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			const form = document.querySelector('form')!;
			fireEvent.submit(form);

			await waitFor(() => {
				expect(mockAddFeedback).not.toHaveBeenCalled();
				expect(screen.getByText(/feedback text is required/i)).toBeInTheDocument();
			});
		});


		it('prevents submission of inline feedback without file path', async () => {
			const user = userEvent.setup();

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'INLINE');

			await user.type(screen.getByLabelText(/feedback text/i), 'Fix this');
			await user.type(screen.getByLabelText(/line number/i), '42');

			await user.click(screen.getByRole('button', { name: /add feedback/i }));

			await waitFor(() => {
				expect(mockAddFeedback).not.toHaveBeenCalled();
				expect(screen.getByText(/file path is required for inline comments/i)).toBeInTheDocument();
			});
		});

		it('prevents submission of inline feedback with invalid line number', async () => {
			const user = userEvent.setup();

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'INLINE');

			await user.type(screen.getByLabelText(/feedback text/i), 'Fix this');
			await user.type(screen.getByLabelText(/file path/i), 'src/main.go');
			await user.type(screen.getByLabelText(/line number/i), '-5');

			const form = document.querySelector('form')!;
			fireEvent.submit(form);

			await waitFor(() => {
				expect(mockAddFeedback).not.toHaveBeenCalled();
				expect(screen.getByText(/line number must be positive/i)).toBeInTheDocument();
			});
		});
	});

	describe('SC-11: Error handling', () => {
		it('displays error message when feedback creation fails', async () => {
			const user = userEvent.setup();
			mockAddFeedback.mockRejectedValue(new Error('Network error'));

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await user.type(screen.getByLabelText(/feedback text/i), 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add feedback/i }));

			await waitFor(() => {
				expect(screen.getByText(/failed to add feedback/i)).toBeInTheDocument();
			});
		});

		it('displays error message when feedback list loading fails', async () => {
			mockListFeedback.mockRejectedValue(new Error('Database error'));

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByText(/failed to load feedback/i)).toBeInTheDocument();
			});
		});

		it('displays error message when sending feedback fails', async () => {
			const user = userEvent.setup();
			const mockFeedback = [createMockFeedback({ received: false })];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });
			mockSendFeedback.mockRejectedValue(new Error('API error'));

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /send pending/i })).toBeInTheDocument();
			});

			await user.click(screen.getByRole('button', { name: /send pending/i }));

			await waitFor(() => {
				expect(screen.getByText(/failed to send feedback/i)).toBeInTheDocument();
			});
		});
	});

	describe('SC-12: Accessibility', () => {
		it('has proper ARIA labels for form controls', () => {
			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			expect(screen.getByLabelText(/feedback text/i)).toHaveAttribute('aria-describedby');
			expect(screen.getByLabelText(/feedback type/i)).toHaveAttribute('aria-describedby');
			expect(screen.getByLabelText(/timing/i)).toHaveAttribute('aria-describedby');
		});

		it('supports keyboard navigation', async () => {
			const user = userEvent.setup();
			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			const textArea = screen.getByLabelText(/feedback text/i);
			const typeSelect = screen.getByLabelText(/feedback type/i);
			const timingSelect = screen.getByLabelText(/timing/i);
			const submitButton = screen.getByRole('button', { name: /add feedback/i });

			// Should be able to tab through form controls
			textArea.focus();
			await user.tab();
			expect(typeSelect).toHaveFocus();
			await user.tab();
			expect(timingSelect).toHaveFocus();
			await user.tab();
			expect(submitButton).toHaveFocus();
		});

		it('announces feedback status to screen readers', async () => {
			const mockFeedback = [
				createMockFeedback({
					id: 'feedback-1',
					text: 'Test feedback',
					received: false,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: mockFeedback });

			render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);

			await waitFor(() => {
				expect(screen.getByRole('status')).toBeInTheDocument();
				expect(screen.getByRole('status')).toHaveTextContent(/1 pending feedback/i);
			});
		});
	});

	describe('Form interaction flows', () => {
		it('successfully creates general feedback', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			await user.type(screen.getByLabelText(/feedback text/i), 'Great work!');

			const typeSelect = screen.getByLabelText(/feedback type/i);
			await user.selectOptions(typeSelect, 'GENERAL');

			const timingSelect = screen.getByLabelText(/timing/i);
			await user.selectOptions(timingSelect, 'WHEN_DONE');

			await user.click(screen.getByRole('button', { name: /add feedback/i }));

			await waitFor(() => {
				expect(mockAddFeedback).toHaveBeenCalledWith({
					projectId: 'test-project',
					taskId: 'TASK-123',
					type: FeedbackType.GENERAL,
					text: 'Great work!',
					timing: FeedbackTiming.WHEN_DONE,
					file: '',
					line: 0,
				});
			});
		});

		it('clears form after successful submission', async () => {
			const user = userEvent.setup();

			await act(async () => {
				render(<FeedbackPanel task={mockTask} onTaskUpdate={mockOnTaskUpdate} />);
			});

			const textArea = screen.getByLabelText(/feedback text/i);
			await user.type(textArea, 'Test feedback');
			await user.click(screen.getByRole('button', { name: /add feedback/i }));

			await waitFor(() => {
				expect(textArea).toHaveValue('');
			});
		});
	});
});