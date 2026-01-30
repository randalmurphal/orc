import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import { create as createProto } from '@bufbuild/protobuf';
import { dashboardClient } from '@/lib/client';
import {
	GetStatsRequestSchema,
	GetCostSummaryRequestSchema,
	GetDailyMetricsRequestSchema,
	GetMetricsRequestSchema,
	GetTopInitiativesRequestSchema,
	GetComparisonRequestSchema,
} from '@/gen/orc/v1/dashboard_pb';

// Types
export type StatsPeriod = '24h' | '7d' | '30d' | 'all';

export interface Outcomes {
	completed: number;
	withRetries: number;
	failed: number;
}

export interface TasksPerDay {
	day: string;
	count: number;
}

export interface TopInitiative {
	name: string;
	taskCount: number;
}

export interface TopFile {
	path: string;
	modifyCount: number;
}

export interface SummaryStats {
	tasksCompleted: number;
	tokensUsed: number;
	totalCost: number;
	avgTime: number; // seconds
	successRate: number; // 0-100
}

export interface WeeklyChanges {
	tasks: number; // percentage change from previous period
	tokens: number;
	cost: number;
	successRate: number;
}

interface CacheEntry {
	timestamp: number;
	data: StatsData;
}

interface StatsData {
	activityData: Map<string, number>;
	outcomes: Outcomes;
	tasksPerDay: TasksPerDay[];
	topInitiatives: TopInitiative[];
	topFiles: TopFile[];
	summaryStats: SummaryStats;
	weeklyChanges: WeeklyChanges | null;
}

interface StatsState {
	// Current period
	period: StatsPeriod;

	// Data (keyed by period for caching)
	activityData: Map<string, number>;
	outcomes: Outcomes;
	tasksPerDay: TasksPerDay[];
	topInitiatives: TopInitiative[];
	topFiles: TopFile[];
	summaryStats: SummaryStats;

	// Weekly changes (derived)
	weeklyChanges: WeeklyChanges | null;

	// Loading/error state
	loading: boolean;
	error: string | null;

	// Cache (internal)
	_cache: Map<StatsPeriod, CacheEntry>;

	// Fetch guard to prevent concurrent fetches for the same period (TASK-526)
	_fetchingPeriod: StatsPeriod | null;
}

interface StatsActions {
	fetchStats: (period: StatsPeriod, projectId?: string) => Promise<void>;
	setPeriod: (period: StatsPeriod) => void;
	reset: () => void;
}

export type StatsStore = StatsState & StatsActions;

// Cache duration: 5 minutes
const CACHE_DURATION_MS = 5 * 60 * 1000;

// Initial state
// TASK-526: loading starts as true to show skeleton immediately on mount
const initialState: StatsState = {
	period: '7d',
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
	loading: true, // TASK-526: true to show skeleton immediately
	error: null,
	_cache: new Map(),
	_fetchingPeriod: null, // TASK-526: fetch guard for preventing double fetches
};

// No longer needed - using Connect RPC types directly from proto

// Helper to convert period to API query param
function periodToQueryParam(period: StatsPeriod): string {
	switch (period) {
		case '24h':
			return 'day';
		case '7d':
			return 'week';
		case '30d':
			return 'month';
		case 'all':
			return 'all';
	}
}

// Helper to generate activity data (heatmap format)
// Returns Map<'YYYY-MM-DD', count>
function generateActivityData(
	tasksPerDay: TasksPerDay[]
): Map<string, number> {
	const activityMap = new Map<string, number>();
	for (const entry of tasksPerDay) {
		activityMap.set(entry.day, entry.count);
	}
	return activityMap;
}

export const useStatsStore = create<StatsStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		fetchStats: async (period: StatsPeriod, projectId?: string) => {
			const state = get();

			// TASK-526: Prevent duplicate fetches for the same period
			if (state._fetchingPeriod === period) {
				return;
			}

			// Check cache
			const cached = state._cache.get(period);
			const now = Date.now();
			if (cached && now - cached.timestamp < CACHE_DURATION_MS) {
				// Use cached data
				set({
					period,
					activityData: cached.data.activityData,
					outcomes: cached.data.outcomes,
					tasksPerDay: cached.data.tasksPerDay,
					topInitiatives: cached.data.topInitiatives,
					topFiles: cached.data.topFiles,
					summaryStats: cached.data.summaryStats,
					weeklyChanges: cached.data.weeklyChanges,
					loading: false,
					error: null,
				});
				return;
			}

			// TASK-526: Set fetch guard before starting fetch
			set({ loading: true, error: null, _fetchingPeriod: period });

			try {
				// Determine days to fetch based on period
				let daysToFetch: number;
				switch (period) {
					case '24h':
						daysToFetch = 1;
						break;
					case '7d':
						daysToFetch = 7;
						break;
					case '30d':
						daysToFetch = 30;
						break;
					case 'all':
						daysToFetch = 365; // Fetch up to a year for 'all'
						break;
				}

				// Fetch all endpoints in parallel using Connect RPC
				const [statsResponse, costResponse, dailyMetricsResponse, metricsResponse, topInitiativesResponse, comparisonResponse] = await Promise.all([
					dashboardClient.getStats(createProto(GetStatsRequestSchema, { projectId: projectId ?? '' })),
					dashboardClient.getCostSummary(
						createProto(GetCostSummaryRequestSchema, { projectId: projectId ?? '', period: periodToQueryParam(period) })
					),
					dashboardClient.getDailyMetrics(
						createProto(GetDailyMetricsRequestSchema, { projectId: projectId ?? '', days: daysToFetch })
					),
					dashboardClient.getMetrics(
						createProto(GetMetricsRequestSchema, { projectId: projectId ?? '', period: periodToQueryParam(period) })
					),
					dashboardClient.getTopInitiatives(
						createProto(GetTopInitiativesRequestSchema, { projectId: projectId ?? '', limit: 4 })
					),
					dashboardClient.getComparison(
						createProto(GetComparisonRequestSchema, { projectId: projectId ?? '', period: periodToQueryParam(period) })
					),
				]);

				// Extract data from proto responses
				const dashboardStats = statsResponse.stats;
				const costSummary = costResponse.summary;
				const dailyMetrics = dailyMetricsResponse.stats?.days ?? [];

				// Build tasksPerDay from daily metrics (real data now!)
				const tasksPerDay: TasksPerDay[] = dailyMetrics.map((day) => ({
					day: day.date,
					count: day.tasksCompleted,
				}));

				// Calculate totals from daily metrics for the period
				let periodTokensUsed = 0;
				let periodCost = 0;
				let periodCompleted = 0;
				let periodFailed = 0;
				for (const day of dailyMetrics) {
					periodTokensUsed += day.tokensUsed;
					periodCost += day.costUsd;
					periodCompleted += day.tasksCompleted;
					periodFailed += day.tasksFailed;
				}

				// Fall back to dashboard stats for task counts if daily metrics is empty
				const completedCount = periodCompleted || (dashboardStats?.taskCounts?.completed ?? 0);
				const failedCount = periodFailed || (dashboardStats?.taskCounts?.failed ?? 0);

				// Build outcomes
				const outcomes: Outcomes = {
					completed: completedCount,
					withRetries: 0, // Not tracked separately in current API
					failed: failedCount,
				};

				// Build activity data from tasks per day
				const activityData = generateActivityData(tasksPerDay);

				// Build top initiatives from GetTopInitiatives API (TASK-553)
				const topInitiatives: TopInitiative[] = (topInitiativesResponse.initiatives ?? []).map((init) => ({
					name: init.title || init.id,
					taskCount: init.taskCount,
				}));

				// Build top files (placeholder - would need new API endpoint)
				const topFiles: TopFile[] = [];

				// Calculate summary stats
				const totalTasks = completedCount + failedCount;
				const successRate = totalTasks > 0
					? (completedCount / totalTasks) * 100
					: 0;

				// Use period-accurate token count from daily metrics
				// Fall back to cost summary or today's tokens if daily metrics unavailable
				const tokensUsed = periodTokensUsed || (dashboardStats?.todayTokens?.totalTokens ?? 0);

				// Extract avgTime from GetMetrics API (TASK-553)
				const avgTime = metricsResponse.metrics?.avgTaskDurationSeconds ?? 0;

				const summaryStats: SummaryStats = {
					tasksCompleted: completedCount,
					tokensUsed: tokensUsed,
					totalCost: costSummary?.totalCostUsd ?? periodCost ?? dashboardStats?.todayCostUsd ?? 0,
					avgTime: avgTime,
					successRate: Math.round(successRate * 10) / 10,
				};

				// Build weekly changes from GetComparison API (TASK-608)
				const comparison = comparisonResponse.comparison;
				let weeklyChanges: WeeklyChanges | null = null;
				if (comparison) {
					const prevTokens = comparison.previous?.totalTokens?.totalTokens ?? 0;
					const currTokens = comparison.current?.totalTokens?.totalTokens ?? 0;
					const tokensChangePct = prevTokens > 0
						? ((currTokens - prevTokens) / prevTokens) * 100
						: 0;

					weeklyChanges = {
						tasks: comparison.tasksChangePct ?? 0,
						tokens: tokensChangePct,
						cost: comparison.costChangePct ?? 0,
						successRate: comparison.successRateChangePct ?? 0,
					};
				}

				const statsData: StatsData = {
					activityData,
					outcomes,
					tasksPerDay,
					topInitiatives,
					topFiles,
					summaryStats,
					weeklyChanges,
				};

				// TASK-526: Get fresh state for cache update to avoid overwriting
				// concurrent fetches for different periods
				const freshState = get();
				const newCache = new Map(freshState._cache);
				newCache.set(period, {
					timestamp: now,
					data: statsData,
				});

				set({
					period,
					activityData,
					outcomes,
					tasksPerDay,
					topInitiatives,
					topFiles,
					summaryStats,
					weeklyChanges,
					loading: false,
					error: null,
					_cache: newCache,
					_fetchingPeriod: null, // TASK-526: Clear fetch guard
				});
			} catch (error) {
				const errorMessage = error instanceof Error
					? error.message
					: 'Failed to fetch stats';
				set({
					loading: false,
					error: errorMessage,
					_fetchingPeriod: null, // TASK-526: Clear fetch guard on error too
				});
			}
		},

		setPeriod: (period: StatsPeriod) => {
			const state = get();
			if (state.period !== period) {
				// TASK-526: Only update period - component's useEffect will trigger fetch
				// This prevents double fetch (setPeriod + useEffect both calling fetchStats)
				set({ period });
			}
		},

		reset: () => {
			set(initialState);
		},
	}))
);

// Selector hooks
export const useStatsPeriod = () => useStatsStore((state) => state.period);
export const useStatsLoading = () => useStatsStore((state) => state.loading);
export const useStatsError = () => useStatsStore((state) => state.error);
export const useActivityData = () => useStatsStore((state) => state.activityData);
export const useOutcomes = () => useStatsStore((state) => state.outcomes);
export const useTasksPerDay = () => useStatsStore((state) => state.tasksPerDay);
export const useTopInitiatives = () => useStatsStore((state) => state.topInitiatives);
export const useTopFiles = () => useStatsStore((state) => state.topFiles);
export const useSummaryStats = () => useStatsStore((state) => state.summaryStats);
export const useWeeklyChanges = () => useStatsStore((state) => state.weeklyChanges);
