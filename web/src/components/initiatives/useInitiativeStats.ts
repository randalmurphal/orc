/**
 * useInitiativeStats hook - provides initiative statistics with WebSocket updates.
 *
 * Computes stats from task and initiative stores, and will handle real-time
 * updates when the stats_update WebSocket event is implemented.
 */

import { useMemo, useEffect, useState } from 'react';
import { useTaskStore, useInitiativeStore, useInitiatives } from '@/stores';
import { useWebSocket } from '@/hooks';
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

	const { on, status: wsStatus } = useWebSocket();

	// Track if we've received the first load of data
	const [hasInitialData, setHasInitialData] = useState(false);

	// Listen for stats_update events from WebSocket (when backend implements it)
	const [wsStats, setWsStats] = useState<InitiativeStats | null>(null);

	useEffect(() => {
		if (wsStatus !== 'connected') return;

		// When stats_update is implemented, this will receive real-time updates
		// For now, this is a placeholder that will work once the backend sends it
		const unsub = on('all', (event) => {
			if ('event' in event && event.event === ('stats_update' as never)) {
				const statsData = event.data as InitiativeStats;
				setWsStats(statsData);
			}
		});

		return unsub;
	}, [on, wsStatus]);

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
			(i) => i.status === 'active'
		).length;

		// Total tasks
		const totalTasks = tasks.length;

		// Tasks created this week
		const oneWeekAgo = new Date();
		oneWeekAgo.setDate(oneWeekAgo.getDate() - 7);
		const tasksThisWeek = tasks.filter((t) => {
			const createdAt = new Date(t.created_at);
			return createdAt >= oneWeekAgo;
		}).length;

		// Completion rate
		const completedTasks = tasks.filter((t) => t.status === 'completed').length;
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

	// Use WebSocket stats if available, otherwise use computed stats
	const stats = wsStats ?? computedStats;
	const loading = !hasInitialData || (initiativesLoading && tasksLoading);

	return { stats, loading };
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
	const prevStatsRef = useMemo(() => ({ current: stats }), []);

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
