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
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ChangesTabEnhanced } from './ChangesTabEnhanced';
import { createMockFeedback } from '@/test/factories';
import { FeedbackType, FeedbackTiming } from '@/gen/orc/v1/feedback_pb';

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
		getDiff: vi.fn().mockResolvedValue({
			diff: {
				base: 'main',
				head: 'orc/TASK-001',
				stats: {
					filesChanged: 1,
					additions: 1,
					deletions: 1,
				},
				files: [
					{
						path: 'src/main.ts',
						status: 'modified',
						additions: 1,
						deletions: 1,
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
				// API doesn't support type filter, so we load all and filter client-side
				expect(mockListFeedback).toHaveBeenCalledWith({
					projectId: mockProjectId,
					taskId: 'TASK-001',
					excludeReceived: false,
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
				// Multiple indicators may render (old/new columns), so use getAllByText
				expect(screen.getAllByText('💬').length).toBeGreaterThan(0);
			});
		});
	});

	describe('SC-3: Creating inline feedback via API', () => {
		it('calls feedbackClient.addFeedback when inline feedback is submitted', async () => {
			const user = userEvent.setup();
			render(<ChangesTabEnhanced taskId="TASK-001" />);

			// Wait for diff to load (content appears in both columns in split view)
			await waitFor(() => {
				expect(screen.getAllByText('line 1').length).toBeGreaterThan(0);
			});

			// Find a line number cell and hover using fireEvent
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			// Multiple buttons may appear (old + new columns), take the first
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			// Type and submit
			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Test inline comment');
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
						type: FeedbackType.INLINE,
						file: 'src/main.ts',
						line: 1,
					}),
				],
			}); // After submission

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			// Wait for diff to load (content appears in both columns in split view)
			await waitFor(() => {
				expect(screen.getAllByText('line 1').length).toBeGreaterThan(0);
			});

			// Add feedback - find a line number cell using fireEvent
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			// Multiple buttons may appear (old + new columns), take the first
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Test inline comment');
			await user.keyboard('{Enter}');

			// Verify list was refreshed
			await waitFor(() => {
				expect(mockListFeedback).toHaveBeenCalledTimes(2);
			});

			// Verify new indicator appears
			await waitFor(() => {
				// Multiple indicators may render (old/new columns), so use getAllByText
				expect(screen.getAllByText('💬').length).toBeGreaterThan(0);
			});
		});
	});

	describe('SC-7: Feedback appears with file:line location', () => {
		it('feedback is stored with correct file path and line number', async () => {
			const user = userEvent.setup();
			render(<ChangesTabEnhanced taskId="TASK-001" />);

			// Wait for diff to load
			await waitFor(() => {
				expect(screen.getAllByText('new line 2').length).toBeGreaterThan(0);
			});

			// Find an addition line number cell (line 2 is an addition)
			const newLineNumberCells = document.querySelectorAll('.line-number.new');
			// Find the one with "2" for the addition line
			let additionLineCell: Element | null = null;
			newLineNumberCells.forEach((cell) => {
				if (cell.textContent?.includes('2')) {
					additionLineCell = cell;
				}
			});
			fireEvent.mouseEnter(additionLineCell!);

			// Multiple buttons may appear (old + new columns), take the first
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Comment on new line');
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

			// Wait for diff to load (content appears in both columns in split view)
			await waitFor(() => {
				expect(screen.getAllByText('line 3').length).toBeGreaterThan(0);
			});

			// Find a line number cell for line 3
			const lineNumberCells = document.querySelectorAll('.line-number');
			let line3Cell: Element | null = null;
			lineNumberCells.forEach((cell) => {
				if (cell.textContent === '3') {
					line3Cell = cell;
				}
			});
			fireEvent.mouseEnter(line3Cell!);

			// Multiple buttons may appear (old + new columns), take the first
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Check error handling');
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

			// Wait for diff to load (content appears in both columns in split view)
			await waitFor(() => {
				expect(screen.getAllByText('line 1').length).toBeGreaterThan(0);
			});

			// Try to add feedback - find a line number cell
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			// Multiple buttons may appear (old + new columns), take the first
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Test');
			await user.keyboard('{Enter}');

			// Should show error (may appear in multiple places: inline input + feedback error area)
			await waitFor(() => {
				expect(screen.getAllByText(/failed to add feedback/i).length).toBeGreaterThan(0);
			});
		});

		it('keeps input open with preserved text on API error', async () => {
			const user = userEvent.setup();
			mockAddFeedback.mockRejectedValue(new Error('API error'));

			render(<ChangesTabEnhanced taskId="TASK-001" />);

			// Wait for diff to load (content appears in both columns in split view)
			await waitFor(() => {
				expect(screen.getAllByText('line 1').length).toBeGreaterThan(0);
			});

			// Find a line number cell
			const lineNumberCells = document.querySelectorAll('.line-number');
			const lineNumberCell = lineNumberCells[0];
			fireEvent.mouseEnter(lineNumberCell!);

			// Multiple buttons may appear (old + new columns), take the first
			const addButtons = await screen.findAllByRole('button', { name: /add feedback/i });
			fireEvent.click(addButtons[0]);

			const input = await screen.findByPlaceholderText(/add feedback/i);
			await user.type(input, 'Important feedback');
			await user.keyboard('{Enter}');

			// Input should still be open with text preserved
			await waitFor(() => {
				expect(screen.queryByPlaceholderText(/add feedback/i)).toHaveValue('Important feedback');
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

			// Wait for diff to load (content appears in both columns in split view)
			await waitFor(() => {
				expect(screen.getAllByText('line 1').length).toBeGreaterThan(0);
			});

			// Should only show indicators for src/main.ts (may appear in multiple columns for split view)
			// In split view, the same line appears in old and new columns, so we may have 2 indicators
			// for line 1 of src/main.ts, but none for src/other.ts which isn't in the diff
			const indicators = screen.getAllByText('💬');
			// At least one indicator should be present
			expect(indicators.length).toBeGreaterThan(0);
			// But we shouldn't have more than what's expected for a single file
			// In split view with one line having feedback, expect up to 2 (old + new columns)
			expect(indicators.length).toBeLessThanOrEqual(2);
		});
	});
});
