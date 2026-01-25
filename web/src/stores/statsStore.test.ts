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
			// TASK-526: loading starts as true to show skeleton immediately
			expect(state.loading).toBe(true);
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

		// TASK-526: setPeriod now only updates period, component's useEffect triggers fetch
		it('should update period without fetching (component useEffect handles fetch)', () => {
			// setPeriod only updates period - it doesn't call fetchStats
			// The component's useEffect with [fetchStats, period] deps triggers the fetch
			useStatsStore.getState().setPeriod('30d');

			// Period should be updated immediately
			expect(useStatsStore.getState().period).toBe('30d');

			// No fetch call from setPeriod itself (component handles this)
			expect(global.fetch).not.toHaveBeenCalled();
		});

		it('should not update period when setting same period', () => {
			// Set initial period
			useStatsStore.setState({ period: '7d' });

			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			useStatsStore.getState().setPeriod('7d');

			// Should not fetch since period didn't change
			expect(global.fetch).not.toHaveBeenCalled();
			// Period should still be 7d
			expect(useStatsStore.getState().period).toBe('7d');
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

	// =========================================================================
	// TASK-526: Bug fix tests - Infinite loading skeleton
	// =========================================================================

	describe('TASK-526: Period change triggers exactly one fetch (SC-4)', () => {
		beforeEach(() => {
			global.fetch = vi.fn().mockResolvedValue({
				ok: true,
				json: () => Promise.resolve({
					running: 0,
					paused: 0,
					blocked: 0,
					completed: 5,
					failed: 0,
					today: 1,
					total: 5,
					tokens: 10000,
					cost: 1.0,
				}),
			});
		});

		// TASK-526: setPeriod now only updates period, component's useEffect calls fetchStats
		// This test verifies that calling fetchStats for the same period is guarded
		it('fetchStats with same period is guarded to prevent double fetch', async () => {
			// Clear any previous calls
			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			// Act: Call fetchStats twice for the same period simultaneously
			const fetch1 = useStatsStore.getState().fetchStats('30d');
			const fetch2 = useStatsStore.getState().fetchStats('30d'); // Should be blocked by guard

			await Promise.all([fetch1, fetch2]);

			// Assert: Only one fetchStats should have actually made fetch calls
			// Each fetchStats makes 2 parallel calls (dashboard + cost)
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			expect(fetchCalls.length).toBe(2);
		});

		it('rapid period changes via setPeriod only update period synchronously', () => {
			// Clear any previous calls
			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			// Act: Rapidly change periods via setPeriod
			// Note: setPeriod no longer calls fetchStats - it only updates the period
			useStatsStore.getState().setPeriod('24h');
			useStatsStore.getState().setPeriod('30d');
			useStatsStore.getState().setPeriod('7d');
			useStatsStore.getState().setPeriod('all');

			// Assert: Final period should be 'all'
			expect(useStatsStore.getState().period).toBe('all');

			// No fetches should have been made (component's useEffect would trigger these)
			expect(global.fetch).not.toHaveBeenCalled();
		});
	});

	describe('TASK-526: Concurrent fetches do not corrupt cache (SC-5)', () => {
		beforeEach(() => {
			// Use real timers for concurrent fetch tests (setTimeout must work)
			vi.useRealTimers();
		});

		afterEach(() => {
			// Restore fake timers for other tests
			vi.useFakeTimers();
		});

		it('concurrent fetches for same period should not corrupt cache', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 10,
				failed: 0,
				today: 1,
				total: 10,
				tokens: 50000,
				cost: 5.0,
			};

			const mockCost: MockCostSummary = {
				period: 'week',
				start: '2026-01-10',
				end: '2026-01-17',
				total_cost_usd: 5.0,
				total_input_tokens: 40000,
				total_output_tokens: 10000,
				total_tokens: 50000,
				entry_count: 10,
			};

			// Simulate slow responses
			global.fetch = vi.fn().mockImplementation(async (url: string) => {
				await new Promise((resolve) => setTimeout(resolve, 50));
				if (url.includes('dashboard')) {
					return { ok: true, json: () => Promise.resolve(mockDashboard) };
				}
				return { ok: true, json: () => Promise.resolve(mockCost) };
			});

			// Act: Trigger two concurrent fetches for the same period
			const fetch1 = useStatsStore.getState().fetchStats('7d');
			const fetch2 = useStatsStore.getState().fetchStats('7d');

			await Promise.all([fetch1, fetch2]);

			// Assert: Cache should have correct data, not corrupted
			const state = useStatsStore.getState();
			expect(state.summaryStats.tasksCompleted).toBe(10);
			expect(state.summaryStats.tokensUsed).toBe(50000);
			expect(state._cache.has('7d')).toBe(true);
			expect(state._cache.get('7d')?.data.summaryStats.tasksCompleted).toBe(10);
		}, 10000);

		it('concurrent fetches for different periods should not overwrite each other', async () => {
			const mockDashboard: MockDashboardStats = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 10,
				failed: 0,
				today: 1,
				total: 10,
				tokens: 50000,
				cost: 5.0,
			};

			// Different cost data for different periods
			global.fetch = vi.fn().mockImplementation(async (url: string) => {
				await new Promise((resolve) => setTimeout(resolve, 50));
				if (url.includes('dashboard')) {
					return { ok: true, json: () => Promise.resolve(mockDashboard) };
				}
				// Return different data based on period in URL
				const isWeek = url.includes('period=week');
				return {
					ok: true,
					json: () =>
						Promise.resolve({
							period: isWeek ? 'week' : 'month',
							start: '2026-01-10',
							end: '2026-01-17',
							total_cost_usd: isWeek ? 5.0 : 15.0,
							total_input_tokens: 40000,
							total_output_tokens: 10000,
							total_tokens: isWeek ? 50000 : 150000,
							entry_count: 10,
						}),
				};
			});

			// Act: Trigger fetches for different periods concurrently
			const fetch7d = useStatsStore.getState().fetchStats('7d');
			const fetch30d = useStatsStore.getState().fetchStats('30d');

			await Promise.all([fetch7d, fetch30d]);

			// Assert: Both periods should be cached
			const state = useStatsStore.getState();
			expect(state._cache.has('7d')).toBe(true);
			expect(state._cache.has('30d')).toBe(true);
		}, 10000);
	});

	describe('TASK-526: Initial loading state (SC-2)', () => {
		it('initial loading state should be true to show skeleton immediately', () => {
			// Reset store to get fresh initial state
			useStatsStore.getState().reset();

			// TASK-526 FIX: Initial loading is now true to show skeleton immediately
			const state = useStatsStore.getState();
			expect(state.loading).toBe(true);
			expect(state._fetchingPeriod).toBeNull();
		});
	});

	describe('TASK-526: Edge cases from specification', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('zero completed tasks shows 0 values, not empty state', async () => {
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

			const mockCost: MockCostSummary = {
				period: 'week',
				start: '2026-01-10',
				end: '2026-01-17',
				total_cost_usd: 0,
				total_input_tokens: 0,
				total_output_tokens: 0,
				total_tokens: 0,
				entry_count: 0,
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
			expect(state.summaryStats.tasksCompleted).toBe(0);
			expect(state.summaryStats.tokensUsed).toBe(0);
			expect(state.summaryStats.totalCost).toBe(0);
			expect(state.summaryStats.successRate).toBe(0);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});

		it('loading becomes false after fetch completes', async () => {
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

			// Simulate a delayed response
			let resolveResponse: () => void;
			const delayedPromise = new Promise<void>((resolve) => {
				resolveResponse = resolve;
			});

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async () => {
				await delayedPromise;
				return {
					ok: true,
					json: () => Promise.resolve(mockDashboard),
				};
			});

			// Start fetch
			const fetchPromise = useStatsStore.getState().fetchStats('7d');

			// Loading should be true while fetching
			expect(useStatsStore.getState().loading).toBe(true);

			// Complete the fetch
			resolveResponse!();
			await fetchPromise;

			// Loading should be false after completion
			expect(useStatsStore.getState().loading).toBe(false);
		});

		it('cache expired triggers fresh fetch', async () => {
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
			expect(useStatsStore.getState()._cache.has('7d')).toBe(true);

			// Advance time past cache duration (5 minutes + buffer)
			vi.advanceTimersByTime(6 * 60 * 1000);

			// Clear previous mock calls
			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			// Setup fresh mocks for the refetch
			(global.fetch as ReturnType<typeof vi.fn>)
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve({ ...mockDashboard, completed: 10 }),
				})
				.mockResolvedValueOnce({
					ok: true,
					json: () => Promise.resolve(mockCost),
				});

			// Fetch again after cache expired
			await useStatsStore.getState().fetchStats('7d');

			// Should have made new fetch calls
			expect(global.fetch).toHaveBeenCalled();
			// Data should be updated
			expect(useStatsStore.getState().summaryStats.tasksCompleted).toBe(10);
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
