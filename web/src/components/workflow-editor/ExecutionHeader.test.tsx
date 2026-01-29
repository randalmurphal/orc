/**
 * TDD Tests for ExecutionHeader Component
 *
 * Tests for TASK-639: Live execution visualization on workflow canvas
 *
 * Success Criteria Coverage:
 * - SC-5: Header shows "Running" badge with pulse animation when workflow run is active
 * - SC-6: Header shows live session duration, total tokens, and total cost
 * - SC-8: Cancel button stops the workflow run and updates UI
 *
 * Error paths:
 * - Cancel request fails → show toast error, keep current state
 * - Run completes while viewing → update badge to "Completed"
 * - Event stream disconnects → show reconnecting indicator
 */

import { describe, it, expect, beforeEach, vi, afterEach, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RunStatus } from '@/gen/orc/v1/workflow_pb';

// Mock toast
vi.mock('@/stores', async () => {
	const actual = await vi.importActual('@/stores');
	return {
		...actual,
		toast: {
			error: vi.fn(),
			success: vi.fn(),
			info: vi.fn(),
		},
	};
});

// Import mocked modules for assertions
import { toast } from '@/stores';

// Import the actual component and its props type
import { ExecutionHeader, type ExecutionHeaderProps } from './ExecutionHeader';

// Mock IntersectionObserver for any React Flow internals
beforeAll(() => {
	class MockIntersectionObserver {
		observe() {}
		unobserve() {}
		disconnect() {}
	}
	Object.defineProperty(window, 'IntersectionObserver', {
		value: MockIntersectionObserver,
		writable: true,
	});
});

describe('ExecutionHeader', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('SC-5: Running badge with pulse animation', () => {
		it('should show "Running" badge when runStatus is RUNNING', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m 23s',
				totalTokens: 45200,
				totalCost: 1.23,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Running badge should be visible
			const badge = screen.queryByText(/Running/i);
			// Note: Test will fail until component is implemented
			expect(badge).toBeInTheDocument();
		});

		it('should apply pulse animation class to Running badge', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '0m',
				totalTokens: 0,
				totalCost: 0,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Badge should have pulse animation class
			const badge = screen.queryByText(/Running/i);
			expect(badge).toBeInTheDocument();
			expect(badge?.closest('.execution-badge')).toHaveClass('execution-badge--pulse');
		});

		it('should show "Completed" badge when runStatus is COMPLETED', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.COMPLETED,
				duration: '10m 45s',
				totalTokens: 120000,
				totalCost: 3.50,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Completed badge should be visible
			expect(screen.queryByText(/Completed/i)).toBeInTheDocument();
		});

		it('should show "Failed" badge when runStatus is FAILED', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.FAILED,
				duration: '2m 15s',
				totalTokens: 15000,
				totalCost: 0.45,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Failed badge should be visible with error styling
			expect(screen.queryByText(/Failed/i)).toBeInTheDocument();
		});

		it('should show "Cancelled" badge when runStatus is CANCELLED', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.CANCELLED,
				duration: '1m 30s',
				totalTokens: 8000,
				totalCost: 0.25,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Cancelled badge should be visible
			expect(screen.queryByText(/Cancelled/i)).toBeInTheDocument();
		});
	});

	describe('SC-6: Live session metrics display', () => {
		it('should display duration in header', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m 23s',
				totalTokens: 45200,
				totalCost: 1.23,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Duration should be displayed
			expect(screen.queryByText('5m 23s')).toBeInTheDocument();
		});

		it('should display token count with formatting', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m 23s',
				totalTokens: 45200,
				totalCost: 1.23,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Tokens should be displayed with K formatting
			// 45200 should be displayed as "45.2K" or similar
			expect(screen.queryByText(/45\.?2?\s*K/i)).toBeInTheDocument();
		});

		it('should display cost with dollar formatting', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m 23s',
				totalTokens: 45200,
				totalCost: 1.23,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Cost should be displayed with $ prefix
			expect(screen.queryByText(/\$1\.23/)).toBeInTheDocument();
		});

		it('should format very long duration correctly (hours)', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '2h 34m',
				totalTokens: 500000,
				totalCost: 15.50,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Should display as "2h 34m" not "154m"
			expect(screen.queryByText('2h 34m')).toBeInTheDocument();
		});

		it('should not display metrics section when runStatus is PENDING', () => {
			// Arrange: No active run yet
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.PENDING,
				duration: '0m',
				totalTokens: 0,
				totalCost: 0,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Metrics section should be hidden for pending runs
			// The component should not render metrics until run starts
			expect(screen.queryByText(/\$0\.00/)).not.toBeInTheDocument();
		});
	});

	describe('SC-8: Cancel button', () => {
		it('should show Cancel button when run is RUNNING', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Cancel button should be visible
			expect(screen.queryByRole('button', { name: /Cancel/i })).toBeInTheDocument();
		});

		it('should not show Cancel button when run is COMPLETED', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.COMPLETED,
				duration: '10m',
				totalTokens: 50000,
				totalCost: 1.50,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Cancel button should NOT be visible
			expect(screen.queryByRole('button', { name: /Cancel/i })).not.toBeInTheDocument();
		});

		it('should call onCancel when Cancel button is clicked and confirmed', async () => {
			// Arrange
			const user = userEvent.setup();
			const mockOnCancel = vi.fn().mockResolvedValue(undefined);
			const props = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: mockOnCancel,
			};

			// Act
			render(<ExecutionHeader {...props} />);
			const cancelButton = screen.getByRole('button', { name: /^Cancel$/i });
			await user.click(cancelButton);

			// Click confirm button in the dialog
			const confirmButton = await screen.findByRole('button', { name: /^Confirm$/i });
			await user.click(confirmButton);

			// Assert: onCancel should be called
			expect(mockOnCancel).toHaveBeenCalledTimes(1);
		});

		it('should show confirmation dialog before cancelling', async () => {
			// Arrange
			const user = userEvent.setup();
			const mockOnCancel = vi.fn().mockResolvedValue(undefined);
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: mockOnCancel,
			};

			// Act
			render(<ExecutionHeader {...props} />);
			const cancelButton = screen.getByRole('button', { name: /Cancel/i });
			await user.click(cancelButton);

			// Assert: Confirmation dialog should appear
			await waitFor(() => {
				expect(screen.queryByText(/Are you sure/i)).toBeInTheDocument();
			});
		});

		it('should disable Cancel button while cancel is in progress', async () => {
			// Arrange
			const user = userEvent.setup();
			// Create a pending promise to simulate slow cancel
			let resolveCancelPromise: () => void;
			const pendingCancel = new Promise<void>((resolve) => {
				resolveCancelPromise = resolve;
			});
			const mockOnCancel = vi.fn().mockReturnValue(pendingCancel);
			const props = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: mockOnCancel,
			};

			// Act
			render(<ExecutionHeader {...props} />);
			const cancelButton = screen.getByRole('button', { name: /^Cancel$/i });

			// Click cancel to show confirmation dialog
			await user.click(cancelButton);

			// Click confirm button
			const confirmButton = await screen.findByRole('button', { name: /^Confirm$/i });
			await user.click(confirmButton);

			// Assert: Buttons should be disabled during cancel (text changes to "Cancelling...")
			await waitFor(() => {
				// Both cancel button and confirm button show "Cancelling..." while in progress
				// Get all buttons with "Cancelling..." text and verify both are disabled
				const cancellingButtons = screen.getAllByRole('button', { name: /Cancelling/i });
				expect(cancellingButtons.length).toBeGreaterThan(0);
				cancellingButtons.forEach((btn) => expect(btn).toBeDisabled());
			});

			// Cleanup
			resolveCancelPromise!();
		});

		it('should show toast error when cancel fails', async () => {
			// Arrange
			const user = userEvent.setup();
			const mockOnCancel = vi.fn().mockRejectedValue(new Error('Network error'));
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: mockOnCancel,
			};

			// Act
			render(<ExecutionHeader {...props} />);
			const cancelButton = screen.getByRole('button', { name: /Cancel/i });
			await user.click(cancelButton);

			// Confirm if dialog is shown
			const confirmButton = screen.queryByRole('button', { name: /Confirm/i });
			if (confirmButton) {
				await user.click(confirmButton);
			}

			// Assert: Toast error should be shown
			await waitFor(() => {
				expect(toast.error).toHaveBeenCalledWith(expect.stringContaining('Failed to cancel'));
			});
		});

		it('should keep running state when cancel fails', async () => {
			// Arrange
			const user = userEvent.setup();
			const mockOnCancel = vi.fn().mockRejectedValue(new Error('Network error'));
			const props = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: mockOnCancel,
			};

			// Act
			render(<ExecutionHeader {...props} />);
			const cancelButton = screen.getByRole('button', { name: /^Cancel$/i });
			await user.click(cancelButton);

			// Click confirm button
			const confirmButton = await screen.findByRole('button', { name: /^Confirm$/i });
			await user.click(confirmButton);

			// Wait for error handling
			await waitFor(() => {
				expect(toast.error).toHaveBeenCalled();
			});

			// Assert: Badge should still show "Running" (use exact match for badge text)
			const badge = screen.getByText('Running');
			expect(badge).toHaveClass('execution-badge--running');
		});
	});

	describe('Event stream disconnection', () => {
		it('should show reconnecting indicator when isReconnecting is true', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: vi.fn(),
				isReconnecting: true,
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Reconnecting indicator should be visible
			expect(screen.queryByText(/Reconnecting/i)).toBeInTheDocument();
		});

		it('should hide reconnecting indicator when connection is restored', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '5m',
				totalTokens: 1000,
				totalCost: 0.05,
				onCancel: vi.fn(),
				isReconnecting: false,
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Reconnecting indicator should NOT be visible
			expect(screen.queryByText(/Reconnecting/i)).not.toBeInTheDocument();
		});
	});

	describe('Edge cases', () => {
		it('should handle zero duration', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '0s',
				totalTokens: 0,
				totalCost: 0,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Should display without error
			expect(screen.queryByText('0s')).toBeInTheDocument();
		});

		it('should handle very large token counts', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '1h',
				totalTokens: 1500000, // 1.5M tokens
				totalCost: 45.00,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Should format as "1.5M" or similar
			expect(screen.queryByText(/1\.5\s*M/i)).toBeInTheDocument();
		});

		it('should handle small cost values with 2 decimal places', () => {
			// Arrange
			const props: ExecutionHeaderProps = {
				runStatus: RunStatus.RUNNING,
				duration: '1m',
				totalTokens: 100,
				totalCost: 0.01,
				onCancel: vi.fn(),
			};

			// Act
			render(<ExecutionHeader {...props} />);

			// Assert: Should show $0.01
			expect(screen.queryByText(/\$0\.01/)).toBeInTheDocument();
		});
	});
});
