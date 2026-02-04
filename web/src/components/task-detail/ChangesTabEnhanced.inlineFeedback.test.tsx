/**
 * Integration tests for ChangesTabEnhanced inline feedback wiring
 *
 * These tests verify that ChangesTabEnhanced correctly:
 * 1. Loads existing inline feedback for the task
 * 2. Passes feedback list to diff components
 * 3. Provides onAddInlineFeedback callback that calls the API
 *
 * CRITICAL: These tests verify the WIRING exists. Without them,
 * DiffHunk could work perfectly but never receive the callback,
 * resulting in dead code.
 *
 * Success Criteria Coverage:
 * - SC-3: Submitting the inline form creates a Feedback with type=INLINE, correct file and line
 * - SC-7: Feedback added from diff appears in FeedbackPanel with file:line location
 * - SC-8: Agent context includes file and line when feedback is sent
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChangesTabEnhanced } from './ChangesTabEnhanced';
import { createMockFeedback, createMockTask } from '@/test/factories';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';

// Mock stores
const mockProjectId = 'test-project';
vi.mock('@/stores', () => ({
	useCurrentProjectId: () => mockProjectId,
}));

// Mock feedback client
const mockAddFeedback = vi.fn();
const mockListFeedback = vi.fn();
vi.mock('@/lib/client', () => ({
	feedbackClient: {
		addFeedback: (...args: unknown[]) => mockAddFeedback(...args),
		listFeedback: (...args: unknown[]) => mockListFeedback(...args),
	},
	taskClient: {
		getTaskDiff: vi.fn().mockResolvedValue({
			diff: {
				files: [
					{
						path: 'src/main.ts',
						changeType: 'modified',
						hunks: [
							{
								oldStart: 1,
								oldLines: 3,
								newStart: 1,
								newLines: 3,
								lines: [
									{ type: 'context', content: 'line 1', oldLine: 1, newLine: 1 },
									{ type: 'deletion', content: 'old line 2', oldLine: 2 },
									{ type: 'addition', content: 'new line 2', newLine: 2 },
									{ type: 'context', content: 'line 3', oldLine: 3, newLine: 3 },
								],
							},
						],
					},
				],
			},
		}),
	},
}));

// Mock toast
vi.mock('@/stores/uiStore', () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

describe('ChangesTabEnhanced inline feedback integration', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockListFeedback.mockResolvedValue({ feedback: [] });
		mockAddFeedback.mockResolvedValue({
			feedback: createMockFeedback({
				id: 'new-fb',
				type: FeedbackType.INLINE,
			}),
		});
	});

	describe('Loading inline feedback', () => {
		it('loads inline feedback for the task on mount', async () => {
			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(mockListFeedback).toHaveBeenCalledWith({
					projectId: mockProjectId,
					taskId: 'TASK-001',
					type: FeedbackType.INLINE,
				});
			});
		});

		it('passes loaded feedback to diff components', async () => {
			const existingFeedback = [
				createMockFeedback({
					id: 'fb-1',
					text: 'Existing comment',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 2,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback: existingFeedback });

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			// Wait for data to load and verify indicator appears
			await waitFor(() => {
				expect(screen.getByText('💬')).toBeInTheDocument();
			});
		});
	});

	describe('SC-3: Creating inline feedback via API', () => {
		it('calls feedbackClient.addFeedback when inline feedback is submitted', async () => {
			const user = userEvent.setup();
			render(<ChangesTabEnhanced taskId="TASK-001" />);

			// Wait for diff to load
			await waitFor(() => {
				expect(screen.getByText('line 1')).toBeInTheDocument();
			});

			// Hover over line 1 and click +
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			// Type and submit
			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Test inline comment');
			await user.keyboard('{Enter}');

			// Verify API call
			await waitFor(() => {
				expect(mockAddFeedback).toHaveBeenCalledWith({
					projectId: mockProjectId,
					taskId: 'TASK-001',
					type: FeedbackType.INLINE,
					text: 'Test inline comment',
					timing: FeedbackTiming.WHEN_DONE,
					file: 'src/main.ts',
					line: 1,
				});
			});
		});

		it('refreshes feedback list after successful submission', async () => {
			const user = userEvent.setup();
			mockListFeedback.mockResolvedValueOnce({ feedback: [] }); // Initial load
			mockListFeedback.mockResolvedValueOnce({
				feedback: [
					createMockFeedback({
						id: 'new-fb',
						text: 'Test inline comment',
						file: 'src/main.ts',
						line: 1,
					}),
				],
			}); // After submission

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(screen.getByText('line 1')).toBeInTheDocument();
			});

			// Add feedback
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Test inline comment');
			await user.keyboard('{Enter}');

			// Verify list was refreshed
			await waitFor(() => {
				expect(mockListFeedback).toHaveBeenCalledTimes(2);
			});

			// Verify new indicator appears
			await waitFor(() => {
				expect(screen.getByText('💬')).toBeInTheDocument();
			});
		});
	});

	describe('SC-7: Feedback appears with file:line location', () => {
		it('feedback is stored with correct file path and line number', async () => {
			const user = userEvent.setup();
			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(screen.getByText('new line 2')).toBeInTheDocument();
			});

			// Find the addition line and add feedback
			const additionRow = screen.getByText('new line 2').closest('tr');
			const lineNumberCell = additionRow?.querySelector('.line-number.new');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Comment on new line');
			await user.keyboard('{Enter}');

			await waitFor(() => {
				expect(mockAddFeedback).toHaveBeenCalledWith(
					expect.objectContaining({
						file: 'src/main.ts',
						line: 2,
					})
				);
			});
		});
	});

	describe('SC-8: Agent context includes file and line', () => {
		it('feedback API request includes file and line for agent consumption', async () => {
			const user = userEvent.setup();
			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(screen.getByText('line 3')).toBeInTheDocument();
			});

			// Add feedback on line 3
			const line3Cell = screen.getByText('3').closest('.line-number');
			await user.hover(line3Cell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Check error handling');
			await user.keyboard('{Enter}');

			// Verify the full request structure for agent consumption
			await waitFor(() => {
				expect(mockAddFeedback).toHaveBeenCalledWith({
					projectId: mockProjectId,
					taskId: 'TASK-001',
					type: FeedbackType.INLINE,
					text: 'Check error handling',
					timing: FeedbackTiming.WHEN_DONE,
					file: 'src/main.ts',
					line: 3,
				});
			});
		});
	});

	describe('Error handling', () => {
		it('shows error toast when feedback creation fails', async () => {
			const user = userEvent.setup();
			mockAddFeedback.mockRejectedValue(new Error('API error'));

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(screen.getByText('line 1')).toBeInTheDocument();
			});

			// Try to add feedback
			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			await user.type(screen.getByPlaceholderText(/add feedback/i), 'Test');
			await user.keyboard('{Enter}');

			// Should show error
			await waitFor(() => {
				expect(screen.getByText(/failed to add feedback/i)).toBeInTheDocument();
			});
		});

		it('keeps input open with preserved text on API error', async () => {
			const user = userEvent.setup();
			mockAddFeedback.mockRejectedValue(new Error('API error'));

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(screen.getByText('line 1')).toBeInTheDocument();
			});

			const lineNumberCell = screen.getByText('1').closest('.line-number');
			await user.hover(lineNumberCell!);

			const addButton = await screen.findByRole('button', { name: /add feedback/i });
			await user.click(addButton);

			const input = screen.getByPlaceholderText(/add feedback/i);
			await user.type(input, 'Important feedback');
			await user.keyboard('{Enter}');

			// Input should still be open with text preserved
			await waitFor(() => {
				expect(screen.getByPlaceholderText(/add feedback/i)).toHaveValue('Important feedback');
			});
		});
	});

	describe('Feedback filtering', () => {
		it('only shows feedback indicators for current file', async () => {
			const feedback = [
				createMockFeedback({
					id: 'fb-1',
					type: FeedbackType.INLINE,
					file: 'src/main.ts',
					line: 1,
				}),
				createMockFeedback({
					id: 'fb-2',
					type: FeedbackType.INLINE,
					file: 'src/other.ts', // Different file
					line: 1,
				}),
			];
			mockListFeedback.mockResolvedValue({ feedback });

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			await waitFor(() => {
				expect(screen.getByText('line 1')).toBeInTheDocument();
			});

			// Should only show one indicator (for src/main.ts)
			const indicators = screen.getAllByText('💬');
			expect(indicators).toHaveLength(1);
		});
	});
});
