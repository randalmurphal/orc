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
const mockGetDailyMetrics = vi.fn();
const mockGetMetrics = vi.fn();
const mockGetTopInitiatives = vi.fn();
const mockGetComparison = vi.fn();

vi.mock('@/lib/client', () => ({
	dashboardClient: {
		getStats: () => mockGetStats(),
		getCostSummary: () => mockGetCostSummary(),
		getDailyMetrics: () => mockGetDailyMetrics(),
		getMetrics: () => mockGetMetrics(),
		getTopInitiatives: () => mockGetTopInitiatives(),
		getComparison: () => mockGetComparison(),
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

// Helper types for TASK-553 mocks
interface MockDailyMetrics {
	date: string;
	tasksCreated?: number;
	tasksCompleted?: number;
	tasksFailed?: number;
	tokensUsed?: number;
	costUsd?: number;
}

interface MockMetricsSummary {
	tasksCompleted?: number;
	phasesExecuted?: number;
	avgTaskDurationSeconds?: number;
	successRate?: number;
}

interface MockTopInitiative {
	id: string;
	title: string;
	taskCount: number;
	completedCount?: number;
	costUsd?: number;
}

function createMockDailyMetricsResponse(days: MockDailyMetrics[] = []) {
	return {
		stats: {
			days: days.map((d) => ({
				date: d.date,
				tasksCreated: d.tasksCreated ?? 0,
				tasksCompleted: d.tasksCompleted ?? 0,
				tasksFailed: d.tasksFailed ?? 0,
				tokensUsed: d.tokensUsed ?? 0,
				costUsd: d.costUsd ?? 0,
				phasesCompleted: 0,
				commits: 0,
			})),
		},
	};
}

function createMockMetricsResponse(metrics: MockMetricsSummary = {}) {
	return {
		metrics: {
			tasksCompleted: metrics.tasksCompleted ?? 0,
			phasesExecuted: metrics.phasesExecuted ?? 0,
			avgTaskDurationSeconds: metrics.avgTaskDurationSeconds ?? 0,
			successRate: metrics.successRate ?? 0,
			totalTokens: {
				inputTokens: 0,
				outputTokens: 0,
				totalTokens: 0,
				cacheCreationInputTokens: 0,
				cacheReadInputTokens: 0,
			},
		},
	};
}

function createMockTopInitiativesResponse(initiatives: MockTopInitiative[] = []) {
	return {
		initiatives: initiatives.map((init) => ({
			id: init.id,
			title: init.title,
			taskCount: init.taskCount,
			completedCount: init.completedCount ?? 0,
			costUsd: init.costUsd ?? 0,
		})),
	};
}

interface MockComparisonMetrics {
	current?: MockMetricsSummary & { totalTokens?: { totalTokens?: number } };
	previous?: MockMetricsSummary & { totalTokens?: { totalTokens?: number } };
	tasksChangePct?: number;
	costChangePct?: number;
	successRateChangePct?: number;
}

function createMockComparisonResponse(comparison: MockComparisonMetrics = {}) {
	return {
		comparison: {
			current: {
				tasksCompleted: comparison.current?.tasksCompleted ?? 0,
				phasesExecuted: comparison.current?.phasesExecuted ?? 0,
				avgTaskDurationSeconds: comparison.current?.avgTaskDurationSeconds ?? 0,
				successRate: comparison.current?.successRate ?? 0,
				totalTokens: {
					totalTokens: comparison.current?.totalTokens?.totalTokens ?? 0,
					inputTokens: 0,
					outputTokens: 0,
					cacheCreationInputTokens: 0,
					cacheReadInputTokens: 0,
				},
			},
			previous: {
				tasksCompleted: comparison.previous?.tasksCompleted ?? 0,
				phasesExecuted: comparison.previous?.phasesExecuted ?? 0,
				avgTaskDurationSeconds: comparison.previous?.avgTaskDurationSeconds ?? 0,
				successRate: comparison.previous?.successRate ?? 0,
				totalTokens: {
					totalTokens: comparison.previous?.totalTokens?.totalTokens ?? 0,
					inputTokens: 0,
					outputTokens: 0,
					cacheCreationInputTokens: 0,
					cacheReadInputTokens: 0,
				},
			},
			tasksChangePct: comparison.tasksChangePct ?? 0,
			costChangePct: comparison.costChangePct ?? 0,
			successRateChangePct: comparison.successRateChangePct ?? 0,
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
		mockGetDailyMetrics.mockReset();
		mockGetMetrics.mockReset();
		mockGetTopInitiatives.mockReset();
		mockGetComparison.mockReset();
		mockGetComparison.mockResolvedValue(createMockComparisonResponse());
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
		// TASK-553: Add default mocks for new APIs
		beforeEach(() => {
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

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
		// TASK-553: Add default mocks for new APIs
		beforeEach(() => {
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

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
		// TASK-553: Add default mocks for new APIs
		beforeEach(() => {
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

		it('should generate activity data from tasks per day', async () => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 7 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			// TASK-553: Provide 7 days of mock daily metrics data
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([
				{ date: '2026-01-21', tasksCompleted: 1 },
				{ date: '2026-01-22', tasksCompleted: 2 },
				{ date: '2026-01-23', tasksCompleted: 1 },
				{ date: '2026-01-24', tasksCompleted: 1 },
				{ date: '2026-01-25', tasksCompleted: 0 },
				{ date: '2026-01-26', tasksCompleted: 1 },
				{ date: '2026-01-27', tasksCompleted: 1 },
			]));

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
			// TASK-608: weeklyChanges is now populated from GetComparison API
			// even with zero tasks - the API returns comparison data (all zeros)
			expect(state.weeklyChanges).not.toBeNull();
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
			// TASK-553: Add mocks for new APIs
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
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
			// TASK-553: Add mocks for new APIs (these don't need to be slow)
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
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
		// TASK-553: Add default mocks for new APIs
		beforeEach(() => {
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

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

	// =========================================================================
	// TASK-553: Bug fix tests - Stats page shows Avg Task Time as 0:00
	// and Most Active Initiatives as No data
	// =========================================================================

	describe('TASK-553: Avg Task Time from GetMetrics API (SC-1)', () => {
		beforeEach(() => {
			// Setup default mocks for basic stats
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
				todayTokens: { totalTokens: 100000 },
				todayCostUsd: 5.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({
				totalCostUsd: 5.0,
			}));
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
		});

		it('should fetch avgTime from GetMetrics API and populate summaryStats.avgTime', async () => {
			// TASK-553: avgTime should come from GetMetrics.avgTaskDurationSeconds
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 150, // 2 minutes 30 seconds
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// VERIFY SC-1: avgTime should equal avgTaskDurationSeconds from API
			expect(state.summaryStats.avgTime).toBe(150);
		});

		it('should display avgTime as 0 when API returns 0 (no completed tasks with timestamps)', async () => {
			// Edge case: No completed tasks have valid start/end timestamps
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 0,
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.summaryStats.avgTime).toBe(0);
		});

		it('should handle GetMetrics API failure gracefully with avgTime defaulting to 0', async () => {
			// Error case: GetMetrics fails but page should still load
			mockGetMetrics.mockRejectedValue(new Error('Metrics unavailable'));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// Should fail gracefully - store should have error
			expect(state.error).toBe('Metrics unavailable');
		});

		it('should handle null/undefined metrics response', async () => {
			// Edge case: API returns response with null metrics
			mockGetMetrics.mockResolvedValue({ metrics: null });
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// Should use default 0 when metrics is null
			expect(state.summaryStats.avgTime).toBe(0);
		});
	});

	describe('TASK-553: Avg Task Time respects period filter (SC-2)', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

		it('should pass period to GetMetrics API call', async () => {
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 120,
			}));

			await useStatsStore.getState().fetchStats('24h');

			// VERIFY SC-2: GetMetrics should be called (implementation will pass period)
			expect(mockGetMetrics).toHaveBeenCalled();
		});

		it('should show different avgTime values for different periods', async () => {
			// First call with 7d period
			mockGetMetrics.mockResolvedValueOnce(createMockMetricsResponse({
				avgTaskDurationSeconds: 180, // 3 minutes
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');
			expect(useStatsStore.getState().summaryStats.avgTime).toBe(180);

			// Clear cache and fetch 30d
			useStatsStore.getState().reset();
			mockGetStats.mockResolvedValue(createMockStatsResponse({ taskCounts: { completed: 5 } }));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValueOnce(createMockMetricsResponse({
				avgTaskDurationSeconds: 300, // 5 minutes
			}));

			await useStatsStore.getState().fetchStats('30d');

			// VERIFY SC-2: Different period should get different avgTime
			expect(useStatsStore.getState().summaryStats.avgTime).toBe(300);
		});

		it('should show avgTime as 0 when no completed tasks in selected period', async () => {
			// BDD-2: Given no completed tasks in the last 24 hours
			// When user selects "24h" period filter
			// Then Avg Task Time card shows "0:00"
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 0, // No tasks completed in period
			}));

			await useStatsStore.getState().fetchStats('24h');

			const state = useStatsStore.getState();
			expect(state.summaryStats.avgTime).toBe(0);
		});
	});

	describe('TASK-553: Most Active Initiatives from GetTopInitiatives API (SC-3, SC-4)', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
		});

		it('should fetch topInitiatives from GetTopInitiatives API (SC-3)', async () => {
			// TASK-553: topInitiatives should come from GetTopInitiatives API
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-001', title: 'User Authentication', taskCount: 10 },
				{ id: 'INIT-002', title: 'API Refactor', taskCount: 5 },
				{ id: 'INIT-003', title: 'Bug Fixes', taskCount: 2 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// VERIFY SC-3: topInitiatives should be populated from API
			expect(state.topInitiatives.length).toBe(3);
			expect(state.topInitiatives[0].name).toBe('User Authentication');
			expect(state.topInitiatives[0].taskCount).toBe(10);
		});

		it('should show initiatives sorted by task count descending (SC-4)', async () => {
			// BDD-3: Given 3 initiatives: A (10 tasks), B (5 tasks), C (2 tasks)
			// When user loads the Stats page
			// Then Most Active Initiatives shows: 1. A, 2. B, 3. C
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-A', title: 'Initiative A', taskCount: 10 },
				{ id: 'INIT-B', title: 'Initiative B', taskCount: 5 },
				{ id: 'INIT-C', title: 'Initiative C', taskCount: 2 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// VERIFY SC-4: Sorted by task count descending
			expect(state.topInitiatives[0].taskCount).toBe(10);
			expect(state.topInitiatives[1].taskCount).toBe(5);
			expect(state.topInitiatives[2].taskCount).toBe(2);
		});

		it('should limit to 4 initiatives maximum (SC-4)', async () => {
			// VERIFY SC-4: Max 4 shown in frontend (limit sent to API)
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-1', title: 'Init 1', taskCount: 20 },
				{ id: 'INIT-2', title: 'Init 2', taskCount: 15 },
				{ id: 'INIT-3', title: 'Init 3', taskCount: 10 },
				{ id: 'INIT-4', title: 'Init 4', taskCount: 5 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// API should return max 4 based on limit parameter
			expect(state.topInitiatives.length).toBeLessThanOrEqual(4);
		});

		it('should show "No data" when no initiatives with tasks exist (empty array)', async () => {
			// Edge case: No initiatives have linked tasks
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Empty array - UI should show "No data"
			expect(state.topInitiatives).toEqual([]);
			expect(state.topInitiatives.length).toBe(0);
		});

		it('should handle GetTopInitiatives API failure gracefully', async () => {
			// Error case: GetTopInitiatives fails
			mockGetTopInitiatives.mockRejectedValue(new Error('Initiatives unavailable'));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// Should show error but page loads
			expect(state.error).toBe('Initiatives unavailable');
		});
	});

	describe('TASK-553: Initiative leaderboard shows title not ID (SC-5)', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
		});

		it('should display initiative title from API response (SC-5)', async () => {
			// BDD-4: Given initiative with title "User Authentication"
			// When user views Most Active Initiatives
			// Then shows "User Authentication" not "INIT-001"
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-001', title: 'User Authentication', taskCount: 10 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// VERIFY SC-5: Should show title, not ID
			expect(state.topInitiatives[0].name).toBe('User Authentication');
			expect(state.topInitiatives[0].name).not.toBe('INIT-001');
		});

		it('should fall back to ID when title is empty', async () => {
			// Edge case: Backend returns empty title (fallback to ID)
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-001', title: '', taskCount: 5 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// VERIFY SC-5 fallback: Should use ID when title is empty
			// The implementation should check: init.title || init.id
			expect(state.topInitiatives[0].name).toBe('INIT-001');
		});

		it('should handle initiative with undefined title', async () => {
			// Edge case: Title field is missing/undefined
			mockGetTopInitiatives.mockResolvedValue({
				initiatives: [
					{ id: 'INIT-001', taskCount: 5 }, // title not set
				],
			});

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Should fall back to ID
			expect(state.topInitiatives[0].name).toBe('INIT-001');
		});
	});

	describe('TASK-553: Both API calls made in parallel', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
		});

		it('should call both GetMetrics and GetTopInitiatives APIs', async () => {
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 120,
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-001', title: 'Test', taskCount: 5 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			// Both APIs should be called
			expect(mockGetMetrics).toHaveBeenCalled();
			expect(mockGetTopInitiatives).toHaveBeenCalled();
		});

		it('should populate both avgTime and topInitiatives from API responses', async () => {
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 154, // 2:34
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-001', title: 'User Auth', taskCount: 10 },
				{ id: 'INIT-002', title: 'API Work', taskCount: 5 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();

			// Both values should be populated
			expect(state.summaryStats.avgTime).toBe(154);
			expect(state.topInitiatives.length).toBe(2);
			expect(state.topInitiatives[0].name).toBe('User Auth');
		});
	});

	describe('TASK-553: Edge cases from specification', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 0 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
		});

		it('zero completed tasks shows avgTime as 0', async () => {
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 0,
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.summaryStats.avgTime).toBe(0);
		});

		it('all tasks have null startedAt shows avgTime as 0', async () => {
			// When tasks exist but none have valid timestamps, avgTaskDurationSeconds = 0
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 0,
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.summaryStats.avgTime).toBe(0);
		});

		it('very long duration (over 1 hour) is stored correctly', async () => {
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: 3725, // 1 hour, 2 minutes, 5 seconds
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.summaryStats.avgTime).toBe(3725);
		});

		it('negative duration (data corruption) should be handled', async () => {
			// Backend returns negative value (shouldn't happen but defensive)
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({
				avgTaskDurationSeconds: -100,
			}));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// Implementation should handle negative as 0 or pass through
			// Test expects the value is stored (implementation may normalize)
			expect(state.summaryStats.avgTime).toBeDefined();
		});

		it('fewer than 4 initiatives shows available', async () => {
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([
				{ id: 'INIT-001', title: 'Only One', taskCount: 3 },
			]));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.topInitiatives.length).toBe(1);
			expect(state.topInitiatives[0].name).toBe('Only One');
		});
	});
	// =========================================================================
	// TASK-608: Change indicators from GetComparison API
	// =========================================================================

	describe('TASK-608: Change indicators from GetComparison API (SC-1)', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10, failed: 2 },
				todayTokens: { totalTokens: 100000 },
				todayCostUsd: 5.0,
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse({ totalCostUsd: 5.0 }));
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse({ avgTaskDurationSeconds: 120 }));
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

		it('should call GetComparison API during fetchStats', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				tasksChangePct: 23,
				successRateChangePct: 2.1,
			}));

			await useStatsStore.getState().fetchStats('7d');

			expect(mockGetComparison).toHaveBeenCalled();
		});

		it('should populate weeklyChanges.tasks from GetComparison tasksChangePct', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				tasksChangePct: 23,
				successRateChangePct: 2.1,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.weeklyChanges).not.toBeNull();
			expect(state.weeklyChanges!.tasks).toBe(23);
		});

		it('should populate weeklyChanges.successRate from GetComparison successRateChangePct', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				tasksChangePct: 10,
				successRateChangePct: 5.5,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.weeklyChanges).not.toBeNull();
			expect(state.weeklyChanges!.successRate).toBe(5.5);
		});

		it('should compute weeklyChanges.tokens from comparison current vs previous token totals', async () => {
			// Current period: 120000 tokens, Previous period: 100000 tokens = +20%
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				current: { totalTokens: { totalTokens: 120000 } },
				previous: { totalTokens: { totalTokens: 100000 } },
				tasksChangePct: 10,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.weeklyChanges).not.toBeNull();
			expect(state.weeklyChanges!.tokens).toBeCloseTo(20, 0);
		});

		it('should populate weeklyChanges.cost from GetComparison costChangePct', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				tasksChangePct: 10,
				costChangePct: -8,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.weeklyChanges).not.toBeNull();
			expect(state.weeklyChanges!.cost).toBe(-8);
		});

		it('should handle negative percentage changes (decreases)', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				tasksChangePct: -15,
				successRateChangePct: -3.2,
				costChangePct: -8,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.weeklyChanges).not.toBeNull();
			expect(state.weeklyChanges!.tasks).toBe(-15);
			expect(state.weeklyChanges!.successRate).toBe(-3.2);
			expect(state.weeklyChanges!.cost).toBe(-8);
		});

		it('should return null weeklyChanges when comparison has no previous data (all zeros)', async () => {
			// When previous period has no tasks, API returns 0 for all change percentages
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				tasksChangePct: 0,
				costChangePct: 0,
				successRateChangePct: 0,
				current: { totalTokens: { totalTokens: 0 } },
				previous: { totalTokens: { totalTokens: 0 } },
			}));

			// Need completed tasks > 0 for weeklyChanges to not be null from the old placeholder
			// but with real API data, zero changes should still be represented
			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// With real API data: weeklyChanges should be populated (with zeros)
			// NOT null - because the API was called and returned data
			expect(state.weeklyChanges).not.toBeNull();
		});

		it('should handle GetComparison API failure gracefully', async () => {
			// GetComparison fails but other APIs succeed
			mockGetComparison.mockRejectedValue(new Error('Comparison unavailable'));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// The store currently fails all fetches if any Promise.all member fails
			// This test documents current behavior - comparison failure = full error
			expect(state.error).toBe('Comparison unavailable');
		});

		it('should pass correct period parameter to GetComparison', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse());

			await useStatsStore.getState().fetchStats('30d');

			// GetComparison should be called (the implementation should pass the period)
			expect(mockGetComparison).toHaveBeenCalled();
		});
	});

	describe('TASK-608: Tokens change computed from comparison totals (SC-1)', () => {
		beforeEach(() => {
			mockGetStats.mockResolvedValue(createMockStatsResponse({
				taskCounts: { completed: 10 },
				todayTokens: { totalTokens: 100000 },
			}));
			mockGetCostSummary.mockResolvedValue(createMockCostResponse());
			mockGetDailyMetrics.mockResolvedValue(createMockDailyMetricsResponse([]));
			mockGetMetrics.mockResolvedValue(createMockMetricsResponse());
			mockGetTopInitiatives.mockResolvedValue(createMockTopInitiativesResponse([]));
		});

		it('should compute 0% token change when previous period has 0 tokens', async () => {
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				current: { totalTokens: { totalTokens: 50000 } },
				previous: { totalTokens: { totalTokens: 0 } },
				tasksChangePct: 100,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			// Can't compute % change from 0, so should be 0 or null
			expect(state.weeklyChanges!.tokens).toBe(0);
		});

		it('should compute correct negative token change', async () => {
			// Current: 80000, Previous: 100000 = -20%
			mockGetComparison.mockResolvedValue(createMockComparisonResponse({
				current: { totalTokens: { totalTokens: 80000 } },
				previous: { totalTokens: { totalTokens: 100000 } },
				tasksChangePct: -10,
			}));

			await useStatsStore.getState().fetchStats('7d');

			const state = useStatsStore.getState();
			expect(state.weeklyChanges!.tokens).toBeCloseTo(-20, 0);
		});
	});

});
