/**
 * Unit Tests for DashboardCostSummary component
 *
 * Success Criteria Coverage:
 * - SC-10: Dashboard page renders a cost summary section showing current month total
 * - SC-11: DashboardCostSummary fetches data via GetCostReport RPC on load
 *
 * Failure Modes:
 * - On API error, widget shows inline error message (not full-page error)
 * - When GlobalDB has no cost data, widget shows "$0.00"
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { DashboardCostSummary } from './DashboardCostSummary';

// Mock the client module
vi.mock('@/lib/client', () => ({
	dashboardClient: {
		getCostReport: vi.fn(),
	},
}));

import { dashboardClient } from '@/lib/client';

const mockGetCostReport = dashboardClient.getCostReport as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
});

describe('DashboardCostSummary', () => {
	// --- SC-10: Renders cost summary section with current month total ---

	describe('SC-10: renders cost summary with total', () => {
		it('should render the cost summary section', async () => {
			mockGetCostReport.mockResolvedValueOnce({
				totalCostUsd: 142.50,
				breakdowns: [
					{ key: 'opus', costUsd: 100.00 },
					{ key: 'sonnet', costUsd: 42.50 },
				],
			});

			render(<DashboardCostSummary projectId="proj-orc" />);

			await waitFor(() => {
				// Should display the total cost formatted as dollars
				expect(screen.getByText(/\$142\.50/)).toBeInTheDocument();
			});
		});

		it('should show $0.00 when no cost data exists', async () => {
			mockGetCostReport.mockResolvedValueOnce({
				totalCostUsd: 0,
				breakdowns: [],
			});

			render(<DashboardCostSummary projectId="proj-empty" />);

			await waitFor(() => {
				expect(screen.getByText(/\$0\.00/)).toBeInTheDocument();
			});
		});

		it('should display budget status when budget is configured', async () => {
			mockGetCostReport.mockResolvedValueOnce({
				totalCostUsd: 75.00,
				breakdowns: [],
				budgetLimitUsd: 100.00,
				budgetPercentUsed: 75.0,
			});

			render(<DashboardCostSummary projectId="proj-orc" />);

			await waitFor(() => {
				// Should show budget percentage
				expect(screen.getByText(/75%/)).toBeInTheDocument();
			});
		});

		it('should display breakdown entries', async () => {
			mockGetCostReport.mockResolvedValueOnce({
				totalCostUsd: 100.00,
				breakdowns: [
					{ key: 'opus', costUsd: 70.00 },
					{ key: 'sonnet', costUsd: 30.00 },
				],
			});

			render(<DashboardCostSummary projectId="proj-orc" />);

			await waitFor(() => {
				expect(screen.getByText(/opus/i)).toBeInTheDocument();
				expect(screen.getByText(/sonnet/i)).toBeInTheDocument();
			});
		});
	});

	// --- SC-11: Fetches data via GetCostReport RPC on mount ---

	describe('SC-11: fetches data on mount', () => {
		it('should call getCostReport on mount', async () => {
			mockGetCostReport.mockResolvedValueOnce({
				totalCostUsd: 0,
				breakdowns: [],
			});

			render(<DashboardCostSummary projectId="proj-orc" />);

			await waitFor(() => {
				expect(mockGetCostReport).toHaveBeenCalledTimes(1);
			});
		});

		it('should pass projectId in the request', async () => {
			mockGetCostReport.mockResolvedValueOnce({
				totalCostUsd: 0,
				breakdowns: [],
			});

			render(<DashboardCostSummary projectId="proj-abc" />);

			await waitFor(() => {
				expect(mockGetCostReport).toHaveBeenCalledWith(
					expect.objectContaining({
						projectId: 'proj-abc',
					})
				);
			});
		});

		it('should show loading state initially', () => {
			// Never resolve the promise to keep loading state
			mockGetCostReport.mockReturnValue(new Promise(() => {}));

			render(<DashboardCostSummary projectId="proj-orc" />);

			// Should show some loading indicator
			expect(
				screen.getByText(/loading/i) || screen.getByRole('progressbar') || screen.getByTestId('cost-loading')
			).toBeTruthy();
		});
	});

	// --- Failure Modes ---

	describe('error handling', () => {
		it('should show inline error message on API failure', async () => {
			mockGetCostReport.mockRejectedValueOnce(new Error('connection refused'));

			render(<DashboardCostSummary projectId="proj-orc" />);

			await waitFor(() => {
				expect(screen.getByText(/failed to load cost data/i)).toBeInTheDocument();
			});
		});

		it('should NOT render a full-page error on API failure', async () => {
			mockGetCostReport.mockRejectedValueOnce(new Error('server error'));

			render(<DashboardCostSummary projectId="proj-orc" />);

			await waitFor(() => {
				// The error should be inline, not a full error page
				const errorEl = screen.getByText(/failed to load/i);
				// It should be inside the cost summary section, not a page-level error
				expect(errorEl.closest('.dashboard')).toBeNull();
			});
		});
	});
});
