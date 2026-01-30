/**
 * StatsRow component - displays key initiative statistics in a horizontal row.
 * Shows Active Initiatives, Total Tasks, Completion Rate, and Total Cost.
 * Supports real-time updates via WebSocket and includes loading skeleton state.
 */

import { forwardRef, useEffect, useState, type HTMLAttributes } from 'react';
import {
	formatNumber,
	formatCost,
	formatPercentage,
	formatTrend,
} from '@/lib/format';
import './StatsRow.css';

// =============================================================================
// Types
// =============================================================================

export interface InitiativeStats {
	activeInitiatives: number;
	totalTasks: number;
	tasksThisWeek: number;
	completionRate: number; // 0-100
	totalCost: number; // In dollars
	trends?: {
		initiatives: number; // +/- change
		tasks: number;
		completionRate: number;
		cost: number;
	};
}

export interface StatsRowProps extends HTMLAttributes<HTMLDivElement> {
	stats: InitiativeStats;
	loading?: boolean;
	className?: string;
}

export interface StatCardProps {
	label: string;
	value: string | number;
	trend?: number;
	color?: 'default' | 'green' | 'amber' | 'red' | 'purple';
	loading?: boolean;
}

// =============================================================================
// Icons
// =============================================================================

function TrendUpIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.5"
			aria-hidden="true"
		>
			<polyline points="18 15 12 9 6 15" />
		</svg>
	);
}

function TrendDownIcon() {
	return (
		<svg
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.5"
			aria-hidden="true"
		>
			<polyline points="6 9 12 15 18 9" />
		</svg>
	);
}

// =============================================================================
// StatCard Component
// =============================================================================

/**
 * Individual stat card within the StatsRow.
 */
export function StatCard({
	label,
	value,
	trend,
	color = 'default',
	loading = false,
}: StatCardProps) {
	const [animatedValue, setAnimatedValue] = useState(value);
	const [isAnimating, setIsAnimating] = useState(false);

	// Animate value changes
	useEffect(() => {
		if (value !== animatedValue && !loading) {
			setIsAnimating(true);
			const timer = setTimeout(() => {
				setAnimatedValue(value);
				setIsAnimating(false);
			}, 150);
			return () => clearTimeout(timer);
		}
	}, [value, animatedValue, loading]);

	const valueClasses = [
		'stats-row-card-value',
		`stats-row-card-value-${color}`,
		isAnimating ? 'stats-row-card-value-animating' : '',
	]
		.filter(Boolean)
		.join(' ');

	const hasTrend = trend !== undefined && trend !== 0;
	const isPositiveTrend = (trend ?? 0) > 0;

	// For cost, a negative trend (down) is positive (good)
	// For everything else, up is positive
	const trendClasses = [
		'stats-row-card-trend',
		isPositiveTrend ? 'stats-row-card-trend-positive' : 'stats-row-card-trend-negative',
	].join(' ');

	if (loading) {
		return (
			<article
				className="stats-row-card stats-row-card-loading"
				aria-label={`${label} loading`}
				aria-busy="true"
			>
				<div className="stats-row-card-label-skeleton" aria-hidden="true" />
				<div className="stats-row-card-value-skeleton" aria-hidden="true" />
			</article>
		);
	}

	return (
		<article
			className="stats-row-card"
			aria-label={`${label}: ${animatedValue}`}
			role="region"
		>
			<div className="stats-row-card-label">{label}</div>
			<div className={valueClasses}>{animatedValue}</div>
			{hasTrend && (
				<div
					className={trendClasses}
					role="status"
					aria-live="polite"
					aria-label={`Trend: ${formatTrend(trend!)} ${isPositiveTrend ? 'increase' : 'decrease'}`}
				>
					{isPositiveTrend ? <TrendUpIcon /> : <TrendDownIcon />}
					<span>{formatTrend(trend!)}</span>
				</div>
			)}
		</article>
	);
}

// =============================================================================
// StatsRow Component
// =============================================================================

/**
 * StatsRow component displaying 4 key statistics for initiatives.
 *
 * @example
 * // Basic usage
 * <StatsRow
 *   stats={{
 *     activeInitiatives: 3,
 *     totalTasks: 71,
 *     tasksThisWeek: 12,
 *     completionRate: 68,
 *     totalCost: 47.82,
 *   }}
 * />
 *
 * @example
 * // With trends
 * <StatsRow
 *   stats={{
 *     activeInitiatives: 3,
 *     totalTasks: 71,
 *     tasksThisWeek: 12,
 *     completionRate: 68,
 *     totalCost: 47.82,
 *     trends: {
 *       initiatives: 1,
 *       tasks: 12,
 *       completionRate: 5,
 *       cost: -2.5,
 *     },
 *   }}
 * />
 *
 * @example
 * // Loading state
 * <StatsRow stats={defaultStats} loading />
 */
export const StatsRow = forwardRef<HTMLDivElement, StatsRowProps>(
	({ stats, loading = false, className = '', ...props }, ref) => {
		const classes = ['stats-row', className].filter(Boolean).join(' ');

		// Format values for display
		const activeInitiativesValue = formatNumber(stats.activeInitiatives);
		const totalTasksValue = formatNumber(stats.totalTasks);
		const completionRateValue = formatPercentage(stats.completionRate);
		const totalCostValue = formatCost(stats.totalCost);

		// Determine colors based on values
		const completionColor =
			stats.completionRate >= 80
				? 'green'
				: stats.completionRate >= 50
					? 'amber'
					: 'red';

		return (
			<div
				ref={ref}
				className={classes}
				role="region"
				aria-label="Initiative statistics overview"
				{...props}
			>
				<StatCard
					label="Active Initiatives"
					value={activeInitiativesValue}
					trend={stats.trends?.initiatives}
					color="purple"
					loading={loading}
				/>
				<StatCard
					label="Total Tasks"
					value={totalTasksValue}
					trend={stats.trends?.tasks}
					color="default"
					loading={loading}
				/>
				<StatCard
					label="Completion Rate"
					value={completionRateValue}
					trend={stats.trends?.completionRate}
					color={completionColor}
					loading={loading}
				/>
				<StatCard
					label="Total Cost"
					value={totalCostValue}
					trend={stats.trends?.cost}
					color="amber"
					loading={loading}
				/>
			</div>
		);
	}
);

StatsRow.displayName = 'StatsRow';

