/**
 * Cost Analytics Store
 *
 * Manages cost data state for the CostAnalyticsView including:
 * - Time period selection
 * - Cost timeseries data
 * - Model breakdown data
 * - Summary statistics
 * - Budget configuration
 * - Breakdown table data with group-by functionality
 */

import { create } from 'zustand';
import { subscribeWithSelector } from 'zustand/middleware';
import type { CostDataPoint } from '@/components/cost/CostTimeseriesChart';

// =============================================================================
// Types
// =============================================================================

export type CostPeriod = '24h' | '7d' | '30d' | 'all';
export type GroupByOption = 'model' | 'phase' | 'initiative' | 'task';

export interface CostBreakdownItem {
	name: string;
	task_count: number;
	tokens: number;
	cost: number;
	avg_per_task: number;
}

export interface CostBudget {
	monthly_limit: number | null;
	current_spend: number;
	period_start: string;
	period_end: string;
}

export interface CostSummaryStats {
	totalCost: number;
	tokensUsed: number;
	avgPerTask: number;
}

interface CacheEntry {
	timestamp: number;
	costTimeseries: CostDataPoint[];
	modelBreakdown: CostBreakdownItem[];
	summaryStats: CostSummaryStats;
	budget: CostBudget | null;
	breakdownData: Record<GroupByOption, CostBreakdownItem[]>;
}

// =============================================================================
// State
// =============================================================================

interface CostState {
	// Current settings
	period: CostPeriod;
	groupBy: GroupByOption;
	showModels: boolean;

	// Data
	costTimeseries: CostDataPoint[];
	modelBreakdown: CostBreakdownItem[];
	summaryStats: CostSummaryStats;
	budget: CostBudget | null;
	breakdownData: CostBreakdownItem[];

	// Loading/error state
	loading: boolean;
	error: string | null;

	// Internal cache
	_cache: Map<CostPeriod, CacheEntry>;
}

interface CostActions {
	fetchCostData: (period: CostPeriod) => Promise<void>;
	setPeriod: (period: CostPeriod) => void;
	setGroupBy: (groupBy: GroupByOption) => void;
	setShowModels: (show: boolean) => void;
	updateBudget: (limit: number) => Promise<void>;
	reset: () => void;
}

export type CostStore = CostState & CostActions;

// Cache duration: 2 minutes
const CACHE_DURATION_MS = 2 * 60 * 1000;

// Initial state
const initialState: CostState = {
	period: '7d',
	groupBy: 'model',
	showModels: false,
	costTimeseries: [],
	modelBreakdown: [],
	summaryStats: {
		totalCost: 0,
		tokensUsed: 0,
		avgPerTask: 0,
	},
	budget: null,
	breakdownData: [],
	loading: false,
	error: null,
	_cache: new Map(),
};

// =============================================================================
// API Helper Functions
// =============================================================================

/** Convert period to API query param */
function periodToQueryParam(period: CostPeriod): string {
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

/** Fetch cost breakdown from API */
async function fetchCostBreakdown(
	period: CostPeriod,
	groupBy: GroupByOption
): Promise<CostBreakdownItem[]> {
	try {
		const res = await fetch(
			`/api/cost/breakdown?by=${groupBy}&period=${periodToQueryParam(period)}`
		);
		if (!res.ok) return [];
		const data = await res.json();
		return data.items || [];
	} catch {
		return [];
	}
}

/** Fetch cost timeseries from API */
async function fetchCostTimeseries(period: CostPeriod): Promise<CostDataPoint[]> {
	try {
		const granularity = period === '24h' ? 'hour' : period === '7d' ? 'day' : 'day';
		const res = await fetch(
			`/api/cost/timeseries?period=${periodToQueryParam(period)}&granularity=${granularity}`
		);
		if (!res.ok) return [];
		const data = await res.json();
		return data.points || [];
	} catch {
		return [];
	}
}

/** Fetch cost budget from API */
async function fetchCostBudget(): Promise<CostBudget | null> {
	try {
		const res = await fetch('/api/cost/budget');
		if (!res.ok) return null;
		return res.json();
	} catch {
		return null;
	}
}

/** Fetch metrics summary for token/cost totals */
async function fetchMetricsSummary(
	period: CostPeriod
): Promise<{ totalCost: number; tokensUsed: number; taskCount: number }> {
	try {
		const since = period === 'all' ? '365d' : period;
		const res = await fetch(`/api/metrics/summary?since=${since}`);
		if (!res.ok) return { totalCost: 0, tokensUsed: 0, taskCount: 0 };
		const data = await res.json();
		return {
			totalCost: data.total_cost || 0,
			tokensUsed: (data.total_input || 0) + (data.total_output || 0),
			taskCount: data.task_count || 0,
		};
	} catch {
		return { totalCost: 0, tokensUsed: 0, taskCount: 0 };
	}
}

// =============================================================================
// Store
// =============================================================================

export const useCostStore = create<CostStore>()(
	subscribeWithSelector((set, get) => ({
		...initialState,

		fetchCostData: async (period: CostPeriod) => {
			const state = get();

			// Check cache
			const cached = state._cache.get(period);
			const now = Date.now();
			if (cached && now - cached.timestamp < CACHE_DURATION_MS) {
				// Use cached data
				const currentGroupBy = state.groupBy;
				set({
					period,
					costTimeseries: cached.costTimeseries,
					modelBreakdown: cached.modelBreakdown,
					summaryStats: cached.summaryStats,
					budget: cached.budget,
					breakdownData: cached.breakdownData[currentGroupBy] || cached.breakdownData.model,
					loading: false,
					error: null,
				});
				return;
			}

			set({ loading: true, error: null });

			try {
				// Fetch all data in parallel
				const [timeseries, modelBreakdown, budget, metrics, phaseBreakdown, initiativeBreakdown, taskBreakdown] =
					await Promise.all([
						fetchCostTimeseries(period),
						fetchCostBreakdown(period, 'model'),
						fetchCostBudget(),
						fetchMetricsSummary(period),
						fetchCostBreakdown(period, 'phase'),
						fetchCostBreakdown(period, 'initiative'),
						fetchCostBreakdown(period, 'task'),
					]);

				// Calculate summary stats
				const summaryStats: CostSummaryStats = {
					totalCost: metrics.totalCost,
					tokensUsed: metrics.tokensUsed,
					avgPerTask: metrics.taskCount > 0 ? metrics.totalCost / metrics.taskCount : 0,
				};

				// Build breakdown data for all group-by options
				const breakdownData: Record<GroupByOption, CostBreakdownItem[]> = {
					model: modelBreakdown,
					phase: phaseBreakdown,
					initiative: initiativeBreakdown,
					task: taskBreakdown,
				};

				// Update cache
				const newCache = new Map(state._cache);
				newCache.set(period, {
					timestamp: now,
					costTimeseries: timeseries,
					modelBreakdown,
					summaryStats,
					budget,
					breakdownData,
				});

				const currentGroupBy = state.groupBy;
				set({
					period,
					costTimeseries: timeseries,
					modelBreakdown,
					summaryStats,
					budget,
					breakdownData: breakdownData[currentGroupBy] || modelBreakdown,
					loading: false,
					error: null,
					_cache: newCache,
				});
			} catch (error) {
				const errorMessage = error instanceof Error ? error.message : 'Failed to fetch cost data';
				set({
					loading: false,
					error: errorMessage,
				});
			}
		},

		setPeriod: (period: CostPeriod) => {
			const state = get();
			if (state.period !== period) {
				set({ period });
				get().fetchCostData(period);
			}
		},

		setGroupBy: (groupBy: GroupByOption) => {
			const state = get();
			if (state.groupBy !== groupBy) {
				// Check if we have cached breakdown data for this group-by
				const cached = state._cache.get(state.period);
				if (cached && cached.breakdownData[groupBy]) {
					set({
						groupBy,
						breakdownData: cached.breakdownData[groupBy],
					});
				} else {
					set({ groupBy });
					// Refetch will be needed
					get().fetchCostData(state.period);
				}
			}
		},

		setShowModels: (show: boolean) => {
			set({ showModels: show });
		},

		updateBudget: async (limit: number) => {
			
				const res = await fetch('/api/cost/budget', {
					method: 'PUT',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ monthly_limit: limit }),
				});
				if (!res.ok) {
					const error = await res.json().catch(() => ({ error: 'Failed to update budget' }));
					throw new Error(error.error || 'Failed to update budget');
				}
				const budget = await res.json();
				set({ budget });

				// Invalidate cache
				const state = get();
				const newCache = new Map(state._cache);
				for (const [key, entry] of newCache) {
					newCache.set(key, { ...entry, budget });
				}
				set({ _cache: newCache });
		},

		reset: () => {
			set(initialState);
		},
	}))
);

// =============================================================================
// Selector Hooks
// =============================================================================

export const useCostPeriod = () => useCostStore((state) => state.period);
export const useCostLoading = () => useCostStore((state) => state.loading);
export const useCostError = () => useCostStore((state) => state.error);
export const useCostTimeseries = () => useCostStore((state) => state.costTimeseries);
export const useModelBreakdown = () => useCostStore((state) => state.modelBreakdown);
export const useCostSummary = () => useCostStore((state) => state.summaryStats);
export const useCostBudget = () => useCostStore((state) => state.budget);
export const useBreakdownData = () => useCostStore((state) => state.breakdownData);
export const useGroupBy = () => useCostStore((state) => state.groupBy);
export const useShowModels = () => useCostStore((state) => state.showModels);
