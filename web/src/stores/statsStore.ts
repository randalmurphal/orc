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
				// Fetch both endpoints in parallel
				const [dashboardRes, costRes] = await Promise.all([
					fetch('/api/dashboard/stats'),
					fetch(`/api/cost/summary?period=${periodToQueryParam(period)}`),
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

				// Build tasksPerDay from available data
				// For now, generate mock data based on period since the API
				// doesn't return daily breakdown yet
				const tasksPerDay = generateTasksPerDay(
					dashboardData?.completed ?? 0,
					period
				);

				// Build outcomes
				const outcomes: Outcomes = {
					completed: dashboardData?.completed ?? 0,
					withRetries: 0, // Not tracked separately in current API
					failed: dashboardData?.failed ?? 0,
				};

				// Build activity data from tasks per day
				const activityData = generateActivityData(tasksPerDay);

				// Build top initiatives (placeholder - would need new API endpoint)
				const topInitiatives: TopInitiative[] = [];

				// Build top files (placeholder - would need new API endpoint)
				const topFiles: TopFile[] = [];

				// Calculate summary stats
				const totalTasks = (dashboardData?.completed ?? 0) + (dashboardData?.failed ?? 0);
				const successRate = totalTasks > 0
					? ((dashboardData?.completed ?? 0) / totalTasks) * 100
					: 0;

				const summaryStats: SummaryStats = {
					tasksCompleted: dashboardData?.completed ?? 0,
					tokensUsed: costData?.total_tokens ?? dashboardData?.tokens ?? 0,
					totalCost: costData?.total_cost_usd ?? dashboardData?.cost ?? 0,
					avgTime: 0, // Would need execution time data from API
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

// Helper to generate mock tasks per day based on total count and period
function generateTasksPerDay(
	totalCompleted: number,
	period: StatsPeriod
): TasksPerDay[] {
	const result: TasksPerDay[] = [];
	const now = new Date();
	let days: number;

	switch (period) {
		case '24h':
			days = 1;
			break;
		case '7d':
			days = 7;
			break;
		case '30d':
			days = 30;
			break;
		case 'all':
			days = 90; // Show last 90 days for 'all'
			break;
	}

	// Distribute tasks across days (simple even distribution for now)
	const avgPerDay = Math.ceil(totalCompleted / days);
	let remaining = totalCompleted;

	for (let i = days - 1; i >= 0; i--) {
		const date = new Date(now);
		date.setDate(date.getDate() - i);
		const dateStr = date.toISOString().split('T')[0];

		// Assign tasks, ensuring we don't exceed remaining
		const count = Math.min(avgPerDay, remaining);
		remaining -= count;

		result.push({
			day: dateStr,
			count,
		});
	}

	return result;
}

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
