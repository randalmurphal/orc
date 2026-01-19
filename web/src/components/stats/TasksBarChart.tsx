/**
 * TasksBarChart component - displays tasks completed per day as a bar chart.
 * Shows 7 bars (Mon-Sun) with heights proportional to task counts.
 * Includes tooltips showing exact counts on hover.
 */

import { forwardRef, type HTMLAttributes } from 'react';
import { Tooltip } from '../ui/Tooltip';
import './TasksBarChart.css';

// =============================================================================
// Types
// =============================================================================

export interface DayData {
	day: string;
	count: number;
}

export interface TasksBarChartProps extends HTMLAttributes<HTMLDivElement> {
	data: DayData[];
	loading?: boolean;
	className?: string;
}

// =============================================================================
// Constants
// =============================================================================

const MIN_BAR_HEIGHT = 4;
const MAX_BAR_HEIGHT = 140; // Leave room for label (160px container - 20px for label/gap)

// =============================================================================
// Utility Functions
// =============================================================================

/**
 * Calculate bar height based on count relative to max value.
 * Returns minimum height for zero values to ensure visibility.
 */
export function calculateBarHeight(count: number, maxCount: number): number {
	if (count === 0) return MIN_BAR_HEIGHT;
	const normalizedMax = Math.max(maxCount, 1); // Avoid division by zero
	const height = (count / normalizedMax) * MAX_BAR_HEIGHT;
	return Math.min(MAX_BAR_HEIGHT, Math.max(MIN_BAR_HEIGHT, height));
}

// =============================================================================
// TasksBarChart Component
// =============================================================================

/**
 * TasksBarChart displays tasks completed per day of the week.
 *
 * @example
 * // Basic usage
 * <TasksBarChart
 *   data={[
 *     { day: 'Mon', count: 12 },
 *     { day: 'Tue', count: 18 },
 *     { day: 'Wed', count: 9 },
 *     { day: 'Thu', count: 24 },
 *     { day: 'Fri', count: 16 },
 *     { day: 'Sat', count: 6 },
 *     { day: 'Sun', count: 20 },
 *   ]}
 * />
 *
 * @example
 * // Loading state
 * <TasksBarChart data={[]} loading />
 */
export const TasksBarChart = forwardRef<HTMLDivElement, TasksBarChartProps>(
	({ data, loading = false, className = '', ...props }, ref) => {
		const classes = ['tasks-bar-chart', className].filter(Boolean).join(' ');

		// Calculate max count for scaling
		const maxCount = data.length > 0 ? Math.max(...data.map((d) => d.count)) : 0;

		if (loading) {
			return (
				<div
					ref={ref}
					className={`${classes} tasks-bar-chart-loading`}
					role="img"
					aria-label="Tasks per day chart loading"
					aria-busy="true"
					{...props}
				>
					{Array.from({ length: 7 }).map((_, i) => (
						<div key={i} className="tasks-bar-chart-group">
							<div
								className="tasks-bar-chart-bar-skeleton"
								style={{ height: `${40 + Math.random() * 60}px` }}
								aria-hidden="true"
							/>
							<div className="tasks-bar-chart-label-skeleton" aria-hidden="true" />
						</div>
					))}
				</div>
			);
		}

		// Handle empty data
		if (data.length === 0) {
			return (
				<div
					ref={ref}
					className={classes}
					role="img"
					aria-label="Tasks per day chart - no data"
					{...props}
				>
					<div className="tasks-bar-chart-empty">No data available</div>
				</div>
			);
		}

		return (
			<div
				ref={ref}
				className={classes}
				role="img"
				aria-label={`Tasks per day chart showing ${data.map((d) => `${d.day}: ${d.count}`).join(', ')}`}
				{...props}
			>
				{data.map((item) => {
					const height = calculateBarHeight(item.count, maxCount);
					return (
						<div key={item.day} className="tasks-bar-chart-group">
							<Tooltip
								content={`${item.count} task${item.count !== 1 ? 's' : ''}`}
								side="top"
							>
								<div
									className="tasks-bar-chart-bar"
									style={{ height: `${height}px` }}
									role="presentation"
									aria-hidden="true"
								/>
							</Tooltip>
							<span className="tasks-bar-chart-label">{item.day}</span>
						</div>
					);
				})}
			</div>
		);
	}
);

TasksBarChart.displayName = 'TasksBarChart';

// =============================================================================
// Default Data (for demo/story purposes)
// =============================================================================

export const defaultWeekData: DayData[] = [
	{ day: 'Mon', count: 0 },
	{ day: 'Tue', count: 0 },
	{ day: 'Wed', count: 0 },
	{ day: 'Thu', count: 0 },
	{ day: 'Fri', count: 0 },
	{ day: 'Sat', count: 0 },
	{ day: 'Sun', count: 0 },
];
