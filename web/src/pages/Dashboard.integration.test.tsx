/**
 * Integration Tests for Dashboard.tsx rendering DashboardCostSummary
 *
 * Success Criteria Coverage:
 * - SC-12: Dashboard.tsx renders DashboardCostSummary and passes project context
 *
 * This is an INTEGRATION test. It renders the actual Dashboard component
 * and verifies that DashboardCostSummary is present in the output.
 * This test FAILS if Dashboard doesn't import and render DashboardCostSummary.
 *
 * Wiring verification:
 * - new_component_path: @/components/dashboard/DashboardCostSummary.tsx
 * - imported_by: @/pages/Dashboard.tsx
 * - integration_test_verifies: Dashboard renders DashboardCostSummary
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';

// Mock all API clients used by Dashboard
vi.mock('@/lib/client', () => ({
	dashboardClient: {
		getStats: vi.fn(),
		getCostReport: vi.fn(),
	},
	initiativeClient: {
		listInitiatives: vi.fn(),
	},
}));

// Mock stores
vi.mock('@/stores', () => ({
	useTaskStore: Object.assign(
		vi.fn((selector: unknown) => {
			if (typeof selector === 'function') {
				return (selector as (s: { tasks: never[] }) => unknown)({ tasks: [] });
			}
			return { tasks: [], subscribe: vi.fn(() => vi.fn()) };
		}),
		{ subscribe: vi.fn(() => vi.fn()) }
	),
	useWsStatus: vi.fn(() => 'connected'),
}));

vi.mock('@/stores/projectStore', () => ({
	useCurrentProjectId: vi.fn(() => 'proj-test'),
}));

vi.mock('@/hooks', () => ({
	useDocumentTitle: vi.fn(),
}));

import { dashboardClient, initiativeClient } from '@/lib/client';
import { Dashboard } from './Dashboard';

const mockGetStats = dashboardClient.getStats as ReturnType<typeof vi.fn>;
const mockGetCostReport = (dashboardClient as Record<string, unknown>).getCostReport as ReturnType<typeof vi.fn>;
const mockListInitiatives = initiativeClient.listInitiatives as ReturnType<typeof vi.fn>;

function renderDashboard() {
	return render(
		<MemoryRouter>
			<Dashboard />
		</MemoryRouter>
	);
}

// Provide minimal successful API responses
function setupDefaultMocks() {
	mockGetStats.mockResolvedValue({
		stats: {
			taskCounts: { all: 5, active: 2, completed: 3, failed: 0, running: 1, blocked: 0 },
			runningTasks: [],
			recentCompletions: [],
			pendingDecisions: 0,
			todayTokens: { inputTokens: 0, outputTokens: 0, totalTokens: 0, cacheCreationInputTokens: 0, cacheReadInputTokens: 0 },
			todayCostUsd: 0,
		},
	});
	mockListInitiatives.mockResolvedValue({ initiatives: [] });
	mockGetCostReport.mockResolvedValue({
		totalCostUsd: 42.00,
		breakdowns: [],
	});
}

beforeEach(() => {
	vi.clearAllMocks();
	setupDefaultMocks();
});

describe('Dashboard - DashboardCostSummary Integration (SC-12)', () => {
	it('should render DashboardCostSummary component', async () => {
		renderDashboard();

		await waitFor(() => {
			// The DashboardCostSummary component should render and show cost data.
			// This test FAILS if Dashboard doesn't import/render DashboardCostSummary.
			// We look for a cost-related element that DashboardCostSummary would render.
			const costSection = screen.getByTestId('cost-summary') ||
				screen.getByText(/cost/i);
			expect(costSection).toBeInTheDocument();
		});
	});

	it('should fetch cost report data via getCostReport', async () => {
		renderDashboard();

		await waitFor(() => {
			// DashboardCostSummary should call getCostReport on mount
			expect(mockGetCostReport).toHaveBeenCalled();
		});
	});

	it('should pass project context to DashboardCostSummary', async () => {
		renderDashboard();

		await waitFor(() => {
			// The getCostReport call should include the project ID
			expect(mockGetCostReport).toHaveBeenCalledWith(
				expect.objectContaining({
					projectId: 'proj-test',
				})
			);
		});
	});

	it('should still render rest of dashboard when cost widget fails', async () => {
		mockGetCostReport.mockRejectedValue(new Error('cost API down'));

		renderDashboard();

		await waitFor(() => {
			// Dashboard stats section should still load even if cost widget errors
			expect(screen.getByText(/Quick Stats/i) || screen.getByText(/Running/i)).toBeTruthy();
		});
	});
});
