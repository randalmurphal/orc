/**
 * TasksBarChart component - displays tasks completed per day as a bar chart.
 * Shows 7 bars (Mon-Sun) with heights proportional to task counts.
 * Includes tooltips showing exact counts on hover.
 */

import { forwardRef, type HTMLAttributes } from 'react';
import { Tooltip } from '../ui/Tooltip';
import {
	calculateBarHeight,
	getSkeletonBarHeight,
	type DayData,
} from './barChartUtils';
import './TasksBarChart.css';

// =============================================================================
// Types
// =============================================================================

export interface TasksBarChartProps extends HTMLAttributes<HTMLDivElement> {
	data: DayData[];
	loading?: boolean;
	className?: string;
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
								style={{ height: `${getSkeletonBarHeight(i)}px` }}
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
