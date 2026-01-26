/**
 * useInitiativeStats hook - provides initiative statistics with real-time updates.
 *
 * Computes stats from task and initiative stores. Stats update automatically
 * when store data changes via Connect RPC event streaming.
 */

import { useMemo, useEffect, useState, useRef } from 'react';
import { useTaskStore, useInitiativeStore, useInitiatives } from '@/stores';
import { InitiativeStatus } from '@/gen/orc/v1/initiative_pb';
import { TaskStatus } from '@/gen/orc/v1/task_pb';
import { timestampToDate } from '@/lib/time';
import type { InitiativeStats } from './StatsRow';

/**
 * Hook to get initiative statistics with real-time updates.
 *
 * @returns Stats object and loading state
 *
 * @example
 * const { stats, loading } = useInitiativeStats();
 * return <StatsRow stats={stats} loading={loading} />;
 */
export function useInitiativeStats(): {
	stats: InitiativeStats;
	loading: boolean;
} {
	const tasks = useTaskStore((state) => state.tasks);
	const initiatives = useInitiatives(); // Returns array from Map
	const initiativesLoading = useInitiativeStore((state) => state.loading);
	const tasksLoading = useTaskStore((state) => state.loading);

	// Track if we've received the first load of data
	const [hasInitialData, setHasInitialData] = useState(false);

	// Mark as having initial data once stores are loaded
	useEffect(() => {
		if (!initiativesLoading && !tasksLoading) {
			setHasInitialData(true);
		}
	}, [initiativesLoading, tasksLoading]);

	// Compute stats from stores if no WebSocket stats available
	const computedStats = useMemo<InitiativeStats>(() => {
		// Count active initiatives
		const activeInitiatives = initiatives.filter(
			(i) => i.status === InitiativeStatus.ACTIVE
		).length;

		// Total tasks
		const totalTasks = tasks.length;

		// Tasks created this week
		const oneWeekAgo = new Date();
		oneWeekAgo.setDate(oneWeekAgo.getDate() - 7);
		const tasksThisWeek = tasks.filter((t) => {
			const createdAt = timestampToDate(t.createdAt);
			return createdAt && createdAt >= oneWeekAgo;
		}).length;

		// Completion rate
		const completedTasks = tasks.filter((t) => t.status === TaskStatus.COMPLETED).length;
		const completionRate = totalTasks > 0 ? (completedTasks / totalTasks) * 100 : 0;

		// Total cost (placeholder - would need token usage tracking)
		// For now, estimate based on task states' token usage if available
		const totalCost = 0; // This would be computed from actual usage data

		return {
			activeInitiatives,
			totalTasks,
			tasksThisWeek,
			completionRate,
			totalCost,
		};
	}, [tasks, initiatives]);

	const loading = !hasInitialData || (initiativesLoading && tasksLoading);

	return { stats: computedStats, loading };
}

/**
 * Hook to subscribe to stats updates with callback.
 * Useful for components that need to animate or react to changes.
 *
 * @param onChange - Callback fired when stats change
 */
export function useStatsSubscription(onChange: (stats: InitiativeStats) => void): void {
	const { stats } = useInitiativeStats();

	// Use ref to track previous stats for comparison
	const prevStatsRef = useRef(stats);

	useEffect(() => {
		// Only fire callback if stats actually changed
		const hasChanged =
			stats.activeInitiatives !== prevStatsRef.current.activeInitiatives ||
			stats.totalTasks !== prevStatsRef.current.totalTasks ||
			stats.completionRate !== prevStatsRef.current.completionRate ||
			stats.totalCost !== prevStatsRef.current.totalCost;

		if (hasChanged) {
			prevStatsRef.current = stats;
			onChange(stats);
		}
	}, [stats, onChange, prevStatsRef]);
}

/**
 * Calculate trends by comparing current stats to previous period.
 * This is a utility function that could be used when backend provides historical data.
 */
export function calculateTrends(
	current: InitiativeStats,
	previous: InitiativeStats
): NonNullable<InitiativeStats['trends']> {
	return {
		initiatives: current.activeInitiatives - previous.activeInitiatives,
		tasks: current.totalTasks - previous.totalTasks,
		completionRate: current.completionRate - previous.completionRate,
		cost: current.totalCost - previous.totalCost,
	};
}
