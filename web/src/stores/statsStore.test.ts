import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
	useStatsStore,
	type StatsPeriod,
	type Outcomes,
	type TasksPerDay,
	type SummaryStats,
} from './statsStore';

// Mock the Connect client
const mockGetStats = vi.fn();
const mockGetCostSummary = vi.fn();

vi.mock('@/lib/client', () => ({
	dashboardClient: {
		getStats: () => mockGetStats(),
		getCostSummary: () => mockGetCostSummary(),
	},
}));

// Helper types matching the proto response structure
interface MockTaskCounts {
	all?: number;
	active?: number;
	completed?: number;
	failed?: number;
	running?: number;
	blocked?: number;
}

interface MockTokenUsage {
	inputTokens?: number;
	outputTokens?: number;
	totalTokens?: number;
	cacheCreationInputTokens?: number;
	cacheReadInputTokens?: number;
}

interface MockDashboardStats {
	taskCounts?: MockTaskCounts;
	runningTasks?: unknown[];
	recentCompletions?: unknown[];
	pendingDecisions?: number;
	todayTokens?: MockTokenUsage;
	todayCostUsd?: number;
}

interface MockCostSummary {
	totalCostUsd?: number;
	byPeriod?: unknown[];
	byModel?: Record<string, number>;
	byCategory?: Record<string, number>;
}

// Helper to create mock proto responses
function createMockStatsResponse(stats: MockDashboardStats = {}) {
	return {
		stats: {
			taskCounts: {
				all: 0,
				active: 0,
				completed: 0,
				failed: 0,
				running: 0,
				blocked: 0,
				...stats.taskCounts,
			},
			runningTasks: stats.runningTasks ?? [],
			recentCompletions: stats.recentCompletions ?? [],
			pendingDecisions: stats.pendingDecisions ?? 0,
			todayTokens: {
				inputTokens: 0,
				outputTokens: 0,
				totalTokens: 0,
				cacheCreationInputTokens: 0,
				cacheReadInputTokens: 0,
				...stats.todayTokens,
			},
			todayCostUsd: stats.todayCostUsd ?? 0,
		},
	};
}

function createMockCostResponse(summary: MockCostSummary = {}) {
	return {
		summary: {
			totalCostUsd: summary.totalCostUsd ?? 0,
			byPeriod: summary.byPeriod ?? [],
			byModel: summary.byModel ?? {},
			byCategory: summary.byCategory ?? {},
		},
	};
}

describe('StatsStore', () => {
	beforeEach(() => {
		// Reset store before each test
		useStatsStore.getState().reset();
		vi.useFakeTimers();
		// Reset mocks
		mockGetStats.mockReset();
		mockGetCostSummary.mockReset();
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
		it('should set loading state during fetch', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
				todayTokens: { totalTokens: 100000 },
				todayCostUsd: 5.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 5.0,
			}));

			const fetchPromise = useStatsStore.getState().fetchStats('7d');

			// Loading should be true immediately
			expect(useStatsStore.getState().loading).toBe(true);

			await fetchPromise;

			// Loading should be false after fetch
			expect(useStatsStore.getState().loading).toBe(false);
		});

		it('should fetch and populate stats correctly', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
				todayTokens: { totalTokens: 100000 },
				todayCostUsd: 5.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 8.5,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			expect(state.period).toBe('7d');
			expect(state.outcomes.completed).toBe(10);
			expect(state.outcomes.failed).toBe(2);
			expect(state.summaryStats.tasksCompleted).toBe(10);
			expect(state.summaryStats.tokensUsed).toBe(100000); // From todayTokens (Connect doesn't have period token totals)
			expect(state.summaryStats.totalCost).toBe(8.5); // From cost summary
			expect(state.summaryStats.successRate).toBeCloseTo(83.3, 0);
			expect(state.loading).toBe(false);
			expect(state.error).toBeNull();
		});

		it('should handle partial data gracefully', async () => {
			// Dashboard returns data, cost endpoint fails
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 5, failed: 1 },
				todayTokens: { totalTokens: 50000 },
				todayCostUsd: 2.5,
			}));
			mockGetCostSummary.mockRejectedValue(new Error('Not found'));

			await useStatsStore.getState().fetchStats('24h');

			const state = useStatsStore.getState();

			// Should show error since Connect calls reject unlike REST fallback
			expect(state.loading).toBe(false);
			expect(state.error).toBe('Not found');
		});

		it('should handle fetch errors', async () => {
			mockGetStats.mockRejectedValue(new Error('Network error'));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			expect(state.loading).toBe(false);
			expect(state.error).toBe('Network error');
		});

		it('should fetch with different periods', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse());
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());

			// Test each period
			const periods: StatsPeriod[] = ['24h', '7d', '30d', 'all'];

			for (const period of periods) {
				useStatsStore.getState().reset();
				mockGetStats.mockClear();
				mockGetCostSummary.mockClear();
				await useStatsStore.getState().fetchStats(period);

				// Verify Connect client was called
				expect(mockGetStats).toHaveBeenCalled();
				expect(mockGetCostSummary).toHaveBeenCalled();
			}
		});

		it('should use cache for subsequent requests', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 5 },
				todayTokens: { totalTokens: 10000 },
				todayCostUsd: 1.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 1.0,
			}));

			// First fetch
			await useStatsStore.getState().fetchStats('7d');

			// Clear mock to count new calls
			mockGetStats.mockClear();
			mockGetCostSummary.mockClear();

			// Second fetch within cache window
			await useStatsStore.getState().fetchStats('7d');

			// Should not have made new calls (used cache)
			expect(mockGetStats).not.toHaveBeenCalled();
			expect(mockGetCostSummary).not.toHaveBeenCalled();
		});

		it('should refetch after cache expires', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 5 },
				todayTokens: { totalTokens: 10000 },
				todayCostUsd: 1.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 1.0,
			}));

			// First fetch
			await useStatsStore.getState().fetchStats('7d');

			// Clear mocks
			mockGetStats.mockClear();
			mockGetCostSummary.mockClear();

			// Advance time past cache duration (5 minutes)
			vi.advanceTimersByTime(6 * 60 * 1000);

			// Second fetch after cache expires
			await useStatsStore.getState().fetchStats('7d');

			// Should have made new calls
			expect(mockGetStats).toHaveBeenCalled();
			expect(mockGetCostSummary).toHaveBeenCalled();
		});
	});

	describe('setPeriod', () => {
		// TASK-526: setPeriod now only updates period, component's useEffect triggers fetch
		it('should update period without fetching (component useEffect handles fetch)', () => {
			// setPeriod only updates period - it doesn't call fetchStats
			// The component's useEffect with [fetchStats, period] deps triggers the fetch
			useStatsStore.getState().setPeriod('30d');

			// Period should be updated immediately
			expect(useStatsStore.getState().period).toBe('30d');

			// No Connect calls from setPeriod itself (component handles this)
			expect(mockGetStats).not.toHaveBeenCalled();
			expect(mockGetCostSummary).not.toHaveBeenCalled();
		});

		it('should not update period when setting same period', () => {
			// Set initial period
			useStatsStore.setState({ period: '7d' });

			mockGetStats.mockClear();
			mockGetCostSummary.mockClear();

			useStatsStore.getState().setPeriod('7d');

			// Should not fetch since period didn't change
			expect(mockGetStats).not.toHaveBeenCalled();
			expect(mockGetCostSummary).not.toHaveBeenCalled();
			// Period should still be 7d
			expect(useStatsStore.getState().period).toBe('7d');
		});
	});

	describe('reset', () => {
		it('should reset to initial state', async () => {
			// First populate with some data
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
				todayTokens: { totalTokens: 50000 },
				todayCostUsd: 3.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());

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
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 7 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());

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
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 8, failed: 2 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// 8 completed out of 10 total = 80%
			expect(state.summaryStats.successRate).toBe(80);
		});

		it('should handle zero tasks gracefully', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 0, failed: 0 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());

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
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 5 },
				todayTokens: { totalTokens: 10000 },
				todayCostUsd: 1.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 1.0,
			}));
		});

		// TASK-526: setPeriod now only updates period, component's useEffect calls fetchStats
		// This test verifies that calling fetchStats for the same period is guarded
		it('fetchStats with same period is guarded to prevent double fetch', async () => {
			// Clear any previous calls
			mockGetStats.mockClear();
			mockGetCostSummary.mockClear();

			// Act: Call fetchStats twice for the same period simultaneously
			const fetch1 = useStatsStore.getState().fetchStats('30d');
			const fetch2 = useStatsStore.getState().fetchStats('30d'); // Should be blocked by guard

			await Promise.all([fetch1, fetch2]);

			// Assert: Only one fetchStats should have actually made calls
			// Second call should be blocked by the _fetchingPeriod guard
			expect(mockGetStats).toHaveBeenCalledTimes(1);
			expect(mockGetCostSummary).toHaveBeenCalledTimes(1);
		});

		it('rapid period changes via setPeriod only update period synchronously', () => {
			// Clear any previous calls
			mockGetStats.mockClear();
			mockGetCostSummary.mockClear();

			// Act: Rapidly change periods via setPeriod
			// Note: setPeriod no longer calls fetchStats - it only updates the period
			useStatsStore.getState().setPeriod('24h');
			useStatsStore.getState().setPeriod('30d');
			useStatsStore.getState().setPeriod('7d');
			useStatsStore.getState().setPeriod('all');

			// Assert: Final period should be 'all'
			expect(useStatsStore.getState().period).toBe('all');

			// No fetches should have been made (component's useEffect would trigger these)
			expect(mockGetStats).not.toHaveBeenCalled();
			expect(mockGetCostSummary).not.toHaveBeenCalled();
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
			// Simulate slow responses
			mockGetStats.mockImplementation(async () => {
				await new Promise((resolve) => setTimeout(resolve, 50));
				return createMockStatsResponse({
					taskCounts: { completed: 10 },
					todayTokens: { totalTokens: 50000 },
					todayCostUsd: 5.0,
				});
			});
			mockGetCostSummary.mockImplementation(async () => {
				await new Promise((resolve) => setTimeout(resolve, 50));
				return createMockCostResponse({ totalCostUsd: 5.0 });
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
			// Simulate slow responses with different data per period
			mockGetStats.mockImplementation(async () => {
				await new Promise((resolve) => setTimeout(resolve, 50));
				return createMockStatsResponse({
					taskCounts: { completed: 10 },
					todayTokens: { totalTokens: 50000 },
					todayCostUsd: 5.0,
				});
			});
			mockGetCostSummary.mockImplementation(async () => {
				await new Promise((resolve) => setTimeout(resolve, 50));
				return createMockCostResponse({ totalCostUsd: 5.0 });
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
		it('zero completed tasks shows 0 values, not empty state', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 0, failed: 0 },
				todayTokens: { totalTokens: 0 },
				todayCostUsd: 0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 0,
			}));

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
			// Simulate a delayed response
			let resolveResponse: () => void;
			const delayedPromise = new Promise<void>((resolve) => {
				resolveResponse = resolve;
			});

			mockGetStats.mockImplementation(async () => {
				await delayedPromise;
				return createMockStatsResponse({
					taskCounts: { completed: 5 },
					todayTokens: { totalTokens: 10000 },
					todayCostUsd: 1.0,
				});
			});
			mockGetCostSummary.mockImplementation(async () => {
				await delayedPromise;
				return createMockCostResponse({ totalCostUsd: 1.0 });
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
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 5 },
				todayTokens: { totalTokens: 10000 },
				todayCostUsd: 1.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 1.0,
			}));

			// First fetch
			await useStatsStore.getState().fetchStats('7d');
			expect(useStatsStore.getState()._cache.has('7d')).toBe(true);

			// Advance time past cache duration (5 minutes + buffer)
			vi.advanceTimersByTime(6 * 60 * 1000);

			// Clear previous mock calls
			mockGetStats.mockClear();
			mockGetCostSummary.mockClear();

			// Setup fresh mocks for the refetch with updated data
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10 },
				todayTokens: { totalTokens: 20000 },
				todayCostUsd: 2.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 2.0,
			}));

			// Fetch again after cache expired
			await useStatsStore.getState().fetchStats('7d');

			// Should have made new calls
			expect(mockGetStats).toHaveBeenCalled();
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
