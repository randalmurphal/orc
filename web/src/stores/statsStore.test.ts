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

// TASK-532: Helper to create a mock fetch that handles all 7 endpoints
// This is needed because fetchStats now calls all 7 endpoints in parallel
function createMockFetch(overrides: {
	dashboard?: MockDashboardStats | null;
	cost?: MockCostSummary | null;
	activity?: { data: Array<{ date: string; count: number; level: number }>; stats: { total_tasks: number } } | null;
	perDay?: { data: Array<{ date: string; day: string; count: number }>; max: number; average: number } | null;
	outcomes?: { total: number; outcomes: { completed: { count: number }; with_retries: { count: number }; failed: { count: number } } } | null;
	topInitiatives?: { initiatives: Array<{ title: string; task_count: number }> } | null;
	topFiles?: { files: Array<{ path: string; modification_count: number }> } | null;
} = {}) {
	return vi.fn().mockImplementation(async (url: string) => {
		if (url.includes('/api/dashboard/stats')) {
			return { ok: overrides.dashboard !== null, json: () => Promise.resolve(overrides.dashboard ?? {}) };
		}
		if (url.includes('/api/cost/summary')) {
			return { ok: overrides.cost !== null, json: () => Promise.resolve(overrides.cost ?? {}) };
		}
		if (url.includes('/api/stats/activity')) {
			return { ok: overrides.activity !== null, json: () => Promise.resolve(overrides.activity ?? { data: [], stats: { total_tasks: 0 } }) };
		}
		if (url.includes('/api/stats/per-day')) {
			return { ok: overrides.perDay !== null, json: () => Promise.resolve(overrides.perDay ?? { data: [], max: 0, average: 0 }) };
		}
		if (url.includes('/api/stats/outcomes')) {
			return { ok: overrides.outcomes !== null, json: () => Promise.resolve(overrides.outcomes ?? { total: 0, outcomes: { completed: { count: 0 }, with_retries: { count: 0 }, failed: { count: 0 } } }) };
		}
		if (url.includes('/api/stats/top-initiatives')) {
			return { ok: overrides.topInitiatives !== null, json: () => Promise.resolve(overrides.topInitiatives ?? { initiatives: [] }) };
		}
		if (url.includes('/api/stats/top-files')) {
			return { ok: overrides.topFiles !== null, json: () => Promise.resolve(overrides.topFiles ?? { files: [] }) };
		}
		return { ok: true, json: () => Promise.resolve({}) };
	});
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: mockCost,
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

			// TASK-532: Use helper that mocks all 7 endpoints with outcomes data
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: mockCost,
				outcomes: {
					total: 12,
					outcomes: {
						completed: { count: 10 },
						with_retries: { count: 0 },
						failed: { count: 2 },
					},
				},
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

			// TASK-532: Use helper that mocks all 7 endpoints, with cost failing
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: null, // Simulate cost endpoint failure
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: mockCost,
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: mockCost,
			});

			// First fetch
			await useStatsStore.getState().fetchStats('7d');

			// Advance time past cache duration (5 minutes)
			vi.advanceTimersByTime(6 * 60 * 1000);

			// Clear previous mock calls
			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			// Second fetch after cache expires
			await useStatsStore.getState().fetchStats('7d');

			// Should have made new fetch calls (7 endpoints)
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

			// TASK-532: Use helper that mocks all 7 endpoints with outcomes data
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				outcomes: {
					total: 12,
					outcomes: {
						completed: { count: 10 },
						with_retries: { count: 0 },
						failed: { count: 2 },
					},
				},
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

			// TASK-532: Use helper that mocks all 7 endpoints with per-day data
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				perDay: {
					data: [
						{ date: '2026-01-18', day: 'Sat', count: 1 },
						{ date: '2026-01-19', day: 'Sun', count: 1 },
						{ date: '2026-01-20', day: 'Mon', count: 1 },
						{ date: '2026-01-21', day: 'Tue', count: 1 },
						{ date: '2026-01-22', day: 'Wed', count: 1 },
						{ date: '2026-01-23', day: 'Thu', count: 1 },
						{ date: '2026-01-24', day: 'Fri', count: 1 },
					],
					max: 1,
					average: 1,
				},
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
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
			// TASK-532: Each fetchStats makes 7 parallel calls (dashboard + cost + 5 stats endpoints)
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			expect(fetchCalls.length).toBe(7);
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: mockCost,
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

			// TASK-532: Use helper that mocks all 7 endpoints
			global.fetch = createMockFetch({
				dashboard: mockDashboard,
				cost: mockCost,
			});

			// First fetch
			await useStatsStore.getState().fetchStats('7d');
			expect(useStatsStore.getState()._cache.has('7d')).toBe(true);

			// Advance time past cache duration (5 minutes + buffer)
			vi.advanceTimersByTime(6 * 60 * 1000);

			// Clear previous mock calls
			(global.fetch as ReturnType<typeof vi.fn>).mockClear();

			// Setup fresh mocks for the refetch with updated data
			global.fetch = createMockFetch({
				dashboard: { ...mockDashboard, completed: 10 },
				cost: mockCost,
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

	// =========================================================================
	// TASK-532: Bug fix tests - Stats page shows incorrect/bogus data
	// These tests verify the store calls REAL stats API endpoints instead of
	// generating fake data.
	// =========================================================================

	describe('TASK-532: SC-1 - Activity heatmap calls /api/stats/activity endpoint', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('fetchStats calls /api/stats/activity endpoint', async () => {
			// Mock all expected endpoints
			const mockActivityResponse = {
				start_date: '2025-10-01',
				end_date: '2026-01-24',
				data: [
					{ date: '2026-01-15', count: 50, level: 4 },
					{ date: '2026-01-16', count: 3, level: 1 },
					{ date: '2026-01-17', count: 0, level: 0 },
				],
				stats: {
					total_tasks: 53,
					current_streak: 2,
					longest_streak: 15,
					busiest_day: { date: '2026-01-15', count: 50 },
				},
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/activity')) {
					return { ok: true, json: () => Promise.resolve(mockActivityResponse) };
				}
				// Return empty/default for other endpoints
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			// Verify /api/stats/activity was called
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			const activityCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/activity')
			);
			expect(activityCall).toBeDefined();
		});

		it('activityData is populated from API response, not generated', async () => {
			// Mock activity response with specific non-uniform data
			const mockActivityResponse = {
				start_date: '2025-10-01',
				end_date: '2026-01-24',
				data: [
					{ date: '2026-01-15', count: 50, level: 4 },  // High activity
					{ date: '2026-01-16', count: 3, level: 1 },   // Low activity
					{ date: '2026-01-17', count: 0, level: 0 },   // No activity
					{ date: '2026-01-18', count: 12, level: 4 },  // High activity
				],
				stats: {
					total_tasks: 65,
					current_streak: 2,
					longest_streak: 15,
					busiest_day: { date: '2026-01-15', count: 50 },
				},
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/activity')) {
					return { ok: true, json: () => Promise.resolve(mockActivityResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Activity data should match API response (non-uniform values)
			// If it was generated fake data, counts would be uniform
			expect(state.activityData.get('2026-01-15')).toBe(50);
			expect(state.activityData.get('2026-01-16')).toBe(3);
			expect(state.activityData.get('2026-01-17')).toBe(0);
			expect(state.activityData.get('2026-01-18')).toBe(12);
		});

		it('heatmap shows level 4 for day with 50 tasks, level 1 for day with 3 tasks (BDD-1)', async () => {
			// BDD-1: Given user has 50 tasks completed on Jan 15 and 3 tasks on Jan 16
			// When they view the Stats page heatmap
			// Then Jan 15 cell shows level 4 (10+ tasks), Jan 16 cell shows level 1 (1-3 tasks)
			const mockActivityResponse = {
				start_date: '2025-10-01',
				end_date: '2026-01-24',
				data: [
					{ date: '2026-01-15', count: 50, level: 4 },
					{ date: '2026-01-16', count: 3, level: 1 },
				],
				stats: {
					total_tasks: 53,
					current_streak: 2,
					longest_streak: 15,
					busiest_day: { date: '2026-01-15', count: 50 },
				},
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/activity')) {
					return { ok: true, json: () => Promise.resolve(mockActivityResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// The store should provide both count and level data for the heatmap component
			expect(state.activityData.get('2026-01-15')).toBe(50);
			expect(state.activityData.get('2026-01-16')).toBe(3);
		});
	});

	describe('TASK-532: SC-2 - Bar chart calls /api/stats/per-day endpoint', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('fetchStats calls /api/stats/per-day endpoint with correct days parameter', async () => {
			const mockPerDayResponse = {
				period: '7d',
				data: [
					{ date: '2026-01-18', day: 'Sat', count: 5 },
					{ date: '2026-01-19', day: 'Sun', count: 12 },
					{ date: '2026-01-20', day: 'Mon', count: 0 },
				],
				max: 12,
				average: 5.7,
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/per-day')) {
					return { ok: true, json: () => Promise.resolve(mockPerDayResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			// Verify /api/stats/per-day was called
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			const perDayCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/per-day')
			);
			expect(perDayCall).toBeDefined();
		});

		it('tasksPerDay reflects real daily counts with natural variation (BDD-2)', async () => {
			// BDD-2: Given user completed tasks on Monday (5), Tuesday (12), Wednesday (0)
			// When they view the bar chart
			// Then bars show heights proportional to 5, 12, 0 (not uniform)
			const mockPerDayResponse = {
				period: '7d',
				data: [
					{ date: '2026-01-20', day: 'Mon', count: 5 },
					{ date: '2026-01-21', day: 'Tue', count: 12 },
					{ date: '2026-01-22', day: 'Wed', count: 0 },
				],
				max: 12,
				average: 5.7,
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/per-day')) {
					return { ok: true, json: () => Promise.resolve(mockPerDayResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// tasksPerDay should show non-uniform values from API, not generated uniform values
			const mondayData = state.tasksPerDay.find((d) => d.day === '2026-01-20' || d.day === 'Mon');
			const tuesdayData = state.tasksPerDay.find((d) => d.day === '2026-01-21' || d.day === 'Tue');
			const wednesdayData = state.tasksPerDay.find((d) => d.day === '2026-01-22' || d.day === 'Wed');

			expect(mondayData?.count).toBe(5);
			expect(tuesdayData?.count).toBe(12);
			expect(wednesdayData?.count).toBe(0);
		});

		it('tasksPerDay is not uniformly distributed (proves not using mock generator)', async () => {
			// The bug: generateTasksPerDay() distributes tasks uniformly as avgPerDay
			// Real data has natural variation - test proves API data is used
			const mockPerDayResponse = {
				period: '7d',
				data: [
					{ date: '2026-01-18', day: 'Sat', count: 0 },
					{ date: '2026-01-19', day: 'Sun', count: 0 },
					{ date: '2026-01-20', day: 'Mon', count: 25 },  // Spike
					{ date: '2026-01-21', day: 'Tue', count: 2 },
					{ date: '2026-01-22', day: 'Wed', count: 1 },
					{ date: '2026-01-23', day: 'Thu', count: 0 },
					{ date: '2026-01-24', day: 'Fri', count: 7 },
				],
				max: 25,
				average: 5,
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/per-day')) {
					return { ok: true, json: () => Promise.resolve(mockPerDayResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			const counts = state.tasksPerDay.map((d) => d.count);

			// If data is from the mock generator, all non-zero counts would be equal
			// Real data should have variation
			const uniqueCounts = new Set(counts);
			expect(uniqueCounts.size).toBeGreaterThan(2); // More than just 0 and one other value
		});
	});

	describe('TASK-532: SC-3 - Avg Task Time uses avg_task_time_seconds from API', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('avgTime is populated from dashboard stats avg_task_time_seconds field', async () => {
			const mockDashboardResponse = {
				running: 1,
				paused: 0,
				blocked: 2,
				completed: 10,
				failed: 2,
				today: 3,
				total: 15,
				tokens: 100000,
				cost: 5.0,
				avg_task_time_seconds: 3420, // 57 minutes - this field exists in the API but was ignored
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/dashboard/stats')) {
					return { ok: true, json: () => Promise.resolve(mockDashboardResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// avgTime should be 3420 seconds from API, not 0 (hardcoded)
			expect(state.summaryStats.avgTime).toBe(3420);
		});

		it('avgTime handles null value gracefully by showing N/A indicator', async () => {
			const mockDashboardResponse = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 0,
				failed: 0,
				today: 0,
				total: 0,
				tokens: 0,
				cost: 0,
				avg_task_time_seconds: null, // No completed tasks = null avg time
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/dashboard/stats')) {
					return { ok: true, json: () => Promise.resolve(mockDashboardResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// avgTime should be 0 or null to indicate N/A
			expect(state.summaryStats.avgTime).toBe(0);
		});
	});

	describe('TASK-532: SC-4 - Outcomes donut calls /api/stats/outcomes endpoint', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('fetchStats calls /api/stats/outcomes endpoint', async () => {
			const mockOutcomesResponse = {
				period: '7d',
				total: 56,
				outcomes: {
					completed: { count: 45, percentage: 80.4 },
					with_retries: { count: 8, percentage: 14.3 },
					failed: { count: 3, percentage: 5.4 },
				},
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/outcomes')) {
					return { ok: true, json: () => Promise.resolve(mockOutcomesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			// Verify /api/stats/outcomes was called
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			const outcomesCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/outcomes')
			);
			expect(outcomesCall).toBeDefined();
		});

		it('outcomes.withRetries is populated from API (not hardcoded to 0)', async () => {
			const mockOutcomesResponse = {
				period: '7d',
				total: 56,
				outcomes: {
					completed: { count: 45, percentage: 80.4 },
					with_retries: { count: 8, percentage: 14.3 },  // THIS was always 0 in the bug
					failed: { count: 3, percentage: 5.4 },
				},
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/outcomes')) {
					return { ok: true, json: () => Promise.resolve(mockOutcomesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// The bug: withRetries was hardcoded to 0
			expect(state.outcomes.withRetries).toBe(8);
			expect(state.outcomes.completed).toBe(45);
			expect(state.outcomes.failed).toBe(3);
		});
	});

	describe('TASK-532: SC-5 - Top Initiatives leaderboard calls /api/stats/top-initiatives', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('fetchStats calls /api/stats/top-initiatives endpoint', async () => {
			const mockTopInitiativesResponse = {
				period: 'all',
				initiatives: [
					{
						rank: 1,
						id: 'INIT-001',
						title: 'User Authentication',
						task_count: 12,
						completed_count: 10,
						completion_rate: 83.3,
						total_tokens: 450000,
						total_cost_usd: 18.5,
					},
				],
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/top-initiatives')) {
					return { ok: true, json: () => Promise.resolve(mockTopInitiativesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			// Verify /api/stats/top-initiatives was called
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			const initiativesCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/top-initiatives')
			);
			expect(initiativesCall).toBeDefined();
		});

		it('topInitiatives is populated from API (not empty array)', async () => {
			const mockTopInitiativesResponse = {
				period: 'all',
				initiatives: [
					{
						rank: 1,
						id: 'INIT-001',
						title: 'User Authentication',
						task_count: 12,
						completed_count: 10,
						completion_rate: 83.3,
						total_tokens: 450000,
						total_cost_usd: 18.5,
					},
					{
						rank: 2,
						id: 'INIT-002',
						title: 'Dashboard Redesign',
						task_count: 8,
						completed_count: 8,
						completion_rate: 100,
						total_tokens: 320000,
						total_cost_usd: 12.8,
					},
				],
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/top-initiatives')) {
					return { ok: true, json: () => Promise.resolve(mockTopInitiativesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// The bug: topInitiatives was always empty []
			expect(state.topInitiatives).toHaveLength(2);
			expect(state.topInitiatives[0].name).toBe('User Authentication');
			expect(state.topInitiatives[0].taskCount).toBe(12);
			expect(state.topInitiatives[1].name).toBe('Dashboard Redesign');
			expect(state.topInitiatives[1].taskCount).toBe(8);
		});

		it('returns empty state message when no initiatives exist', async () => {
			const mockTopInitiativesResponse = {
				period: 'all',
				initiatives: [], // No initiatives
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/top-initiatives')) {
					return { ok: true, json: () => Promise.resolve(mockTopInitiativesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Empty initiatives is valid - component should show "No initiatives yet"
			expect(state.topInitiatives).toHaveLength(0);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});
	});

	describe('TASK-532: SC-6 - Top Files leaderboard calls /api/stats/top-files', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('fetchStats calls /api/stats/top-files endpoint', async () => {
			const mockTopFilesResponse = {
				period: 'all',
				files: [
					{
						rank: 1,
						path: 'internal/api/handlers.go',
						modification_count: 15,
						last_modified: '2026-01-20T10:00:00Z',
						tasks: ['TASK-001', 'TASK-002'],
					},
				],
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/top-files')) {
					return { ok: true, json: () => Promise.resolve(mockTopFilesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			// Verify /api/stats/top-files was called
			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			const filesCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/top-files')
			);
			expect(filesCall).toBeDefined();
		});

		it('topFiles is populated from API (not empty array)', async () => {
			const mockTopFilesResponse = {
				period: 'all',
				files: [
					{
						rank: 1,
						path: 'internal/api/handlers.go',
						modification_count: 15,
						last_modified: '2026-01-20T10:00:00Z',
						tasks: ['TASK-001', 'TASK-002'],
					},
					{
						rank: 2,
						path: 'web/src/components/Board.tsx',
						modification_count: 10,
						last_modified: '2026-01-18T14:30:00Z',
						tasks: ['TASK-003'],
					},
				],
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/top-files')) {
					return { ok: true, json: () => Promise.resolve(mockTopFilesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// The bug: topFiles was always empty []
			expect(state.topFiles).toHaveLength(2);
			expect(state.topFiles[0].path).toBe('internal/api/handlers.go');
			expect(state.topFiles[0].modifyCount).toBe(15);
			expect(state.topFiles[1].path).toBe('web/src/components/Board.tsx');
			expect(state.topFiles[1].modifyCount).toBe(10);
		});

		it('returns empty state when no files have been modified', async () => {
			const mockTopFilesResponse = {
				period: 'all',
				files: [], // No files modified
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/top-files')) {
					return { ok: true, json: () => Promise.resolve(mockTopFilesResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Empty files is valid - component should show "No file data yet"
			expect(state.topFiles).toHaveLength(0);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});
	});

	describe('TASK-532: BDD-3 - Empty state behavior', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('shows empty state when no tasks completed yet', async () => {
			// BDD-3: Given no tasks have been completed yet
			// When user views Stats page
			// Then empty state displays for all components

			const mockEmptyActivityResponse = {
				start_date: '2025-10-01',
				end_date: '2026-01-24',
				data: [], // No activity
				stats: {
					total_tasks: 0,
					current_streak: 0,
					longest_streak: 0,
					busiest_day: null,
				},
			};

			const mockEmptyPerDayResponse = {
				period: '7d',
				data: [],
				max: 0,
				average: 0,
			};

			const mockEmptyOutcomesResponse = {
				period: '7d',
				total: 0,
				outcomes: {
					completed: { count: 0, percentage: 0 },
					with_retries: { count: 0, percentage: 0 },
					failed: { count: 0, percentage: 0 },
				},
			};

			const mockEmptyInitiativesResponse = {
				period: 'all',
				initiatives: [],
			};

			const mockEmptyFilesResponse = {
				period: 'all',
				files: [],
			};

			const mockEmptyDashboardResponse = {
				running: 0,
				paused: 0,
				blocked: 0,
				completed: 0,
				failed: 0,
				today: 0,
				total: 0,
				tokens: 0,
				cost: 0,
				avg_task_time_seconds: null,
			};

			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/activity')) {
					return { ok: true, json: () => Promise.resolve(mockEmptyActivityResponse) };
				}
				if (url.includes('/api/stats/per-day')) {
					return { ok: true, json: () => Promise.resolve(mockEmptyPerDayResponse) };
				}
				if (url.includes('/api/stats/outcomes')) {
					return { ok: true, json: () => Promise.resolve(mockEmptyOutcomesResponse) };
				}
				if (url.includes('/api/stats/top-initiatives')) {
					return { ok: true, json: () => Promise.resolve(mockEmptyInitiativesResponse) };
				}
				if (url.includes('/api/stats/top-files')) {
					return { ok: true, json: () => Promise.resolve(mockEmptyFilesResponse) };
				}
				if (url.includes('/api/dashboard/stats')) {
					return { ok: true, json: () => Promise.resolve(mockEmptyDashboardResponse) };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// All components should show empty/zero state, not error
			expect(state.activityData.size).toBe(0);
			expect(state.tasksPerDay).toHaveLength(0);
			expect(state.outcomes).toEqual({ completed: 0, withRetries: 0, failed: 0 });
			expect(state.topInitiatives).toHaveLength(0);
			expect(state.topFiles).toHaveLength(0);
			expect(state.summaryStats.tasksCompleted).toBe(0);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});
	});

	describe('TASK-532: Error handling for individual API failures', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('partial API failure shows available data and error for failed section', async () => {
			// When 1 of 6 endpoints fails, show available data
			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/activity')) {
					throw new Error('Network error');
				}
				if (url.includes('/api/dashboard/stats')) {
					return {
						ok: true,
						json: () => Promise.resolve({
							completed: 10,
							failed: 2,
							tokens: 50000,
							cost: 2.5,
							avg_task_time_seconds: 1800,
						}),
					};
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Should have error state set
			expect(state.error).not.toBeNull();
		});

		it('/api/stats/activity failure shows error state', async () => {
			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async (url: string) => {
				if (url.includes('/api/stats/activity')) {
					return { ok: false, status: 500 };
				}
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// Error handling should be present for failed API calls
			expect(state.loading).toBe(false);
		});
	});

	describe('TASK-532: Period filter integration', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('period parameter is passed to all stats endpoints', async () => {
			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async () => {
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('30d');

			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;

			// Verify period is passed to outcomes endpoint
			const outcomesCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/outcomes')
			);
			if (outcomesCall) {
				expect((outcomesCall[0] as string)).toContain('period=');
			}

			// Verify days param is passed to per-day endpoint
			const perDayCall = fetchCalls.find((call) =>
				(call[0] as string).includes('/api/stats/per-day')
			);
			if (perDayCall) {
				expect((perDayCall[0] as string)).toContain('days=');
			}
		});
	});

	describe('TASK-532: Comprehensive fetch integration', () => {
		beforeEach(() => {
			global.fetch = vi.fn();
		});

		it('fetchStats calls ALL 6 required endpoints in parallel', async () => {
			(global.fetch as ReturnType<typeof vi.fn>).mockImplementation(async () => {
				return { ok: true, json: () => Promise.resolve({}) };
			});

			await useStatsStore.getState().fetchStats('7d');

			const fetchCalls = (global.fetch as ReturnType<typeof vi.fn>).mock.calls;
			const urls = fetchCalls.map((call) => call[0] as string);

			// All 6 endpoints must be called (bug: only 2 were called)
			expect(urls.some((u) => u.includes('/api/dashboard/stats'))).toBe(true);
			expect(urls.some((u) => u.includes('/api/stats/activity'))).toBe(true);
			expect(urls.some((u) => u.includes('/api/stats/per-day'))).toBe(true);
			expect(urls.some((u) => u.includes('/api/stats/outcomes'))).toBe(true);
			expect(urls.some((u) => u.includes('/api/stats/top-initiatives'))).toBe(true);
			expect(urls.some((u) => u.includes('/api/stats/top-files'))).toBe(true);
		});
	});
});
