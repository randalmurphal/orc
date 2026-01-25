import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';

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
// TASK-532: Helper to convert period for comparison endpoint (only accepts 7d, 30d)
function periodToComparisonPeriod(period: StatsPeriod): string | null {
	switch (period) {
		case '7d':
			return '7d';
		case '30d':
			return '30d';
		case '24h':
		case 'all':
			return null; // Comparison endpoint doesn't support these periods
	}
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
	fetchStats: (period: StatsPeriod) => Promise<void>;
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

// API response types (matching backend)
interface DashboardStatsResponse {
	running: number;
	paused: number;
	blocked: number;
	completed: number;
	failed: number;
	today: number;
	total: number;
	tokens: number;
	cache_creation_input_tokens?: number;
	cache_read_input_tokens?: number;
	cost: number;
	avg_task_time_seconds?: number | null;
}

interface CostSummaryResponse {
	period: string;
	start: string;
	end: string;
	total_cost_usd: number;
	total_input_tokens: number;
	total_output_tokens: number;
	total_tokens: number;
	entry_count: number;
	by_project?: Record<string, number>;
	by_phase?: Record<string, number>;
}

// TASK-532: New API response types for stats endpoints
interface ActivityDayResponse {
	date: string;
	count: number;
	level: number;
}

interface ActivityStatsResponse {
	total_tasks: number;
	current_streak: number;
	longest_streak: number;
	busiest_day: { date: string; count: number } | null;
}

interface ActivityResponse {
	start_date: string;
	end_date: string;
	data: ActivityDayResponse[];
	stats: ActivityStatsResponse;
}

interface PerDayDataResponse {
	date: string;
	day: string;
	count: number;
}

interface PerDayResponse {
	period: string;
	data: PerDayDataResponse[];
	max: number;
	average: number;
}

interface OutcomeCountResponse {
	count: number;
	percentage: number;
}

interface OutcomesResponse {
	period: string;
	total: number;
	outcomes: {
		completed: OutcomeCountResponse;
		with_retries: OutcomeCountResponse;
		failed: OutcomeCountResponse;
	};
}

interface TopInitiativeDataResponse {
	rank: number;
	id: string;
	title: string;
	task_count: number;
	completed_count: number;
	completion_rate: number;
	total_tokens: number;
	total_cost_usd: number;
}

interface TopInitiativesResponse {
	period: string;
	initiatives: TopInitiativeDataResponse[];
}

interface TopFileDataResponse {
	rank: number;
	path: string;
	modification_count: number;
	last_modified: string;
	tasks: string[];
}

interface TopFilesResponse {
	period: string;
	files: TopFileDataResponse[];
}
// TASK-532: API response type for comparison endpoint (SC-6)
interface ComparisonPeriodStats {
	tasks: number;
	tokens: number;
	cost: number;
	success_rate: number;
}

interface ComparisonChangeStats {
	tasks: number;
	tokens: number;
	cost: number;
	success_rate: number;
}

interface ComparisonResponse {
	current: ComparisonPeriodStats;
	previous: ComparisonPeriodStats;
	changes: ComparisonChangeStats;
}

// Helper to convert period to API query param (for cost summary)
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

// TASK-532: Helper to convert period to days for per-day endpoint
function periodToDays(period: StatsPeriod): number {
	switch (period) {
		case '24h':
			return 1;
		case '7d':
			return 7;
		case '30d':
			return 30;
		case 'all':
			return 30; // Use 30 days for 'all' (endpoint max is 30)
	}
}

// TASK-532: Helper to convert period to API period param (for outcomes, top-initiatives, top-files)
function periodToApiPeriod(period: StatsPeriod): string {
	switch (period) {
		case '24h':
			return '24h';
		case '7d':
			return '7d';
		case '30d':
			return '30d';
		case 'all':
			return 'all';
	}
}

// Helper to calculate weekly changes
function calculateWeeklyChanges(
	currentStats: SummaryStats,
	_period: StatsPeriod
): WeeklyChanges | null {
	// For now, return null since we'd need historical data from the API
	// to calculate actual weekly changes. This is a placeholder for
	// when the backend provides comparison data.
	if (currentStats.tasksCompleted === 0) {
		return null;
	}

	// Placeholder: return zeros indicating no change data available
	return {
		tasks: 0,
		tokens: 0,
		cost: 0,
		successRate: 0,
	};
}

export const useStatsStore = create<StatsStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		fetchStats: async (period: StatsPeriod) => {
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
					weeklyChanges: calculateWeeklyChanges(cached.data.summaryStats, period),
					loading: false,
					error: null,
				});
				return;
			}

			// TASK-526: Set fetch guard before starting fetch
			set({ loading: true, error: null, _fetchingPeriod: period });

			try {
				// TASK-532: Fetch ALL 6 required endpoints in parallel
				const apiPeriod = periodToApiPeriod(period);
				const days = periodToDays(period);

				const [
					dashboardRes,
					costRes,
					activityRes,
					perDayRes,
					outcomesRes,
					topInitiativesRes,
					topFilesRes,
				] = await Promise.all([
					fetch('/api/dashboard/stats'),
					fetch(`/api/cost/summary?period=${periodToQueryParam(period)}`),
					fetch('/api/stats/activity'),
					fetch(`/api/stats/per-day?days=${days}`),
					fetch(`/api/stats/outcomes?period=${apiPeriod}`),
					fetch(`/api/stats/top-initiatives?period=${apiPeriod}`),
					fetch(`/api/stats/top-files?period=${apiPeriod}`),
				]);

				// Handle dashboard stats
				let dashboardData: DashboardStatsResponse | null = null;
				if (dashboardRes.ok) {
					dashboardData = await dashboardRes.json();
				}

				// Handle cost summary (may not exist yet)
				let costData: CostSummaryResponse | null = null;
				if (costRes.ok) {
					costData = await costRes.json();
				}

				// TASK-532: Handle activity endpoint (SC-1)
				let activityApiData: ActivityResponse | null = null;
				if (activityRes.ok) {
					activityApiData = await activityRes.json();
				}

				// TASK-532: Handle per-day endpoint (SC-2)
				let perDayApiData: PerDayResponse | null = null;
				if (perDayRes.ok) {
					perDayApiData = await perDayRes.json();
				}

				// TASK-532: Handle outcomes endpoint (SC-4)
				let outcomesApiData: OutcomesResponse | null = null;
				if (outcomesRes.ok) {
					outcomesApiData = await outcomesRes.json();
				}

				// TASK-532: Handle top-initiatives endpoint (SC-5)
				let topInitiativesApiData: TopInitiativesResponse | null = null;
				if (topInitiativesRes.ok) {
					topInitiativesApiData = await topInitiativesRes.json();
				}

				// TASK-532: Handle top-files endpoint (SC-6)
				let topFilesApiData: TopFilesResponse | null = null;
				if (topFilesRes.ok) {
					topFilesApiData = await topFilesRes.json();
				}

				// TASK-532: Build activityData from /api/stats/activity response (SC-1)
				const activityData: Map<string, number> = new Map();
				if (activityApiData?.data) {
					for (const day of activityApiData.data) {
						activityData.set(day.date, day.count);
					}
				}

				// TASK-532: Build tasksPerDay from /api/stats/per-day response (SC-2)
				const tasksPerDay: TasksPerDay[] = (perDayApiData?.data ?? []).map((d) => ({
					day: d.date, // Use date as the day key for consistency
					count: d.count,
				}));

				// TASK-532: Build outcomes from /api/stats/outcomes response (SC-4)
				const outcomes: Outcomes = outcomesApiData?.outcomes
					? {
							completed: outcomesApiData.outcomes.completed.count,
							withRetries: outcomesApiData.outcomes.with_retries.count,
							failed: outcomesApiData.outcomes.failed.count,
						}
					: {
							completed: dashboardData?.completed ?? 0,
							withRetries: 0,
							failed: dashboardData?.failed ?? 0,
						};

				// TASK-532: Build topInitiatives from /api/stats/top-initiatives response (SC-5)
				const topInitiatives: TopInitiative[] = (topInitiativesApiData?.initiatives ?? []).map((i) => ({
					name: i.title,
					taskCount: i.task_count,
				}));

				// TASK-532: Build topFiles from /api/stats/top-files response (SC-6)
				const topFiles: TopFile[] = (topFilesApiData?.files ?? []).map((f) => ({
					path: f.path,
					modifyCount: f.modification_count,
				}));

				// Calculate summary stats
				const totalTasks = (dashboardData?.completed ?? 0) + (dashboardData?.failed ?? 0);
				const successRate = totalTasks > 0
					? ((dashboardData?.completed ?? 0) / totalTasks) * 100
					: 0;

				// TASK-532: Use avg_task_time_seconds from dashboard stats (SC-3)
				const avgTime = dashboardData?.avg_task_time_seconds ?? 0;

				const summaryStats: SummaryStats = {
					tasksCompleted: dashboardData?.completed ?? 0,
					tokensUsed: costData?.total_tokens ?? dashboardData?.tokens ?? 0,
					totalCost: costData?.total_cost_usd ?? dashboardData?.cost ?? 0,
					avgTime: avgTime,
					successRate: Math.round(successRate * 10) / 10,
				};

				const statsData: StatsData = {
					activityData,
					outcomes,
					tasksPerDay,
					topInitiatives,
					topFiles,
					summaryStats,
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
					weeklyChanges: calculateWeeklyChanges(summaryStats, period),
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
