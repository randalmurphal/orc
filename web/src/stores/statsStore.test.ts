import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	useStatsStore,
	type StatsPeriod,
	type Outcomes,
	type TasksPerDay,
	type SummaryStats,
} from './statsStore';

// Mock fetch response types
interface MockDashboardStats {
	running: number;
	paused: number;
	blocked: number;
	completed: number;
	failed: number;
	today: number;
	total: number;
	tokens: number;
	cost: number;
}

interface MockCostSummary {
	period: string;
	start: string;
	end: string;
	total_cost_usd: number;
	total_input_tokens: number;
	total_output_tokens: number;
	total_tokens: number;
	entry_count: number;
}

describe('StatsStore', () => {
	beforeEach(() => {
		// Reset store before each test
		useStatsStore.getState().reset();
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
	});

	describe('initial state', () => {
		it('should have correct default values', () => {
			const state = useStatsStore.getState();

			expect(state.period).toBe('7d');
			expect(state.activityData).toBeInstanceOf(Map);
			expect(state.activityData.size).toBe(0);
			expect(state.outcomes).toEqual({ completed: 0, withRetries: 0, failed: 0 });
			expect(state.tasksPerDay).toEqual([]);
			expect(state.topInitiatives).toEqual([]);
			expect(state.topFiles).toEqual([]);
			expect(state.summaryStats).toEqual({
				tasksCompleted: 0,
				tokensUsed: 0,
				totalCost: 0,
				avgTime: 0,
				successRate: 0,
			});
			expect(state.weeklyChanges).toBeNull();
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});
	});

	describe('fetchStats', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('should set loading state during fetch', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 1,
				paused: 0,
				blocked: 2,
				completed: 10,
				failed: 2,
				today: 3,
				total: 15,
				tokens: 100000,
				cost: 5.0,
			};

			const mockCost: MockCostSummary = {
				period: 'week',
				start: '2026-01-10',
				end: '2026-01-17',
				total_cost_usd: 5.0,
				total_input_tokens: 80000,
				total_output_tokens: 20000,
				total_tokens: 100000,
				entry_count: 12,
			};

			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockCost),
				});

			const fetchPromise = useStatsStore.getState().fetchStats('7d');

			// Loading should be true immediately
			expect(useStatsStore.getState().loading).toBe(true);

			await fetchPromise;

			// Loading should be false after fetch
			expect(useStatsStore.getState().loading).toBe(false);
		});

		it('should fetch and populate stats correctly', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 1,
				paused: 0,
				blocked: 2,
				completed: 10,
				failed: 2,
				today: 3,
				total: 15,
				tokens: 100000,
				cost: 5.0,
			};

			const mockCost: MockCostSummary = {
				period: 'week',
				start: '2026-01-10',
				end: '2026-01-17',
				total_cost_usd: 8.5,
				total_input_tokens: 80000,
				total_output_tokens: 20000,
				total_tokens: 150000,
				entry_count: 12,
			};

			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockCost),
				});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			expect(state.period).toBe('7d');
			expect(state.outcomes.completed).toBe(10);
			expect(state.outcomes.failed).toBe(2);
			expect(state.summaryStats.tasksCompleted).toBe(10);
			expect(state.summaryStats.tokensUsed).toBe(150000); // From cost summary
			expect(state.summaryStats.totalCost).toBe(8.5); // From cost summary
			expect(state.summaryStats.successRate).toBeCloseTo(83.3, 0);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});

		it('should handle partial data gracefully', async () => {
			// Dashboard returns data, cost endpoint fails
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 5,
				failed: 1,
				today: 1,
				total: 6,
				tokens: 50000,
				cost: 2.5,
			};

			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: false,
					status: 404,
				});

			await useStatsStore.getState().fetchStats('24h');

			const state = useStatsStore.getState();

			// Should use dashboard data as fallback
			expect(state.summaryStats.tasksCompleted).toBe(5);
			expect(state.summaryStats.tokensUsed).toBe(50000);
			expect(state.summaryStats.totalCost).toBe(2.5);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});

		it('should handle fetch errors', async () => {
			(global.fetch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(
				new Error('Network error')
			);

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			expect(state.loading).toBe(false);
			expect(state.error).toBe('Network error');
		});

		it('should convert periods to correct API params', async () => {
			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValue({
					ok: true,
					json: () => Promise.resolve({}),
				});

			// Test each period
			const periods: { period: StatsPeriod; expected: string }[] = [
				{ period: '24h', expected: 'day' },
				{ period: '7d', expected: 'week' },
				{ period: '30d', expected: 'month' },
				{ period: 'all', expected: 'all' },
			];

			for (const { period, expected } of periods) {
				useStatsStore.getState().reset();
				await useStatsStore.getState().fetchStats(period);

				expect(global.fetch).toHaveBeenCalledWith(
					`/api/cost/summary?period=${expected}`
				);
			}
		});

		it('should use cache for subsequent requests', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 5,
				failed: 0,
				today: 1,
				total: 5,
				tokens: 10000,
				cost: 1.0,
			};

			const mockCost: MockCostSummary = {
				period: 'week',
				start: '2026-01-10',
				end: '2026-01-17',
				total_cost_usd: 1.0,
				total_input_tokens: 8000,
				total_output_tokens: 2000,
				total_tokens: 10000,
				entry_count: 5,
			};

			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockCost),
				});

			// First fetch
			await useStatsStore.getState().fetchStats('7d');

			// Clear mock to count new calls
			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			// Second fetch within cache window
			await useStatsStore.getState().fetchStats('7d');

			// Should not have made new fetch calls
			expect(global.fetch).not.toHaveBeenCalled();
		});

		it('should refetch after cache expires', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 5,
				failed: 0,
				today: 1,
				total: 5,
				tokens: 10000,
				cost: 1.0,
			};

			const mockCost: MockCostSummary = {
				period: 'week',
				start: '2026-01-10',
				end: '2026-01-17',
				total_cost_usd: 1.0,
				total_input_tokens: 8000,
				total_output_tokens: 2000,
				total_tokens: 10000,
				entry_count: 5,
			};

			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValue({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				});

			// Mock different responses for both endpoints
			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockCost),
				});

			// First fetch
			await useStatsStore.getState().fetchStats('7d');

			// Advance time past cache duration (5 minutes)
			vi.advanceTimersByTime(6 * 60 * 1000);

			// Set up new mocks for refetch
			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockCost),
				});

			// Second fetch after cache expires
			await useStatsStore.getState().fetchStats('7d');

			// Should have made new fetch calls
			expect(global.fetch).toHaveBeenCalled();
		});
	});

	describe('setPeriod', () => {
		beforeEach(() => {
			global.fetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({}),
			});
		});

		it('should trigger fetch when period changes', async () => {
			// setPeriod calls fetchStats internally, which is async
			// We need to wait for the fetch to complete
			useStatsStore.getState().setPeriod('30d');

			// Wait for the async fetch to complete
			await vi.waitFor(() => {
				expect(global.fetch).toHaveBeenCalled();
			});

			// After fetch completes, period should be updated
			await vi.waitFor(() => {
				expect(useStatsStore.getState().period).toBe('30d');
			});
		});

		it('should not fetch when setting same period', () => {
			// Set initial period
			useStatsStore.setState({ period: '7d' });

			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			useStatsStore.getState().setPeriod('7d');

			// Should not fetch since period didn't change
			expect(global.fetch).not.toHaveBeenCalled();
		});
	});

	describe('reset', () => {
		it('should reset to initial state', async () => {
			// First populate with some data
			const mockDashboard: MockDashboardStats = {
				running: 1,
				paused: 0,
				blocked: 0,
				completed: 10,
				failed: 2,
				today: 3,
				total: 12,
				tokens: 50000,
				cost: 3.0,
			};

			global.fetch = vi.fn()
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve({}),
				});

			await useStatsStore.getState().fetchStats('30d');

			// Verify state was populated
			expect(useStatsStore.getState().outcomes.completed).toBe(10);

			// Reset
			useStatsStore.getState().reset();

			// Verify initial state
			const state = useStatsStore.getState();
			expect(state.period).toBe('7d');
			expect(state.outcomes).toEqual({ completed: 0, withRetries: 0, failed: 0 });
			expect(state.summaryStats.tasksCompleted).toBe(0);
			expect(state._cache.size).toBe(0);
		});
	});

	describe('derived data', () => {
		it('should generate activity data from tasks per day', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 7,
				failed: 0,
				today: 1,
				total: 7,
				tokens: 10000,
				cost: 1.0,
			};

			global.fetch = vi.fn()
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve({}),
				});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Activity data should be a map
			expect(state.activityData).toBeInstanceOf(Map);

			// Tasks per day should have entries for 7 days
			expect(state.tasksPerDay.length).toBe(7);

			// Each task per day entry should have the expected shape
			for (const entry of state.tasksPerDay) {
				expect(entry).toHaveProperty('day');
				expect(entry).toHaveProperty('count');
				expect(typeof entry.day).toBe('string');
				expect(typeof entry.count).toBe('number');
			}
		});

		it('should calculate success rate correctly', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 8,
				failed: 2,
				today: 1,
				total: 10,
				tokens: 10000,
				cost: 1.0,
			};

			global.fetch = vi.fn()
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve({}),
				});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// 8 completed out of 10 total = 80%
			expect(state.summaryStats.successRate).toBe(80);
		});

		it('should handle zero tasks gracefully', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 0,
				failed: 0,
				today: 0,
				total: 0,
				tokens: 0,
				cost: 0,
			};

			global.fetch = vi.fn()
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve({}),
				});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			expect(state.summaryStats.successRate).toBe(0);
			expect(state.weeklyChanges).toBeNull();
		});
	});

	describe('selector hooks', () => {
		it('should export individual selectors', async () => {
			const {
				useStatsPeriod,
				useStatsLoading,
				useStatsError,
				useActivityData,
				useOutcomes,
				useTasksPerDay,
				useTopInitiatives,
				useTopFiles,
				useSummaryStats,
				useWeeklyChanges,
			} = await import('./statsStore');

			// Verify all selectors are functions
			expect(typeof useStatsPeriod).toBe('function');
			expect(typeof useStatsLoading).toBe('function');
			expect(typeof useStatsError).toBe('function');
			expect(typeof useActivityData).toBe('function');
			expect(typeof useOutcomes).toBe('function');
			expect(typeof useTasksPerDay).toBe('function');
			expect(typeof useTopInitiatives).toBe('function');
			expect(typeof useTopFiles).toBe('function');
			expect(typeof useSummaryStats).toBe('function');
			expect(typeof useWeeklyChanges).toBe('function');
		});
	});

	describe('type exports', () => {
		it('should export all required types', async () => {
			// This test verifies types compile correctly
			const outcomes: Outcomes = { completed: 1, withRetries: 0, failed: 0 };
			const tasksPerDay: TasksPerDay = { day: '2026-01-18', count: 5 };
			const summaryStats: SummaryStats = {
				tasksCompleted: 10,
				tokensUsed: 50000,
				totalCost: 5.0,
				avgTime: 120,
				successRate: 90,
			};
			const period: StatsPeriod = '7d';

			expect(outcomes.completed).toBe(1);
			expect(tasksPerDay.day).toBe('2026-01-18');
			expect(summaryStats.successRate).toBe(90);
			expect(period).toBe('7d');
		});
	});
});
