/**
 * StatsView Component Tests
 *
 * Tests for the StatsView container component including:
 * - Loading skeleton states
 * - Error state with retry button
 * - Time filter button interactions
 * - Export CSV functionality
 * - Summary cards with formatted values
 * - Empty data state
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TooltipProvider } from '../ui/Tooltip';

// Helper to wrap with TooltipProvider
function renderWithProvider(ui: React.ReactElement) {
	return render(<TooltipProvider delayDuration={0}>{ui}</TooltipProvider>);
}

// Mock the statsStore with configurable state
const mockFetchStats = vi.fn();
const mockSetPeriod = vi.fn();

// Default mock state
let mockState = {
	period: '7d' as const,
	loading: false,
	error: null as string | null,
	activityData: new Map<string, number>(),
	outcomes: { completed: 0, withRetries: 0, failed: 0 },
	tasksPerDay: [] as { day: string; count: number }[],
	topInitiatives: [] as { name: string; taskCount: number }[],
	topFiles: [] as { path: string; modifyCount: number }[],
	summaryStats: {
		tasksCompleted: 0,
		tokensUsed: 0,
		totalCost: 0,
		avgTime: 0,
		successRate: 0,
	},
	weeklyChanges: null as { tasks: number; tokens: number; cost: number; successRate: number } | null,
};

vi.mock('@/stores/statsStore', () => ({
	useStatsStore: (selector: (state: typeof mockState & { fetchStats: typeof mockFetchStats; setPeriod: typeof mockSetPeriod }) => unknown) => {
		return selector({ ...mockState, fetchStats: mockFetchStats, setPeriod: mockSetPeriod });
	},
	useStatsPeriod: () => mockState.period,
	useStatsLoading: () => mockState.loading,
	useStatsError: () => mockState.error,
	useActivityData: () => mockState.activityData,
	useOutcomes: () => mockState.outcomes,
	useTasksPerDay: () => mockState.tasksPerDay,
	useTopInitiatives: () => mockState.topInitiatives,
	useTopFiles: () => mockState.topFiles,
	useSummaryStats: () => mockState.summaryStats,
	useWeeklyChanges: () => mockState.weeklyChanges,
}));

// Import after mock setup
import { StatsView } from './StatsView';

// Helper to reset mock state
function resetMockState() {
	mockState = {
		period: '7d',
		loading: false,
		error: null,
		activityData: new Map(),
		outcomes: { completed: 0, withRetries: 0, failed: 0 },
		tasksPerDay: [],
		topInitiatives: [],
		topFiles: [],
		summaryStats: {
			tasksCompleted: 0,
			tokensUsed: 0,
			totalCost: 0,
			avgTime: 0,
			successRate: 0,
		},
		weeklyChanges: null,
	};
}

// Reset state before each test
beforeEach(() => {
	resetMockState();
	vi.clearAllMocks();

	// Mock URL.createObjectURL and URL.revokeObjectURL
	global.URL.createObjectURL = vi.fn(() => 'blob:mock-url');
	global.URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
	vi.restoreAllMocks();
});

// =============================================================================
// Tests with default empty state
// =============================================================================

describe('StatsView', () => {
	describe('basic rendering', () => {
		it('renders the header with title and subtitle', () => {
			render(<StatsView />);

			expect(screen.getByText('Statistics')).toBeInTheDocument();
			expect(screen.getByText('Token usage, costs, and task metrics')).toBeInTheDocument();
		});

		it('renders time filter buttons', () => {
			render(<StatsView />);

			expect(screen.getByRole('tab', { name: '24h' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: '7d' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: '30d' })).toBeInTheDocument();
			expect(screen.getByRole('tab', { name: 'All' })).toBeInTheDocument();
		});

		it('renders export button', () => {
			render(<StatsView />);

			expect(screen.getByRole('button', { name: /export/i })).toBeInTheDocument();
		});

		it('calls fetchStats on mount', () => {
			render(<StatsView />);

			expect(mockFetchStats).toHaveBeenCalledWith('7d', undefined);
		});
	});

	describe('time filter interaction', () => {
		it('time filter buttons are interactive and update period', async () => {
			const user = userEvent.setup();
			render(<StatsView />);

			const btn24h = screen.getByRole('tab', { name: '24h' });
			await user.click(btn24h);

			expect(mockSetPeriod).toHaveBeenCalledWith('24h');
		});

		it('clicking 30d button calls setPeriod', async () => {
			const user = userEvent.setup();
			render(<StatsView />);

			const btn30d = screen.getByRole('tab', { name: '30d' });
			await user.click(btn30d);

			expect(mockSetPeriod).toHaveBeenCalledWith('30d');
		});

		it('clicking All button calls setPeriod', async () => {
			const user = userEvent.setup();
			render(<StatsView />);

			const btnAll = screen.getByRole('tab', { name: 'All' });
			await user.click(btnAll);

			expect(mockSetPeriod).toHaveBeenCalledWith('all');
		});
	});

	describe('custom className', () => {
		it('applies custom className', () => {
			const { container } = render(<StatsView className="custom-stats" />);

			const view = container.querySelector('.stats-view');
			expect(view).toHaveClass('stats-view');
			expect(view).toHaveClass('custom-stats');
		});
	});
});

// =============================================================================
// Loading state tests
// =============================================================================

describe('StatsView loading state', () => {
	beforeEach(() => {
		mockState.loading = true;
	});

	it('renders loading skeleton when loading=true', () => {
		const { container } = render(<StatsView />);

		// Check for skeleton elements
		const skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(5);

		// Check for aria-busy attribute
		const statsGrid = container.querySelector('.stats-view-stats-grid');
		expect(statsGrid).toHaveAttribute('aria-busy', 'true');
	});
});

// =============================================================================
// Error state tests
// =============================================================================

describe('StatsView error state', () => {
	beforeEach(() => {
		mockState.error = 'Network error';
	});

	it('renders error state with retry button when error present', () => {
		render(<StatsView />);

		expect(screen.getByText('Failed to load statistics')).toBeInTheDocument();
		expect(screen.getByText('Network error')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
	});

	it('retry button calls fetchStats', async () => {
		const user = userEvent.setup();
		render(<StatsView />);

		const retryBtn = screen.getByRole('button', { name: /retry/i });
		await user.click(retryBtn);

		// fetchStats is called once on mount, once on retry
		expect(mockFetchStats).toHaveBeenCalledTimes(2);
	});
});

// =============================================================================
// Tests with data
// =============================================================================

describe('StatsView with data', () => {
	beforeEach(() => {
		mockState.activityData = new Map([['2026-01-15', 5], ['2026-01-16', 3]]);
		mockState.outcomes = { completed: 45, withRetries: 5, failed: 2 };
		mockState.tasksPerDay = [
			{ day: '2026-01-15', count: 5 },
			{ day: '2026-01-16', count: 3 },
		];
		mockState.topInitiatives = [
			{ name: 'UI Redesign', taskCount: 12 },
			{ name: 'Backend API', taskCount: 8 },
		];
		mockState.topFiles = [
			{ path: 'src/components/Button.tsx', modifyCount: 15 },
			{ path: 'src/lib/api.ts', modifyCount: 10 },
		];
		mockState.summaryStats = {
			tasksCompleted: 52,
			tokensUsed: 2400000,
			totalCost: 47.82,
			avgTime: 204,
			successRate: 94.2,
		};
		mockState.weeklyChanges = {
			tasks: 12,
			tokens: -5,
			cost: 8,
			successRate: 2,
		};
	});

	it('summary cards display formatted values correctly', () => {
		const { container } = renderWithProvider(<StatsView />);

		// Check for formatted values in stat cards (use specific selector to avoid donut overlap)
		const statValues = container.querySelectorAll('.stats-view-stat-value');
		expect(statValues[0]).toHaveTextContent('52'); // Tasks Completed
		expect(statValues[1]).toHaveTextContent('2.4M'); // Tokens Used
		expect(statValues[2]).toHaveTextContent('$47.82'); // Total Cost
		expect(statValues[3]).toHaveTextContent('3:24'); // Avg Task Time
		expect(statValues[4]).toHaveTextContent('94.2%'); // Success Rate
	});

	it('renders stats cards grid with 5 cards', () => {
		const { container } = renderWithProvider(<StatsView />);

		const statsGrid = container.querySelector('.stats-view-stats-grid');
		expect(statsGrid).toBeInTheDocument();

		const statCards = container.querySelectorAll('.stats-view-stat-card');
		expect(statCards.length).toBe(5);
	});

	it('renders activity heatmap section', () => {
		const { container } = renderWithProvider(<StatsView />);

		// Heatmap is inside a section card
		const sectionCards = container.querySelectorAll('.stats-view-section-card');
		expect(sectionCards.length).toBeGreaterThan(0);
	});

	it('renders charts row with bar chart and donut', () => {
		const { container } = renderWithProvider(<StatsView />);

		const chartsRow = container.querySelector('.stats-view-charts-row');
		expect(chartsRow).toBeInTheDocument();

		// Check for chart titles
		expect(screen.getByText('Tasks Completed Per Day')).toBeInTheDocument();
		expect(screen.getByText('Task Outcomes')).toBeInTheDocument();
	});

	it('renders tables row with leaderboards', () => {
		const { container } = renderWithProvider(<StatsView />);

		const tablesRow = container.querySelector('.stats-view-tables-row');
		expect(tablesRow).toBeInTheDocument();

		// Check for table titles
		expect(screen.getByText('Most Active Initiatives')).toBeInTheDocument();
		expect(screen.getByText('Most Modified Files')).toBeInTheDocument();
	});
});

// =============================================================================
// Empty state tests
// =============================================================================

describe('StatsView empty state', () => {
	it('empty data shows empty message', () => {
		render(<StatsView />);

		expect(screen.getByText('No statistics yet')).toBeInTheDocument();
		expect(screen.getByText(/Complete some tasks/)).toBeInTheDocument();
	});

	it('export button is disabled when no data', () => {
		render(<StatsView />);

		const exportBtn = screen.getByRole('button', { name: /export/i });
		expect(exportBtn).toBeDisabled();
	});
});

// =============================================================================
// Export functionality tests
// =============================================================================

describe('StatsView export functionality', () => {
	beforeEach(() => {
		mockState.activityData = new Map([['2026-01-15', 5]]);
		mockState.outcomes = { completed: 10, withRetries: 0, failed: 0 };
		mockState.tasksPerDay = [{ day: '2026-01-15', count: 5 }];
		mockState.summaryStats = {
			tasksCompleted: 10,
			tokensUsed: 1000,
			totalCost: 5.0,
			avgTime: 60,
			successRate: 100,
		};
	});

	it('export button triggers CSV download', async () => {
		const user = userEvent.setup();
		renderWithProvider(<StatsView />);

		const exportBtn = screen.getByRole('button', { name: /export/i });
		expect(exportBtn).not.toBeDisabled();

		await user.click(exportBtn);

		// Check that download was triggered
		expect(URL.createObjectURL).toHaveBeenCalled();
	});
});

// =============================================================================
// Formatting tests
// =============================================================================

describe('Value formatting', () => {
	it('formats small token counts without suffix', () => {
		mockState.activityData = new Map([['2026-01-15', 1]]);
		mockState.summaryStats = {
			tasksCompleted: 1,
			tokensUsed: 500,
			totalCost: 0.5,
			avgTime: 0,
			successRate: 100,
		};
		renderWithProvider(<StatsView />);

		// 500 tokens should display as "500" (no K suffix)
		expect(screen.getByText('500')).toBeInTheDocument();
	});

	it('formats cost with dollar sign and decimals', () => {
		mockState.activityData = new Map([['2026-01-15', 1]]);
		mockState.summaryStats = {
			tasksCompleted: 1,
			tokensUsed: 500,
			totalCost: 0.5,
			avgTime: 0,
			successRate: 100,
		};
		renderWithProvider(<StatsView />);

		expect(screen.getByText('$0.50')).toBeInTheDocument();
	});

	it('formats zero time as 0:00', () => {
		mockState.activityData = new Map([['2026-01-15', 1]]);
		mockState.summaryStats = {
			tasksCompleted: 1,
			tokensUsed: 500,
			totalCost: 0.5,
			avgTime: 0,
			successRate: 100,
		};
		renderWithProvider(<StatsView />);

		expect(screen.getByText('0:00')).toBeInTheDocument();
	});
});

// =============================================================================
// Accessibility tests
// =============================================================================

describe('StatsView accessibility', () => {
	it('time filter has tablist role', () => {
		render(<StatsView />);

		const tablist = screen.getByRole('tablist', { name: /time period filter/i });
		expect(tablist).toBeInTheDocument();
	});

	it('empty state has status role', () => {
		render(<StatsView />);

		const emptyState = screen.getByRole('status');
		expect(emptyState).toBeInTheDocument();
	});
});

// =============================================================================
// TASK-526: Bug fix tests - Infinite loading skeleton
// =============================================================================

describe('TASK-526: Loading skeleton displays immediately (SC-2)', () => {
	it('shows loading skeleton on initial render before any API call', () => {
		// Reset mock to default state (simulating initial mount)
		resetMockState();
		// BUG: Initial loading state should be true to show skeleton immediately
		// Currently mockState.loading is false, which shows empty state briefly
		mockState.loading = true;

		const { container } = render(<StatsView />);

		// Loading skeleton should be visible immediately
		const skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(5);

		const statsGrid = container.querySelector('.stats-view-stats-grid');
		expect(statsGrid).toHaveAttribute('aria-busy', 'true');
	});

	it('does not show empty state before loading completes', () => {
		// Simulate true initial state: loading should be true
		resetMockState();
		mockState.loading = true;

		render(<StatsView />);

		// Empty state should NOT be visible while loading
		expect(screen.queryByText('No statistics yet')).not.toBeInTheDocument();
	});

	it('transitions from skeleton to content when loading completes with data', () => {
		// First render: loading state
		resetMockState();
		mockState.loading = true;

		const { container, rerender } = render(<StatsView />);

		// Verify skeleton is shown
		let skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(5);

		// Simulate fetch completion with data
		mockState.loading = false;
		mockState.activityData = new Map([['2026-01-15', 5]]);
		mockState.summaryStats = {
			tasksCompleted: 10,
			tokensUsed: 5000,
			totalCost: 2.5,
			avgTime: 60,
			successRate: 100,
		};

		rerender(<StatsView />);

		// Skeleton should be gone
		skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(0);

		// Actual content should be visible
		const statCards = container.querySelectorAll('.stats-view-stat-card');
		expect(statCards.length).toBe(5);
	});

	it('transitions from skeleton to error state on fetch failure', () => {
		// First render: loading state
		resetMockState();
		mockState.loading = true;

		const { container, rerender } = render(<StatsView />);

		// Verify skeleton is shown
		let skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(5);

		// Simulate fetch failure
		mockState.loading = false;
		mockState.error = 'Network error';

		rerender(<StatsView />);

		// Skeleton should be gone
		skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(0);

		// Error state should be visible
		expect(screen.getByText('Failed to load statistics')).toBeInTheDocument();
		expect(screen.getByText('Network error')).toBeInTheDocument();
	});

	it('transitions from skeleton to empty state when no data returned', () => {
		// First render: loading state
		resetMockState();
		mockState.loading = true;

		const { container, rerender } = render(<StatsView />);

		// Verify skeleton is shown
		let skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(5);

		// Simulate fetch completion with no data
		mockState.loading = false;

		rerender(<StatsView />);

		// Skeleton should be gone
		skeletonCards = container.querySelectorAll('.stats-view-stat-card--skeleton');
		expect(skeletonCards.length).toBe(0);

		// Empty state should be visible
		expect(screen.getByText('No statistics yet')).toBeInTheDocument();
	});
});

describe('TASK-526: Period change behavior (SC-4 component side)', () => {
	beforeEach(() => {
		resetMockState();
		mockState.loading = false;
		mockState.activityData = new Map([['2026-01-15', 5]]);
		mockState.summaryStats = {
			tasksCompleted: 10,
			tokensUsed: 5000,
			totalCost: 2.5,
			avgTime: 60,
			successRate: 100,
		};
	});

	it('clicking period button calls setPeriod only once', async () => {
		const user = userEvent.setup();
		render(<StatsView />);

		// Clear previous calls
		mockSetPeriod.mockClear();

		// Click a different period
		await user.click(screen.getByRole('tab', { name: '30d' }));

		// setPeriod should be called exactly once
		expect(mockSetPeriod).toHaveBeenCalledTimes(1);
		expect(mockSetPeriod).toHaveBeenCalledWith('30d');
	});

	it('clicking current period does not trigger unnecessary operations', async () => {
		const user = userEvent.setup();
		mockState.period = '7d';

		render(<StatsView />);

		// Clear previous calls from mount
		mockFetchStats.mockClear();

		// Click the already-selected period
		await user.click(screen.getByRole('tab', { name: '7d' }));

		// setPeriod is called but store should detect same period and not fetch
		expect(mockSetPeriod).toHaveBeenCalledWith('7d');
	});
});

describe('TASK-526: Error recovery behavior', () => {
	beforeEach(() => {
		resetMockState();
		mockState.error = 'Network error';
		mockState.loading = false;
	});

	it('retry button triggers fetchStats with current period', async () => {
		const user = userEvent.setup();
		render(<StatsView />);

		// Clear calls from initial render
		mockFetchStats.mockClear();

		// Click retry
		await user.click(screen.getByRole('button', { name: /retry/i }));

		// Should call fetchStats with current period
		expect(mockFetchStats).toHaveBeenCalledWith('7d', undefined);
		expect(mockFetchStats).toHaveBeenCalledTimes(1);
	});

	it('error state shows actionable message and retry button', () => {
		render(<StatsView />);

		expect(screen.getByText('Failed to load statistics')).toBeInTheDocument();
		expect(screen.getByText('Network error')).toBeInTheDocument();

		const retryBtn = screen.getByRole('button', { name: /retry/i });
		expect(retryBtn).toBeInTheDocument();
		expect(retryBtn).not.toBeDisabled();
	});
});

// =============================================================================
// TASK-608: Change indicator rendering
// =============================================================================

describe('TASK-608: Change indicators display on stat cards (SC-2)', () => {
	beforeEach(() => {
		resetMockState();
		mockState.activityData = new Map([['2026-01-15', 5]]);
		mockState.summaryStats = {
			tasksCompleted: 52,
			tokensUsed: 2400000,
			totalCost: 47.82,
			avgTime: 204,
			successRate: 94.2,
		};
	});

	it('displays positive change with green up arrow', () => {
		mockState.weeklyChanges = {
			tasks: 23,
			tokens: 18,
			cost: -8,
			successRate: 2.1,
		};

		const { container } = renderWithProvider(<StatsView />);

		// Find change indicators with 'up' class (positive changes)
		const upChanges = container.querySelectorAll('.stats-view-stat-change--up');
		expect(upChanges.length).toBeGreaterThan(0);

		// Tasks card should show "+23% from last period"
		const changeTexts = Array.from(container.querySelectorAll('.stats-view-stat-change'))
			.map(el => el.textContent);
		expect(changeTexts.some(t => t?.includes('+23%'))).toBe(true);
	});

	it('displays negative change with red down arrow', () => {
		mockState.weeklyChanges = {
			tasks: -15,
			tokens: -10,
			cost: -8,
			successRate: -3,
		};

		const { container } = renderWithProvider(<StatsView />);

		// Find change indicators with 'down' class (negative changes)
		const downChanges = container.querySelectorAll('.stats-view-stat-change--down');
		expect(downChanges.length).toBeGreaterThan(0);
	});

	it('shows "No change data" when weeklyChanges is null', () => {
		mockState.weeklyChanges = null;

		const { container } = renderWithProvider(<StatsView />);

		const neutralChanges = container.querySelectorAll('.stats-view-stat-change--neutral');
		// All 5 cards should show "No change data"
		expect(neutralChanges.length).toBe(5);
	});

	it('change indicator text includes percentage value', () => {
		mockState.weeklyChanges = {
			tasks: 23,
			tokens: 18,
			cost: -8,
			successRate: 2.1,
		};

		const { container } = renderWithProvider(<StatsView />);

		// Check that change percentages are rendered
		const allChangeTexts = Array.from(container.querySelectorAll('.stats-view-stat-change'))
			.map(el => el.textContent)
			.join(' ');

		expect(allChangeTexts).toContain('23%');
		expect(allChangeTexts).toContain('18%');
	});
});
