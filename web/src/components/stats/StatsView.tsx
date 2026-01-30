/**
 * StatsView container component - assembles the complete statistics
 * page with summary cards, activity heatmap, charts, and leaderboard tables.
 */

import { useEffect, useCallback, useMemo } from 'react';
import {
	useStatsStore,
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
	type StatsPeriod,
} from '@/stores/statsStore';
import { useCurrentProjectId } from '@/stores';
import { ActivityHeatmap, type ActivityData } from './ActivityHeatmap';
import { TasksBarChart } from './TasksBarChart';
import type { DayData } from './barChartUtils';
import { OutcomesDonut } from './OutcomesDonut';
import { LeaderboardTable, type LeaderboardItem } from './LeaderboardTable';
import { Button } from '@/components/ui/Button';
import { Icon } from '@/components/ui/Icon';
import { formatNumber, formatCost } from '@/lib/format';
import './StatsView.css';

// =============================================================================
// Types
// =============================================================================

export interface StatsViewProps {
	className?: string;
}

type StatCardColor = 'purple' | 'amber' | 'green' | 'blue';

interface StatCardProps {
	label: string;
	value: string;
	icon: React.ReactNode;
	iconColor: StatCardColor;
	change: number | null;
	changeLabel?: string;
}

/** Format seconds to mm:ss format (e.g., "3:24") */
function formatTime(seconds: number): string {
	const mins = Math.floor(seconds / 60);
	const secs = Math.floor(seconds % 60);
	return `${mins}:${secs.toString().padStart(2, '0')}`;
}

/** Format success rate to percentage string (e.g., "94.2%") */
function formatRate(rate: number): string {
	return `${rate.toFixed(1)}%`;
}

/** Generate CSV content from stats data */
function generateCSV(
	tasksPerDay: { day: string; count: number }[],
	summaryStats: {
		tasksCompleted: number;
		tokensUsed: number;
		totalCost: number;
		successRate: number;
	}
): string {
	const headers = ['date', 'tasks_completed', 'tokens_used', 'cost', 'success_rate'];
	const rows = tasksPerDay.map((entry) => [
		entry.day,
		entry.count,
		summaryStats.tokensUsed,
		summaryStats.totalCost.toFixed(2),
		summaryStats.successRate.toFixed(1),
	]);

	return [headers.join(','), ...rows.map((row) => row.join(','))].join('\n');
}

/** Download CSV file */
function downloadCSV(content: string, filename: string): void {
	const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.setAttribute('href', url);
	link.setAttribute('download', filename);
	link.style.visibility = 'hidden';
	document.body.appendChild(link);
	link.click();
	document.body.removeChild(link);
	URL.revokeObjectURL(url);
}

// =============================================================================
// StatCard Component
// =============================================================================

function StatCard({ label, value, icon, iconColor, change, changeLabel }: StatCardProps) {
	const isPositive = change !== null && change >= 0;
	const hasChange = change !== null && change !== 0;

	return (
		<div className="stats-view-stat-card">
			<div className="stats-view-stat-header">
				<span className="stats-view-stat-label">{label}</span>
				<div className={`stats-view-stat-icon stats-view-stat-icon--${iconColor}`}>
					{icon}
				</div>
			</div>
			<div className="stats-view-stat-value">{value}</div>
			{hasChange && (
				<div className={`stats-view-stat-change stats-view-stat-change--${isPositive ? 'up' : 'down'}`}>
					<Icon name={isPositive ? 'chevron-up' : 'chevron-down'} size={12} />
					{isPositive ? '+' : ''}{change.toFixed(0)}% {changeLabel || 'from last period'}
				</div>
			)}
			{!hasChange && (
				<div className="stats-view-stat-change stats-view-stat-change--neutral">
					No change data
				</div>
			)}
		</div>
	);
}

// =============================================================================
// Skeleton Components
// =============================================================================

function StatCardSkeleton() {
	return (
		<div className="stats-view-stat-card stats-view-stat-card--skeleton" aria-hidden="true">
			<div className="stats-view-stat-header">
				<div className="stats-view-skeleton-label" />
				<div className="stats-view-skeleton-icon" />
			</div>
			<div className="stats-view-skeleton-value" />
			<div className="stats-view-skeleton-change" />
		</div>
	);
}

function StatsViewSkeleton() {
	return (
		<>
			{/* Stats cards skeleton */}
			<div className="stats-view-stats-grid" aria-busy="true" aria-label="Loading statistics">
				<StatCardSkeleton />
				<StatCardSkeleton />
				<StatCardSkeleton />
				<StatCardSkeleton />
				<StatCardSkeleton />
			</div>

			{/* Heatmap skeleton */}
			<div className="stats-view-section-card">
				<div className="stats-view-skeleton-heatmap" />
			</div>

			{/* Charts row skeleton */}
			<div className="stats-view-charts-row">
				<div className="stats-view-section-card stats-view-chart-card">
					<div className="stats-view-skeleton-chart" />
				</div>
				<div className="stats-view-section-card stats-view-chart-card">
					<div className="stats-view-skeleton-donut" />
				</div>
			</div>

			{/* Tables row skeleton */}
			<div className="stats-view-tables-row">
				<div className="stats-view-section-card">
					<div className="stats-view-skeleton-table" />
				</div>
				<div className="stats-view-section-card">
					<div className="stats-view-skeleton-table" />
				</div>
			</div>
		</>
	);
}

// =============================================================================
// Error State
// =============================================================================

interface StatsViewErrorProps {
	error: string;
	onRetry: () => void;
}

function StatsViewError({ error, onRetry }: StatsViewErrorProps) {
	return (
		<div className="stats-view-error" role="alert">
			<div className="stats-view-error-icon">
				<Icon name="alert-circle" size={24} />
			</div>
			<h2 className="stats-view-error-title">Failed to load statistics</h2>
			<p className="stats-view-error-desc">{error}</p>
			<Button variant="secondary" onClick={onRetry}>
				Retry
			</Button>
		</div>
	);
}

// =============================================================================
// Empty State
// =============================================================================

function StatsViewEmpty() {
	return (
		<div className="stats-view-empty" role="status">
			<div className="stats-view-empty-icon">
				<Icon name="bar-chart" size={32} />
			</div>
			<h2 className="stats-view-empty-title">No statistics yet</h2>
			<p className="stats-view-empty-desc">
				Complete some tasks to start seeing your activity metrics and trends.
			</p>
		</div>
	);
}

// =============================================================================
// Time Filter Component
// =============================================================================

const TIME_PERIODS: { value: StatsPeriod; label: string }[] = [
	{ value: '24h', label: '24h' },
	{ value: '7d', label: '7d' },
	{ value: '30d', label: '30d' },
	{ value: 'all', label: 'All' },
];

interface TimeFilterProps {
	period: StatsPeriod;
	onPeriodChange: (period: StatsPeriod) => void;
}

function TimeFilter({ period, onPeriodChange }: TimeFilterProps) {
	return (
		<div className="stats-view-time-filter" role="tablist" aria-label="Time period filter">
			{TIME_PERIODS.map(({ value, label }) => (
				<button
					key={value}
					type="button"
					role="tab"
					aria-selected={period === value}
					className={`stats-view-time-btn ${period === value ? 'stats-view-time-btn--active' : ''}`}
					onClick={() => onPeriodChange(value)}
				>
					{label}
				</button>
			))}
		</div>
	);
}

// =============================================================================
// StatsView Component
// =============================================================================

/**
 * StatsView displays comprehensive statistics including summary cards,
 * activity heatmap, charts, and leaderboard tables.
 *
 * @example
 * <StatsView />
 */
export function StatsView({ className = '' }: StatsViewProps) {
	const projectId = useCurrentProjectId();
	const fetchStats = useStatsStore((state) => state.fetchStats);
	const setPeriod = useStatsStore((state) => state.setPeriod);

	const period = useStatsPeriod();
	const loading = useStatsLoading();
	const error = useStatsError();
	const activityData = useActivityData();
	const outcomes = useOutcomes();
	const tasksPerDay = useTasksPerDay();
	const topInitiatives = useTopInitiatives();
	const topFiles = useTopFiles();
	const summaryStats = useSummaryStats();
	const weeklyChanges = useWeeklyChanges();

	// Fetch stats on mount
	useEffect(() => {
		fetchStats(period, projectId ?? undefined);
	}, [fetchStats, period, projectId]);

	// Handle period change
	const handlePeriodChange = useCallback(
		(newPeriod: StatsPeriod) => {
			setPeriod(newPeriod);
		},
		[setPeriod]
	);

	// Handle retry
	const handleRetry = useCallback(() => {
		fetchStats(period, projectId ?? undefined);
	}, [fetchStats, period, projectId]);

	// Handle export
	const handleExport = useCallback(() => {
		const csv = generateCSV(tasksPerDay, summaryStats);
		const timestamp = new Date().toISOString().split('T')[0];
		downloadCSV(csv, `orc-stats-${period}-${timestamp}.csv`);
	}, [tasksPerDay, summaryStats, period]);

	// Transform activity data for heatmap
	const heatmapData: ActivityData[] = useMemo(() => {
		const result: ActivityData[] = [];
		activityData.forEach((count, date) => {
			result.push({ date, count });
		});
		return result;
	}, [activityData]);

	// Transform tasks per day for bar chart (last 7 days)
	const barChartData: DayData[] = useMemo(() => {
		const dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
		const last7 = tasksPerDay.slice(-7);
		return last7.map((entry) => {
			const date = new Date(entry.day);
			return {
				day: dayNames[date.getDay()],
				count: entry.count,
			};
		});
	}, [tasksPerDay]);

	// Transform top initiatives for leaderboard
	const initiativeItems: LeaderboardItem[] = useMemo(() => {
		return topInitiatives.slice(0, 4).map((initiative, index) => ({
			rank: index + 1,
			name: initiative.name,
			value: `${initiative.taskCount} tasks`,
		}));
	}, [topInitiatives]);

	// Transform top files for leaderboard
	const fileItems: LeaderboardItem[] = useMemo(() => {
		return topFiles.slice(0, 4).map((file, index) => ({
			rank: index + 1,
			name: file.path,
			value: `${file.modifyCount}×`,
		}));
	}, [topFiles]);

	// Check if we have any data
	const hasData = summaryStats.tasksCompleted > 0 || summaryStats.tokensUsed > 0;

	const classes = ['stats-view', className].filter(Boolean).join(' ');

	return (
		<div className={classes}>
			<header className="stats-view-header">
				<div className="stats-view-header-text">
					<h1 className="stats-view-title">Statistics</h1>
					<p className="stats-view-subtitle">Token usage, costs, and task metrics</p>
				</div>
				<div className="stats-view-header-actions">
					<TimeFilter period={period} onPeriodChange={handlePeriodChange} />
					<Button
						variant="ghost"
						size="sm"
						leftIcon={<Icon name="download" size={14} />}
						onClick={handleExport}
						disabled={loading || !hasData}
					>
						Export
					</Button>
				</div>
			</header>

			<div className="stats-view-content">
				{loading && <StatsViewSkeleton />}

				{!loading && error && <StatsViewError error={error} onRetry={handleRetry} />}

				{!loading && !error && !hasData && <StatsViewEmpty />}

				{!loading && !error && hasData && (
					<>
						{/* Stats Cards Grid */}
						<div className="stats-view-stats-grid">
							<StatCard
								label="Tasks Completed"
								value={summaryStats.tasksCompleted.toString()}
								icon={<Icon name="check-circle" size={12} />}
								iconColor="purple"
								change={weeklyChanges?.tasks ?? null}
							/>
							<StatCard
								label="Tokens Used"
								value={formatNumber(summaryStats.tokensUsed)}
								icon={<Icon name="zap" size={12} />}
								iconColor="amber"
								change={weeklyChanges?.tokens ?? null}
							/>
							<StatCard
								label="Total Cost"
								value={formatCost(summaryStats.totalCost)}
								icon={<Icon name="dollar" size={12} />}
								iconColor="green"
								change={weeklyChanges?.cost ?? null}
							/>
							<StatCard
								label="Avg Task Time"
								value={formatTime(summaryStats.avgTime)}
								icon={<Icon name="clock" size={12} />}
								iconColor="blue"
								change={null}
								changeLabel="faster"
							/>
							<StatCard
								label="Success Rate"
								value={formatRate(summaryStats.successRate)}
								icon={<Icon name="shield" size={12} />}
								iconColor="green"
								change={weeklyChanges?.successRate ?? null}
								changeLabel="improvement"
							/>
						</div>

						{/* Activity Heatmap */}
						<div className="stats-view-section-card">
							<ActivityHeatmap
								data={heatmapData}
								title="Task Activity"
								loading={false}
							/>
						</div>

						{/* Charts Row */}
						<div className="stats-view-charts-row">
							<div className="stats-view-section-card stats-view-chart-card stats-view-chart-card--bar">
								<div className="stats-view-chart-header">
									<span className="stats-view-chart-title">Tasks Completed Per Day</span>
								</div>
								<TasksBarChart data={barChartData} loading={false} />
							</div>
							<div className="stats-view-section-card stats-view-chart-card stats-view-chart-card--donut">
								<div className="stats-view-chart-header">
									<span className="stats-view-chart-title">Task Outcomes</span>
								</div>
								<OutcomesDonut
									completed={outcomes.completed}
									withRetries={outcomes.withRetries}
									failed={outcomes.failed}
								/>
							</div>
						</div>

						{/* Tables Row */}
						<div className="stats-view-tables-row">
							<LeaderboardTable
								title="Most Active Initiatives"
								items={initiativeItems}
							/>
							<LeaderboardTable
								title="Most Modified Files"
								items={fileItems}
								isFilePath
							/>
						</div>
					</>
				)}
			</div>
		</div>
	);
}
